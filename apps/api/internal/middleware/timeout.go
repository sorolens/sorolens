package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Default request budgets (issue #154). Handlers get RequestTimeout to finish
// before the client receives a 503; long-lived streams get StreamTimeout
// before their context is cancelled and the stream ends.
const (
	DefaultRequestTimeout = 30 * time.Second
	DefaultStreamTimeout  = 5 * time.Minute
)

// streamWriteGrace is how far past the stream deadline the connection write
// deadline is pushed, so the handler can flush its last frame after its
// context is cancelled instead of the server cutting the write mid-frame.
const streamWriteGrace = 5 * time.Second

// Timeout caps request handling at d (issue #154). The handler runs with a
// context that is cancelled at the deadline, so store calls using
// r.Context() abort; if the handler has not finished by then the client gets
// a 503 with the standard JSON error envelope.
//
// It is built on http.TimeoutHandler, which buffers the response and
// discards any write the handler makes after the deadline, so a slow handler
// can never race the 503 onto the wire. The buffering means the wrapped
// handler cannot stream: use StreamTimeout for SSE routes.
//
// A panic in the handler is re-raised on the request goroutine, so
// Recoverer higher in the chain still turns it into a 500.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			th := http.TimeoutHandler(next, d, timeoutBody(r))
			th.ServeHTTP(&timeoutJSONWriter{ResponseWriter: w}, r)
		})
	}
}

// StreamTimeout bounds a long-lived streaming response (SSE) at d. It does
// not buffer, so http.Flusher reaches the handler, and it cannot send a 503:
// the stream's headers are already out when the deadline fires. Instead the
// request context is cancelled and a well-behaved stream handler returns;
// EventSource clients reconnect on their own.
//
// It also moves the connection's write deadline to d (plus a short grace),
// so the server's global WriteTimeout does not cut the stream first.
func StreamTimeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// ErrNotSupported (e.g. httptest.ResponseRecorder) leaves the
			// server's own deadline in place, which is the safe fallback.
			_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(d + streamWriteGrace))

			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// timeoutBody renders the 503 body in the same envelope as every other API
// error, carrying the request ID so a timed-out call can be traced in logs.
func timeoutBody(r *http.Request) string {
	b, _ := json.Marshal(map[string]any{
		"error": map[string]string{
			"code":       "TIMEOUT",
			"message":    "request exceeded the server time limit",
			"request_id": GetRequestID(r.Context()),
		},
	})
	return string(b)
}

// timeoutJSONWriter labels the http.TimeoutHandler 503 as JSON. On the
// timeout path TimeoutHandler writes the status without copying any handler
// headers, so a 503 with no Content-Type is the timeout response.
type timeoutJSONWriter struct {
	http.ResponseWriter
}

func (t *timeoutJSONWriter) WriteHeader(code int) {
	if code == http.StatusServiceUnavailable && t.Header().Get("Content-Type") == "" {
		t.Header().Set("Content-Type", "application/json")
	}
	t.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (t *timeoutJSONWriter) Unwrap() http.ResponseWriter { return t.ResponseWriter }
