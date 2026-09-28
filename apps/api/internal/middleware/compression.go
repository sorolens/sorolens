package middleware

import (
	"bytes"
	"compress/gzip"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
)

const (
	// CompressionMinSize is the smallest response body (in bytes) that will
	// be compressed. Smaller bodies are passed through uncompressed: the
	// CPU cost and header overhead of compressing them outweighs the
	// bandwidth saving the feature targets (issue sorolens#153).
	CompressionMinSize = 1024

	// compressionVaryValue is the value set on the Vary header so caches
	// never serve a compressed representation to a client that did not ask
	// for one (or vice versa).
	compressionVaryValue = "Accept-Encoding"
)

// compressedContentPrefixes lists content types whose payloads are already
// compressed on the wire. Re-compressing them wastes CPU and can even grow
// the body. Matching is prefix-based so parameters such as "; charset=utf-8"
// do not affect the decision.
var compressedContentPrefixes = []string{
	"image/",
	"video/",
	"audio/",
	"application/zip",
	"application/gzip",
	"application/x-gzip",
	"application/pdf",
	"application/wasm",
	"application/octet-stream",
	"font/",
}

// excludedPathSuffixes lists path suffixes that must never be compressed:
// event streams are flushed incrementally and need to be observable by
// intermediaries as they are produced.
var excludedPathSuffixes = []string{
	"/stream",
	"/stream/events",
	"/events/stream",
	"/ws",
}

// Compression returns middleware that compresses responses with brotli or
// gzip when the client advertises support via Accept-Encoding.
//
// Behaviour (issue sorolens#153):
//   - Content-Encoding: br is preferred when offered at equal quality
//     (better ratio), gzip otherwise; q-values decide when they differ.
//   - Bodies smaller than CompressionMinSize bytes are passed through
//     uncompressed (the head and first bytes are buffered to decide).
//   - Responses whose Content-Type is already a compressed format (images,
//     video, archives, wasm, ...) are never re-compressed.
//   - Server-sent event and WebSocket endpoints are excluded so incremental
//     flushing and upgrade semantics are preserved.
//   - Vary: Accept-Encoding is always added so shared caches key correctly.
//
// The request is never modified; Accept-Encoding handling is read-only.
func Compression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ensure caches never conflate compressed and uncompressed variants,
		// including on responses this middleware ultimately passes through.
		addVaryAcceptEncoding(w.Header())

		encoding := negotiatedEncoding(r.Header.Get("Accept-Encoding"))
		if encoding == "" || isExcludedPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		rec := &compressionRecorder{
			ResponseWriter: w,
			encoding:       encoding,
		}
		next.ServeHTTP(rec, r)
		rec.finish()
	})
}

// compressionRecorder buffers the response head (status + headers) and the
// first bytes of the body, then decides whether to compress based on the
// declared Content-Type and the observed body size. Until the decision is
// committed nothing is written to the client, so headers can still be
// adjusted and a small or incompressible response can fall back to a plain
// pass-through.
type compressionRecorder struct {
	http.ResponseWriter
	encoding string

	headWritten bool
	status      int
	contentType string
	skip        bool
	decided     bool

	gzW *gzip.Writer
	brW *brotli.Writer
	buf bytes.Buffer
}

// WriteHeader records the status and content type, then feeds the decision
// logic. The head itself is only forwarded once the decision is committed.
func (c *compressionRecorder) WriteHeader(status int) {
	if c.headWritten || c.status != 0 {
		return
	}
	c.status = status
	c.contentType = headerMediaType(c.Header().Get("Content-Type"))
	if c.contentType == "" || isCompressedContentType(c.contentType) {
		// No compressible body declared, or the payload is already a
		// compressed format: pass through untouched.
		c.skip = true
	}
	c.decide(nil)
}

// Write implements http.ResponseWriter. The first Write also commits the
// response head if the handler skipped WriteHeader.
func (c *compressionRecorder) Write(p []byte) (int, error) {
	if c.status == 0 && !c.headWritten {
		c.status = http.StatusOK
		c.contentType = headerMediaType(c.Header().Get("Content-Type"))
		if c.contentType == "" || isCompressedContentType(c.contentType) {
			c.skip = true
		}
	}
	if c.decided {
		if c.skip {
			return c.ResponseWriter.Write(p)
		}
		return c.compressedWrite(p)
	}
	return c.decide(p)
}

// decide accumulates p (when non-nil) and commits the compression decision
// once the minimum-size threshold is reached, or immediately when the
// response is known to be incompressible. Responses still below the
// threshold stay buffered; a nil p with a small buffer (WriteHeader-only
// responses) therefore waits for finish().
func (c *compressionRecorder) decide(p []byte) (int, error) {
	if p != nil {
		c.buf.Write(p)
	}
	if c.skip || c.buf.Len() >= CompressionMinSize {
		return c.commitBuffered()
	}
	return len(p), nil
}

