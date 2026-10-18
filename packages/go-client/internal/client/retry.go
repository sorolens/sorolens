package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RetryConfig configures retry and backoff behavior for outgoing HTTP requests.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts after an initial failure.
	// Defaults to 3. Set to 0 to disable retries.
	MaxRetries int

	// InitialBackoff is the starting delay before the first retry attempt.
	// Defaults to 100ms.
	InitialBackoff time.Duration

	// MaxBackoff is the upper limit for exponential backoff delays.
	// Defaults to 10s.
	MaxBackoff time.Duration

	// BackoffFactor is the multiplier applied to backoff duration after each retry.
	// Defaults to 2.0.
	BackoffFactor float64

	// RetryAfterCap is the maximum duration to wait when honoring a Retry-After header.
	// Defaults to 10s.
	RetryAfterCap time.Duration

	// RetryableStatusCodes is the set of HTTP status codes that trigger a retry.
	// Defaults to 429 (Too Many Requests), 502 (Bad Gateway), 503 (Service Unavailable),
	// and 504 (Gateway Timeout).
	RetryableStatusCodes []int

	// RetryableNetworkErrors determines whether transport and network errors
	// (timeouts, connection resets, EOF) trigger a retry when context is still active.
	// Defaults to true.
	RetryableNetworkErrors bool

	// OnRetry is an optional callback invoked before each retry attempt.
	// It receives the attempt number (1-based), the request, the last response (if any),
	// the last error (if any), and the planned wait duration.
	OnRetry func(attempt int, req *http.Request, resp *http.Response, err error, wait time.Duration)
}

// DefaultRetryConfig returns a production-ready default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		BackoffFactor:  2.0,
		RetryAfterCap:  10 * time.Second,
		RetryableStatusCodes: []int{
			http.StatusTooManyRequests,     // 429
			http.StatusBadGateway,         // 502
			http.StatusServiceUnavailable, // 503
			http.StatusGatewayTimeout,     // 504
		},
		RetryableNetworkErrors: true,
	}
}

// RetryRoundTripper implements http.RoundTripper with context-aware exponential
// backoff, Retry-After header handling, and request body replay.
type RetryRoundTripper struct {
	Base   http.RoundTripper
	Config RetryConfig
	mu     sync.RWMutex
}

// NewRetryRoundTripper constructs a RetryRoundTripper wrapping base with the given config.
func NewRetryRoundTripper(base http.RoundTripper, cfg RetryConfig) *RetryRoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &RetryRoundTripper{
		Base:   base,
		Config: cfg,
	}
}

// RoundTrip executes the request, retrying retriable failures with context-aware backoff.
func (r *RetryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := r.Base
	if base == nil {
		base = http.DefaultTransport
	}

	r.mu.RLock()
	cfg := r.Config
	r.mu.RUnlock()

	if cfg.MaxRetries <= 0 {
		return base.RoundTrip(req)
	}

	// Buffer request body if GetBody is nil so it can be replayed on retry.
	if req.Body != nil && req.GetBody == nil {
		bodyBytes, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("go-client: buffer request body: %w", err)
		}
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	var lastResp *http.Response
	var lastErr error
	backoff := cfg.InitialBackoff
	if backoff <= 0 {
		backoff = 100 * time.Millisecond
	}

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			// Check if context was cancelled before sleeping
			if err := req.Context().Err(); err != nil {
				return nil, err
			}

			wait := backoff
			if lastResp != nil {
				if retryAfter := parseRetryAfter(lastResp.Header.Get("Retry-After")); retryAfter > 0 {
					wait = retryAfter
					if cfg.RetryAfterCap > 0 && wait > cfg.RetryAfterCap {
						wait = cfg.RetryAfterCap
					}
				}
			}
			if cfg.MaxBackoff > 0 && wait > cfg.MaxBackoff {
				wait = cfg.MaxBackoff
			}

			if cfg.OnRetry != nil {
				cfg.OnRetry(attempt, req, lastResp, lastErr, wait)
			}

			timer := time.NewTimer(wait)
			select {
			case <-req.Context().Done():
				timer.Stop()
				return nil, req.Context().Err()
			case <-timer.C:
			}

			// Advance exponential backoff for next round
			factor := cfg.BackoffFactor
			if factor <= 1.0 {
				factor = 2.0
			}
			backoff = time.Duration(float64(backoff) * factor)
			if cfg.MaxBackoff > 0 && backoff > cfg.MaxBackoff {
				backoff = cfg.MaxBackoff
			}

			// Rewind request body for next attempt
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, fmt.Errorf("go-client: rewind request body: %w", err)
				}
				req.Body = body
			}
		}

		resp, err := base.RoundTrip(req)
		lastResp = resp
		lastErr = err

		if req.Context().Err() != nil {
			return resp, req.Context().Err()
		}

		if attempt == cfg.MaxRetries || !r.shouldRetry(cfg, resp, err) {
			return resp, err
		}

		// Prepare for next retry: drain and close response body to reuse connection
		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
		}
	}

	return lastResp, lastErr
}

