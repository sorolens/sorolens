package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// errTraceUnavailable stands in for a backing store failure.
var errTraceUnavailable = errors.New("store unavailable")

// txHash builds a valid 64-character lowercase hex transaction hash.
func txHash(c byte) string { return strings.Repeat(string(c), 64) }

type traceNodeBody struct {
	SpanID       string          `json:"span_id"`
	ParentSpanID string          `json:"parent_span_id"`
	ContractID   string          `json:"contract_id"`
	FunctionName string          `json:"function_name"`
	CPU          int64           `json:"cpu"`
	Mem          int64           `json:"mem"`
	FeeShare     int64           `json:"fee_share"`
	Depth        int             `json:"depth"`
	Children     []traceNodeBody `json:"children"`
}

type traceBody struct {
	TxHash    string        `json:"tx_hash"`
	Status    string        `json:"status"`
	Network   string        `json:"network"`
	Ledger    uint32        `json:"ledger"`
	Root      traceNodeBody `json:"root"`
	EdgeCount int           `json:"edge_count"`
	HasEdges  bool          `json:"has_edges"`
	Truncated bool          `json:"truncated"`
}

// seededTraceStore returns a store holding one nested, one sibling and one
// deeper call for a single transaction.
func seededTraceStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	if err := ms.BatchInsertInvocations(nil, []store.Invocation{{
		TxHash:             txHash('a'),
		ContractID:         "CROOTCONTRACT",
		Network:            "testnet",
		Ledger:             1234,
		LedgerClosedAt:     time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Status:             "SUCCESS",
		FunctionName:       "orchestrate",
		ResourceFeeCharged: 1000,
	}}); err != nil {
		t.Fatal(err)
	}
	for _, e := range []store.CallEdge{
		{TxHash: txHash('a'), ParentSpanID: "0", ChildSpanID: "0.0", CalleeContractID: "CMIDDLE", FunctionName: "middle", CPU: 300, Depth: 1},
		{TxHash: txHash('a'), ParentSpanID: "0.0", ChildSpanID: "0.0.0", CalleeContractID: "CLEAF", FunctionName: "leaf_fn", CPU: 50, Depth: 2},
		{TxHash: txHash('a'), ParentSpanID: "0", ChildSpanID: "0.1", CalleeContractID: "CSIBLING", FunctionName: "other", CPU: 100, FeeShare: 250, Depth: 1},
	} {
		ms.AddCallEdge(e)
	}
	return ms
}

