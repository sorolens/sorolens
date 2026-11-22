package sorobanrpc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProviderPool_CircuitBreakerAndRecovery(t *testing.T) {
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeReq(t, r)
		writeResult(w, req.ID, map[string]any{"sequence": 100, "protocolVersion": 22})
	}))
	defer srv2.Close()

	pool := NewProviderPool([]ProviderEndpoint{
		{URL: srv1.URL, Weight: 1},
		{URL: srv2.URL, Weight: 1},
	})

	// Trigger failures on srv1 to trip circuit breaker
	for i := 0; i < 3; i++ {
		client, err := pool.SelectBestClient(0)
		if err != nil {
			t.Fatalf("select client: %v", err)
		}
		start := time.Now()
		_, err = client.GetLatestLedger(context.Background())
		pool.RecordResult(client.Endpoint(), time.Since(start), errors.New("fail"))
	}

	metrics := pool.Metrics()
	foundTripped := false
	for _, m := range metrics {
		if m.URL == srv1.URL && m.State == "tripped" {
			foundTripped = true
		}
	}
	if !foundTripped {
		T := t
		T.Fatalf("expected srv1 to be tripped, metrics: %+v", metrics)
	}

	// Next selection should prefer srv2
	client, err := pool.SelectBestClient(0)
	if err != nil {
		t.Fatalf("select best client: %v", err)
	}
	if client.Endpoint() != srv2.URL {
		t.Fatalf("want srv2 URL %s, got %s", srv2.URL, client.Endpoint())
	}
}
