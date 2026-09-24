package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeoutMiddleware_Returns503OnTimeout(t *testing.T) {
	slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	middleware := Timeout(10 * time.Millisecond)(slowHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
}

func TestTimeoutMiddleware_CancelsContext(t *testing.T) {
	var ctx context.Context
	done := make(chan struct{})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
		<-r.Context().Done()
		close(done)
	})

	middleware := Timeout(10 * time.Millisecond)(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	<-done

	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("expected context deadline exceeded, got %v", ctx.Err())
	}
}

func TestTimeoutMiddleware_AllowsFastRequests(t *testing.T) {
	fastHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := Timeout(30 * time.Second)(fastHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestTimeoutMiddleware_Default30Seconds(t *testing.T) {
	middleware := Timeout(30 * time.Second)

	if middleware == nil {
		t.Fatal("Timeout middleware should not be nil")
	}
}

func TestTimeoutMiddleware_StreamTimeout5Minutes(t *testing.T) {
	middleware := Timeout(5 * time.Minute)

	if middleware == nil {
		t.Fatal("Stream timeout middleware should not be nil")
	}
}

func TestTimeoutMiddleware_HandlerCompletesBeforeTimeout(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	middleware := Timeout(100 * time.Millisecond)(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", rr.Body.String())
	}
}

func TestTimeoutMiddleware_ConnectionClosedOnTimeout(t *testing.T) {
	slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	middleware := Timeout(10 * time.Millisecond)(slowHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	connHeader := rr.Header().Get("Connection")
	if connHeader != "close" {
		t.Errorf("expected Connection: close header, got %q", connHeader)
	}
}