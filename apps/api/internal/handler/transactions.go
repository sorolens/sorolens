package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// transactionResponse is the body of GET /api/v1/transactions/{hash}.
type transactionResponse struct {
	TxHash     string              `json:"tx_hash"`
	Ledger     uint32              `json:"ledger"`
	Status     string              `json:"status,omitempty"`
	Network    string              `json:"network,omitempty"`
	Invocation *invocationResponse `json:"invocation,omitempty"`
	Events     []eventResponse     `json:"events"`
	Trace      *traceResponse      `json:"trace,omitempty"`
}

// ledgerResponse is the body of GET /api/v1/ledgers/{sequence}.
type ledgerResponse struct {
	Sequence        uint32                           `json:"sequence"`
	ClosedAt        string                           `json:"closed_at,omitempty"`
	TxCount         int64                            `json:"tx_count"`
	InvocationCount int64                            `json:"invocation_count"`
	FailedCount     int64                            `json:"failed_count"`
	EventCount      int64                            `json:"event_count"`
	TopContracts    []contractActivitySummaryResponse `json:"top_contracts"`
}

type contractActivitySummaryResponse struct {
	ContractID  string `json:"contract_id"`
	EventCount  int64  `json:"event_count"`
	InvokeCount int64  `json:"invocation_count"`
}

// GetTransaction handles GET /api/v1/transactions/{hash}.
// It returns the invocation, all events, and cross-contract call trace for
// the transaction in a single response. Any component that is not yet indexed
// is omitted; a 404 is returned only when no data at all exists for the hash.
func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	txHash := strings.ToLower(chi.URLParam(r, "hash"))
	if !txHashRegex.MatchString(txHash) {
		writeError(w, r, http.StatusBadRequest, CodeInvalidInput, "invalid transaction hash")
		return
	}

	ctx := r.Context()

	// Load all three data sources; partial results are still served.
	inv, invErr := h.Store.GetInvocation(ctx, txHash)
	missingInv := errors.Is(invErr, store.ErrNotFound)
	if invErr != nil && !missingInv {
		h.Logger.Error("get transaction invocation", "err", invErr, "tx_hash", txHash)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load invocation")
		return
	}

	edges, edgesErr := h.Store.GetCallEdges(ctx, txHash)
	if edgesErr != nil {
		h.Logger.Error("get transaction call edges", "err", edgesErr, "tx_hash", txHash)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load call graph")
		return
	}

	events, eventsErr := h.Store.GetEventsByTxHash(ctx, txHash)
	if eventsErr != nil {
		h.Logger.Error("get transaction events", "err", eventsErr, "tx_hash", txHash)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load events")
		return
	}

	if missingInv && len(edges) == 0 && len(events) == 0 {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "transaction not found")
		return
	}

	resp := transactionResponse{
		TxHash: txHash,
		Events: make([]eventResponse, 0, len(events)),
	}

	if !missingInv {
		ir := invocationFromStore(inv)
		resp.Invocation = &ir
		resp.Ledger = inv.Ledger
		resp.Status = inv.Status
		resp.Network = inv.Network
	}

	for _, e := range events {
		resp.Events = append(resp.Events, eventFromStore(e))
		if resp.Ledger == 0 {
			resp.Ledger = e.Ledger
		}
	}

	// Build call trace if edges or invocation exist.
	root := &traceNode{SpanID: "0"}
	if !missingInv {
		root.ContractID = inv.ContractID
		root.FunctionName = inv.FunctionName
		root.CPU = inv.CPUInsn
		root.Mem = inv.MemByte
		root.FeeShare = inv.ResourceFeeCharged
	}
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
			truncated = true
			root.Children = append(root.Children, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	if !missingInv || len(edges) > 0 {
		trace := &traceResponse{
			TxHash:    txHash,
			Root:      root,
			EdgeCount: len(edges),
			HasEdges:  len(edges) > 0,
			Truncated: truncated,
		}
		if !missingInv {
			trace.Status = inv.Status
			trace.Network = inv.Network
			trace.Ledger = inv.Ledger
		}
		resp.Trace = trace
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetLedger handles GET /api/v1/ledgers/{sequence}.
// It returns aggregated statistics for the ledger: close time, tx count,
// invocation count, failed invocations, event count, and the top contracts by
// event volume.
func (h *Handler) GetLedger(w http.ResponseWriter, r *http.Request) {
	seqStr := chi.URLParam(r, "sequence")
	seq64, err := strconv.ParseUint(seqStr, 10, 32)
	if err != nil || seq64 == 0 {
		writeError(w, r, http.StatusBadRequest, CodeInvalidInput, "sequence must be a positive integer")
		return
	}
	sequence := uint32(seq64)

	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}

	sum, err := h.Store.GetLedgerSummary(r.Context(), sequence, network)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "ledger not found")
		return
	}
	if err != nil {
		h.Logger.Error("get ledger summary", "err", err, "sequence", sequence)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load ledger")
		return
	}

	top := make([]contractActivitySummaryResponse, len(sum.TopContracts))
	for i, c := range sum.TopContracts {
		top[i] = contractActivitySummaryResponse{
			ContractID:  c.ContractID,
			EventCount:  c.EventCount,
			InvokeCount: c.InvokeCount,
		}
	}

	var closedAt string
	if !sum.ClosedAt.IsZero() {
		closedAt = sum.ClosedAt.UTC().Format("2006-01-02T15:04:05Z")
	}

	writeJSON(w, http.StatusOK, ledgerResponse{
		Sequence:        sum.Sequence,
		ClosedAt:        closedAt,
		TxCount:         sum.TxCount,
		InvocationCount: sum.InvocationCount,
		FailedCount:     sum.FailedCount,
		EventCount:      sum.EventCount,
		TopContracts:    top,
	})
}
