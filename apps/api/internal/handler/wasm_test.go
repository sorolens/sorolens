package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestGetContractWasm(t *testing.T) {
	ms := store.NewMockStore()
	code := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00} // Wasm magic
	hash := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	if err := ms.UpsertContract(nil, store.Contract{
		ID: "CONTRACT_A", Network: "testnet", Status: "active", WasmHash: hash,
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.UpsertContractWasm(nil, hash, code); err != nil {
		t.Fatal(err)
	}

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/CONTRACT_A/wasm", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/wasm" {
		t.Errorf("Content-Type = %q, want application/wasm", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, want immutable long-cache", cc)
	}
	if got := w.Body.Bytes(); string(got) != string(code) {
		t.Errorf("body mismatch: got %x want %x", got, code)
	}
	if wh := w.Header().Get("X-Wasm-Hash"); wh != hash {
		t.Errorf("X-Wasm-Hash = %q, want %q", wh, hash)
	}
}

func TestGetContractWasm_NotFetchedYet(t *testing.T) {
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{
		ID: "CONTRACT_A", Network: "testnet", Status: "active",
		WasmHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}); err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/CONTRACT_A/wasm", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
	var env map[string]any
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error envelope: %v", env)
	}
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("want code NOT_FOUND, got %v", errObj["code"])
	}
}

func TestGetContractWasm_ContractNotFound(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/MISSING/wasm", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
}
