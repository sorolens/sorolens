package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	// Delta seconds
	if got := parseRetryAfter("120"); got != 120*time.Second {
		t.Errorf("got %v, want 120s", got)
	}
	if got := parseRetryAfter("0"); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
	if got := parseRetryAfter("-5"); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
	if got := parseRetryAfter("invalid"); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
	if got := parseRetryAfter(""); got != 0 {
		t.Errorf("got %v, want 0", got)
	}

	// HTTP-date in the future
	future := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
	got := parseRetryAfter(future)
	if got < 25*time.Second || got > 35*time.Second {
		t.Errorf("got %v, expected ~30s", got)
	}

	// HTTP-date in the past
	past := time.Now().Add(-30 * time.Second).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(past); got != 0 {
		t.Errorf("got %v, want 0 for past date", got)
	}
}

func TestRetry_429WithDeltaSeconds(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		att := attempts.Add(1)
		if att < 3 {
			w.Header().Set("Retry-After", "0") // 0 delta seconds triggers fast retry
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limited"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"contracts":   []any{},
			"next_cursor": "",
		})
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil,
		WithBackoff(1*time.Millisecond, 10*time.Millisecond),
		WithMaxRetries(3),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := c.ListContracts(context.Background(), &ListContractsParams{})
	if err != nil {
		t.Fatalf("ListContracts: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode())
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

func TestRetry_429WithHTTPDate(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		att := attempts.Add(1)
		if att == 1 {
			future := time.Now().Add(10 * time.Millisecond).UTC().Format(http.TimeFormat)
			w.Header().Set("Retry-After", future)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"contracts":   []any{},
			"next_cursor": "",
		})
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil,
		WithBackoff(1*time.Millisecond, 50*time.Millisecond),
		WithMaxRetries(2),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := c.ListContracts(context.Background(), &ListContractsParams{})
	if err != nil {
		t.Fatalf("ListContracts: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode())
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestRetry_Transient5xx(t *testing.T) {
	var attempts atomic.Int32
	statusSequence := []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusOK}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(attempts.Add(1) - 1)
		if idx < len(statusSequence)-1 {
			w.WriteHeader(statusSequence[idx])
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"contracts": []any{}, "next_cursor": ""})
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil,
		WithBackoff(1*time.Millisecond, 5*time.Millisecond),
		WithMaxRetries(4),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := c.ListContracts(context.Background(), &ListContractsParams{})
	if err != nil {
		t.Fatalf("ListContracts: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode())
	}
	if got := attempts.Load(); got != 4 {
		t.Fatalf("attempts = %d, want 4", got)
	}
}

func TestRetry_MaxRetriesExhausted(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	const maxRetries = 2
	c, err := New(server.URL, "sl_test", nil,
		WithBackoff(1*time.Millisecond, 2*time.Millisecond),
		WithMaxRetries(maxRetries),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := c.ListContracts(context.Background(), &ListContractsParams{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.StatusCode() != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode())
	}
	// Initial attempt + 2 retries = 3 attempts total
	if got := attempts.Load(); got != int32(maxRetries+1) {
		t.Fatalf("attempts = %d, want %d", got, maxRetries+1)
	}
}

func TestRetry_NonRetryable4xx(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "not found"})
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil,
		WithBackoff(1*time.Millisecond, 2*time.Millisecond),
		WithMaxRetries(3),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := c.GetContract(context.Background(), "CNOTFOUND")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.StatusCode() != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode())
	}
	// Should not retry 404
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}

func TestRetry_ContextCancelledDuringBackoff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil,
		WithBackoff(500*time.Millisecond, 1*time.Second),
		WithMaxRetries(3),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = c.ListContracts(ctx, &ListContractsParams{})
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context error, got %v", err)
	}
}

func TestRetry_RequestBodyRewound(t *testing.T) {
	var attempts atomic.Int32
	var receivedBodies []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		att := attempts.Add(1)
		body, _ := io.ReadAll(r.Body)
		receivedBodies = append(receivedBodies, string(body))
		if att == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAH2",
			"network": "testnet",
			"status":  "pending",
		})
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil,
		WithBackoff(1*time.Millisecond, 2*time.Millisecond),
		WithMaxRetries(2),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	label := "my-contract"
	resp, err := c.RegisterContract(context.Background(), RegisterContractJSONRequestBody{
		Id:      "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAH2",
		Network: RegisterContractJSONBodyNetworkTestnet,
		Label:   &label,
	})
	if err != nil {
		t.Fatalf("RegisterContract: %v", err)
	}
	if resp.StatusCode() != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode())
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
	if len(receivedBodies) != 2 {
		t.Fatalf("received %d bodies, want 2", len(receivedBodies))
	}
	if receivedBodies[0] != receivedBodies[1] {
		t.Errorf("body on retry differed: %q vs %q", receivedBodies[0], receivedBodies[1])
	}
}

func TestRetry_WithoutRetries(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil, WithoutRetries())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := c.ListContracts(context.Background(), &ListContractsParams{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.StatusCode() != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", resp.StatusCode())
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1 when retries disabled", got)
	}
}

func TestRetry_OnRetryCallback(t *testing.T) {
	var attempts atomic.Int32
	var retryCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		att := attempts.Add(1)
		if att < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"contracts": []any{}, "next_cursor": ""})
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil,
		WithBackoff(1*time.Millisecond, 5*time.Millisecond),
		WithMaxRetries(3),
		WithOnRetry(func(attempt int, req *http.Request, resp *http.Response, err error, wait time.Duration) {
			retryCount.Add(1)
			if attempt <= 0 {
				t.Errorf("attempt must be >= 1, got %d", attempt)
			}
			if resp == nil && err == nil {
				t.Error("expected non-nil resp or err")
			}
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := c.ListContracts(context.Background(), &ListContractsParams{})
	if err != nil {
		t.Fatalf("ListContracts: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode())
	}
	if got := retryCount.Load(); got != 2 {
		t.Fatalf("onRetry calls = %d, want 2", got)
	}
}
