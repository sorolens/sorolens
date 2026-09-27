package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const (
	tagContractA = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	tagContractB = "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
)

// seedTagStore returns a store with a contributor identity and two tracked
// contracts on different networks so tag filtering can be combined with the
// network filter.
func seedTagStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := seedRoleStore(t)
	for _, c := range []store.Contract{
		{ID: tagContractA, Network: "testnet", Label: "a", Status: "active"},
		{ID: tagContractB, Network: "mainnet", Label: "b", Status: "active"},
	} {
		if err := ms.UpsertContract(nil, c); err != nil {
			t.Fatal(err)
		}
	}
	return ms
}

// doTagRequest issues a request with a JSON body and optional X-User-ID.
func doTagRequest(srv http.Handler, method, path string, body any, user string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		buf.Write(b)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.Header.Set("X-User-ID", user)
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func decodeTags(t *testing.T, body []byte) []string {
	t.Helper()
	var parsed struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("decode tags: %v", err)
	}
	return parsed.Tags
}

func TestAddContractTagPersists(t *testing.T) {
	srv := newTestHandler(seedTagStore(t), true, true)

	w := doTagRequest(srv, http.MethodPost,
		"/api/v1/contracts/"+tagContractA+"/tags", map[string]string{"tag": "prod"}, contributorUser)
	if w.Code != http.StatusOK {
		t.Fatalf("add tag: want 200, got %d: %s", w.Code, w.Body.String())
	}
	if tags := decodeTags(t, w.Body.Bytes()); len(tags) != 1 || tags[0] != "prod" {
		t.Fatalf("add tag: want [prod], got %v", tags)
	}

	// The tag must persist: the contract detail read returns it.
	w = doTagRequest(srv, http.MethodGet, "/api/v1/contracts/"+tagContractA, nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get contract: want 200, got %d", w.Code)
	}
	var contract struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &contract); err != nil {
		t.Fatal(err)
	}
	if len(contract.Tags) != 1 || contract.Tags[0] != "prod" {
		t.Fatalf("persisted tags: want [prod], got %v", contract.Tags)
	}
}

func TestAddContractTagIsIdempotent(t *testing.T) {
	srv := newTestHandler(seedTagStore(t), true, true)
	body := map[string]string{"tag": "prod"}

	for i := 0; i < 2; i++ {
		w := doTagRequest(srv, http.MethodPost,
			"/api/v1/contracts/"+tagContractA+"/tags", body, contributorUser)
		if w.Code != http.StatusOK {
			t.Fatalf("attempt %d: want 200, got %d", i+1, w.Code)
		}
		if tags := decodeTags(t, w.Body.Bytes()); len(tags) != 1 {
			t.Fatalf("attempt %d: want 1 tag, got %v", i+1, tags)
		}
	}
}

func TestAddContractTagNormalizesInput(t *testing.T) {
	srv := newTestHandler(seedTagStore(t), true, true)

	w := doTagRequest(srv, http.MethodPost,
		"/api/v1/contracts/"+tagContractA+"/tags", map[string]string{"tag": "  Prod  "}, contributorUser)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if tags := decodeTags(t, w.Body.Bytes()); len(tags) != 1 || tags[0] != "prod" {
		t.Fatalf("want normalized [prod], got %v", tags)
	}
}

func TestAddContractTagRejectsInvalidTag(t *testing.T) {
	srv := newTestHandler(seedTagStore(t), true, true)

	for _, tag := range []string{"", "   ", "bad tag", "UPPER!", "toolongtoolongtoolongtoolongtoolongtoolong"} {
		w := doTagRequest(srv, http.MethodPost,
			"/api/v1/contracts/"+tagContractA+"/tags", map[string]string{"tag": tag}, contributorUser)
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("tag %q: want 422, got %d", tag, w.Code)
		}
	}
}

func TestAddContractTagUnknownContract(t *testing.T) {
	srv := newTestHandler(seedTagStore(t), true, true)

	w := doTagRequest(srv, http.MethodPost, "/api/v1/contracts/CNOPE/tags",
		map[string]string{"tag": "prod"}, contributorUser)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestAddContractTagRequiresContributor(t *testing.T) {
	srv := newTestHandler(seedTagStore(t), true, true)

	// Anonymous callers pass scope enforcement (public read/write surface)
	// but the contributor role middleware rejects them.
	w := doTagRequest(srv, http.MethodPost, "/api/v1/contracts/"+tagContractA+"/tags",
		map[string]string{"tag": "prod"}, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: want 401, got %d", w.Code)
	}
}

func TestRemoveContractTag(t *testing.T) {
	srv := newTestHandler(seedTagStore(t), true, true)

	if w := doTagRequest(srv, http.MethodPost, "/api/v1/contracts/"+tagContractA+"/tags",
		map[string]string{"tag": "prod"}, contributorUser); w.Code != http.StatusOK {
		t.Fatalf("seed tag: want 200, got %d", w.Code)
	}

	w := doTagRequest(srv, http.MethodDelete,
		"/api/v1/contracts/"+tagContractA+"/tags/prod", nil, contributorUser)
	if w.Code != http.StatusNoContent {
		t.Fatalf("remove tag: want 204, got %d: %s", w.Code, w.Body.String())
	}

	// Removing it again is a no-op.
	w = doTagRequest(srv, http.MethodDelete,
		"/api/v1/contracts/"+tagContractA+"/tags/prod", nil, contributorUser)
	if w.Code != http.StatusNoContent {
		t.Fatalf("idempotent remove: want 204, got %d", w.Code)
	}
}

func TestListContractsFiltersByTag(t *testing.T) {
	ms := seedTagStore(t)
	if err := ms.AddContractTag(nil, tagContractA, "prod"); err != nil {
		t.Fatal(err)
	}
	if err := ms.AddContractTag(nil, tagContractB, "staging"); err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts?tag=prod", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	contracts := decodeContracts(t, w.Body.Bytes())
	if len(contracts) != 1 || contracts[0]["id"] != tagContractA {
		t.Fatalf("tag=prod: want only %s, got %+v", tagContractA, contracts)
	}
	if tags, _ := contracts[0]["tags"].([]any); len(tags) != 1 || tags[0] != "prod" {
		t.Fatalf("tag=prod: response should carry tags, got %v", contracts[0]["tags"])
	}

	// A tag nobody carries yields an empty list, not an error.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/contracts?tag=missing", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if got := len(decodeContracts(t, w.Body.Bytes())); got != 0 {
		t.Fatalf("tag=missing: want 0 contracts, got %d", got)
	}
}

func TestListContractsCombinesTagAndNetwork(t *testing.T) {
	ms := seedTagStore(t)
	// Both contracts carry the tag, but only one is on testnet.
	if err := ms.AddContractTag(nil, tagContractA, "prod"); err != nil {
		t.Fatal(err)
	}
	if err := ms.AddContractTag(nil, tagContractB, "prod"); err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts?tag=prod&network=testnet", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	contracts := decodeContracts(t, w.Body.Bytes())
	if len(contracts) != 1 || contracts[0]["id"] != tagContractA {
		t.Fatalf("tag+network: want only %s, got %+v", tagContractA, contracts)
	}
}

func TestListContractsRejectsInvalidTag(t *testing.T) {
	srv := newTestHandler(seedTagStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts?tag=not%20a%20tag", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", w.Code)
	}
}
