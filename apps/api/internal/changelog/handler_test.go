// Package changelog_test provides integration-style tests for the changelog
// HTTP handlers. Tests use the real handler code against an in-memory fake
// store so they exercise the full request → handler → store → response path
// without requiring a live database.
package changelog_test

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/changelog"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- in-memory store fake --------------------------------------------------

type fakeStore struct {
	contracts []store.Contract
	versions  []store.ContractVersion
}

func (f *fakeStore) UpsertContract(_ context.Context, c store.Contract) error {
	f.contracts = append(f.contracts, c)
	return nil
}

func (f *fakeStore) GetContract(_ context.Context, id string) (store.Contract, error) {
	for _, c := range f.contracts {
		if c.ID == id {
			return c, nil
		}
	}
	return store.Contract{}, store.ErrNotFound
}

func (f *fakeStore) ListContracts(_ context.Context, _ string, _ int, _ store.ContractFilters) ([]store.Contract, string, error) {
	return f.contracts, "", nil
}

func (f *fakeStore) BatchInsertEvents(_ context.Context, _ []store.Event) error { return nil }

func (f *fakeStore) BatchInsertInvocations(_ context.Context, _ []store.Invocation) error {
	return nil
}

func (f *fakeStore) UpsertStorageEntries(_ context.Context, _ []store.StorageEntry) error {
	return nil
}

func (f *fakeStore) GetSyncState(_ context.Context, id string) (store.SyncState, error) {
	return store.SyncState{ContractID: id}, nil
}

func (f *fakeStore) UpsertSyncState(_ context.Context, _ store.SyncState) error { return nil }

func (f *fakeStore) GetGlobalStats(_ context.Context) (store.GlobalStats, error) {
	return store.GlobalStats{}, nil
}

func (f *fakeStore) CreateNextMonthPartition(_ context.Context) error { return nil }

func (f *fakeStore) CreateMonthlyPartitionIfNotExists(_ context.Context, _ int, _ int) error {
	return nil
}

func (f *fakeStore) GetIndexerCursor(_ context.Context, _ string) (uint32, error) {
	return 0, nil
}

func (f *fakeStore) SetIndexerCursor(_ context.Context, _ string, _ uint32) error {
	return nil
}

func (f *fakeStore) BatchInsertWithCursor(_ context.Context, _ string, _ uint32, _ []store.Event, _ []store.Invocation, _ store.SyncState) error {
	return nil
}

func (f *fakeStore) RecordContractVersion(_ context.Context, v store.ContractVersion) error {
	for _, existing := range f.versions {
		if existing.ContractID == v.ContractID && existing.WasmHash == v.WasmHash {
			return nil
		}
	}
	f.versions = append(f.versions, v)
	return nil
}

func (f *fakeStore) ListContractVersions(_ context.Context, contractID string) ([]store.ContractVersion, error) {
	var out []store.ContractVersion
	for _, v := range f.versions {
		if v.ContractID == contractID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (f *fakeStore) GetLatestContractVersion(_ context.Context, contractID string) (store.ContractVersion, error) {
	var latest store.ContractVersion
	found := false
	for _, v := range f.versions {
		if v.ContractID == contractID {
			if !found || v.FirstSeenLedger > latest.FirstSeenLedger {
				latest = v
				found = true
			}
		}
	}
	if !found {
		return store.ContractVersion{}, store.ErrNotFound
	}
	return latest, nil
}

// ---- helpers ---------------------------------------------------------------

const testContractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"

func seededStore() *fakeStore {
	return &fakeStore{
		contracts: []store.Contract{{
			ID:      testContractID,
			Network: "testnet",
			Status:  "active",
			AddedAt: time.Now(),
		}},
		versions: []store.ContractVersion{
			{
				ID:              1,
				ContractID:      testContractID,
				WasmHash:        "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd",
				FirstSeenLedger: 100000,
				TxHash:          "tx_genesis",
				RecordedAt:      time.Now().Add(-24 * time.Hour),
			},
			{
				ID:              2,
				ContractID:      testContractID,
				WasmHash:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				FirstSeenLedger: 200000,
				TxHash:          "tx_upgrade",
				RecordedAt:      time.Now(),
			},
		},
	}
}

func makeRequest(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Host = "sorolens.example.com"
	return req
}

// ---- tests -----------------------------------------------------------------

// TestGetChangelog_200_returnsVersionArray is an end-to-end integration test:
//
//	ingestion → store → GET /contracts/{id}/changelog → 200 JSON array.
func TestGetChangelog_200_returnsVersionArray(t *testing.T) {
	h := changelog.New(seededStore())

	req := makeRequest(http.MethodGet, "/contracts/"+testContractID+"/changelog")
	w := httptest.NewRecorder()
	h.GetChangelog(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type: want application/json, got %q", ct)
	}

	var payload []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload) != 2 {
		t.Fatalf("expected 2 changelog entries, got %d", len(payload))
	}
	// Verify the first entry fields.
	if payload[0]["wasm_hash"] != "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd" {
		t.Errorf("first entry wasm_hash mismatch: %v", payload[0]["wasm_hash"])
	}
}

// TestGetChangelog_404_unknownContract verifies a 404 for contracts not in the store.
func TestGetChangelog_404_unknownContract(t *testing.T) {
	h := changelog.New(seededStore())

	req := makeRequest(http.MethodGet, "/contracts/CUNKNOWN/changelog")
	w := httptest.NewRecorder()
	h.GetChangelog(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Result().StatusCode)
	}
}

