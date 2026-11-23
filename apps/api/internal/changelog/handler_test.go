// Package changelog_test provides integration-style tests for the changelog
// HTTP handlers. Tests use the real handler code against an in-memory fake
// store so they exercise the full request → handler → store → response path
// without requiring a live database.
package changelog_test

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
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

func (f *fakeStore) UntrackContract(_ context.Context, id string) error {
	for i, c := range f.contracts {
		if c.ID == id {
			f.contracts[i].Status = "paused"
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) ListContracts(_ context.Context, _ string, _ int, _ store.ContractFilters) ([]store.Contract, string, error) {
	return f.contracts, "", nil
}

func (f *fakeStore) BatchInsertEvents(_ context.Context, _ []store.Event) error { return nil }

func (f *fakeStore) BatchInsertInvocations(_ context.Context, _ []store.Invocation) error {
	return nil
}

func (f *fakeStore) BatchInsertCallEdges(_ context.Context, _ []store.CallEdge) error {
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

func (f *fakeStore) MaintainPartitions(_ context.Context, _ int) error { return nil }

func (f *fakeStore) ArchiveOldEvents(_ context.Context, _ int) (int64, error) { return 0, nil }

func (f *fakeStore) GetIndexerCursor(_ context.Context, _ string) (uint32, error) {
	return 0, nil
}

func (f *fakeStore) SetIndexerCursor(_ context.Context, _ string, _ uint32) error {
	return nil
}

func (f *fakeStore) BatchInsertWithCursor(_ context.Context, _ string, _ uint32, _ []store.Event, _ []store.Invocation, _ []store.CallEdge, _ store.SyncState) error {
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

func (f *fakeStore) SearchEventValues(_ context.Context, _, _ string, _ int) ([]store.Event, string, error) {
	return nil, "", nil
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

// errorBody decodes the standard {"error": {...}} envelope.
func errorBody(t *testing.T, w *httptest.ResponseRecorder) (code, message string) {
	t.Helper()
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("error content-type: want application/json, got %q", ct)
	}
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	return env.Error.Code, env.Error.Message
}

// TestGetChangelog_404_errorEnvelope verifies an unknown contract gets the
// same JSON error envelope as every other handler, not a text/plain body.
func TestGetChangelog_404_errorEnvelope(t *testing.T) {
	h := changelog.New(seededStore())

	w := httptest.NewRecorder()
	h.GetChangelog(w, makeRequest(http.MethodGet, "/contracts/CUNKNOWN/changelog"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if code, msg := errorBody(t, w); code != "NOT_FOUND" || msg != "contract not found" {
		t.Fatalf("error: want NOT_FOUND / contract not found, got %s / %s", code, msg)
	}
}

// TestGetChangelog_200_emptyHistory verifies a tracked contract with no
// recorded versions returns an empty JSON array rather than null or an error.
func TestGetChangelog_200_emptyHistory(t *testing.T) {
	s := &fakeStore{contracts: []store.Contract{{ID: "CEMPTY", Status: "active"}}}
	h := changelog.New(s)

	w := httptest.NewRecorder()
	h.GetChangelog(w, makeRequest(http.MethodGet, "/contracts/CEMPTY/changelog"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := strings.TrimSpace(w.Body.String()); got != "[]" {
		t.Fatalf("expected [], got %s", got)
	}
}

// TestGetFeed_404_unknownContract verifies the feed rejects an unknown
// contract with the JSON error envelope.
func TestGetFeed_404_unknownContract(t *testing.T) {
	h := changelog.New(seededStore())

	w := httptest.NewRecorder()
	h.GetFeed(w, makeRequest(http.MethodGet, "/contracts/CUNKNOWN/changelog/feed"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if code, _ := errorBody(t, w); code != "NOT_FOUND" {
		t.Fatalf("error code: want NOT_FOUND, got %s", code)
	}
}

// TestGetFeed_200_emptyHistory verifies a contract with no recorded versions
// gets a well-formed Atom feed with no entries.
func TestGetFeed_200_emptyHistory(t *testing.T) {
	s := &fakeStore{contracts: []store.Contract{{ID: "CEMPTY", Status: "active"}}}
	h := changelog.New(s)

	w := httptest.NewRecorder()
	h.GetFeed(w, makeRequest(http.MethodGet, "/contracts/CEMPTY/changelog/feed"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var feed struct {
		XMLName xml.Name   `xml:"http://www.w3.org/2005/Atom feed"`
		Entries []struct{} `xml:"entry"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &feed); err != nil {
		t.Fatalf("parse atom feed: %v\nbody: %s", err, w.Body.String())
	}
	if len(feed.Entries) != 0 {
		t.Fatalf("expected no entries, got %d", len(feed.Entries))
	}
}

// TestGetFeed_linksKeepMountPrefix verifies the feed's links include the
// prefix the handler is mounted under, so they resolve to live routes.
func TestGetFeed_linksKeepMountPrefix(t *testing.T) {
	h := changelog.New(seededStore())

	w := httptest.NewRecorder()
	h.GetFeed(w, makeRequest(http.MethodGet, "/api/v1/contracts/"+testContractID+"/changelog/feed"))

	var feed struct {
		ID      string `xml:"id"`
		Entries []struct {
			Link struct {
				Href string `xml:"href,attr"`
			} `xml:"link"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &feed); err != nil {
		t.Fatalf("parse atom feed: %v", err)
	}
	base := "http://sorolens.example.com/api/v1/contracts/" + testContractID
	if feed.ID != base+"/changelog/feed" {
		t.Errorf("feed id: want %s/changelog/feed, got %s", base, feed.ID)
	}
	for _, e := range feed.Entries {
		if e.Link.Href != base+"/changelog" {
			t.Errorf("entry link: want %s/changelog, got %s", base, e.Link.Href)
		}
	}
}

// TestGetBadge_unknownContractShowsUnknown verifies an untracked contract
// still gets an SVG badge (with the badge's caching header) instead of an
// error, so an embedded README image never breaks.
func TestGetBadge_unknownContractShowsUnknown(t *testing.T) {
	h := changelog.New(seededStore())

	w := httptest.NewRecorder()
	h.GetBadge(w, makeRequest(http.MethodGet, "/contracts/CUNKNOWN/changelog/badge"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Fatalf("content-type: want image/svg+xml, got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache, max-age=0" {
		t.Fatalf("cache-control: want no-cache, max-age=0, got %q", cc)
	}
	if !strings.Contains(w.Body.String(), "unknown") {
		t.Fatalf("expected badge to show 'unknown', got: %s", w.Body.String())
	}
}

// failingStore fails the version lookups to exercise the 500 path.
type failingStore struct{ *fakeStore }

func (failingStore) ListContractVersions(context.Context, string) ([]store.ContractVersion, error) {
	return nil, errors.New("db down")
}

func (failingStore) GetLatestContractVersion(context.Context, string) (store.ContractVersion, error) {
	return store.ContractVersion{}, errors.New("db down")
}

// TestChangelog_500_errorEnvelope verifies store failures surface as a 500
// with the JSON error envelope on every endpoint.
func TestChangelog_500_errorEnvelope(t *testing.T) {
	h := changelog.New(failingStore{seededStore()})

	for name, serve := range map[string]http.HandlerFunc{
		"changelog": h.GetChangelog,
		"feed":      h.GetFeed,
		"badge":     h.GetBadge,
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			serve(w, makeRequest(http.MethodGet, "/contracts/"+testContractID+"/changelog"))
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("expected 500, got %d", w.Code)
			}
			if code, _ := errorBody(t, w); code != "INTERNAL" {
				t.Fatalf("error code: want INTERNAL, got %s", code)
			}
		})
	}
}

func (f *fakeStore) SearchContracts(_ context.Context, query string, limit int) ([]store.Contract, error) {
	return nil, nil
}

func (f *fakeStore) SearchEvents(_ context.Context, query string, limit int) ([]store.Event, error) {
	return nil, nil
}

func (f *fakeStore) SearchFunctions(_ context.Context, query string, limit int) ([]store.FunctionMatch, error) {
	return nil, nil
}
