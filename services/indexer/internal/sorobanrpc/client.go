// Package sorobanrpc is the indexer's Soroban JSON-RPC 2.0 client. It
// implements poller.RPCClient (plus the optional poller.TransactionLister) over
// HTTP so the poller reads real chain data instead of the old stub adapter.
//
// The wire handling mirrors apps/api/internal/soroban. That package cannot be
// imported from here: it sits under apps/api/internal and belongs to a
// different Go module, so Go's internal-package rule forbids the import.
//
// Every request gets its own timeout. HTTP 429, HTTP 5xx and transport
// failures are retried a bounded number of times with capped exponential
// backoff (a 429 Retry-After header is honoured up to the cap). JSON-RPC
// application errors are not retried and surface as *RPCError, which keeps the
// node's error code.
package sorobanrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

const (
	// DefaultTimeout bounds a single HTTP round trip.
	DefaultTimeout = 30 * time.Second
	// DefaultMaxRetries is how many times a retriable failure is retried
	// after the initial attempt.
	DefaultMaxRetries = 3
	// DefaultBackoffBase is the delay before the first retry; it doubles on
	// each further retry up to DefaultBackoffMax.
	DefaultBackoffBase = 500 * time.Millisecond
	// DefaultBackoffMax caps the delay between retries.
	DefaultBackoffMax = 10 * time.Second

	// eventsPageLimit is the page size requested from getEvents.
	eventsPageLimit = 1000
	// maxEventPages bounds how many getEvents pages one GetEvents call
	// follows, so a pathological window cannot loop forever.
	maxEventPages = 100
	// maxResponseBytes caps how much of a response body is read.
	maxResponseBytes = 64 << 20
)

// Option customises a Client built by New.
type Option func(*Client)

// WithTimeout sets the per-request timeout. Non-positive values keep the
// default.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// WithMaxRetries sets how many times a retriable failure is retried. A
// negative value disables retries.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		if n < 0 {
			n = 0
		}
		c.maxRetries = n
	}
}

// WithBackoff sets the base and maximum delay between retries. Non-positive
// values keep the defaults.
func WithBackoff(base, max time.Duration) Option {
	return func(c *Client) {
		if base > 0 {
			c.backoffBase = base
		}
		if max > 0 {
			c.backoffMax = max
		}
	}
}

// WithHTTPClient replaces the underlying *http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// Client calls a single Soroban RPC endpoint. It is safe for concurrent use.
type Client struct {
	endpoint    string
	httpClient  *http.Client
	timeout     time.Duration
	maxRetries  int
	backoffBase time.Duration
	backoffMax  time.Duration
	nextID      atomic.Int64
}

// New returns a Client for endpoint.
func New(endpoint string, opts ...Option) *Client {
	c := &Client{
		endpoint:    endpoint,
		httpClient:  &http.Client{},
		timeout:     DefaultTimeout,
		maxRetries:  DefaultMaxRetries,
		backoffBase: DefaultBackoffBase,
		backoffMax:  DefaultBackoffMax,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Endpoint returns the URL the client calls.
func (c *Client) Endpoint() string { return c.endpoint }

// ---- errors ---------------------------------------------------------------

// RPCError is a JSON-RPC application error returned by the node. It is never
// retried.
type RPCError struct {
	Method  string          `json:"-"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("soroban rpc %s: code %d: %s", e.Method, e.Code, e.Message)
}

// HTTPError is a non-200 HTTP response. 429 and 5xx are retriable.
type HTTPError struct {
	Method     string
	StatusCode int
	retryAfter time.Duration
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("soroban rpc %s: HTTP %d", e.Method, e.StatusCode)
}

// Retriable reports whether the request may succeed if retried.
func (e *HTTPError) Retriable() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500
}

// ErrTransactionNotFound is returned by GetTransaction when the node reports
// NOT_FOUND (unknown hash, or outside the node's retention window).
var ErrTransactionNotFound = errors.New("soroban rpc: transaction not found")

// transportError wraps a failed HTTP round trip (connection refused, reset,
// per-request timeout). It is retriable unless the caller's context is done.
type transportError struct {
	method string
	err    error
}

func (e *transportError) Error() string {
	return fmt.Sprintf("soroban rpc %s: %v", e.method, e.err)
}
func (e *transportError) Unwrap() error { return e.err }

// ---- transport ------------------------------------------------------------

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
}

// call performs method with params and decodes the result into out, retrying
// retriable failures with capped exponential backoff.
func (c *Client) call(ctx context.Context, method string, params any, out any) error {
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID.Add(1),
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return fmt.Errorf("soroban rpc %s: marshal request: %w", method, err)
	}

	delay := c.backoffBase
	var lastErr error
	for attempt := 0; ; attempt++ {
		lastErr = c.doOnce(ctx, method, body, out)
		if lastErr == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !retriable(lastErr) || attempt >= c.maxRetries {
			break
		}

		wait := delay
		var he *HTTPError
		if errors.As(lastErr, &he) && he.retryAfter > wait {
			wait = he.retryAfter
		}
		if wait > c.backoffMax {
			wait = c.backoffMax
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
		delay *= 2
		if delay > c.backoffMax {
			delay = c.backoffMax
		}
	}
	if attempt := c.maxRetries; attempt > 0 && retriable(lastErr) {
		return fmt.Errorf("soroban rpc %s: giving up after %d retries: %w", method, attempt, lastErr)
	}
	return lastErr
}

func retriable(err error) bool {
	var he *HTTPError
	if errors.As(err, &he) {
		return he.Retriable()
	}
	var te *transportError
	return errors.As(err, &te)
}

// doOnce performs a single HTTP round trip.
func (c *Client) doOnce(ctx context.Context, method string, body []byte, out any) error {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("soroban rpc %s: build request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &transportError{method: method, err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes)) //nolint:errcheck
		return &HTTPError{
			Method:     method,
			StatusCode: resp.StatusCode,
			retryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}

	var envelope rpcResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&envelope); err != nil {
		if reqCtx.Err() != nil {
			// The per-request deadline fired mid-body: treat like any other
			// timed-out round trip.
			return &transportError{method: method, err: reqCtx.Err()}
		}
		return fmt.Errorf("soroban rpc %s: decode response: %w", method, err)
	}
	if envelope.Error != nil {
		envelope.Error.Method = method
		return envelope.Error
	}
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return fmt.Errorf("soroban rpc %s: response has neither result nor error", method)
	}
	if err := json.Unmarshal(envelope.Result, out); err != nil {
		return fmt.Errorf("soroban rpc %s: decode result: %w", method, err)
	}
	return nil
}

// parseRetryAfter reads a Retry-After header given in delta-seconds. HTTP-date
// values are ignored and fall back to the computed backoff.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	secs, err := strconv.Atoi(v)
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}
