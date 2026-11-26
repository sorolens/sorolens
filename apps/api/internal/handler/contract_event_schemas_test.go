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

func TestGetContractEventSchemas(t *testing.T) {
	ms := store.NewMockStore()
	id := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"

	// 1. Unknown contract returns 404
	srv := newTestHandler(ms, true, true)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+id+"/event-schemas", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown contract, got %d", rr.Code)
	}

	// 2. Track contract
	_ = ms.UpsertContract(context.Background(), store.Contract{
		ID:      id,
		Network: "testnet",
		Status:  "active",
	})

	// 3. Insert inferred event schema
	_ = ms.UpsertContractEventSchema(context.Background(), store.ContractEventSchema{
		ContractID:      id,
		EventName:       "swap",
		TopicArity:      3,
		TopicTypes:      []string{"string", "string", "string"},
		ValueShape:      "number",
		ObservedShapes:  []store.ObservedShape{{TopicArity: 3, TopicTypes: []string{"string", "string", "string"}, ValueType: "number", Count: 5}},
		FirstSeenLedger: 100,
		LastSeenLedger:  150,
		UpdatedAt:       time.Now(),
	})

	// 4. Seed declared SEP-48 spec (complements/takes precedence)
	tree := json.RawMessage(`{"functions":[{"name":"swap"}]}`)
	_ = ms.UpsertContractSpec(context.Background(), store.ContractSpec{
		ContractID: id,
		Spec:       tree,
	})

	rr2 := httptest.NewRecorder()
	srv.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+id+"/event-schemas", nil))
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}

	body := rr2.Body.String()
	if !strings.Contains(body, `"event_name":"swap"`) || !strings.Contains(body, `"declared_spec":`) {
		t.Errorf("missing expected schema fields in response: %s", body)
	}
}
