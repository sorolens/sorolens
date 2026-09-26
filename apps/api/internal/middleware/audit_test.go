package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type recordingAuditWriter struct {
	mu     sync.Mutex
	events []store.AuditEvent
	block  chan struct{} // when non-nil, writes wait on it
}

func (w *recordingAuditWriter) InsertAuditEvent(ctx context.Context, e store.AuditEvent) error {
	if w.block != nil {
		select {
		case <-w.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.events = append(w.events, e)
	return nil
}

func (w *recordingAuditWriter) wait(t *testing.T, n int) []store.AuditEvent {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		w.mu.Lock()
		got := append([]store.AuditEvent(nil), w.events...)
		w.mu.Unlock()
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("want %d audit rows, got %d", n, len(got))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestAuditNeverBlocksOnStalledStore(t *testing.T) {
	writer := &recordingAuditWriter{block: make(chan struct{})}
	defer close(writer.block)
	h := Audit(writer, nil, "/")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	done := make(chan struct{})
	go func() {
		defer close(done)
		// More requests than the in-flight cap: overflow rows are dropped
		// instead of blocking the request path.
		for i := 0; i < auditMaxInFlight+10; i++ {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/contracts", strings.NewReader("{}")))
			if w.Code != http.StatusCreated {
				t.Errorf("want 201, got %d", w.Code)
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("requests blocked on a stalled audit store")
	}
}

func TestAuditRecordsPanicAs500(t *testing.T) {
	writer := &recordingAuditWriter{}
	h := Audit(writer, nil, "/")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic must propagate to the recoverer")
			}
		}()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodDelete, "/api/v1/api-keys/k1", nil))
	}()

	e := writer.wait(t, 1)[0]
	if e.Status != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", e.Status)
	}
}

func TestAuditHashesUnreadBody(t *testing.T) {
	writer := &recordingAuditWriter{}
	// The handler never reads the body; the middleware must still hash it.
	h := Audit(writer, nil, "/")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/api/v1/x", strings.NewReader("hello")))

	e := writer.wait(t, 1)[0]
	const helloSHA256 = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if e.RequestBodyHash != helloSHA256 {
		t.Fatalf("want sha256(hello), got %q", e.RequestBodyHash)
	}
	if e.Status != http.StatusAccepted {
		t.Fatalf("want 202, got %d", e.Status)
	}
}

func TestActor(t *testing.T) {
	cases := []struct {
		name   string
		header map[string]string
		want   string
	}{
		{"user id", map[string]string{"X-User-ID": "u1"}, "u1"},
		{"github login", map[string]string{"X-GitHub-Login": "octo"}, "github:octo"},
		{"api key", map[string]string{"Authorization": "Bearer secret-token"}, "apikey:" + store.HashKey("secret-token")[:12]},
		{"anonymous", nil, "anonymous"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			for k, v := range tc.header {
				r.Header.Set(k, v)
			}
			if got := Actor(r); got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestResourceType(t *testing.T) {
	cases := map[string]string{
		"/api/v1/contracts":              "contracts",
		"/api/v1/contracts/{id}":         "contracts",
		"/api/v1/admin/keys/{id}":        "keys",
		"/api/v1/watched-accounts/{id}":  "watched-accounts",
		"/api/v1/watchlist/{contractId}": "watchlist",
	}
	for pattern, want := range cases {
		if got := resourceType(pattern); got != want {
			t.Errorf("%s: want %q, got %q", pattern, want, got)
		}
	}
}
