// Command backfill-callgraph materialises the cross-contract call graph of
// transactions that were indexed before call-graph tracing existed.
//
// It is deliberately dependency-free and stream-oriented: it reads transaction
// hashes, asks a Soroban RPC node for each transaction's diagnostic events,
// rebuilds the call tree with the same parser the poller uses, and writes the
// resulting call_edges INSERTs to stdout. scripts/backfill-callgraph.sh wires
// that up end to end by feeding it the tx hashes the invocations table is
// missing edges for and piping the SQL into psql.
//
// Usage:
//
//	# hashes on stdin, SQL on stdout
//	psql "$DATABASE_URL" -Atc 'SELECT tx_hash FROM invocations' \
//	  | backfill-callgraph -rpc-url "$SOROBAN_RPC_URL" -network testnet \
//	  | psql "$DATABASE_URL"
//
//	# or an explicit list
//	backfill-callgraph -rpc-url "$SOROBAN_RPC_URL" -tx-hashes abc…,def…
//
// The command is safe to re-run: call_edges is keyed by (tx_hash,
// child_span_id) and every INSERT uses ON CONFLICT DO NOTHING, so a second pass
// is a no-op rather than a duplicate.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/callgraph"
)

// txHashPattern is the shape of a Soroban transaction hash.
const txHashLen = 64

type config struct {
	rpcURL  string
	network string
	hashes  []string
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
	timeout time.Duration
}

func main() {
	rpcURL := flag.String("rpc-url", os.Getenv("SOROBAN_RPC_URL"), "Soroban RPC endpoint (defaults to $SOROBAN_RPC_URL)")
	network := flag.String("network", defaultString(os.Getenv("SOROBAN_NETWORK"), "testnet"), "network to stamp on the rows: testnet | mainnet | futurenet")
	hashList := flag.String("tx-hashes", "", "comma-separated transaction hashes; when empty they are read one per line from stdin")
	timeout := flag.Duration("timeout", 30*time.Second, "per-request RPC timeout")
	flag.Parse()

	if strings.TrimSpace(*rpcURL) == "" {
		fmt.Fprintln(os.Stderr, "backfill-callgraph: -rpc-url (or $SOROBAN_RPC_URL) is required")
		os.Exit(2)
	}

	var hashes []string
	for _, h := range strings.Split(*hashList, ",") {
		if h = strings.TrimSpace(h); h != "" {
			hashes = append(hashes, h)
		}
	}

	cfg := config{
		rpcURL:  *rpcURL,
		network: *network,
		hashes:  hashes,
		stdin:   os.Stdin,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		timeout: *timeout,
	}

	if err := run(context.Background(), cfg); err != nil {
		fmt.Fprintf(os.Stderr, "backfill-callgraph: %v\n", err)
		os.Exit(1)
	}
}

