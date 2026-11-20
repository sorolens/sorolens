// Command mock-rpc is the Soroban JSON-RPC stub used by the integration
// suite. It serves one real contract event for a fixed contract and ledger so
// the indexer can be exercised end to end without a live Stellar network.
//
// The fixture is deliberately strict: getEvents only returns the event when
// the request matches the contract, the event type and the ledger window the
// indexer is expected to ask for. A bug in the indexer's request (wrong
// contract, wrong range) therefore produces no rows rather than silently
// passing the test.
package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type faultConfig struct {
	TimeoutMs      int    `json:"timeout_ms"`
	StatusCode     int    `json:"status_code"`
	Truncate       bool   `json:"truncate"`
	DropConnection bool   `json:"drop_connection"`
	FailCount      int    `json:"fail_count"`
	Method         string `json:"method"`
}

var (
	faultMu      sync.Mutex
	activeFaults faultConfig
)

const (
	contractID   = "CBIELTK6YBZJU5UP2WWQEUCYKLPU6AUNZ2BQ4WWFEIE3USCIHMXQDAMA"
	latestLedger = 1_000_000
	eventLedger  = 999_999
	eventID      = "0000000000999999-0000000001"
	txHash       = "9f4c0d5b2e8a7f6c1d3b5a7e9c2f4d6b8a1c3e5f7d9b2a4c6e8f1d3b5a7c9e2f"
	// Base64 XDR ScVal values: Symbol("transfer"), I128(1000), and a
	// TransactionResult with code txSuccess and feeCharged 100. Generated with
	// stellar/go-stellar-sdk, so API consumers can decode them.
	topicXDR  = "AAAADwAAAAh0cmFuc2Zlcg=="
	valueXDR  = "AAAACgAAAAAAAAAAAAAAAAAAA+g="
	resultXDR = "AAAAAAAAAGQAAAAAAAAAAAAAAAA="
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   any             `json:"error,omitempty"`
}

type eventFilter struct {
	Type        string   `json:"type"`
	ContractIDs []string `json:"contractIds"`
}

type getEventsParams struct {
	StartLedger uint32        `json:"startLedger"`
	EndLedger   uint32        `json:"endLedger"`
	Filters     []eventFilter `json:"filters"`
	Pagination  *struct {
		Cursor string `json:"cursor"`
	} `json:"pagination"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/fault/configure", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var cfg faultConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		faultMu.Lock()
		activeFaults = cfg
		faultMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "configured", "fault": cfg})
	})
	mux.HandleFunc("/fault/reset", func(w http.ResponseWriter, r *http.Request) {
		faultMu.Lock()
		activeFaults = faultConfig{}
		faultMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
	})
	mux.HandleFunc("/fault/status", func(w http.ResponseWriter, r *http.Request) {
		faultMu.Lock()
		cfg := activeFaults
		faultMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cfg)
	})
	mux.HandleFunc("/", handleRPC)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("mock RPC listening on :%s (contract %s, ledger %d)", port, contractID, eventLedger)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func handleRPC(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON-RPC body", http.StatusBadRequest)
		return
	}

	faultMu.Lock()
	cfg := activeFaults
	faultActive := false
	if cfg.Method == "" || cfg.Method == req.Method {
		if cfg.TimeoutMs > 0 || cfg.StatusCode > 0 || cfg.Truncate || cfg.DropConnection {
			faultActive = true
			if cfg.FailCount > 0 {
				activeFaults.FailCount--
				if activeFaults.FailCount == 0 {
					// auto-clear after exhausted fail count
					activeFaults = faultConfig{}
				}
			}
		}
	}
	faultMu.Unlock()

	if faultActive {
		if cfg.TimeoutMs > 0 {
			time.Sleep(time.Duration(cfg.TimeoutMs) * time.Millisecond)
		}
		if cfg.DropConnection {
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, _ := hj.Hijack()
				if conn != nil {
					_ = conn.Close()
					return
				}
			}
			http.Error(w, "connection dropped", http.StatusInternalServerError)
			return
		}
		if cfg.StatusCode > 0 && cfg.StatusCode != http.StatusOK {
			http.Error(w, "injected fault", cfg.StatusCode)
			return
		}
		if cfg.Truncate {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"events":[`))
			return
		}
	}

	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "getLatestLedger":
		resp.Result = map[string]any{
			"id":              "00000000000f4240",
			"sequence":        latestLedger,
			"protocolVersion": 23,
		}
	case "getEvents":
		resp.Result = getEvents(req.Params)
	case "getTransaction":
		resp.Result = getTransaction(req.Params)
	case "getLedgerEntries":
		resp.Result = map[string]any{"entries": []any{}, "latestLedger": latestLedger}
	case "getTransactions":
		resp.Result = map[string]any{"transactions": []any{}, "latestLedger": latestLedger, "cursor": ""}
	case "getNetwork":
		resp.Result = map[string]any{
			"passphrase":      "Test SDF Network ; September 2015",
			"protocolVersion": 23,
		}
	default:
		// Contracts, health and simulation probes are not part of this flow.
		resp.Result = map[string]any{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func getEvents(raw json.RawMessage) any {
	empty := map[string]any{"events": []any{}, "latestLedger": latestLedger, "cursor": ""}

	var params getEventsParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return empty
	}
	// The fixture fits in a single page; a cursor request means the indexer is
	// asking for a page that does not exist.
	if params.Pagination != nil && params.Pagination.Cursor != "" {
		return empty
	}
	if params.StartLedger > 0 && eventLedger < params.StartLedger {
		return empty
	}
	if params.EndLedger > 0 && eventLedger > params.EndLedger {
		return empty
	}

	matched := false
	for _, f := range params.Filters {
		if f.Type != "" && f.Type != "contract" {
			continue
		}
		for _, id := range f.ContractIDs {
			if id == contractID {
				matched = true
			}
		}
	}
	if !matched {
		return empty
	}

	return map[string]any{
		"events": []any{map[string]any{
			"type":                     "contract",
			"ledger":                   eventLedger,
			"ledgerClosedAt":           time.Now().UTC().Format(time.RFC3339),
			"contractId":               contractID,
			"id":                       eventID,
			"inSuccessfulContractCall": true,
			"txHash":                   txHash,
			"topic":                    []string{topicXDR},
			"value":                    valueXDR,
			"transactionIndex":         0,
		}},
		"latestLedger": latestLedger,
		"cursor":       "",
	}
}

func getTransaction(raw json.RawMessage) any {
	var params struct {
		Hash string `json:"hash"`
	}
	_ = json.Unmarshal(raw, &params)
	if params.Hash != txHash {
		return map[string]any{"status": "NOT_FOUND"}
	}
	return map[string]any{
		"status":           "SUCCESS",
		"ledger":           eventLedger,
		"createdAt":        time.Now().Unix(),
		"applicationOrder": 1,
		"resultXdr":        resultXDR,
	}
}