// commitBuffered writes the buffered head and body. When the response is
// compressible the body goes through the selected encoder.
func (c *compressionRecorder) commitBuffered() (int, error) {
	if c.decided {
		return 0, nil
	}
	c.decided = true
	payload := c.buf.Bytes()
	c.buf = bytes.Buffer{}

	c.writeHead()
	if c.skip {
		return c.ResponseWriter.Write(payload)
	}

	c.Header().Set("Content-Encoding", c.encoding)
	c.Header().Del("Content-Length")
	if c.encoding == "br" {
		c.brW = brotli.NewWriter(c.ResponseWriter)
	} else {
		c.gzW = gzip.NewWriter(c.ResponseWriter)
	}
	return c.compressedWrite(payload)
}

// writeHead forwards the recorded status exactly once.
func (c *compressionRecorder) writeHead() {
	if c.headWritten {
		return
	}
	c.headWritten = true
	c.ResponseWriter.WriteHeader(c.statusOrOK())
}

func (c *compressionRecorder) statusOrOK() int {
	if c.status == 0 {
		return http.StatusOK
	}
	return c.status
}

func (c *compressionRecorder) compressedWrite(p []byte) (int, error) {
	if c.brW != nil {
		return c.brW.Write(p)
	}
	if c.gzW != nil {
		return c.gzW.Write(p)
	}
	return c.ResponseWriter.Write(p)
}

// finish is called once the wrapped handler has returned. It closes the
// active encoder (flushing trailer bytes) for compressed responses and
// flushes any response that never crossed the size threshold (header-only
// responses, small bodies) through uncompressed, so no response is lost.
func (c *compressionRecorder) finish() {
	if c.decided {
		if !c.skip {
			if c.brW != nil {
				_ = c.brW.Close()
			}
			if c.gzW != nil {
				_ = c.gzW.Close()
			}
		}
		return
	}
	c.decided = true
	payload := c.buf.Bytes()
	c.buf = bytes.Buffer{}
	c.writeHead()
	_, _ = c.ResponseWriter.Write(payload)
}

// Flush implements http.Flusher. Before the compression decision it forwards
// to the underlying writer (no-op there if it does not flush); after it, the
// active encoder is flushed so streaming endpoints still make progress.
func (c *compressionRecorder) Flush() {
	if c.decided && !c.skip {
		if c.brW != nil {
			_ = c.brW.Flush()
		}
		if c.gzW != nil {
			_ = c.gzW.Flush()
		}
	}
	if flusher, ok := c.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// negotiatedEncoding picks the response encoding from an Accept-Encoding
// header. Brotli and gzip are the only encodings offered; when both are
// acceptable the client's q-value order decides, with brotli winning ties.
// A coding listed with q=0 is explicitly not acceptable and can neither win
// nor be selected via the wildcard; when neither offered coding is
// acceptable the result is "" (identity).
func negotiatedEncoding(acceptEncoding string) string {
	best := ""
	bestQ := 0.0
	rejected := map[string]bool{}
	wildcardQ := -1.0
	for _, part := range strings.Split(acceptEncoding, ",") {
		coding, q := parseEncodingPart(part)
		switch coding {
		case "br", "gzip":
			if q <= 0 {
				rejected[coding] = true
				continue
			}
			// Higher q wins; on equal q brotli wins regardless of listing
			// order (better ratio at similar CPU cost).
			if q > bestQ || (q == bestQ && coding == "br" && best == "gzip") {
				best = coding
				bestQ = q
			}
		case "*":
			if q > wildcardQ {
				wildcardQ = q
			}
		}
	}
	if best != "" {
		return best
	}
	if wildcardQ > 0 {
		// The wildcard accepts any coding; prefer gzip as the safe choice,
		// falling back to brotli when gzip is explicitly refused.
		if !rejected["gzip"] {
			return "gzip"
		}
		if !rejected["br"] {
			return "br"
		}
	}
	return ""
}

// parseEncodingPart parses one comma-separated element of Accept-Encoding
// into its coding and quality (defaulting to 1, clamped to [0, 1]).
func parseEncodingPart(part string) (string, float64) {
	fields := strings.Split(strings.TrimSpace(part), ";")
	coding := strings.ToLower(strings.TrimSpace(fields[0]))
	q := 1.0
	for _, attr := range fields[1:] {
		attr = strings.TrimSpace(attr)
		if value, found := strings.CutPrefix(attr, "q="); found {
			if parsed, err := strconv.ParseFloat(value, 64); err == nil {
				q = parsed
			}
			break
		}
	}
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	return coding, q
}

// headerMediaType extracts the bare media type (lowercased, parameters
// stripped) from a Content-Type header value. Returns "" when absent or
// unparseable.
func headerMediaType(contentType string) string {
	if contentType == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return strings.ToLower(mediaType)
}

// isCompressedContentType reports whether ct is a media type whose payload is
// already compressed, so an extra Content-Encoding would be wasteful.
func isCompressedContentType(ct string) bool {
	for _, prefix := range compressedContentPrefixes {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}
	return false
}

// isExcludedPath reports whether the request path must never be compressed
// (server-sent event streams and WebSocket endpoints).
func isExcludedPath(path string) bool {
	for _, suffix := range excludedPathSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

// addVaryAcceptEncoding appends Accept-Encoding to the Vary header without
// duplicating an entry that is already present.
func addVaryAcceptEncoding(h http.Header) {
	for _, v := range h.Values("Vary") {
		for _, field := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(field), compressionVaryValue) {
				return
			}
		}
	}
	h.Add("Vary", compressionVaryValue)
}
