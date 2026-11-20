package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/coldstorage"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Default export job settings.
const (
	MaxConcurrentExportsPerUser = 3
	MaxExportRows               = 5_000_000
	DefaultExportTTL            = 24 * time.Hour
)

type createExportRequest struct {
	Type    string          `json:"type"`   // "events" or "snapshot"
	Format  string          `json:"format"` // "csv" or "ndjson"
	Filters json.RawMessage `json:"filters"`
}

type exportJobResponse struct {
	ID           string                `json:"id"`
	UserID       string                `json:"user_id"`
	Type         string                `json:"type"`
	Format       string                `json:"format"`
	Filters      json.RawMessage       `json:"filters"`
	Status       store.ExportJobStatus `json:"status"`
	Progress     int                   `json:"progress"`
	ErrorMessage *string               `json:"error_message,omitempty"`
	DownloadURL  *string               `json:"download_url,omitempty"`
	ExpiresAt    *string               `json:"expires_at,omitempty"`
	CreatedAt    string                `json:"created_at"`
	UpdatedAt    string                `json:"updated_at"`
}

func toExportJobResponse(j store.ExportJob) exportJobResponse {
	resp := exportJobResponse{
		ID:           j.ID,
		UserID:       j.UserID,
		Type:         j.Type,
		Format:       j.Format,
		Filters:      j.Filters,
		Status:       j.Status,
		Progress:     j.Progress,
		ErrorMessage: j.ErrorMessage,
		DownloadURL:  j.DownloadURL,
		CreatedAt:    j.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    j.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if j.ExpiresAt != nil {
		exp := j.ExpiresAt.UTC().Format(time.RFC3339)
		resp.ExpiresAt = &exp
	}
	return resp
}

func (h *Handler) objectStore() coldstorage.ObjectStore {
	if h.ObjectStore != nil {
		return h.ObjectStore
	}
	return coldstorage.NewMemoryStore()
}

func (h *Handler) exportSecret() []byte {
	if h.ExportSigningKey != "" {
		return []byte(h.ExportSigningKey)
	}
	if h.ReportSigningKey != "" {
		return []byte(h.ReportSigningKey)
	}
	return []byte("sorolens-export-default-secret")
}

func (h *Handler) signExportToken(jobID, userID string, expUnix int64) string {
	mac := hmac.New(sha256.New, h.exportSecret())
	msg := fmt.Sprintf("%s:%s:%d", jobID, userID, expUnix)
	mac.Write([]byte(msg))
	return fmt.Sprintf("%d.%s", expUnix, hex.EncodeToString(mac.Sum(nil)))
}

func (h *Handler) verifyExportToken(jobID, userID, token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	expUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expUnix {
		return false // expired
	}
	expected := h.signExportToken(jobID, userID, expUnix)
	return hmac.Equal([]byte(token), []byte(expected))
}

func (h *Handler) logError(msg string, args ...any) {
	if h.Logger != nil {
		h.Logger.Error(msg, args...)
	}
}

// CreateExport handles POST /api/v1/exports.
func (h *Handler) CreateExport(w http.ResponseWriter, r *http.Request) {
	userID := middleware.Actor(r)
	if userID == "" {
		userID = "anonymous"
	}

	var req createExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, CodeInvalidInput, "malformed request body")
		return
	}

	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if req.Type != "events" && req.Type != "snapshot" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "type must be 'events' or 'snapshot'")
		return
	}

	req.Format = strings.ToLower(strings.TrimSpace(req.Format))
	if req.Format != "csv" && req.Format != "ndjson" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "format must be 'csv' or 'ndjson'")
		return
	}

	// Concurrency check: maximum active jobs per user
	active, err := h.Store.ListActiveExportJobs(r.Context(), userID)
	if err != nil {
		h.logError("check active export jobs", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to check active export jobs")
		return
	}
	if len(active) >= MaxConcurrentExportsPerUser {
		writeError(w, r, http.StatusTooManyRequests, "CONCURRENCY_LIMIT_EXCEEDED",
			fmt.Sprintf("concurrency limit exceeded: maximum %d active export jobs allowed", MaxConcurrentExportsPerUser))
		return
	}

	job := store.ExportJob{
		UserID:   userID,
		Type:     req.Type,
		Format:   req.Format,
		Filters:  req.Filters,
		Status:   store.ExportJobStatusPending,
		Progress: 0,
	}

	created, err := h.Store.CreateExportJob(r.Context(), job)
	if err != nil {
		h.logError("create export job", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create export job")
		return
	}

	// Run export in background worker
	go h.runExportJob(created)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(toExportJobResponse(created))
}

// GetExport handles GET /api/v1/exports/{id}.
func (h *Handler) GetExport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.Actor(r)

	job, err := h.Store.GetExportJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "export job not found")
			return
		}
		h.logError("get export job", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to get export job")
		return
	}

	// Verify user ownership if authenticated
	if userID != "" && job.UserID != "" && job.UserID != "anonymous" && job.UserID != userID {
		writeError(w, r, http.StatusForbidden, CodeForbidden, "not authorized to access this export job")
		return
	}

	// Check expiry
	if job.Status == store.ExportJobStatusCompleted && job.ExpiresAt != nil && time.Now().After(*job.ExpiresAt) {
		job.DownloadURL = nil
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toExportJobResponse(job))
}