// TestGetFeed_200_returnsAtomXML verifies the Atom feed endpoint returns valid XML.
func TestGetFeed_200_returnsAtomXML(t *testing.T) {
	h := changelog.New(seededStore())

	req := makeRequest(http.MethodGet, "/contracts/"+testContractID+"/changelog/feed")
	w := httptest.NewRecorder()
	h.GetFeed(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "atom+xml") {
		t.Fatalf("content-type: want atom+xml, got %q", ct)
	}

	// Parse as generic XML to confirm well-formedness.
	var feed struct {
		XMLName xml.Name `xml:"feed"`
		Title   string   `xml:"title"`
		Entries []struct {
			Title string `xml:"title"`
		} `xml:"entry"`
	}
	body := w.Body.Bytes()
	if err := xml.Unmarshal(body, &feed); err != nil {
		t.Fatalf("parse atom feed: %v\nbody: %s", err, body)
	}
	if len(feed.Entries) != 2 {
		t.Fatalf("expected 2 feed entries, got %d", len(feed.Entries))
	}
}

// TestGetBadge_200_returnsSVG verifies the badge endpoint returns a valid SVG.
func TestGetBadge_200_returnsSVG(t *testing.T) {
	h := changelog.New(seededStore())

	req := makeRequest(http.MethodGet, "/contracts/"+testContractID+"/changelog/badge")
	w := httptest.NewRecorder()
	h.GetBadge(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/svg+xml" {
		t.Fatalf("content-type: want image/svg+xml, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<svg") {
		t.Fatalf("response is not an SVG: %s", body)
	}
	// The badge must display the first 8 chars of the latest hash ("deadbeef").
	if !strings.Contains(body, "deadbeef") {
		t.Fatalf("badge body does not contain expected hash slice 'deadbeef':\n%s", body)
	}
}

// TestGetBadge_unknownHashShowsUnknown verifies that a contract with no recorded
// versions returns a badge with "unknown" rather than an error response.
func TestGetBadge_unknownHashShowsUnknown(t *testing.T) {
	s := &fakeStore{
		contracts: []store.Contract{{
			ID: "CEMPTY", Network: "testnet", Status: "active", AddedAt: time.Now(),
		}},
	}
	h := changelog.New(s)

	req := makeRequest(http.MethodGet, "/contracts/CEMPTY/changelog/badge")
	w := httptest.NewRecorder()
	h.GetBadge(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for contract with no versions, got %d", w.Result().StatusCode)
	}
	if !strings.Contains(w.Body.String(), "unknown") {
		t.Fatalf("expected badge to show 'unknown', got: %s", w.Body.String())
	}
}


func (f *fakeStore) SearchContracts(_ context.Context, query string, limit int) ([]store.Contract, error) {
	return nil, nil
}
