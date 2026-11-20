package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/coldstorage"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func newTestRouterWithExports(t *testing.T) (http.Handler, *store.MockStore, coldstorage.ObjectStore) {
	t.Helper()
	s := store.NewMockStore()
	memStore := coldstorage.NewMemoryStore()

	_ = s.UpsertContract(context.Background(), store.Contract{
		ID:      "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Network: "testnet",
		Status:  "active",
	})
	_ = s.BatchInsertEvents(context.Background(), []store.Event{
		{
			ID:         "ev-1",
			ContractID: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Network:    "testnet",
			TxHash:     "tx-1",
			Ledger:     100,
			Type:       "contract",
		},
	})

	h := &handler.Handler{
		Store:            s,
		DB:               &store.MockPinger{Healthy: true},
		Redis:            &store.MockPinger{Healthy: true},
		RedisClient:      &mockRedisClient{},
		ObjectStore:      memStore,
		ExportSigningKey: "test-export-secret-key",
		Logger:           slog.Default(),
	}

	return router.New(h, 1<<20), s, memStore
}

func TestCreateAndCompleteExportJob(t *testing.T) {
	r, _, _ := newTestRouterWithExports(t)

	body := map[string]any{
		"type":   "events",
		"format": "csv",
		"filters": map[string]any{
			"contract_id": "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			"limit":       100,
		},
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/exports", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	jobID, ok := resp["id"].(string)
	if !ok || jobID == "" {
		t.Fatalf("missing job id in response: %+v", resp)
	}

	// Wait for background worker to complete
	time.Sleep(150 * time.Millisecond)

	// Fetch job
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/exports/"+jobID, nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", getW.Code, getW.Body.String())
	}

	var getResp map[string]any
	_ = json.Unmarshal(getW.Body.Bytes(), &getResp)

	if getResp["status"] != "completed" {
		t.Fatalf("expected status completed, got %v (err: %v)", getResp["status"], getResp["error_message"])
	}

	dlURL, ok := getResp["download_url"].(string)
	if !ok || dlURL == "" {
		t.Fatalf("expected non-empty download_url, got %v", getResp["download_url"])
	}

	// Download the export
	dlReq := httptest.NewRequest(http.MethodGet, dlURL, nil)
	dlW := httptest.NewRecorder()
	r.ServeHTTP(dlW, dlReq)

	if dlW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK downloading export, got %d: %s", dlW.Code, dlW.Body.String())
	}

	content := dlW.Body.String()
	if !strings.Contains(content, "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA") {
		t.Fatalf("expected export content to contain contract ID, got: %s", content)
	}
}

func TestExportJobConcurrencyLimit(t *testing.T) {
	r, s, _ := newTestRouterWithExports(t)

	// Pre-populate 3 active jobs for anonymous user
	for i := 1; i <= 3; i++ {
		_, _ = s.CreateExportJob(context.Background(), store.ExportJob{
			UserID: "anonymous",
			Type:   "events",
			Format: "csv",
			Status: store.ExportJobStatusProcessing,
		})
	}

	body := map[string]any{
		"type":   "events",
		"format": "csv",
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/exports", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExportDownloadTokenRejection(t *testing.T) {
	r, s, memStore := newTestRouterWithExports(t)

	objKey := "exports/anonymous/job-test.csv"
	_ = memStore.Put(context.Background(), objKey, []byte("data"))

	exp := time.Now().Add(1 * time.Hour)
	_, _ = s.CreateExportJob(context.Background(), store.ExportJob{
		ID:        "job-test",
		UserID:    "anonymous",
		Type:      "events",
		Format:    "csv",
		Status:    store.ExportJobStatusCompleted,
		ObjectKey: &objKey,
		ExpiresAt: &exp,
	})

	// Missing token -> 403
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/exports/job-test/download", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for missing token, got %d", w1.Code)
	}

	// Invalid token -> 403
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/exports/job-test/download?token=invalid.token", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for invalid token, got %d", w2.Code)
	}
}

func TestDeleteExportJob(t *testing.T) {
	r, s, memStore := newTestRouterWithExports(t)

	objKey := "exports/anonymous/job-del.csv"
	_ = memStore.Put(context.Background(), objKey, []byte("data"))

	_, _ = s.CreateExportJob(context.Background(), store.ExportJob{
		ID:        "job-del",
		UserID:    "anonymous",
		Type:      "events",
		Format:    "csv",
		Status:    store.ExportJobStatusCompleted,
		ObjectKey: &objKey,
	})

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/exports/job-del", nil)
	delW := httptest.NewRecorder()
	r.ServeHTTP(delW, delReq)

	if delW.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content, got %d: %s", delW.Code, delW.Body.String())
	}

	// Object store key should be deleted
	if _, err := memStore.Get(context.Background(), objKey); err == nil {
		t.Fatalf("expected object to be deleted from store")
	}
}
