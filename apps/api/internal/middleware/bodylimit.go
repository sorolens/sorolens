package middleware

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
)

// BodyLimit returns middleware that rejects request bodies larger than
// maxBytes with a 413 Payload Too Large response. A maxBytes value of zero or
// less disables the limit and returns the next handler unchanged.
//
// Two layers of enforcement are applied:
//
//  1. A declared Content-Length above the limit is rejected before any body
//     bytes are read, so a client cannot make the server buffer an oversized
//     payload only to reject it afterwards.
//  2. Every other request body is wrapped in http.MaxBytesReader, which bounds
//     the read even when Content-Length is absent (chunked transfer encoding)
//     and aborts the connection once the limit is exceeded. If a handler trips
//     that limit while reading and never wrote a response, the middleware
//     converts the outcome into the same 413 error body.
//
// Requests without a body are passed through untouched, which keeps streaming
// endpoints such as /api/v1/stream/events on the unwrapped ResponseWriter.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil || r.ContentLength == 0 {
				next.ServeHTTP(w, r)
				return
			}

			if r.ContentLength > maxBytes {
				writePayloadTooLarge(w, r, maxBytes)
				return
			}

			body := &maxBytesBody{ReadCloser: http.MaxBytesReader(w, r.Body, maxBytes)}
			r.Body = body

			rec := &responseTracker{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			if body.exceeded && !rec.wroteHeader {
				writePayloadTooLarge(rec, r, maxBytes)
			}
		})
	}
}

// maxBytesBody records whether the wrapped http.MaxBytesReader refused a read
// because the request body exceeded the configured limit. Handlers usually
// surface that error themselves; recording it lets the middleware turn a
// handler that gives up without writing into a 413 instead of a silent 200.
type maxBytesBody struct {
	io.ReadCloser
	exceeded bool
}

func (b *maxBytesBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		b.exceeded = true
	}
	return n, err
}

// responseTracker records whether a handler wrote a response so the body limit
// middleware never overwrites one. Unwrap keeps http.NewResponseController
// able to reach the underlying writer's Flusher, Hijacker and friends.
type responseTracker struct {
	http.ResponseWriter
	wroteHeader bool
}

func (t *responseTracker) WriteHeader(statusCode int) {
	t.wroteHeader = true
	t.ResponseWriter.WriteHeader(statusCode)
}

func (t *responseTracker) Write(b []byte) (int, error) {
	t.wroteHeader = true
	return t.ResponseWriter.Write(b)
}

func (t *responseTracker) Unwrap() http.ResponseWriter { return t.ResponseWriter }

func writePayloadTooLarge(w http.ResponseWriter, r *http.Request, maxBytes int64) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	// Ignore encode errors (e.g., client disconnect while writing the response); there's no meaningful recovery here.
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":       "PAYLOAD_TOO_LARGE",
			"message":    "request body exceeds the " + strconv.FormatInt(maxBytes, 10) + " byte limit",
			"request_id": GetRequestID(r.Context()),
		},
	})
}
