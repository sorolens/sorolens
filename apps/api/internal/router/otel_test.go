package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOTelMiddleware(t *testing.T) {
	tp, err := InitTracer("test")
	if err != nil {
		t.Fatalf("InitTracer failed: %v", err)
	}
	if tp != nil {
		defer tp.Shutdown(context.Background())
	}

	handler := OTelMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
