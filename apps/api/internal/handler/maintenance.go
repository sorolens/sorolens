package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type createMaintenanceWindowRequest struct {
	Name                  string    `json:"name"`
	Scope                 string    `json:"scope"`
	TargetID              string    `json:"target_id,omitempty"`
	StartTime             time.Time `json:"start_time"`
	EndTime               time.Time `json:"end_time"`
	Timezone              string    `json:"timezone,omitempty"`
	Recurrence            string    `json:"recurrence,omitempty"`
	SuppressNotifications *bool     `json:"suppress_notifications,omitempty"`
}

type updateMaintenanceWindowRequest struct {
	Name                  string    `json:"name"`
	StartTime             time.Time `json:"start_time"`
	EndTime               time.Time `json:"end_time"`
	Timezone              string    `json:"timezone,omitempty"`
	Recurrence            string    `json:"recurrence,omitempty"`
	SuppressNotifications *bool     `json:"suppress_notifications,omitempty"`
}

// CreateMaintenanceWindow handles POST /api/v1/maintenance-windows (issue #790).
func (h *Handler) CreateMaintenanceWindow(w http.ResponseWriter, r *http.Request) {
	var req createMaintenanceWindowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid request body")
		return
	}

	if req.Name == "" || req.StartTime.IsZero() || req.EndTime.IsZero() {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "name, start_time, and end_time are required")
		return
	}
	if !req.EndTime.After(req.StartTime) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "end_time must be after start_time")
		return
	}
	if req.Scope == "" {
		req.Scope = "global"
	}
	if req.Scope != "global" && req.Scope != "contract" && req.Scope != "group" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "scope must be global, contract, or group")
		return
	}
	if (req.Scope == "contract" || req.Scope == "group") && req.TargetID == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "target_id required for contract or group scope")
		return
	}

	suppress := true
	if req.SuppressNotifications != nil {
		suppress = *req.SuppressNotifications
	}

	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid timezone")
		return
	}

	recurrence := req.Recurrence
	if recurrence == "" {
		recurrence = "none"
	}

	win := store.MaintenanceWindow{
		Name:                  req.Name,
		Scope:                 req.Scope,
		TargetID:              req.TargetID,
		StartTime:             req.StartTime,
		EndTime:               req.EndTime,
		Timezone:              tz,
		Recurrence:            recurrence,
		SuppressNotifications: suppress,
		CreatedBy:             middleware.Actor(r),
	}

	created, err := h.Store.CreateMaintenanceWindow(r.Context(), win)
	if err != nil {
		if errors.Is(err, store.ErrInvalidMaintenanceWindow) {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
			return
		}
		h.Logger.Error("create maintenance window", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create maintenance window")
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// GetMaintenanceWindow handles GET /api/v1/maintenance-windows/{id} (issue #790).
func (h *Handler) GetMaintenanceWindow(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid maintenance window id")
		return
	}

	win, err := h.Store.GetMaintenanceWindow(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrMaintenanceWindowNotFound) || errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "maintenance window not found")
			return
		}
		h.Logger.Error("get maintenance window", "id", id, "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch maintenance window")
		return
	}

	writeJSON(w, http.StatusOK, win)
}

// ListMaintenanceWindows handles GET /api/v1/maintenance-windows (issue #790).
func (h *Handler) ListMaintenanceWindows(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	targetID := r.URL.Query().Get("target_id")

	windows, err := h.Store.ListMaintenanceWindows(r.Context(), scope, targetID)
	if err != nil {
		h.Logger.Error("list maintenance windows", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list maintenance windows")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"maintenance_windows": windows,
	})
}

// UpdateMaintenanceWindow handles PUT /api/v1/maintenance-windows/{id} (issue #790).
func (h *Handler) UpdateMaintenanceWindow(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid maintenance window id")
		return
	}

	var req updateMaintenanceWindowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid request body")
		return
	}

	if req.Name == "" || req.StartTime.IsZero() || req.EndTime.IsZero() {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "name, start_time, and end_time are required")
		return
	}
	if !req.EndTime.After(req.StartTime) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "end_time must be after start_time")
		return
	}

	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid timezone")
		return
	}

	recurrence := req.Recurrence
	if recurrence == "" {
		recurrence = "none"
	}

	suppress := true
	if req.SuppressNotifications != nil {
		suppress = *req.SuppressNotifications
	}

	win := store.MaintenanceWindow{
		ID:                    id,
		Name:                  req.Name,
		StartTime:             req.StartTime,
		EndTime:               req.EndTime,
		Timezone:              tz,
		Recurrence:            recurrence,
		SuppressNotifications: suppress,
	}

	updated, err := h.Store.UpdateMaintenanceWindow(r.Context(), win)
	if err != nil {
		if errors.Is(err, store.ErrMaintenanceWindowNotFound) || errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "maintenance window not found")
			return
		}
		if errors.Is(err, store.ErrInvalidMaintenanceWindow) {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
			return
		}
		h.Logger.Error("update maintenance window", "id", id, "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to update maintenance window")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// DeleteMaintenanceWindow handles DELETE /api/v1/maintenance-windows/{id} (issue #790).
func (h *Handler) DeleteMaintenanceWindow(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid maintenance window id")
		return
	}

	if err := h.Store.DeleteMaintenanceWindow(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrMaintenanceWindowNotFound) || errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "maintenance window not found")
			return
		}
		h.Logger.Error("delete maintenance window", "id", id, "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete maintenance window")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
