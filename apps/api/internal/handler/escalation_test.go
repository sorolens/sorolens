package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestListEscalationPoliciesDocumentedShape(t *testing.T) {
	ms := store.NewMockStore()
	if _, err := ms.CreateEscalationPolicy(nil, store.EscalationPolicy{Name: "oncall"}); err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/escalation-policies", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["escalation_policies"]; ok {
		t.Error("response still uses undocumented key escalation_policies")
	}
	if _, ok := body["next_cursor"]; !ok {
		t.Error("response missing next_cursor")
	}
	var policies []map[string]any
	if err := json.Unmarshal(body["policies"], &policies); err != nil {
		t.Fatalf("policies missing or malformed: %v", err)
	}
	if len(policies) != 1 || policies[0]["name"] != "oncall" {
		t.Errorf("policies = %v, want one named oncall", policies)
	}
}
