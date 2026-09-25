package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

const (
	// ETagMaxBufferSize is the largest response body (in bytes) the ETag
	// middleware is willing to buffer in order to hash it. Larger responses
	// are passed through untouched (no ETag) so the middleware never holds
	// unbounded memory for streaming or huge payloads.
	ETagMaxBufferSize = 1 << 20
)

// ETag returns middleware that emits a weak validator (body hash) on
// cacheable GET/HEAD responses and evaluates If-None-Match per RFC 7232,
// answering matching requests with an empty 304 instead of the body.
//
// Behaviour (issue sorolens#152):
//   - The ETag is a SHA-256 prefix of the response entity (the uncompressed
//     body, so it is stable across content-coding negotiation) wrapped in
//     the weak validator form W/"<hex>"; hashing the entity rather than
//     trusting a row timestamp keeps the middleware handler-agnostic.
//   - Repeated GET with an If-None-Match that weak-matches the current
//     entity receives 304 Not Modified with the ETag header and no body,
//     so the payload never crosses the wire.
//   - Only 200 responses on non-stream paths are validated; everything
//     else (errors, streams, bodies above ETagMaxBufferSize) passes
//     through untouched.
//
// Place this middleware inside Compression (registered after it) so the
// hash covers the uncompressed entity.
func ETag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		rec := &etagRecorder{
			ResponseWriter: w,
			ifNoneMatch:    r.Header.Get("If-None-Match"),
			path:           r.URL.Path,
		}
		next.ServeHTTP(rec, r)
		rec.finish()
	})
}

// etagExcludedPaths lists path suffixes that must never be validated:
// event streams are incremental and their body is not a stable entity.
// Kept local to this file (rather than shared with the compression
// middleware) so this change is independently mergeable.
var etagExcludedPaths = []string{
	"/stream",
	"/stream/events",
	"/events/stream",
	"/ws",
}

// etagExcludedPath reports whether the request path is an event stream or
// WebSocket endpoint that must never be tagged or validated.
func etagExcludedPath(path string) bool {
	for _, suffix := range etagExcludedPaths {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

// etagRecorder buffers the response head and up to ETagMaxBufferSize bytes
// of the body so it can hash the entity and evaluate the conditional
// request before anything reaches the client.
type etagRecorder struct {
	http.ResponseWriter
	ifNoneMatch string
	path        string

	headWritten bool
	status      int
	overflow    bool
	committed   bool
	buf         bytes.Buffer
}

// cacheable reports whether this response participates in conditional
// requests: a 200 on a GET/HEAD endpoint that is not an event stream.
func (e *etagRecorder) cacheable() bool {
	return e.status == http.StatusOK && !etagExcludedPath(e.path)
}

// WriteHeader records the status; the head is forwarded only once the
// middleware has committed (or overflowed).
func (e *etagRecorder) WriteHeader(status int) {
	if e.headWritten || e.status != 0 {
		return
	}
	e.status = status
}

// Write buffers the body for hashing. Bodies beyond ETagMaxBufferSize
// abandon the buffering attempt: the head and everything buffered so far
// are flushed and the remainder passes straight through.
func (e *etagRecorder) Write(p []byte) (int, error) {
	if e.status == 0 && !e.headWritten {
		e.status = http.StatusOK
	}
	if e.committed {
		return e.ResponseWriter.Write(p)
	}
	if e.overflow {
		e.writeHead(e.status)
		return e.ResponseWriter.Write(p)
	}
	if e.buf.Len()+len(p) > ETagMaxBufferSize {
		e.overflow = true
		e.committed = true
		e.writeHead(e.status)
		n, err := e.ResponseWriter.Write(e.buf.Bytes())
		e.buf = bytes.Buffer{}
		if err == nil {
			var extra int
			extra, err = e.ResponseWriter.Write(p)
			n += extra
		}
		return n, err
	}
	e.buf.Write(p)
	return len(p), nil
}

// finish commits the response: a 304 for a matching If-None-Match on a
// cacheable response, otherwise the buffered head and body with an ETag
// header attached when one was computed.
func (e *etagRecorder) finish() {
	if e.committed {
		return
	}
	e.committed = true

	var etag string
	if e.cacheable() && !e.overflow {
		etag = computeETag(e.buf.Bytes())
	}

	if etag != "" && ifNoneMatchMatches(e.ifNoneMatch, etag) {
		// RFC 7232 §4.1: 304 MUST NOT carry a body; Content-Length of the
		// withheld representation is dropped and the ETag is retained so
		// the client can keep using its validator.
		e.Header().Del("Content-Length")
		e.Header().Set("ETag", etag)
		e.writeHead(http.StatusNotModified)
		e.buf = bytes.Buffer{}
		return
	}

	if etag != "" {
		e.Header().Set("ETag", etag)
	}
	e.writeHead(e.status)
	_, _ = e.ResponseWriter.Write(e.buf.Bytes())
	e.buf = bytes.Buffer{}
}

// writeHead forwards the given status exactly once.
func (e *etagRecorder) writeHead(status int) {
	if e.headWritten {
		return
	}
	e.headWritten = true
	e.ResponseWriter.WriteHeader(status)
}

// computeETag derives a weak validator from the response entity: a 128-bit
// SHA-256 prefix, hex encoded, wrapped in the weak-validator form. Weak is
// the honest choice for a body hash: byte equality of one representation
// does not guarantee identity across content codings.
func computeETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `W/"` + hex.EncodeToString(sum[:16]) + `"`
}

// ifNoneMatchMatches evaluates an If-None-Match header value against etag
// using the weak comparison defined by RFC 7232 §2.3.2 (the W/ prefix and
// one instance of quotes are ignored on both sides). "*" matches any
// current representation, i.e. any 200 response.
func ifNoneMatchMatches(ifNoneMatch, etag string) bool {
	header := strings.TrimSpace(ifNoneMatch)
	if header == "" {
		return false
	}
	if header == "*" {
		return true
	}
	for _, candidate := range strings.Split(header, ",") {
		if etagEqualWeak(strings.TrimSpace(candidate), etag) {
			return true
		}
	}
	return false
}

// etagEqualWeak reports whether two entity-tag values match under weak
// comparison: an optional W/ prefix and surrounding quotes are stripped
// before comparing the opaque parts.
func etagEqualWeak(a, b string) bool {
	return normalizeETag(a) != "" && normalizeETag(a) == normalizeETag(b)
}

// normalizeETag strips the weakness flag and quoting from an entity-tag.
func normalizeETag(etag string) string {
	etag = strings.TrimSpace(etag)
	if value, found := strings.CutPrefix(etag, "W/"); found {
		etag = value
	}
	return strings.Trim(etag, `"`)
}