// DeleteExport handles DELETE /api/v1/exports/{id}.
func (h *Handler) DeleteExport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.Actor(r)
	if userID == "" {
		userID = "anonymous"
	}

	job, err := h.Store.GetExportJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "export job not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to find export job")
		return
	}

	if job.UserID != "" && job.UserID != "anonymous" && job.UserID != userID {
		writeError(w, r, http.StatusForbidden, CodeForbidden, "not authorized to delete this export job")
		return
	}

	if job.ObjectKey != nil && *job.ObjectKey != "" {
		_ = h.objectStore().Delete(r.Context(), *job.ObjectKey)
	}

	if err := h.Store.DeleteExportJob(r.Context(), id, job.UserID); err != nil {
		h.logError("delete export job", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete export job")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DownloadExport handles GET /api/v1/exports/{id}/download.
func (h *Handler) DownloadExport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	token := r.URL.Query().Get("token")
	callerKey := r.Header.Get("X-API-Key")

	job, err := h.Store.GetExportJob(r.Context(), id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "export not found")
		return
	}

	if job.Status != store.ExportJobStatusCompleted {
		writeError(w, r, http.StatusBadRequest, CodeInvalidInput, "export is not completed")
		return
	}

	if job.ExpiresAt != nil && time.Now().After(*job.ExpiresAt) {
		writeError(w, r, http.StatusGone, "EXPIRED", "export download link has expired")
		return
	}

	if token == "" || !h.verifyExportToken(job.ID, job.UserID, token) {
		writeError(w, r, http.StatusForbidden, CodeForbidden, "invalid or expired download token")
		return
	}

	// Ensure another key cannot download this export if user is tied to key
	if callerKey != "" && job.UserID != "" && job.UserID != callerKey && !strings.Contains(job.UserID, callerKey) {
		writeError(w, r, http.StatusForbidden, CodeForbidden, "download URL cannot be used by another key")
		return
	}

	if job.ObjectKey == nil || *job.ObjectKey == "" {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "export data not found")
		return
	}

	data, err := h.objectStore().Get(r.Context(), *job.ObjectKey)
	if err != nil {
		h.logError("fetch export object", "err", err)
		writeError(w, r, http.StatusNotFound, CodeNotFound, "export file not found")
		return
	}

	contentType := "application/octet-stream"
	if job.Format == "csv" {
		contentType = "text/csv; charset=utf-8"
	} else if job.Format == "ndjson" {
		contentType = "application/x-ndjson"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"export_%s.%s\"", job.ID, job.Format))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) runExportJob(job store.ExportJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Update to processing
	job.Status = store.ExportJobStatusProcessing
	job.Progress = 10
	_ = h.Store.UpdateExportJob(ctx, job)

	var buf bytes.Buffer

	// Extract filters
	var filters struct {
		ContractID string `json:"contract_id"`
		Network    string `json:"network"`
		Type       string `json:"type"`
		From       uint32 `json:"from"`
		To         uint32 `json:"to"`
		Limit      int    `json:"limit"`
	}
	if len(job.Filters) > 0 {
		_ = json.Unmarshal(job.Filters, &filters)
	}

	if filters.Limit <= 0 || filters.Limit > MaxExportRows {
		filters.Limit = 1000
	}

	job.Progress = 40
	_ = h.Store.UpdateExportJob(ctx, job)

	if job.Type == "events" {
		f := store.EventFilters{
			Type:    filters.Type,
			Network: filters.Network,
			From:    filters.From,
			To:      filters.To,
		}

		if job.Format == "csv" {
			if err := h.Store.StreamEventsCSV(ctx, filters.ContractID, f, &buf); err != nil {
				msg := fmt.Sprintf("failed to stream events: %v", err)
				job.Status = store.ExportJobStatusFailed
				job.ErrorMessage = &msg
				_ = h.Store.UpdateExportJob(ctx, job)
				return
			}
		} else {
			events, _, err := h.Store.ListEvents(ctx, filters.ContractID, "", filters.Limit, f)
			if err != nil {
				msg := fmt.Sprintf("failed to list events: %v", err)
				job.Status = store.ExportJobStatusFailed
				job.ErrorMessage = &msg
				_ = h.Store.UpdateExportJob(ctx, job)
				return
			}
			for _, e := range events {
				line, _ := json.Marshal(e)
				buf.Write(line)
				buf.WriteByte('\n')
			}
		}
	} else {
		// Snapshot export
		contract, err := h.Store.GetContract(ctx, filters.ContractID)
		if err != nil {
			msg := fmt.Sprintf("failed to fetch contract snapshot: %v", err)
			job.Status = store.ExportJobStatusFailed
			job.ErrorMessage = &msg
			_ = h.Store.UpdateExportJob(ctx, job)
			return
		}
		data, _ := json.Marshal(contract)
		buf.Write(data)
	}

	objectKey := fmt.Sprintf("exports/%s/%s.%s", job.UserID, job.ID, job.Format)
	if err := h.objectStore().Put(ctx, objectKey, buf.Bytes()); err != nil {
		msg := fmt.Sprintf("failed to store export object: %v", err)
		job.Status = store.ExportJobStatusFailed
		job.ErrorMessage = &msg
		_ = h.Store.UpdateExportJob(ctx, job)
		return
	}

	expiresAt := time.Now().Add(DefaultExportTTL).UTC()
	token := h.signExportToken(job.ID, job.UserID, expiresAt.Unix())
	downloadURL := fmt.Sprintf("/api/v1/exports/%s/download?token=%s", job.ID, token)

	job.Status = store.ExportJobStatusCompleted
	job.Progress = 100
	job.ObjectKey = &objectKey
	job.DownloadURL = &downloadURL
	job.DownloadToken = &token
	job.ExpiresAt = &expiresAt

	_ = h.Store.UpdateExportJob(ctx, job)
}
