package middleware

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	n, err := r.ResponseWriter.Write(p)
	r.bytes += int64(n)
	return n, err
}

// Hijack delegates to the underlying writer so protocol upgrades (WebSocket
// handshake in the /subscribe handler) work through the logger middleware.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("middleware: underlying ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}

// Flush forwards to the underlying writer so SSE handlers behind Logger can
// still stream; without it the w.(http.Flusher) assertion fails.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer, e.g. to
// extend the write deadline for streams and pprof profiles.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Logger logs each request with its resolved route, client IP, response size,
// user agent, status, duration, and request ID.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			route := ""
			if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
				route = routeCtx.RoutePattern()
			}
			logger.Info("request",
				"method", r.Method,
				"route_pattern", route,
				"client_ip", clientIP(r),
				"user_agent", r.UserAgent(),
				"response_bytes", rec.bytes,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", GetRequestID(r.Context()),
			)
		})
	}
}
