package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type GraphNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Count  int    `json:"count"`
}

var contractIDRegex = regexp.MustCompile(`^C[A-Z2-7]{55}$`)

func isValidContractID(id string) bool {
	return contractIDRegex.MatchString(id)
}

func (h *Handler) ContractGraph(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	if contractID == "" {
		writeError(w, r, http.StatusBadRequest, CodeInvalidInput, "missing contract id")
		return
	}

	if !isValidContractID(contractID) {
		writeError(w, r, http.StatusBadRequest, CodeInvalidInput, "invalid contract id")
		return
	}

	// Ensure the contract exists
	_, err := h.Store.GetContract(r.Context(), contractID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
		return
	}

	edgesMap := make(map[string]int)
	nodesMap := make(map[string]bool)
	nodesMap[contractID] = true

	// Paginate through invocations up to a reasonable limit (e.g. 1000)
	cursor := ""
	fetched := 0
	limit := 100
	maxFetch := 1000

	for {
		if fetched >= maxFetch {
			break
		}

		invocations, nextCursor, err := h.Store.ListInvocations(r.Context(), contractID, cursor, limit, store.InvocationFilters{})
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, CodeInternal, err.Error())
			return
		}

		if len(invocations) == 0 {
			break
		}

		for _, inv := range invocations {
			// Add nil/type checks for ArgsDecoded
			if inv.ArgsDecoded == nil {
				continue
			}

			// In a real scenario, this involves analyzing the deep execution trace.
			// Limitation: As we only have decoded arguments, we use a heuristic approach
			// to detect cross-contract calls. This might miss dynamic invocations or
			// misidentify IDs passed as data.
			for _, arg := range inv.ArgsDecoded {
				if strArg, ok := arg.(string); ok && isValidContractID(strArg) && strArg != contractID {
					edgesMap[contractID+"|"+strArg]++
					nodesMap[strArg] = true
				}
			}

			// Limitation: Detecting inbound calls via callback/receive function names is a heuristic.
			// A true inbound edge detection requires caller context from the invocation trace.
			funcName := strings.ToLower(inv.FunctionName)
			if strings.Contains(funcName, "callback") || strings.Contains(funcName, "receive") {
				for _, arg := range inv.ArgsDecoded {
					if strArg, ok := arg.(string); ok && isValidContractID(strArg) && strArg != contractID {
						edgesMap[strArg+"|"+contractID]++
						nodesMap[strArg] = true
					}
				}
			}
		}

		fetched += len(invocations)
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	var nodes []GraphNode
	for id := range nodesMap {
		nodes = append(nodes, GraphNode{ID: id, Label: id[:6] + "..." + id[len(id)-4:]})
	}

	var edges []GraphEdge
	for k, count := range edgesMap {
		parts := strings.Split(k, "|")
		edges = append(edges, GraphEdge{Source: parts[0], Target: parts[1], Count: count})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
		"edges": edges,
		"metadata": map[string]interface{}{
			"heuristic_used": true,
			"warning":        "Edges are inferred from argument inspection due to lack of deep trace context.",
		},
	})
}