func defaultString(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// run executes the backfill and returns the first unrecoverable error. A single
// transaction that cannot be fetched is reported on stderr and skipped: a
// historical backfill over a 7-day retention window must not abort because one
// hash has aged out.
func run(ctx context.Context, cfg config) error {
	hashes, err := resolveHashes(cfg)
	if err != nil {
		return err
	}
	if len(hashes) == 0 {
		fmt.Fprintln(cfg.stderr, "backfill-callgraph: no transaction hashes to process")
		return nil
	}

	client := &rpcClient{url: cfg.rpcURL, http: &http.Client{Timeout: cfg.timeout}}

	var (
		processed int
		withEdges int
		skipped   int
	)
	for i, hash := range hashes {
		hash = strings.ToLower(strings.TrimSpace(hash))
		if !isTxHash(hash) {
			fmt.Fprintf(cfg.stderr, "skipping %q: not a transaction hash\n", hash)
			skipped++
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		tx, err := client.getTransaction(ctx, hash)
		if err != nil {
			fmt.Fprintf(cfg.stderr, "skipping %s: %v\n", hash, err)
			skipped++
			continue
		}

		edges := callgraph.Parse(hash, tx.DiagnosticEventsXdr)
		if len(edges) == 0 {
			// Either the transaction genuinely has no cross-contract calls,
			// or the node does not expose diagnostic events. Neither is an
			// error; report it so an operator can tell the two apart.
			fmt.Fprintf(cfg.stderr, "%s: no call edges (ledger %d, status %s)\n", hash, tx.Ledger, tx.Status)
			processed++
			continue
		}

		sql := renderInserts(cfg.network, hash, tx, edges)
		if _, err := io.WriteString(cfg.stdout, sql); err != nil {
			return fmt.Errorf("write sql: %w", err)
		}
		processed++
		withEdges++

		if (i+1)%100 == 0 {
			fmt.Fprintf(cfg.stderr, "backfill-callgraph: %d/%d transactions processed\n", i+1, len(hashes))
		}
	}

	fmt.Fprintf(cfg.stderr, "backfill-callgraph: done: %d processed (%d with a call graph), %d skipped\n",
		processed, withEdges, skipped)
	return nil
}

// resolveHashes returns the hash list from the flag, or reads one hash per line
// from stdin when no list was given.
func resolveHashes(cfg config) ([]string, error) {
	if len(cfg.hashes) > 0 {
		return cfg.hashes, nil
	}
	scanner := bufio.NewScanner(cfg.stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var out []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// psql -At can emit blank lines; ignore them.
		if line != "" {
			out = append(out, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read tx hashes: %w", err)
	}
	return out, nil
}

func isTxHash(s string) bool {
	if len(s) != txHashLen {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// ---- SQL rendering ---------------------------------------------------------

// renderInserts renders the call_edges INSERTs for one transaction. The rows are
// emitted in call-path order so the SQL reads top-down like the tree it
// describes, and every statement is idempotent.
func renderInserts(network, txHash string, tx *transactionResult, edges []callgraph.Edge) string {
	// The root invocation's resource fee is not recoverable from
	// getTransaction without decoding resultXdr, so the backfill leaves
	// fee_share at its column default. The live indexer fills it in.
	edges = callgraph.DistributeFee(0, edges)

	var b strings.Builder
	fmt.Fprintf(&b, "-- %s (ledger %d, status %s): %d edge(s)\n", txHash, tx.Ledger, tx.Status, len(edges))
	for _, e := range edges {
		fmt.Fprintf(&b,
			"INSERT INTO call_edges (tx_hash, parent_span_id, child_span_id, callee_contract_id, function_name, cpu, mem, fee_share, depth, network, ledger, ledger_closed_at) VALUES (%s, %s, %s, %s, %s, %d, %d, %d, %d, %s, %d, %s) ON CONFLICT (tx_hash, child_span_id) DO NOTHING;\n",
			quote(txHash),
			quote(e.ParentSpanID),
			quote(e.ChildSpanID),
			nullableQuote(e.CalleeContractID),
			nullableQuote(e.FunctionName),
			e.CPU, e.Mem, e.FeeShare, e.Depth,
			quote(network),
			tx.Ledger,
			timestampExpr(tx.CreatedAt),
		)
	}
	return b.String()
}

// timestampExpr renders a timestamptz literal, or NULL when the RPC did not
// report one (FAILED and NOT_FOUND results carry no createdAt).
func timestampExpr(unixSeconds int64) string {
	if unixSeconds <= 0 {
		return "NULL"
	}
	// to_timestamp keeps the value in the database's timezone handling rather
	// than depending on the caller's locale.
	return fmt.Sprintf("to_timestamp(%d)", unixSeconds)
}

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func nullableQuote(s string) string {
	if s == "" {
		return "NULL"
	}
	return quote(s)
}

// ---- minimal JSON-RPC client ----------------------------------------------

// transactionResult is the subset of getTransaction the backfill needs.
type transactionResult struct {
	Status              string   `json:"status"`
	Ledger              uint32   `json:"ledger"`
	CreatedAt           int64    `json:"createdAt"`
	ApplicationOrder    int      `json:"applicationOrder"`
	ResultMetaXdr       string   `json:"resultMetaXdr"`
	DiagnosticEventsXdr []string `json:"diagnosticEventsXdr"`
}

// rpcClient is a minimal JSON-RPC caller. The indexer module intentionally has
// no dependence on a Stellar SDK, and this command needs exactly one method, so
// it speaks the wire protocol directly.
type rpcClient struct {
	url  string
	http *http.Client
}

func (c *rpcClient) getTransaction(ctx context.Context, hash string) (*transactionResult, error) {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getTransaction",
		"params":  map[string]string{"hash": hash},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rpc call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("rpc returned HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		Result *transactionResult `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	if envelope.Result == nil {
		return nil, errors.New("rpc response had neither result nor error")
	}
	if envelope.Result.Status == "NOT_FOUND" {
		return nil, errors.New("transaction is outside the RPC retention window")
	}
	return envelope.Result, nil
}
