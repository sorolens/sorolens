package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- response types ---------------------------------------------------------

type groupResponse struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type groupStatsResponse struct {
	GroupID            string  `json:"group_id"`
	ContractCount      int64   `json:"contract_count"`
	EventCount         int64   `json:"event_count"`
	InvocationCount    int64   `json:"invocation_count"`
	StorageEntryCount  int64   `json:"storage_entry_count"`
	AverageHealthScore float64 `json:"average_health_score"`
}

type groupSummaryResponse struct {
	groupResponse
	Stats groupStatsResponse `json:"stats"`
}

type groupContractResponse struct {
	ContractID     string     `json:"contract_id"`
	Network        string     `json:"network"`
	Label          string     `json:"label"`
	Status         string     `json:"status"`
	HealthScore    *int32     `json:"health_score"`
	LastActivityAt *time.Time `json:"last_activity_at"`
}

type groupDetailResponse struct {
	groupResponse
	Contracts []groupContractResponse `json:"contracts"`
}

type groupDeletedResponse struct {
	Deleted bool `json:"deleted"`
}

type groupMembershipResponse struct {
	GroupID    string `json:"group_id"`
	ContractID string `json:"contract_id"`
}

// ---- helpers ----------------------------------------------------------------

func groupFromStore(g store.Group) groupResponse {
	return groupResponse{
		ID:        g.ID,
		OwnerID:   g.OwnerID,
		Name:      g.Name,
		CreatedAt: g.CreatedAt,
	}
}

func groupStatsFromStore(s store.GroupStats) groupStatsResponse {
	return groupStatsResponse{
		GroupID:            s.GroupID,
		ContractCount:      s.ContractCount,
		EventCount:         s.EventCount,
		InvocationCount:    s.InvocationCount,
		StorageEntryCount:  s.StorageEntryCount,
		AverageHealthScore: s.AverageHealthScore,
	}
}

func groupContractFromStore(c store.GroupContract) groupContractResponse {
	return groupContractResponse{
		ContractID:     c.ContractID,
		Network:        c.Network,
		Label:          c.Label,
		Status:         c.Status,
		HealthScore:    c.HealthScore,
		LastActivityAt: c.LastActivityAt,
	}
}

// requireGroupUser extracts the owner identity for group routes. Group
// ownership uses the same X-User-ID header contract as the watchlist.
func requireGroupUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID := getUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeInvalidInput, "X-User-ID header is required")
		return "", false
	}
	return userID, true
}

// writeGroupError maps store errors to the API error envelope and reports
// whether it handled the error.
func (h *Handler) writeGroupError(w http.ResponseWriter, r *http.Request, err error, notFoundMsg string) bool {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, r, http.StatusNotFound, CodeNotFound, notFoundMsg)
		return true
	case errors.Is(err, store.ErrInvalidGroupName):
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "name is required and must be at most 100 characters")
		return true
	default:
		return false
	}
}