func TestGetInvocationTrace_NestedTree(t *testing.T) {
	srv := newTestHandler(seededTraceStore(t), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/invocations/"+txHash('a')+"/trace", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var body traceBody
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if body.TxHash != txHash('a') || body.Status != "SUCCESS" || body.Network != "testnet" || body.Ledger != 1234 {
		t.Errorf("unexpected envelope: %+v", body)
	}
	if body.EdgeCount != 3 || !body.HasEdges || body.Truncated {
		t.Errorf("unexpected edge metadata: %+v", body)
	}

	// The root is backed by the invocations row, not by a call_edges row.
	root := body.Root
	if root.SpanID != "0" || root.ContractID != "CROOTCONTRACT" || root.FunctionName != "orchestrate" {
		t.Errorf("unexpected root node: %+v", root)
	}
	if root.FeeShare != 1000 {
		t.Errorf("root fee share = %d, want the invocation's resource fee 1000", root.FeeShare)
	}
	if len(root.Children) != 2 {
		t.Fatalf("want 2 top-level children, got %d (%+v)", len(root.Children), root.Children)
	}

	// Children arrive in span-id order: 0.0 (with a nested child) then 0.1.
	mid := root.Children[0]
	if mid.SpanID != "0.0" || mid.ParentSpanID != "0" || mid.Depth != 1 || mid.FunctionName != "middle" {
		t.Errorf("unexpected first child: %+v", mid)
	}
	if len(mid.Children) != 1 {
		t.Fatalf("want 1 grandchild, got %d", len(mid.Children))
	}
	leaf := mid.Children[0]
	if leaf.SpanID != "0.0.0" || leaf.ParentSpanID != "0.0" || leaf.Depth != 2 || leaf.ContractID != "CLEAF" {
		t.Errorf("unexpected grandchild: %+v", leaf)
	}
	if leaf.CPU != 50 {
		t.Errorf("grandchild cpu = %d, want 50", leaf.CPU)
	}

	sibling := root.Children[1]
	if sibling.SpanID != "0.1" || sibling.FunctionName != "other" || sibling.FeeShare != 250 {
		t.Errorf("unexpected sibling: %+v", sibling)
	}
}

func TestGetInvocationTrace_NoEdges(t *testing.T) {
	ms := store.NewMockStore()
	if err := ms.BatchInsertInvocations(nil, []store.Invocation{{
		TxHash:       txHash('b'),
		ContractID:   "CONLY",
		FunctionName: "increment",
		Status:       "SUCCESS",
		Ledger:       10,
	}}); err != nil {
		t.Fatal(err)
	}

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/invocations/"+txHash('b')+"/trace", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body traceBody
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.HasEdges || body.EdgeCount != 0 {
		t.Errorf("want no edges, got %+v", body)
	}
	if body.Root.SpanID != "0" || body.Root.ContractID != "CONLY" || len(body.Root.Children) != 0 {
		t.Errorf("want a bare root from the invocations row, got %+v", body.Root)
	}
}

func TestGetInvocationTrace_EdgesWithoutInvocationRow(t *testing.T) {
	// Defensive: a call graph may exist for a transaction whose root row was
	// pruned. The tree is still returned rather than 404ing.
	ms := store.NewMockStore()
	ms.AddCallEdge(store.CallEdge{
		TxHash: txHash('c'), ParentSpanID: "0", ChildSpanID: "0.0",
		CalleeContractID: "CCALLEE", FunctionName: "transfer", Depth: 1,
	})

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/invocations/"+txHash('c')+"/trace", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body traceBody
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.HasEdges || len(body.Root.Children) != 1 {
		t.Fatalf("want the child hung off a synthesised root, got %+v", body)
	}
	if body.Root.Children[0].ContractID != "CCALLEE" {
		t.Errorf("unexpected child: %+v", body.Root.Children[0])
	}
	if body.Status != "" {
		t.Errorf("status must be empty without an invocation row, got %q", body.Status)
	}
}

func TestGetInvocationTrace_AcceptsUppercaseHash(t *testing.T) {
	srv := newTestHandler(seededTraceStore(t), true, true)
	upper := strings.ToUpper(txHash('a'))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/invocations/"+upper+"/trace", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body traceBody
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.TxHash != txHash('a') {
		t.Errorf("tx hash must be normalised to lowercase, got %q", body.TxHash)
	}
}

func TestGetInvocationTrace_InvalidHash(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	for _, bad := range []string{"nothex", strings.Repeat("z", 64), strings.Repeat("a", 63)} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/invocations/"+bad+"/trace", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("hash %q: want 400, got %d (%s)", bad, w.Code, w.Body.String())
		}
		var env map[string]any
		if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		errObj, ok := env["error"].(map[string]any)
		if !ok {
			t.Fatalf("missing error envelope: %v", env)
		}
		if errObj["code"] != "INVALID_INPUT" {
			t.Errorf("want code INVALID_INPUT, got %v", errObj["code"])
		}
	}
}

func TestGetInvocationTrace_NotFound(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/invocations/"+txHash('d')+"/trace", nil)
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

func TestGetInvocationTrace_StoreError(t *testing.T) {
	ms := store.NewMockStore()
	ms.GetCallEdgesErr = errTraceUnavailable

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/invocations/"+txHash('e')+"/trace", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGetInvocationTrace_OrphanEdgeAttachedToRoot(t *testing.T) {
	ms := store.NewMockStore()
	// A depth cap in the indexer can produce a child whose parent edge was
	// never written. It must still be reachable, flagged as truncated.
	ms.AddCallEdge(store.CallEdge{TxHash: txHash('f'), ParentSpanID: "0.9", ChildSpanID: "0.9.0", FunctionName: "orphan", Depth: 2})
	ms.AddCallEdge(store.CallEdge{TxHash: txHash('f'), ParentSpanID: "0", ChildSpanID: "0.0", FunctionName: "ok", Depth: 1})

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/invocations/"+txHash('f')+"/trace", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body traceBody
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Truncated {
		t.Error("an edge with a missing parent must set truncated")
	}
	if len(body.Root.Children) != 2 {
		t.Fatalf("want both edges under the root, got %+v", body.Root.Children)
	}
}

func TestTraceRoute_RequiresReadScope(t *testing.T) {
	scope, ok := middleware.RequiredScope(http.MethodGet, "/api/v1/invocations/{tx_hash}/trace")
	if !ok {
		t.Fatal("the trace route is missing from the scope table")
	}
	if scope != middleware.ScopeReadContracts {
		t.Errorf("trace route scope = %q, want %q", scope, middleware.ScopeReadContracts)
	}
}
