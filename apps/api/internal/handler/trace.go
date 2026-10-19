package handler

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// traceNode is one frame of a transaction's cross-contract call tree.
//
// span_id is a deterministic call-path string ("0", "0.0", "0.1", "0.0.0"). The
// root node always carries span_id "0" and is backed by the transaction's row in
// the invocations table; every other node is backed by a call_edges row.
type traceNode struct {
	SpanID       string       `json:"span_id"`
	ParentSpanID string       `json:"parent_span_id,omitempty"`
	ContractID   string       `json:"contract_id,omitempty"`
	FunctionName string       `json:"function_name,omitempty"`
	CPU          int64        `json:"cpu"`
	Mem          int64        `json:"mem"`
	FeeShare     int64        `json:"fee_share"`
	Depth        int          `json:"depth"`
	Children     []*traceNode `json:"children"`
}

// traceResponse is the body of GET /api/v1/invocations/{tx_hash}/trace.
type traceResponse struct {
	TxHash    string     `json:"tx_hash"`
	Status    string     `json:"status,omitempty"`
	Network   string     `json:"network,omitempty"`
	Ledger    uint32     `json:"ledger"`
	Root      *traceNode `json:"root"`
	EdgeCount int        `json:"edge_count"`
	// HasEdges is false when the transaction has no recorded cross-contract
	// calls: either it made none, or the RPC node that served it does not
	// expose diagnostic events. The dashboard uses it to explain the empty
	// flame graph instead of rendering a bare root bar.
	HasEdges bool `json:"has_edges"`
	// Truncated is true when the depth/edge caps dropped frames, or when an
	// edge referenced a parent that was not in the result set.
	Truncated bool `json:"truncated"`
}

// txHashRegex matches a Soroban transaction hash: 64 hex characters.
var txHashRegex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// GetInvocationTrace returns the cross-contract call tree of a transaction.
//
// The tree is assembled from the transaction's call_edges rows and rooted at its
// invocations row, so the same transaction that the invocations list already
// shows links straight to a flame graph.
func (h *Handler) GetInvocationTrace(w http.ResponseWriter, r *http.Request) {
	txHash := strings.ToLower(chi.URLParam(r, "tx_hash"))
	if !txHashRegex.MatchString(txHash) {
		writeError(w, r, http.StatusBadRequest, CodeInvalidInput, "invalid transaction hash")
		return
	}

	ctx := r.Context()

	edges, err := h.Store.GetCallEdges(ctx, txHash)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load call graph")
		return
	}

	inv, invErr := h.Store.GetInvocation(ctx, txHash)
	missingInvocation := errors.Is(invErr, store.ErrNotFound)
	if invErr != nil && !missingInvocation {
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load invocation")
		return
	}
	if missingInvocation && len(edges) == 0 {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "transaction not found")
		return
	}

	root := &traceNode{SpanID: "0"}
	if !missingInvocation {
		root.ContractID = inv.ContractID
		root.FunctionName = inv.FunctionName
		root.CPU = inv.CPUInsn
		root.Mem = inv.MemByte
		root.FeeShare = inv.ResourceFeeCharged
	}

	// span ids sort so that a parent always precedes its children (a parent id
	// is a proper prefix of its children's), which is exactly the order
	// GetCallEdges returns.
	nodes := map[string]*traceNode{"0": root}
	truncated := false
	for _, e := range edges {
		node := &traceNode{
			SpanID:       e.ChildSpanID,
			ParentSpanID: e.ParentSpanID,
			ContractID:   e.CalleeContractID,
			FunctionName: e.FunctionName,
			CPU:          e.CPU,
			Mem:          e.Mem,
			FeeShare:     e.FeeShare,
			Depth:        e.Depth,
		}
		nodes[e.ChildSpanID] = node

		parent, ok := nodes[e.ParentSpanID]
		if !ok {
			// A truncated tree can leave a frame whose parent edge was never
			// written; hang it off the root so the data is still visible.
			truncated = true
			root.Children = append(root.Children, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}

	resp := traceResponse{
		TxHash:    txHash,
		Root:      root,
		EdgeCount: len(edges),
		HasEdges:  len(edges) > 0,
		Truncated: truncated,
	}
	if !missingInvocation {
		resp.Status = inv.Status
		resp.Network = inv.Network
		resp.Ledger = inv.Ledger
	}

	writeJSON(w, http.StatusOK, resp)
}
