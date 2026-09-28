package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func ruleItoa(n int64) string { return strconv.FormatInt(n, 10) }

// doRuleRequest issues a JSON request as the contributor user.
func doRuleRequest(t *testing.T, srv http.Handler, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", contributorUser)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	out := map[string]any{}
	if w.Body.Len() > 0 {
		_ = json.NewDecoder(w.Body).Decode(&out)
	}
	return w.Code, out
}

func TestCreateRuleValidatesAndStores(t *testing.T) {
	ms := seedRoleStore(t)
	srv := newTestHandler(ms, true, true)

	code, body := doRuleRequest(t, srv, http.MethodPost, "/api/v1/rules", map[string]string{
		"name":     "high fees",
		"source":   "fee_per_invocation > 0.5 XLM for 5m",
		"severity": "Warning",
	})
	if code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%v)", code, body)
	}
	if body["source"] != "fee_per_invocation > 0.5 XLM for 5m" {
		t.Fatalf("unexpected source %v", body["source"])
	}
	if body["window"] != "5m0s" && body["window"] != "5m" {
		t.Fatalf("window = %v", body["window"])
	}

	rules, err := ms.ListAlertRules(t.Context())
	if err != nil || len(rules) != 1 {
		t.Fatalf("stored rules = %d, err %v", len(rules), err)
	}
	if rules[0].WindowSecs != 300 {
		t.Fatalf("WindowSecs = %d, want 300", rules[0].WindowSecs)
	}
}

func TestCreateRuleRejectsInvalidSource(t *testing.T) {
	srv := newTestHandler(seedRoleStore(t), true, true)

	code, body := doRuleRequest(t, srv, http.MethodPost, "/api/v1/rules", map[string]string{
		"name":   "broken",
		"source": "eror_rate > 0.1",
	})
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d (%v)", code, body)
	}
	errs, ok := body["errors"].([]any)
	if !ok || len(errs) == 0 {
		t.Fatalf("expected structured errors, got %v", body)
	}
	first := errs[0].(map[string]any)
	if first["line"] != float64(1) {
		t.Fatalf("expected line 1, got %v", first["line"])
	}
}

func TestCreateRuleRequiresContributor(t *testing.T) {
	srv := newTestHandler(seedRoleStore(t), true, true)

	body, _ := json.Marshal(map[string]string{"name": "x", "source": "error_rate > 0.1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden && w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous write should be rejected, got %d", w.Code)
	}
}

func TestListAndDeleteRules(t *testing.T) {
	ms := seedRoleStore(t)
	srv := newTestHandler(ms, true, true)

	doRuleRequest(t, srv, http.MethodPost, "/api/v1/rules", map[string]string{
		"name": "a", "source": "error_rate > 0.1",
	})
	doRuleRequest(t, srv, http.MethodPost, "/api/v1/rules", map[string]string{
		"name": "b", "source": "error_rate > 0.2",
	})

	code, body := doRuleRequest(t, srv, http.MethodGet, "/api/v1/rules", nil)
	if code != http.StatusOK {
		t.Fatalf("list: %d", code)
	}
	if rules, ok := body["rules"].([]any); !ok || len(rules) != 2 {
		t.Fatalf("want 2 rules, got %v", body["rules"])
	}

	first := body["rules"].([]any)[0].(map[string]any)
	id := int64(first["id"].(float64))
	code, _ = doRuleRequest(t, srv, http.MethodDelete, "/api/v1/rules/"+ruleItoa(id), nil)
	if code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
	code, _ = doRuleRequest(t, srv, http.MethodDelete, "/api/v1/rules/"+ruleItoa(id), nil)
	if code != http.StatusNotFound {
		t.Fatalf("second delete should 404, got %d", code)
	}
}

func TestSetRuleEnabled(t *testing.T) {
	ms := seedRoleStore(t)
	srv := newTestHandler(ms, true, true)
	_, body := doRuleRequest(t, srv, http.MethodPost, "/api/v1/rules", map[string]string{
		"name": "a", "source": "error_rate > 0.1",
	})
	id := int64(body["id"].(float64))

	code, out := doRuleRequest(t, srv, http.MethodPatch, "/api/v1/rules/"+ruleItoa(id), map[string]bool{"enabled": false})
	if code != http.StatusOK {
		t.Fatalf("patch: %d (%v)", code, out)
	}
	if out["enabled"] != false {
		t.Fatalf("enabled = %v, want false", out["enabled"])
	}
}

func TestValidateRuleEndpoint(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)

	code, body := doRuleRequest(t, srv, http.MethodPost, "/api/v1/rules/validate", map[string]string{
		"source": "avg(cpu_insn_per_invocation) > 5000000 instructions for 30m",
	})
	if code != http.StatusOK {
		t.Fatalf("code %d", code)
	}
	if body["valid"] != true {
		t.Fatalf("want valid, got %v", body)
	}
	if body["normalized"] != "avg(cpu_insn_per_invocation) > 5000000 instructions for 30m" {
		t.Fatalf("normalized = %v", body["normalized"])
	}
}

func TestPreviewRuleFires(t *testing.T) {
	ms := seedRoleStore(t)
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		ms.BatchInsertInvocations(t.Context(), []store.Invocation{{
			TxHash:     "tx" + ruleItoa(int64(i)),
			ContractID: "CABC",
			Network:    "testnet",
			Ledger:     uint32(100 + i),
			// Include the current minute so the latest bucket has data.
			LedgerClosedAt: now.Add(-time.Duration(i) * time.Minute),
			Status:         "SUCCESS",
		}})
	}
	srv := newTestHandler(ms, true, true)

	code, body := doRuleRequest(t, srv, http.MethodPost, "/api/v1/rules/preview", map[string]string{
		"source":      "invocations > 0",
		"contract_id": "CABC",
	})
	if code != http.StatusOK {
		t.Fatalf("preview: %d (%v)", code, body)
	}
	if body["valid"] != true {
		t.Fatalf("valid = %v (%v)", body["valid"], body["errors"])
	}
	if body["fired"] != true {
		t.Fatalf("expected firing preview, got %v (%v)", body["fired"], body["reason"])
	}
}

func TestPreviewRuleNeedsContract(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	code, body := doRuleRequest(t, srv, http.MethodPost, "/api/v1/rules/preview", map[string]string{
		"source": "error_rate > 0.1",
	})
	if code != http.StatusOK {
		t.Fatalf("code %d", code)
	}
	if body["valid"] != true {
		t.Fatalf("valid = %v", body["valid"])
	}
	if body["errors"] == nil {
		t.Fatal("expected a diagnostic asking for a contract")
	}
}

func TestListRuleMetricsAndLibrary(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)

	code, body := doRuleRequest(t, srv, http.MethodGet, "/api/v1/rules/metrics", nil)
	if code != http.StatusOK || body["metrics"] == nil || body["aggregations"] == nil {
		t.Fatalf("metrics: %d %v", code, body)
	}
	code, body = doRuleRequest(t, srv, http.MethodGet, "/api/v1/rules/library", nil)
	if code != http.StatusOK || body["rules"] == nil {
		t.Fatalf("library: %d %v", code, body)
	}
	if rules := body["rules"].([]any); len(rules) == 0 {
		t.Fatal("library should not be empty")
	}
}
