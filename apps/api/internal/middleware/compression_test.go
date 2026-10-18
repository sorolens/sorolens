package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
)

// repeat builds a body of the given size that compresses well.
func repeat(t *testing.T, size int) []byte {
	t.Helper()
	body := strings.Repeat("sorolens-compression-test-payload;", size/34+1)
	return []byte(body[:size])
}

// serve runs a handler behind Compression with the given Accept-Encoding and
// returns the recorder response.
func serve(t *testing.T, acceptEncoding string, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	rec := httptest.NewRecorder()
	Compression(handler).ServeHTTP(rec, req)
	return rec
}

func TestCompressionNegotiatesGzip(t *testing.T) {
	t.Parallel()

	payload := repeat(t, 8192)
	rec := serve(t, "gzip", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	got, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gzip read: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("decompressed body mismatch")
	}
}

func TestCompressionPrefersBrotliWhenClientRanksItFirst(t *testing.T) {
	t.Parallel()

	payload := repeat(t, 8192)
	rec := serve(t, "gzip, br", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	})

	if got := rec.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding = %q, want br", got)
	}
	br := brotli.NewReader(rec.Body)
	got, err := io.ReadAll(br)
	if err != nil {
		t.Fatalf("brotli read: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("decompressed body mismatch")
	}
}

func TestCompressionGzipWinsWhenRankedFirst(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	req.Header.Set("Accept-Encoding", "gzip;q=1.0, br;q=0.5")
	Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(repeat(t, 8192))
	})).ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip (q=1.0 vs 0.5)", got)
	}
}

func TestCompressionUncompressedClientGetsUncompressedResponse(t *testing.T) {
	t.Parallel()

	payload := repeat(t, 8192)
	rec := serve(t, "", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	})

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
	if rec.Body.String() != string(payload) {
		t.Fatalf("body should be passthrough")
	}
}

func TestCompressionIdentityClientGetsUncompressedResponse(t *testing.T) {
	t.Parallel()

	rec := serve(t, "identity", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(repeat(t, 8192))
	})

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
}

func TestCompressionRejectsZeroQualityEncodings(t *testing.T) {
	t.Parallel()

	rec := serve(t, "gzip;q=0, br;q=0", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(repeat(t, 8192))
	})

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty (q=0 means not acceptable)", got)
	}
}

func TestCompressionSkipsSmallResponses(t *testing.T) {
	t.Parallel()

	payload := repeat(t, 512) // below CompressionMinSize (1 KB)
	rec := serve(t, "gzip, br", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	})

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty for small body", got)
	}
	if rec.Body.String() != string(payload) {
		t.Fatalf("small body should be passthrough")
	}
}

func TestCompressionSkipsAlreadyCompressedContentTypes(t *testing.T) {
	t.Parallel()

	payload := repeat(t, 8192)
	for _, ct := range []string{"image/png", "video/mp4", "application/zip", "application/wasm", "font/woff2", "audio/mpeg", "application/pdf"} {
		rec := serve(t, "gzip, br", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", ct)
			_, _ = w.Write(payload)
		})
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Content-Type %s: Content-Encoding = %q, want empty", ct, got)
		}
		if rec.Body.String() != string(payload) {
			t.Fatalf("Content-Type %s: body should be passthrough", ct)
		}
	}
}

func TestCompressionExcludesEventStreamPaths(t *testing.T) {
	t.Parallel()

	payload := repeat(t, 8192)
	for _, path := range []string{"/api/v1/contracts/C123/stream", "/api/v1/stream/events", "/api/v1/ws"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Accept-Encoding", "gzip, br")
		rec := httptest.NewRecorder()
		Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write(payload)
		})).ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("path %s: Content-Encoding = %q, want empty", path, got)
		}
	}
}

