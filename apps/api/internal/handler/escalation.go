package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- request/response types -------------------------------------------------

type escalationStepRequest struct {
	Order      int      `json:"order"`
	Channel    string   `json:"channel"`
	Recipients []string `json:"recipients"`
	DelayMins  int      `json:"delay_mins"`
}

type createEscalationPolicyRequest struct {
	Name  string                   `json:"name"`
	Steps []escalationStepRequest  `json:"steps"`
}

type escalationPolicyResponse struct {
	ID        string                    `json:"id"`
	Name      string                    `json:"name"`
	Steps     []escalationStepResponse  `json:"steps"`
	CreatedAt string                    `json:"created_at"`
	UpdatedAt string                    `json:"updated_at"`
}

type escalationStepResponse struct {
	Order      int      `json:"order"`
	Channel    string   `json:"channel"`
	Recipients []string `json:"recipients"`
	DelayMins  int      `json:"delay_mins"`
}

func escalationPolicyFromStore(p store.EscalationPolicy) escalationPolicyResponse {
	steps := make([]escalationStepResponse, len(p.Steps))
	for i, s := range p.Steps {
		steps[i] = escalationStepResponse{
			Order:      s.Order,
			Channel:    s.Channel,
			Recipients: s.Recipients,
			DelayMins:  s.DelayMins,
		}
	}
	if steps == nil {
		steps = []escalationStepResponse{}
	}
	return escalationPolicyResponse{
		ID:        p.ID,
		Name:      p.Name,
		Steps:     steps,
		CreatedAt: p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt: p.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// ---- handlers ---------------------------------------------------------------

// ListEscalationPolicies handles GET /api/v1/escalation-policies.
func (h *Handler) ListEscalationPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := h.Store.ListEscalationPolicies(r.Context())
	if err != nil {
		h.Logger.Error("list escalation policies", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list escalation policies")
		return
	}
	resp := make([]escalationPolicyResponse, len(policies))
	for i, p := range policies {
		resp[i] = escalationPolicyFromStore(p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"escalation_policies": resp})
}

// CreateEscalationPolicy handles POST /api/v1/escalation-policies.
func (h *Handler) CreateEscalationPolicy(w http.ResponseWriter, r *http.Request) {
	var req createEscalationPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "name is required")
		return
	}
	steps := make([]store.EscalationStep, len(req.Steps))
	for i, s := range req.Steps {
		steps[i] = store.EscalationStep{
			Order:      s.Order,
			Channel:    s.Channel,
			Recipients: s.Recipients,
			DelayMins:  s.DelayMins,
		}
	}
	p, err := h.Store.CreateEscalationPolicy(r.Context(), store.EscalationPolicy{
		Name:  req.Name,
		Steps: steps,
	})
	if err != nil {
		h.Logger.Error("create escalation policy", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create escalation policy")
		return
	}
	writeJSON(w, http.StatusCreated, escalationPolicyFromStore(p))
}

// GetEscalationPolicy handles GET /api/v1/escalation-policies/{id}.
func (h *Handler) GetEscalationPolicy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.Store.GetEscalationPolicy(r.Context(), id)
	if errors.Is(err, store.ErrEscalationPolicyNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "escalation policy not found")
		return
	}
	if err != nil {
		h.Logger.Error("get escalation policy", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to get escalation policy")
		return
	}
	writeJSON(w, http.StatusOK, escalationPolicyFromStore(p))
}

// UpdateEscalationPolicy handles PUT /api/v1/escalation-policies/{id}.
func (h *Handler) UpdateEscalationPolicy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req createEscalationPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "name is required")
		return
	}
	steps := make([]store.EscalationStep, len(req.Steps))
	for i, s := range req.Steps {
		steps[i] = store.EscalationStep{
			Order:      s.Order,
			Channel:    s.Channel,
			Recipients: s.Recipients,
			DelayMins:  s.DelayMins,
		}
	}
	p, err := h.Store.UpdateEscalationPolicy(r.Context(), store.EscalationPolicy{
		ID:    id,
		Name:  req.Name,
		Steps: steps,
	})
	if errors.Is(err, store.ErrEscalationPolicyNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "escalation policy not found")
		return
	}
	if err != nil {
		h.Logger.Error("update escalation policy", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to update escalation policy")
		return
	}
	writeJSON(w, http.StatusOK, escalationPolicyFromStore(p))
}

// DeleteEscalationPolicy handles DELETE /api/v1/escalation-policies/{id}.
func (h *Handler) DeleteEscalationPolicy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Store.DeleteEscalationPolicy(r.Context(), id); err != nil {
		h.Logger.Error("delete escalation policy", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete escalation policy")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
