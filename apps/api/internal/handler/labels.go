package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type labelRequest struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Scope string `json:"scope"`
}

type labelResponse struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Scope string `json:"scope"`
}

func labelFromStore(label store.Label) labelResponse {
	scope := "workspace"
	if label.Public {
		scope = "public"
	}
	return labelResponse{Label: label.Label, Value: label.Value, Scope: scope}
}

// CreateLabel handles POST /api/v1/labels. Workspace labels are owned by the
// X-User-ID identity; public labels are curated by the same contributor route.
func (h *Handler) CreateLabel(w http.ResponseWriter, r *http.Request) {
	workspaceID := strings.TrimSpace(getUserID(r))
	if workspaceID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeInvalidInput, "X-User-ID header is required")
		return
	}
	var req labelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	req.Label, req.Value, req.Scope = strings.TrimSpace(req.Label), strings.TrimSpace(req.Value), strings.TrimSpace(req.Scope)
	if req.Label == "" || req.Value == "" || (req.Scope != "workspace" && req.Scope != "public") {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "label, value, and scope (workspace or public) are required")
		return
	}
	if len(req.Value) != 56 || (req.Value[0] != 'G' && req.Value[0] != 'C') {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "value must be a 56-character Stellar account or contract ID")
		return
	}
	label := store.Label{Label: req.Label, Value: req.Value, WorkspaceID: workspaceID, Public: req.Scope == "public"}
	if err := h.Store.UpsertLabel(r.Context(), label); err != nil {
		h.Logger.Error("upsert label", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to save label")
		return
	}
	writeJSON(w, http.StatusCreated, labelFromStore(label))
}

// ListLabels handles GET /api/v1/labels and only includes public labels plus
// labels belonging to the caller's workspace.
func (h *Handler) ListLabels(w http.ResponseWriter, r *http.Request) {
	labels, err := h.Store.ListLabels(r.Context(), strings.TrimSpace(getUserID(r)), strings.TrimSpace(r.URL.Query().Get("query")))
	if err != nil {
		h.Logger.Error("list labels", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list labels")
		return
	}
	resp := make([]labelResponse, len(labels))
	for i, label := range labels {
		resp[i] = labelFromStore(label)
	}
	writeJSON(w, http.StatusOK, map[string]any{"labels": resp})
}

// ResolveLabel handles GET /api/v1/resolve?query= and resolves only public
// labels or labels owned by the caller's workspace.
func (h *Handler) ResolveLabel(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "query is required")
		return
	}
	if len(query) == 56 && (query[0] == 'G' || query[0] == 'C') {
		writeJSON(w, http.StatusOK, labelResponse{Value: query, Label: query, Scope: "raw"})
		return
	}
	label, err := h.Store.ResolveLabel(r.Context(), strings.TrimSpace(getUserID(r)), query)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "label not found")
		return
	}
	if err != nil {
		h.Logger.Error("resolve label", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to resolve label")
		return
	}
	writeJSON(w, http.StatusOK, labelFromStore(label))
}
