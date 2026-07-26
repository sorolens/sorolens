package soroban

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
	maxRetries     = 5
	maxBackoff     = 30 * time.Second
)

// Client is a Soroban RPC JSON-RPC 2.0 client.
// It retries on HTTP 5xx and 429 responses with exponential backoff and jitter.
type Client struct {
	endpoint   string
	httpClient *http.Client
	timeout    time.Duration
}

// New returns a Client that calls endpoint with per-request timeouts of timeout.
// Pass 0 to use the default 30-second timeout.
func New(endpoint string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{
		endpoint:   endpoint,
		httpClient: &http.Client{},
		timeout:    timeout,
	}
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

// ---- internal transport ---------------------------------------------------

// call makes one JSON-RPC request, retrying on retriable errors.
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
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			wait := backoffDuration(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}

		lastErr = c.doOnce(ctx, reqBody, result)
		if lastErr == nil {
			return nil
		}
		if !isRetriable(lastErr) {
			return lastErr
		}
	}
	return fmt.Errorf("soroban: %s: exceeded %d retries: %w", method, maxRetries, lastErr)
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

func (e *networkError) Error() string { return e.err.Error() }
func (e *networkError) Unwrap() error { return e.err }
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

// ---- backoff --------------------------------------------------------------

// backoffDuration returns the wait time before retry attempt n (1-indexed).
// Strategy: min(2^n seconds, 30s) + uniform jitter in [0, 1s).
func backoffDuration(attempt int) time.Duration {
	base := time.Duration(1<<uint(attempt)) * time.Second
	if base > maxBackoff {
		base = maxBackoff
	}
	jitter := time.Duration(rand.Int63n(int64(time.Second)))
	return base + jitter
}
