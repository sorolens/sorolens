package handler

import (
	"encoding/json"
	"net/http"
	"strings"
)

// maxBatchContracts caps how many contracts one bulk action may target, so a
// single request cannot fan out into an unbounded number of row deletions.
const maxBatchContracts = 100

// Batch action names accepted by BatchContracts.
const (
	batchActionUntrack = "untrack"
	batchActionTag     = "tag"
)

type batchContractsArgs struct {
	// Label is the tag to apply when action is "tag". It is a pointer so a
	// missing value can be distinguished from an explicitly empty one.
	Label *string `json:"label"`
}

type batchContractsRequest struct {
	IDs    []string           `json:"ids"`
	Action string             `json:"action"`
	Args   batchContractsArgs `json:"args"`
}

type batchContractsResponse struct {
	Action    string `json:"action"`
	Requested int    `json:"requested"`
	Affected  int64  `json:"affected"`
}

// BatchContracts handles POST /api/v1/contracts/batch.
//
// It applies one action to many contracts at once:
//
//	{"ids": ["C…"], "action": "untrack"}
//	{"ids": ["C…"], "action": "tag", "args": {"label": "payments"}}
//
// "untrack" permanently removes the contracts and their indexed data; "tag"
// overwrites the label on each contract. Both return how many contracts were
// affected. Unknown IDs are ignored rather than failing the whole batch, so a
// stale selection (for example after a contract was already untracked) still
// succeeds for the remaining contracts.
func (h *Handler) BatchContracts(w http.ResponseWriter, r *http.Request) {
	var req batchContractsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "ids must contain at least one contract")
		return
	}
	if len(req.IDs) > maxBatchContracts {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "ids must contain at most 100 contracts")
		return
	}

	// Validate every ID up front and de-duplicate while preserving order, so a
	// malformed ID cannot silently apply a partial action.
	seen := make(map[string]bool, len(req.IDs))
	ids := make([]string, 0, len(req.IDs))
	for _, raw := range req.IDs {
		id := strings.TrimSpace(raw)
		if !validateContractID(id) {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "each id must be a 56-character string starting with 'C'")
			return
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	switch action {
	case batchActionUntrack:
		affected, err := h.Store.DeleteContracts(r.Context(), ids)
		if err != nil {
			h.Logger.Error("batch untrack contracts", "err", err)
			writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to untrack contracts")
			return
		}
		writeJSON(w, http.StatusOK, batchContractsResponse{
			Action:    action,
			Requested: len(ids),
			Affected:  affected,
		})

	case batchActionTag:
		if req.Args.Label == nil {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "args.label is required for action 'tag'")
			return
		}
		label := strings.TrimSpace(*req.Args.Label)
		if label == "" {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "args.label must not be empty")
			return
		}
		affected, err := h.Store.SetContractLabel(r.Context(), ids, label)
		if err != nil {
			h.Logger.Error("batch tag contracts", "err", err)
			writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to tag contracts")
			return
		}
		writeJSON(w, http.StatusOK, batchContractsResponse{
			Action:    action,
			Requested: len(ids),
			Affected:  affected,
		})

	default:
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "action must be one of: untrack, tag")
	}
}
