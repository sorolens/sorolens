package handler

import (
	"net/http"
	"strings"
)

type searchResultResponse struct {
	Type         string `json:"type"`
	ID           string `json:"id,omitempty"`
	Label        string `json:"label,omitempty"`
	Network      string `json:"network,omitempty"`
	ContractID   string `json:"contract_id,omitempty"`
	TxHash       string `json:"tx_hash,omitempty"`
	FunctionName string `json:"function_name,omitempty"`
}

// Search handles GET /api/v1/search?q=.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "q is required")
		return
	}

	results, err := h.Store.Search(r.Context(), query)
	if err != nil {
		h.Logger.Error("search", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to search")
		return
	}
	response := make([]searchResultResponse, 0, len(results))
	for _, result := range results {
		response = append(response, searchResultResponse{
			Type:         result.Type,
			ID:           result.ID,
			Label:        result.Label,
			Network:      result.Network,
			ContractID:   result.ContractID,
			TxHash:       result.TxHash,
			FunctionName: result.FunctionName,
		})
	}
	writeJSON(w, http.StatusOK, response)
}
