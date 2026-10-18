package handler_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type snapshotExportBody struct {
	SchemaVersion int    `json:"schema_version"`
	ContractID    string `json:"contract_id"`
	Network       string `json:"network"`
	Ledger        uint32 `json:"ledger"`
	Metadata      struct {
		ID      string `json:"id"`
		Network string `json:"network"`
		Status  string `json:"status"`
	} `json:"metadata"`
	Storage []struct {
		KeyXDR   string `json:"key_xdr"`
		ValueXDR string `json:"value_xdr"`
	} `json:"storage"`
	Events []struct {
		ID     string `json:"id"`
		Ledger uint32 `json:"ledger"`
	} `json:"events"`
	Summary struct {
		StorageCount       int    `json:"storage_count"`
		EventCount         int    `json:"event_count"`
		FirstTrackedLedger uint32 `json:"first_tracked_ledger"`
		LastEventID        string `json:"last_event_id"`
		LastEventLedger    uint32 `json:"last_event_ledger"`
	} `json:"summary"`
}

func snapshotExportRequest(t *testing.T, srv http.Handler, acceptEncoding string) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+snapshotContract+"/snapshot.json", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	raw := w.Body.Bytes()
	if w.Header().Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("gzip reader: %v", err)
		}
		defer gz.Close()
		raw, err = io.ReadAll(gz)
		if err != nil {
			t.Fatalf("gunzip: %v", err)
		}
	}
	return w, raw
}

func decodeSnapshotExport(t *testing.T, raw []byte) snapshotExportBody {
	t.Helper()
	var body snapshotExportBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode snapshot export: %v (%s)", err, raw)
	}
	return body
}

func TestContractSnapshotExportShape(t *testing.T) {
	srv := newTestHandler(seedSnapshotStore(t), true, true)

	w, raw := snapshotExportRequest(t, srv, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	body := decodeSnapshotExport(t, raw)

	if got := w.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type: got %q", got)
	}
	wantDisposition := `attachment; filename="` + snapshotContract + `-snapshot.json"`
	if got := w.Header().Get("Content-Disposition"); got != wantDisposition {
		t.Errorf("Content-Disposition: got %q want %q", got, wantDisposition)
	}

	if body.SchemaVersion != 1 {
		t.Errorf("schema_version: want 1, got %d", body.SchemaVersion)
	}
	if body.ContractID != snapshotContract {
		t.Errorf("contract_id: want %s, got %s", snapshotContract, body.ContractID)
	}
	if body.Network != "testnet" || body.Metadata.Network != "testnet" {
		t.Errorf("network: want testnet, got %q / %q", body.Network, body.Metadata.Network)
	}
	if body.Metadata.ID != snapshotContract {
		t.Errorf("metadata.id: want %s, got %s", snapshotContract, body.Metadata.ID)
	}
	if body.Metadata.Status != "active" {
		t.Errorf("metadata.status: want active, got %q", body.Metadata.Status)
	}
	// Keyed off the newest indexed ledger, not wall-clock time.
	if body.Ledger != 3000 {
		t.Errorf("ledger: want 3000, got %d", body.Ledger)
	}

	// Storage collapses history to the live version per key, sorted by key.
	if len(body.Storage) != 2 {
		t.Fatalf("storage: want 2 entries, got %d (%+v)", len(body.Storage), body.Storage)
	}
	if body.Storage[0].KeyXDR != "key1" || body.Storage[0].ValueXDR != "v2" {
		t.Errorf("storage[0]: want key1=v2, got %s=%s", body.Storage[0].KeyXDR, body.Storage[0].ValueXDR)
	}
	if body.Storage[1].KeyXDR != "key2" || body.Storage[1].ValueXDR != "w1" {
		t.Errorf("storage[1]: want key2=w1, got %s=%s", body.Storage[1].KeyXDR, body.Storage[1].ValueXDR)
	}

	// Events are newest first.
	if len(body.Events) != 2 {
		t.Fatalf("events: want 2, got %d", len(body.Events))
	}
	if body.Events[0].ID != "e2" || body.Events[0].Ledger != 3000 {
		t.Errorf("events[0]: want e2@3000, got %s@%d", body.Events[0].ID, body.Events[0].Ledger)
	}
	if body.Events[1].ID != "e1" || body.Events[1].Ledger != 1000 {
		t.Errorf("events[1]: want e1@1000, got %s@%d", body.Events[1].ID, body.Events[1].Ledger)
	}

	if body.Summary.StorageCount != 2 || body.Summary.EventCount != 2 {
		t.Errorf("summary counts: got storage=%d events=%d", body.Summary.StorageCount, body.Summary.EventCount)
	}
	if body.Summary.FirstTrackedLedger != 1000 {
		t.Errorf("summary.first_tracked_ledger: want 1000, got %d", body.Summary.FirstTrackedLedger)
	}
	if body.Summary.LastEventID != "e2" || body.Summary.LastEventLedger != 3000 {
		t.Errorf("summary last event: got %s@%d", body.Summary.LastEventID, body.Summary.LastEventLedger)
	}

	// All four content sections must be present.
	for _, key := range []string{"metadata", "storage", "events", "summary"} {
		if !strings.Contains(string(raw), `"`+key+`"`) {
			t.Errorf("missing section %q in export", key)
		}
	}
}

// The export contains no wall-clock fields, so two reads of an unchanged
// store must be byte-identical.
func TestContractSnapshotExportDeterministic(t *testing.T) {
	srv := newTestHandler(seedSnapshotStore(t), true, true)

	_, first := snapshotExportRequest(t, srv, "")
	_, second := snapshotExportRequest(t, srv, "")
	if !bytes.Equal(first, second) {
		t.Fatalf("exports differ:\n%s\n%s", first, second)
	}
}

func TestContractSnapshotExportGzip(t *testing.T) {
	srv := newTestHandler(seedSnapshotStore(t), true, true)

	w, raw := snapshotExportRequest(t, srv, "gzip")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding: want gzip, got %q", got)
	}
	if got := w.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Errorf("Vary: want Accept-Encoding, got %q", got)
	}
	body := decodeSnapshotExport(t, raw)
	if body.ContractID != snapshotContract || len(body.Storage) != 2 {
		t.Errorf("gzip body mismatch: %+v", body)
	}

	// q=0 means the client refuses gzip, so the body stays uncompressed.
	w, _ = snapshotExportRequest(t, srv, "gzip;q=0")
	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding with q=0: want empty, got %q", got)
	}
}

func TestContractSnapshotExportUnknownContract(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/CUNKNOWN/snapshot.json", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != "NOT_FOUND" {
		t.Errorf("want code=NOT_FOUND, got %q", env.Error.Code)
	}
}

// Empty collections are emitted as [] and valueless summary fields are
// omitted, so downstream parsers never see null.
func TestContractSnapshotExportEmptySections(t *testing.T) {
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{
		ID:      snapshotContract,
		Network: "testnet",
		Status:  "active",
	}); err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)

	w, raw := snapshotExportRequest(t, srv, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(string(raw), `"storage":[]`) {
		t.Errorf("want empty storage array, got %s", raw)
	}
	if !strings.Contains(string(raw), `"events":[]`) {
		t.Errorf("want empty events array, got %s", raw)
	}
	if strings.Contains(string(raw), `"last_event_id"`) {
		t.Errorf("last_event_id must be omitted when there are no events: %s", raw)
	}
}
