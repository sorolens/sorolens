package middleware

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

type timeoutHandler struct {
	handler http.Handler
	timeout time.Duration
}

type timeoutResponseWriter struct {
	http.ResponseWriter
	written atomic.Bool
}

func (t *timeoutResponseWriter) WriteHeader(code int) {
	if !t.written.Load() {
		t.written.Store(true)
		t.ResponseWriter.WriteHeader(code)
	}
}

func (t *timeoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), t.timeout)
	defer cancel()

	r = r.WithContext(ctx)

	tw := &timeoutResponseWriter{ResponseWriter: w}
	done := make(chan struct{})
	go func() {
		t.handler.ServeHTTP(tw, r)
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded && !tw.written.Load() {
			tw.Header().Set("Connection", "close")
			http.Error(tw, "Service Unavailable: Request Timeout", http.StatusServiceUnavailable)
		}
	}
}

func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &timeoutHandler{handler: next, timeout: timeout}
	}
}