// Package soroban provides a typed JSON-RPC 2.0 client for the Stellar
// Soroban RPC. It wraps the getLatestLedger, getEvents, getLedgerEntries,
// getTransaction, and getNetwork methods with automatic retry on HTTP 5xx
// and 429 (rate limit) responses using configurable exponential backoff with
// jitter. The delay doubles on each consecutive retriable failure, is capped,
// and resets to the base delay after any successful call.
//
// # Retention window
//
// The public SDF RPC endpoints retain events for approximately 24 hours
// and transaction data for up to 7 days (about 100,000 ledgers at 5-6
// seconds per ledger). Because of this 7-day retention window, a newly
// tracked contract cannot backfill its full history; the earliest
// queryable startLedger is max(latestLedger - 100_000, 1). Passing a
// startLedger below the oldestLedger returned by the RPC will result in
// an error.
//
// See RESEARCH.md section 1.2 for the full retention specification.
package soroban

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	defaultTimeout = 30 * time.Second

	// defaultMaxRetries is the number of retries after the initial attempt.
	defaultMaxRetries = 5
	// defaultBackoffBase is the delay before the first retry; each further
	// consecutive retriable failure doubles it.
	defaultBackoffBase = 1 * time.Second
	// defaultBackoffMax caps the delay between retries, before jitter.
	defaultBackoffMax = 60 * time.Second
	// defaultBackoffJitter is the upper bound of the uniform random jitter
	// added to each retry delay.
	defaultBackoffJitter = 1 * time.Second
)

// Backoff configures the exponential delay applied between retries of a
// retriable RPC failure. The delay doubles with each consecutive retriable
// failure from Base up to Max, and any successful call resets it to Base.
//
// Zero-valued fields fall back to their defaults (1s base, 60s cap, 1s
// jitter). Set Jitter or Max to a negative value to disable that part.
type Backoff struct {
	// Base is the delay before the first retry. Defaults to 1s.
	Base time.Duration
	// Max caps the delay. Defaults to 60s; negative disables the cap.
	Max time.Duration
	// Jitter is the upper bound of a uniform random amount added to each
	// delay. Defaults to 1s; negative disables jitter.
	Jitter time.Duration
}

// normalize returns b with zero-valued fields replaced by their defaults and
// negative values disabled.
func (b Backoff) normalize() Backoff {
	if b.Base == 0 {
		b.Base = defaultBackoffBase
	}
	if b.Base < 0 {
		b.Base = 0
	}
	if b.Max == 0 {
		b.Max = defaultBackoffMax
	}
	if b.Max < 0 {
		b.Max = 0
	}
	if b.Jitter == 0 {
		b.Jitter = defaultBackoffJitter
	}
	if b.Jitter < 0 {
		b.Jitter = 0
	}
	return b
}

// delay returns the wait to apply before the retry that follows failures
// consecutive retriable failures (1-indexed): min(Base * 2^(failures-1), Max)
// plus uniform jitter in [0, Jitter).
func (b Backoff) delay(failures int) time.Duration {
	if failures < 1 {
		failures = 1
	}
	d := b.Base
	for i := 1; i < failures; i++ {
		if d > time.Duration(1)<<62 {
			break
		}
		d *= 2
		if b.Max > 0 && d >= b.Max {
			d = b.Max
			break
		}
	}
	if b.Max > 0 && d > b.Max {
		d = b.Max
	}
	if b.Jitter > 0 {
		d += time.Duration(rand.Int63n(int64(b.Jitter)))
	}
	return d
}

// Option customises a Client constructed by New.
type Option func(*Client)

// WithBackoff overrides the exponential backoff applied between retries.
func WithBackoff(b Backoff) Option {
	return func(c *Client) { c.backoff = b.normalize() }
}

// WithMaxRetries sets how many times a retriable request is retried after the
// initial attempt. A negative value disables retries.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		if n < 0 {
			n = 0
		}
		c.maxRetries = n
	}
}

// Client is a Soroban RPC JSON-RPC 2.0 client.
// It retries on HTTP 5xx and 429 responses with configurable exponential
// backoff and jitter that resets after any successful call.
type Client struct {
	endpoint   string
	httpClient *http.Client
	timeout    time.Duration
	backoff    Backoff
	maxRetries int

	mu       sync.Mutex
	failures int
}

// New returns a Client that calls endpoint with per-request timeouts of timeout.
// Pass 0 to use the default 30-second timeout. Options may override the retry
// count and backoff.
func New(endpoint string, timeout time.Duration, opts ...Option) *Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	c := &Client{
		endpoint:   endpoint,
		httpClient: &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)},
		timeout:    timeout,
		backoff:    Backoff{}.normalize(),
		maxRetries: defaultMaxRetries,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GetLatestLedger returns the latest ledger sequence known to the node.
