package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestGetContractSpecReturnsParsedTree(t *testing.T) {
	ms := store.NewMockStore()
	id := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"

	tree := json.RawMessage(`{"functions":[{"name":"transfer","inputs":[{"name":"to","type":{"kind":"address"}}],"outputs":[{"kind":"void"}]}]}`)
	if err := ms.UpsertContractSpec(context.Background(), store.ContractSpec{
		ContractID: id,
		Spec:       tree,
		WasmHash:   "a1b2c3d4",
		ParsedAt:   time.Date(2026, 7, 1, 10, 6, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("seed spec: %v", err)
	}

	srv := newTestHandler(ms, true, true)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+id+"/spec", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	for _, want := range []string{
		`"contract_id":"` + id + `"`,
		`"wasm_hash":"a1b2c3d4"`,
		`"spec":`,
		`"kind":"address"`,
		`"name":"transfer"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("response missing %s; body: %s", want, body)
		}
	}
}

// TestGetContractSpecNotFound covers both 404 flavours: an unknown contract,
// and a tracked contract whose spec has not been parsed yet. The messages must
// differ so an operator can tell the two apart.
func TestGetContractSpecNotFound(t *testing.T) {
	const unknown = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	const tracked = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM"

	ms := store.NewMockStore()
	if err := ms.UpsertContract(context.Background(), store.Contract{
		ID:      tracked,
		Network: "testnet",
	}); err != nil {
		t.Fatalf("seed contract: %v", err)
	}

	srv := newTestHandler(ms, true, true)

	t.Run("unknown contract", func(t *testing.T) {
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+unknown+"/spec", nil))

		if rr.Code != http.StatusNotFound {
			t.Fatalf("got status %d, want 404", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "contract not found") {
			t.Errorf("body = %s, want a %q message", rr.Body.String(), "contract not found")
		}
	})

	t.Run("tracked but unparsed", func(t *testing.T) {
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+tracked+"/spec", nil))

		if rr.Code != http.StatusNotFound {
			t.Fatalf("got status %d, want 404", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "no parsed interface spec") {
			t.Errorf("body = %s, want a %q message", rr.Body.String(), "no parsed interface spec")
		}
	})
}
