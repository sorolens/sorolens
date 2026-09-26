package router_test

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// stuckStore simulates a slow DB query: GetGlobalStats blocks until the
// request context is cancelled, as pgx does when its ctx expires.
type stuckStore struct {
	*store.MockStore
	cancelled chan struct{}
}

func (s *stuckStore) GetGlobalStats(ctx context.Context) (store.GlobalStats, error) {
	<-ctx.Done()
	close(s.cancelled)
	return store.GlobalStats{}, ctx.Err()
}

// noLimitRedis keeps the global RateLimit middleware out of the way: it
// counts one request per window, far under the limit.
type noLimitRedis struct{}

func (noLimitRedis) Incr(context.Context, string) (int64, error) { return 1, nil }
func (noLimitRedis) Expire(context.Context, string, time.Duration) (bool, error) {
	return true, nil
}

func timeoutTestHandler(st handler.APIStore) *handler.Handler {
	return &handler.Handler{
		Store:       st,
		DB:          &store.MockPinger{Healthy: true},
		Redis:       &store.MockPinger{Healthy: true},
		RedisClient: noLimitRedis{},
		// Set up front: Handler.Hub() initializes lazily without a lock, and
		// the stream test calls it from a second goroutine.
		StreamHub:      handler.NewStreamHub(),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		RequestTimeout: 50 * time.Millisecond,
		StreamTimeout:  400 * time.Millisecond,
	}
}

// TestRouter_SlowRouteReturns503 wires issue #154 end to end: an /api/v1
// route whose store call hangs gets a 503 at RequestTimeout, and the store
// sees its context cancelled so the DB connection is released.
func TestRouter_SlowRouteReturns503(t *testing.T) {
	st := &stuckStore{MockStore: store.NewMockStore(), cancelled: make(chan struct{})}
	srv := httptest.NewServer(router.New(timeoutTestHandler(st), 1<<20))
	defer srv.Close()

	start := time.Now()
	resp, err := http.Get(srv.URL + "/api/v1/stats/global")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d (%s), want 503", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"TIMEOUT"`) {
		t.Errorf("body = %s, want TIMEOUT error code", body)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("503 took %v, want about 50ms", elapsed)
	}
	select {
	case <-st.cancelled:
	case <-time.After(time.Second):
		t.Error("store call context was never cancelled")
	}
}

// TestRouter_StreamIsNotCappedByRequestTimeout guards the SSE route. The
// default cap must not apply to it (a nested 30s deadline would win over
// 5m), and every writer in the production chain must keep http.Flusher, or
// the handler answers 500 "streaming unsupported".
func TestRouter_StreamIsNotCappedByRequestTimeout(t *testing.T) {
	h := timeoutTestHandler(store.NewMockStore())
	srv := httptest.NewServer(router.New(h, 1<<20))
	defer srv.Close()

	start := time.Now()
	resp, err := http.Get(srv.URL + "/api/v1/stream/events?contract_id=CSTREAM")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (%s), want 200", resp.StatusCode, b)
	}

	// Publish well past RequestTimeout (50ms) but inside StreamTimeout.
	go func() {
		time.Sleep(150 * time.Millisecond)
		h.Hub().PublishEvent(store.Event{ContractID: "CSTREAM"})
	}()

	var types []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.Contains(line, `"type":"connected"`):
			types = append(types, "connected")
		case strings.Contains(line, `"type":"event"`):
			types = append(types, "event")
		}
	}
	elapsed := time.Since(start)

	if len(types) != 2 || types[1] != "event" {
		t.Fatalf("stream frames = %v, want [connected event]", types)
	}
	if elapsed < 300*time.Millisecond {
		t.Errorf("stream closed after %v, before StreamTimeout (400ms)", elapsed)
	}
	if elapsed > 3*time.Second {
		t.Errorf("stream stayed open %v; StreamTimeout did not close it", elapsed)
	}
}