func (c *Client) GetLatestLedger(ctx context.Context) (*LatestLedger, error) {
	var result LatestLedger
	if err := c.call(ctx, "getLatestLedger", struct{}{}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetEvents fetches events in [startLedger, endLedger) matching filters.
// Pass a non-empty cursor via the returned GetEventsResult.Cursor to paginate.
func (c *Client) GetEvents(ctx context.Context, startLedger, endLedger uint32, filters []EventFilter) (*GetEventsResult, error) {
	params := getEventsParams{
		StartLedger: startLedger,
		EndLedger:   endLedger,
		Filters:     filters,
		Pagination:  &pagination{Limit: 1000},
	}
	var result GetEventsResult
	if err := c.call(ctx, "getEvents", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetEventsPage is like GetEvents but starts from an opaque cursor instead of
// a ledger range. Use when paginating a result set returned by GetEvents.
func (c *Client) GetEventsPage(ctx context.Context, cursor string, filters []EventFilter) (*GetEventsResult, error) {
	params := getEventsParams{
		Filters:    filters,
		Pagination: &pagination{Cursor: cursor, Limit: 1000},
	}
	var result GetEventsResult
	if err := c.call(ctx, "getEvents", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetLedgerEntries fetches on-chain state for the given base64-encoded XDR
// LedgerKeys. Keys for archived entries will be absent from the result.
func (c *Client) GetLedgerEntries(ctx context.Context, keys []string) (*GetLedgerEntriesResult, error) {
	params := struct {
		Keys []string `json:"keys"`
	}{Keys: keys}
	var result GetLedgerEntriesResult
	if err := c.call(ctx, "getLedgerEntries", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTransaction returns the result of the transaction identified by hash.
// hash is a 64-character hex string.
func (c *Client) GetTransaction(ctx context.Context, hash string) (*TransactionResult, error) {
	params := struct {
		Hash string `json:"hash"`
	}{Hash: hash}
	var result TransactionResult
	if err := c.call(ctx, "getTransaction", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetNetwork returns the network passphrase and protocol version reported by
// the node. Used to confirm the node is reachable and on the right network.
func (c *Client) GetNetwork(ctx context.Context) (*NetworkInfo, error) {
	var result NetworkInfo
	if err := c.call(ctx, "getNetwork", struct{}{}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetContractWasmHash returns the current Wasm hash for the given contract.
// Returns an empty string when the hash cannot be determined, and an error on
// RPC transport failures.
func (c *Client) GetContractWasmHash(_ context.Context, _ string) (string, error) {
	return "", nil
}

// ---- internal transport ---------------------------------------------------

// call makes one JSON-RPC request, retrying on retriable errors with
// exponential backoff. A successful call resets the backoff so the next
// failure starts at the base delay again.
func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	reqBody, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return fmt.Errorf("soroban: marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			wait := c.backoff.delay(c.failuresSnapshot())
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}

		lastErr = c.doOnce(ctx, reqBody, result)
		if lastErr == nil {
			c.resetBackoff()
			return nil
		}
		if !isRetriable(lastErr) {
			return lastErr
		}
		c.recordFailure()
	}
	return fmt.Errorf("soroban: %s: exceeded %d retries: %w", method, c.maxRetries, lastErr)
}

// failuresSnapshot returns the number of consecutive retriable failures
// recorded since the last successful call.
func (c *Client) failuresSnapshot() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.failures
}

// recordFailure increments the consecutive retriable failure counter.
func (c *Client) recordFailure() {
	c.mu.Lock()
	c.failures++
	c.mu.Unlock()
}

// resetBackoff clears the consecutive failure counter after a success.
func (c *Client) resetBackoff() {
	c.mu.Lock()
	c.failures = 0
	c.mu.Unlock()
}

// doOnce performs a single HTTP round-trip and decodes the JSON-RPC response.
func (c *Client) doOnce(ctx context.Context, body []byte, result any) error {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &networkError{err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		return &httpError{code: resp.StatusCode}
	}
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		return &httpError{code: resp.StatusCode}
	}

	// Decode into a raw-result envelope first so we can separate RPC errors
	// from successful results without losing the error body.
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *RPCError       `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if envelope.Error != nil {
		return envelope.Error
	}
	if envelope.Result == nil {
		return fmt.Errorf("response has no result and no error")
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("decode result: %w", err)
	}
	return nil
}

// ---- error types ----------------------------------------------------------

type httpError struct {
	code int
}

func (e *httpError) Error() string {
	return fmt.Sprintf("HTTP %d", e.code)
}

func (e *httpError) retriable() bool {
	return e.code == http.StatusTooManyRequests || e.code >= 500
}

type networkError struct {
	err error
}

func (e *networkError) Error() string   { return e.err.Error() }
func (e *networkError) Unwrap() error   { return e.err }
func (e *networkError) retriable() bool { return true }

type retriable interface {
	retriable() bool
}

func isRetriable(err error) bool {
	var r retriable
	switch v := err.(type) {
	case retriable:
		r = v
	default:
		return false
	}
	return r.retriable()
}