func TestCompressionSetsVaryAcceptEncoding(t *testing.T) {
	t.Parallel()

	for _, acceptEncoding := range []string{"", "gzip", "identity", "gzip;q=0"} {
		rec := serve(t, acceptEncoding, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("ok"))
		})
		if got := rec.Header().Get("Vary"); got != "Accept-Encoding" {
			t.Fatalf("Accept-Encoding=%q: Vary = %q, want Accept-Encoding", acceptEncoding, got)
		}
	}
}

func TestCompressionVaryNotDuplicated(t *testing.T) {
	t.Parallel()

	rec := serve(t, "gzip", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Accept-Encoding")
		_, _ = w.Write([]byte("ok"))
	})
	if got := rec.Header().Values("Vary"); len(got) != 1 {
		t.Fatalf("Vary = %v, want a single Accept-Encoding entry", got)
	}
}

func TestCompressionLargeBodyCompressed(t *testing.T) {
	t.Parallel()

	payload := strings.Repeat(`{"id":"contract","description":"large json payload for compression"}`, 200)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	})).ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if len(rec.Body.Bytes()) >= len(payload) {
		t.Fatalf("compressed size %d should be smaller than raw %d", rec.Body.Len(), len(payload))
	}
}

func TestCompressionHeaderOnlyResponse(t *testing.T) {
	t.Parallel()

	rec := serve(t, "gzip", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
}

func TestCompressionFlushAfterDecision(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(repeat(t, 8192))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})
	Compression(next).ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if rec.Body.Len() == 0 {
		t.Fatalf("flushed body should have been written")
	}
}

func TestNegotiatedEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty header", "", ""},
		{"gzip only", "gzip", "gzip"},
		{"br only", "br", "br"},
		{"br wins tie", "gzip, br", "br"},
		{"br wins tie either order", "br, gzip", "br"},
		{"higher q wins", "br;q=0.9, gzip;q=1.0", "gzip"},
		{"identity only", "identity", ""},
		{"zero q rejected", "gzip;q=0", ""},
		{"wildcard picks gzip", "*", "gzip"},
		{"wildcard below explicit", "br;q=0.5, *", "br"},
		{"case insensitive", "GZip", "gzip"},
		{"spaces tolerated", "  gzip ; q=0.8 , br ; q=0.9 ", "br"},
		{"zero q gzip falls back to br via wildcard", "gzip;q=0, *", "br"},
		{"zero q gzip wildcard picks br", "gzip;q=0, br, *", "br"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := negotiatedEncoding(tt.in); got != tt.want {
				t.Fatalf("negotiatedEncoding(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsCompressedContentType(t *testing.T) {
	t.Parallel()

	compressed := []string{
		"image/png",
		"image/jpeg; charset=binary",
		"video/webm",
		"audio/ogg",
		"application/zip",
		"application/gzip",
		"application/pdf",
		"application/wasm",
		"font/woff2",
	}
	for _, ct := range compressed {
		if !isCompressedContentType(ct) {
			t.Fatalf("isCompressedContentType(%q) = false, want true", ct)
		}
	}

	plain := []string{
		"application/json",
		"application/json; charset=utf-8",
		"text/html; charset=utf-8",
		"text/event-stream",
		"",
	}
	for _, ct := range plain {
		if isCompressedContentType(ct) {
			t.Fatalf("isCompressedContentType(%q) = true, want false", ct)
		}
	}
}

func TestIsExcludedPath(t *testing.T) {
	t.Parallel()

	excluded := []string{
		"/api/v1/contracts/C123/stream",
		"/api/v1/stream/events",
		"/api/v1/ws",
	}
	for _, p := range excluded {
		if !isExcludedPath(p) {
			t.Fatalf("isExcludedPath(%q) = false, want true", p)
		}
	}

	included := []string{
		"/api/v1/contracts",
		"/api/v1/contracts/C123/events",
		"/api/v1/watchdog/stats",
	}
	for _, p := range included {
		if isExcludedPath(p) {
			t.Fatalf("isExcludedPath(%q) = true, want false", p)
		}
	}
}
