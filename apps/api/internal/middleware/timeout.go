package middleware

import (
	"context"
	"net/http"
	"time"
)

type timeoutHandler struct {
	handler http.Handler
	timeout time.Duration
}

func (t *timeoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), t.timeout)
	defer cancel()

	r = r.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		t.handler.ServeHTTP(w, r)
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			w.Header().Set("Connection", "close")
			http.Error(w, "Service Unavailable: Request Timeout", http.StatusServiceUnavailable)
		}
	}
}

func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &timeoutHandler{handler: next, timeout: timeout}
	}
}