// decodeGroupContractID reads the target contract from the {contractId} path
// segment when present, falling back to a {"contract_id": "..."} JSON body so
// the documented DELETE /groups/{id}/contracts form works too.
func decodeGroupContractID(r *http.Request) (string, bool) {
	if id := chi.URLParam(r, "contractId"); id != "" {
		return id, true
	}
	var req struct {
		ContractID string `json:"contract_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return "", false
	}
	if req.ContractID == "" {
		return "", false
	}
	return req.ContractID, true
}

// ---- handlers ---------------------------------------------------------------

// CreateGroup handles POST /api/v1/groups.
// Body: {"name": "..."}
func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireGroupUser(w, r)
	if !ok {
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}

	g, err := h.Store.CreateGroup(r.Context(), userID, req.Name)
	if err != nil {
		if h.writeGroupError(w, r, err, "group not found") {
			return
		}
		h.Logger.Error("create group", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create group")
		return
	}
	writeJSON(w, http.StatusCreated, groupFromStore(g))
}

// ListGroups handles GET /api/v1/groups. Each group carries the aggregate
// statistics across its member contracts so the list page renders stat cards
// without a request per group.
func (h *Handler) ListGroups(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireGroupUser(w, r)
	if !ok {
		return
	}

	summaries, err := h.Store.ListGroups(r.Context(), userID)
	if err != nil {
		h.Logger.Error("list groups", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list groups")
		return
	}

	resp := make([]groupSummaryResponse, len(summaries))
	for i, s := range summaries {
		resp[i] = groupSummaryResponse{
			groupResponse: groupFromStore(s.Group),
			Stats:         groupStatsFromStore(store.GroupStats{
				GroupID:            s.ID,
				ContractCount:      s.ContractCount,
				EventCount:         s.EventCount,
				InvocationCount:    s.InvocationCount,
				StorageEntryCount:  s.StorageEntryCount,
				AverageHealthScore: s.AverageHealthScore,
			}),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": resp})
}

// GetGroup handles GET /api/v1/groups/{id}. It returns the group together with
// its member contracts and their per-contract health/activity signals.
func (h *Handler) GetGroup(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireGroupUser(w, r)
	if !ok {
		return
	}
	groupID := chi.URLParam(r, "id")

	g, err := h.Store.GetGroup(r.Context(), userID, groupID)
	if err != nil {
		if h.writeGroupError(w, r, err, "group not found") {
			return
		}
		h.Logger.Error("get group", "err", err, "group_id", groupID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch group")
		return
	}

	contracts, err := h.Store.ListGroupContracts(r.Context(), userID, groupID)
	if err != nil {
		if h.writeGroupError(w, r, err, "group not found") {
			return
		}
		h.Logger.Error("list group contracts", "err", err, "group_id", groupID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch group contracts")
		return
	}

	resp := make([]groupContractResponse, len(contracts))
	for i, c := range contracts {
		resp[i] = groupContractFromStore(c)
	}
	writeJSON(w, http.StatusOK, groupDetailResponse{
		groupResponse: groupFromStore(g),
		Contracts:     resp,
	})
}

// UpdateGroup handles PATCH /api/v1/groups/{id}.
// Body: {"name": "..."}
func (h *Handler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireGroupUser(w, r)
	if !ok {
		return
	}
	groupID := chi.URLParam(r, "id")

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}

	g, err := h.Store.UpdateGroup(r.Context(), userID, groupID, req.Name)
	if err != nil {
		if h.writeGroupError(w, r, err, "group not found") {
			return
		}
		h.Logger.Error("update group", "err", err, "group_id", groupID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to update group")
		return
	}
	writeJSON(w, http.StatusOK, groupFromStore(g))
}

// DeleteGroup handles DELETE /api/v1/groups/{id}.
func (h *Handler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireGroupUser(w, r)
	if !ok {
		return
	}
	groupID := chi.URLParam(r, "id")

	if err := h.Store.DeleteGroup(r.Context(), userID, groupID); err != nil {
		if h.writeGroupError(w, r, err, "group not found") {
			return
		}
		h.Logger.Error("delete group", "err", err, "group_id", groupID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete group")
		return
	}
	writeJSON(w, http.StatusOK, groupDeletedResponse{Deleted: true})
}

// AddGroupContract handles POST /api/v1/groups/{id}/contracts.
// Body: {"contract_id": "..."}
func (h *Handler) AddGroupContract(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireGroupUser(w, r)
	if !ok {
		return
	}
	groupID := chi.URLParam(r, "id")

	var req struct {
		ContractID string `json:"contract_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if req.ContractID == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "contract_id is required")
		return
	}

	// Surface a missing contract as 404 rather than letting the membership
	// foreign key fail as a 500.
	if _, err := h.Store.GetContract(r.Context(), req.ContractID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
			return
		}
		h.Logger.Error("add to group: load contract", "err", err, "contract_id", req.ContractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load contract")
		return
	}

	if err := h.Store.AddContractToGroup(r.Context(), userID, groupID, req.ContractID); err != nil {
		if h.writeGroupError(w, r, err, "group not found") {
			return
		}
		h.Logger.Error("add contract to group", "err", err, "group_id", groupID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to add contract to group")
		return
	}
	writeJSON(w, http.StatusCreated, groupMembershipResponse{GroupID: groupID, ContractID: req.ContractID})
}

// RemoveGroupContract handles DELETE /api/v1/groups/{id}/contracts and
// DELETE /api/v1/groups/{id}/contracts/{contractId}. The path-segment form is
// preferred by the UI; the body form matches the documented endpoint.
func (h *Handler) RemoveGroupContract(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireGroupUser(w, r)
	if !ok {
		return
	}
	groupID := chi.URLParam(r, "id")

	contractID, ok := decodeGroupContractID(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "contract_id is required")
		return
	}

	if err := h.Store.RemoveContractFromGroup(r.Context(), userID, groupID, contractID); err != nil {
		if h.writeGroupError(w, r, err, "group not found") {
			return
		}
		h.Logger.Error("remove contract from group", "err", err, "group_id", groupID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to remove contract from group")
		return
	}
	writeJSON(w, http.StatusOK, groupMembershipResponse{GroupID: groupID, ContractID: contractID})
}

// GroupStats handles GET /api/v1/groups/{id}/stats. It aggregates event,
// invocation, and storage counts plus the average cached health score across
// every contract in the group in a single query.
func (h *Handler) GroupStats(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireGroupUser(w, r)
	if !ok {
		return
	}
	groupID := chi.URLParam(r, "id")

	stats, err := h.Store.GetGroupStats(r.Context(), userID, groupID)
	if err != nil {
		if h.writeGroupError(w, r, err, "group not found") {
			return
		}
		h.Logger.Error("group stats", "err", err, "group_id", groupID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch group stats")
		return
	}
	writeJSON(w, http.StatusOK, groupStatsFromStore(stats))
}
