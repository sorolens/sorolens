package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testBodyLimit = 1 << 20 // 1 MiB

func TestBodyLimitRejectsOversizedContentLength(t *testing.T) {
	t.Parallel()

	body := bytes.Repeat([]byte("a"), 2<<20) // 2 MiB

	nextHit := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHit = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	BodyLimit(testBodyLimit)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if nextHit {
		t.Error("next handler was called for an oversized request")
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("response Content-Type = %q, want application/json", got)
	}

	var payload struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if payload.Error.Code != "PAYLOAD_TOO_LARGE" {
		t.Errorf("error.code = %q, want PAYLOAD_TOO_LARGE", payload.Error.Code)
	}
	if payload.Error.Message == "" {
		t.Error("error.message is empty")
	}
}

func TestBodyLimitAllowsRequestsUpToTheLimit(t *testing.T) {
	t.Parallel()

	body := bytes.Repeat([]byte("a"), testBodyLimit)

	readLen := -1
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("handler reading body within limit: %v", err)
		}
		readLen = len(b)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	BodyLimit(testBodyLimit)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if readLen != len(body) {
		t.Errorf("handler read %d bytes, want %d", readLen, len(body))
	}
}

// TestBodyLimitEnforcesLimitWithoutContentLength covers chunked-style requests
// where the length is unknown up front: the read must trip http.MaxBytesReader
// and the middleware must convert a handler that writes nothing into a 413.
func TestBodyLimitEnforcesLimitWithoutContentLength(t *testing.T) {
	t.Parallel()

	body := bytes.Repeat([]byte("a"), 2<<20)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			var maxErr *http.MaxBytesError
			if !errors.As(err, &maxErr) {
				t.Errorf("read error = %v, want *http.MaxBytesError", err)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", bytes.NewReader(body))
	req.ContentLength = -1 // unknown length, as with chunked transfer encoding
	rec := httptest.NewRecorder()

	BodyLimit(testBodyLimit)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestBodyLimitDoesNotOverwriteHandlerResponse(t *testing.T) {
	t.Parallel()

	body := bytes.Repeat([]byte("a"), 2<<20)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", bytes.NewReader(body))
	req.ContentLength = -1
	rec := httptest.NewRecorder()

	BodyLimit(testBodyLimit)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (handler response must be preserved)", rec.Code, http.StatusBadRequest)
	}
}

func TestBodyLimitDisabledForNonPositiveLimit(t *testing.T) {
	t.Parallel()

	body := bytes.Repeat([]byte("a"), 2<<20)

	nextHit := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHit = true
		if _, err := io.ReadAll(r.Body); err != nil {
			t.Errorf("disabled limit must not bound the body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	BodyLimit(0)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !nextHit {
		t.Error("next handler was not called with the limit disabled")
	}
}

func TestBodyLimitLeavesRequestsWithoutBodyAlone(t *testing.T) {
	t.Parallel()

	sameBody := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sameBody = r.Body == http.NoBody
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/events", nil)
	rec := httptest.NewRecorder()

	BodyLimit(testBodyLimit)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !sameBody {
		t.Error("bodyless request had its body replaced")
	}
}