func (r *RetryRoundTripper) shouldRetry(cfg RetryConfig, resp *http.Response, err error) bool {
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		if !cfg.RetryableNetworkErrors {
			return false
		}
		var netErr net.Error
		if errors.As(err, &netErr) {
			return true
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return true
		}
		return isTemporaryError(err)
	}

	if resp == nil {
		return false
	}

	for _, code := range cfg.RetryableStatusCodes {
		if resp.StatusCode == code {
			return true
		}
	}
	return false
}

func isTemporaryError(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "connection reset") ||
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "broken pipe")
}

// parseRetryAfter parses the HTTP Retry-After header.
// It supports delta-seconds ("120") and HTTP-date ("Wed, 21 Oct 2026 07:28:00 GMT").
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	// 1. Delta-seconds integer
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	// 2. HTTP-date formats supported by http.ParseTime
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

// WithRetryConfig returns a ClientOption that sets the retry configuration.
func WithRetryConfig(cfg RetryConfig) ClientOption {
	return func(c *Client) error {
		if hc, ok := c.Client.(*http.Client); ok {
			if rt, ok := hc.Transport.(*RetryRoundTripper); ok {
				rt.mu.Lock()
				rt.Config = cfg
				rt.mu.Unlock()
				return nil
			}
			hc.Transport = NewRetryRoundTripper(hc.Transport, cfg)
			return nil
		}
		c.Client = &http.Client{Transport: NewRetryRoundTripper(http.DefaultTransport, cfg)}
		return nil
	}
}

// WithMaxRetries returns a ClientOption that sets the maximum retry attempts.
func WithMaxRetries(maxRetries int) ClientOption {
	return func(c *Client) error {
		if hc, ok := c.Client.(*http.Client); ok {
			if rt, ok := hc.Transport.(*RetryRoundTripper); ok {
				rt.mu.Lock()
				rt.Config.MaxRetries = maxRetries
				rt.mu.Unlock()
				return nil
			}
			cfg := DefaultRetryConfig()
			cfg.MaxRetries = maxRetries
			hc.Transport = NewRetryRoundTripper(hc.Transport, cfg)
			return nil
		}
		cfg := DefaultRetryConfig()
		cfg.MaxRetries = maxRetries
		c.Client = &http.Client{Transport: NewRetryRoundTripper(http.DefaultTransport, cfg)}
		return nil
	}
}

// WithBackoff returns a ClientOption that sets the initial and maximum backoff durations.
func WithBackoff(initial, max time.Duration) ClientOption {
	return func(c *Client) error {
		if hc, ok := c.Client.(*http.Client); ok {
			if rt, ok := hc.Transport.(*RetryRoundTripper); ok {
				rt.mu.Lock()
				rt.Config.InitialBackoff = initial
				rt.Config.MaxBackoff = max
				rt.mu.Unlock()
				return nil
			}
			cfg := DefaultRetryConfig()
			cfg.InitialBackoff = initial
			cfg.MaxBackoff = max
			hc.Transport = NewRetryRoundTripper(hc.Transport, cfg)
			return nil
		}
		cfg := DefaultRetryConfig()
		cfg.InitialBackoff = initial
		cfg.MaxBackoff = max
		c.Client = &http.Client{Transport: NewRetryRoundTripper(http.DefaultTransport, cfg)}
		return nil
	}
}

// WithoutRetries returns a ClientOption that disables automatic retries.
func WithoutRetries() ClientOption {
	return func(c *Client) error {
		if hc, ok := c.Client.(*http.Client); ok {
			if rt, ok := hc.Transport.(*RetryRoundTripper); ok {
				hc.Transport = rt.Base
				return nil
			}
			return nil
		}
		c.Client = &http.Client{Transport: http.DefaultTransport}
		return nil
	}
}

// WithOnRetry returns a ClientOption that configures an OnRetry callback.
func WithOnRetry(fn func(attempt int, req *http.Request, resp *http.Response, err error, wait time.Duration)) ClientOption {
	return func(c *Client) error {
		if hc, ok := c.Client.(*http.Client); ok {
			if rt, ok := hc.Transport.(*RetryRoundTripper); ok {
				rt.mu.Lock()
				rt.Config.OnRetry = fn
				rt.mu.Unlock()
				return nil
			}
			cfg := DefaultRetryConfig()
			cfg.OnRetry = fn
			hc.Transport = NewRetryRoundTripper(hc.Transport, cfg)
			return nil
		}
		cfg := DefaultRetryConfig()
		cfg.OnRetry = fn
		c.Client = &http.Client{Transport: NewRetryRoundTripper(http.DefaultTransport, cfg)}
		return nil
	}
}
