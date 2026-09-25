package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// etagServe runs a handler behind ETag with the given method and
// If-None-Match header, returning the recorded response.
func etagServe(t *testing.T, method, ifNoneMatch string, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/stats/global", nil)
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	rec := httptest.NewRecorder()
	ETag(handler).ServeHTTP(rec, req)
	return rec
}

// etagJSONHandler returns a deterministic JSON body handler.
func etagJSONHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func TestETagEmittedOnGet(t *testing.T) {
	t.Parallel()

	rec := etagServe(t, http.MethodGet, "", etagJSONHandler(`{"total":1}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("ETag header missing on 200 GET")
	}
	if !strings.HasPrefix(etag, `W/"`) || !strings.HasSuffix(etag, `"`) {
		t.Fatalf("ETag = %q, want weak validator form W/\"...\"", etag)
	}
	if rec.Body.String() != `{"total":1}` {
		t.Fatalf("body should pass through: %q", rec.Body.String())
	}
}

func TestETagStableAndBodySensitive(t *testing.T) {
	t.Parallel()

	h1 := etagJSONHandler(`{"total":1}`)
	h2 := etagJSONHandler(`{"total":2}`)

	e1a := etagServe(t, http.MethodGet, "", h1).Header().Get("ETag")
	e1b := etagServe(t, http.MethodGet, "", h1).Header().Get("ETag")
	e2 := etagServe(t, http.MethodGet, "", h2).Header().Get("ETag")

	if e1a == "" || e1a != e1b {
		t.Fatalf("ETag should be deterministic for the same body: %q vs %q", e1a, e1b)
	}
	if e1a == e2 {
		t.Fatalf("ETag should differ when the body changes: %q vs %q", e1a, e2)
	}
}

func TestETagIfNoneMatchHitReturns304(t *testing.T) {
	t.Parallel()

	body := `{"total":42}`
	first := etagServe(t, http.MethodGet, "", etagJSONHandler(body))
	etag := first.Header().Get("ETag")

	second := etagServe(t, http.MethodGet, etag, etagJSONHandler(body))
	if second.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d (acceptance criterion 1)", second.Code, http.StatusNotModified)
	}
	if second.Body.Len() != 0 {
		t.Fatalf("304 must not carry a body, got %q", second.Body.String())
	}
	if second.Header().Get("ETag") != etag {
		t.Fatalf("304 should retain the ETag header")
	}
}

func TestETagIfNoneMatchMissReturns200(t *testing.T) {
	t.Parallel()

	body := `{"total":42}`
	first := etagServe(t, http.MethodGet, "", etagJSONHandler(body))
	etag := first.Header().Get("ETag")

	// A validator for a different entity must not match.
	stale := `W/"00000000000000000000000000000000"`
	if stale == etag {
		t.Fatalf("test fixture stale etag accidentally equals real etag")
	}
	second := etagServe(t, http.MethodGet, stale, etagJSONHandler(body))
	if second.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (acceptance criterion 2)", second.Code, http.StatusOK)
	}
	if second.Body.String() != body {
		t.Fatalf("miss should return the full body")
	}
	if second.Header().Get("ETag") != etag {
		t.Fatalf("200 should carry the current ETag")
	}
}

func TestETagWeakComparisonIgnoresWPrefix(t *testing.T) {
	t.Parallel()

	body := `{"total":7}`
	etag := etagServe(t, http.MethodGet, "", etagJSONHandler(body)).Header().Get("ETag")
	strongForm := strings.TrimPrefix(etag, "W/")

	rec := etagServe(t, http.MethodGet, strongForm, etagJSONHandler(body))
	if rec.Code != http.StatusNotModified {
		t.Fatalf("weak comparison should ignore the W/ prefix, status = %d", rec.Code)
	}
}

func TestETagWildcardMatchesAnyRepresentation(t *testing.T) {
	t.Parallel()

	rec := etagServe(t, http.MethodGet, "*", etagJSONHandler(`{"ok":true}`))
	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304 for If-None-Match: *", rec.Code)
	}
}

func TestETagListMatchesAnyEntry(t *testing.T) {
	t.Parallel()

	body := `{"total":7}`
	etag := etagServe(t, http.MethodGet, "", etagJSONHandler(body)).Header().Get("ETag")
	list := `W/"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ` + etag

	rec := etagServe(t, http.MethodGet, list, etagJSONHandler(body))
	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304 when any list entry matches", rec.Code)
	}
}

func TestETagOnlyOnGETAndHEAD(t *testing.T) {
	t.Parallel()

	// POST responses are never validated.
	rec := etagServe(t, http.MethodPost, "*", etagJSONHandler(`{"created":true}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST should bypass conditional handling, status = %d", rec.Code)
	}
	if rec.Header().Get("ETag") != "" {
		t.Fatalf("POST should not receive an ETag")
	}

	// HEAD participates in validation like GET.
	head := etagServe(t, http.MethodHead, "*", etagJSONHandler(`{"ok":true}`))
	if head.Code != http.StatusNotModified {
		t.Fatalf("HEAD conditional should 304, status = %d", head.Code)
	}
}

func TestETagNotAppliedToErrors(t *testing.T) {
	t.Parallel()

	rec := etagServe(t, http.MethodGet, "", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	})
	if rec.Header().Get("ETag") != "" {
		t.Fatalf("error responses should not be tagged")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestETagNotAppliedToStreamPaths(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/C1/stream", nil)
	rec := httptest.NewRecorder()
	ETag(etagJSONHandler(`event: update`)).ServeHTTP(rec, req)
	if rec.Header().Get("ETag") != "" {
		t.Fatalf("stream endpoints should not be tagged")
	}
}

// TestETagStreamPathsAreNotBuffered guards SSE behind ETag: the handler
// must see an http.Flusher, and bytes must reach the client before the
// handler returns rather than being held for hashing.
func TestETagStreamPathsAreNotBuffered(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/events", nil)
	rec := httptest.NewRecorder()
	ETag(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("stream handler behind ETag did not receive an http.Flusher")
		}
		_, _ = w.Write([]byte("data: first\n\n"))
		f.Flush()
		if !rec.Flushed || rec.Body.String() != "data: first\n\n" {
			t.Errorf("frame was buffered: flushed=%v body=%q", rec.Flushed, rec.Body.String())
		}
	})).ServeHTTP(rec, req)
}

func TestETagOverflowPassesThrough(t *testing.T) {
	t.Parallel()

	big := strings.Repeat("x", ETagMaxBufferSize+1024)
	rec := etagServe(t, http.MethodGet, "*", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(big))
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.Len() != len(big) {
		t.Fatalf("overflow body should pass through intact: got %d bytes, want %d", rec.Body.Len(), len(big))
	}
	if rec.Header().Get("ETag") != "" {
		t.Fatalf("overflow responses should not be tagged")
	}
}

func TestETagEmptyBodyStillTagged(t *testing.T) {
	t.Parallel()

	rec := etagServe(t, http.MethodGet, "*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// An empty 200 hashes the empty entity; "*" matches any current
	// representation, so this is a 304.
	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304 (empty entity still validates)", rec.Code)
	}
}

func TestIfNoneMatchMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		etag   string
		want   bool
	}{
		{"empty header", "", `W/"aa"`, false},
		{"exact weak match", `W/"aa"`, `W/"aa"`, true},
		{"strong client vs weak server", `"aa"`, `W/"aa"`, true},
		{"weak client vs strong server", `W/"aa"`, `"aa"`, true},
		{"different opaque part", `W/"bb"`, `W/"aa"`, false},
		{"wildcard", "*", `W/"aa"`, true},
		{"list hit", `W/"bb", W/"aa"`, `W/"aa"`, true},
		{"list miss", `W/"bb", W/"cc"`, `W/"aa"`, false},
		{"spaces tolerated", ` W/"aa" `, `W/"aa"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ifNoneMatchMatches(tt.header, tt.etag); got != tt.want {
				t.Fatalf("ifNoneMatchMatches(%q, %q) = %v, want %v", tt.header, tt.etag, got, tt.want)
			}
		})
	}
}
