package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/sorolens/sorolens/apps/api/internal/simulator"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// defaultSimulationService backs handlers whose Handler was built without an
// explicit service (the common case). It caches identical simulations for 30s.
var defaultSimulationService = simulator.NewService(simulator.NewIndexedEngine(), simulator.DefaultCacheTTL)

// simulateRequest is the JSON body accepted by POST /api/v1/simulate.
//
// xdr (or its alias transaction_xdr) is a base64-encoded Soroban transaction
// envelope. network and ledger are optional: network defaults to the network of
// the first touched indexed contract and ledger defaults to the indexer's last
// synced ledger.
type simulateRequest struct {
	XDR            string  `json:"xdr"`
	TransactionXDR string  `json:"transaction_xdr"`
	Network        string  `json:"network"`
	Ledger         *uint32 `json:"ledger"`
	BudgetCPUInsn  int64   `json:"budget_cpu_insn"`
}

type simulateEventResponse struct {
	ContractID string   `json:"contract_id"`
	Ledger     uint32   `json:"ledger"`
	TxHash     string   `json:"tx_hash,omitempty"`
	Type       string   `json:"type,omitempty"`
	TopicXDR   []string `json:"topic_xdr,omitempty"`
	ValueXDR   string   `json:"value_xdr,omitempty"`
}

type simulateMeteringResponse struct {
	CPUInsn            int64 `json:"cpu_insn"`
	MemByte            int64 `json:"mem_byte"`
	LedgerReadByte     int64 `json:"ledger_read_byte"`
	LedgerWriteByte    int64 `json:"ledger_write_byte"`
	ResourceFeeCharged int64 `json:"resource_fee_charged"`
}

type simulateDiagnosticResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type simulateResponse struct {
	Status       string                      `json:"status"`
	Network      string                      `json:"network,omitempty"`
	Ledger       uint32                      `json:"ledger"`
	Contracts    []string                    `json:"contracts"`
	Cached       bool                        `json:"cached"`
	ReturnValue  any                         `json:"return_value,omitempty"`
	ReturnXDR    string                      `json:"return_xdr,omitempty"`
	Events       []simulateEventResponse     `json:"events"`
	Metering     simulateMeteringResponse    `json:"metering"`
	ReplayedFrom string                      `json:"replayed_from_tx_hash,omitempty"`
	Diagnostic   *simulateDiagnosticResponse `json:"diagnostic,omitempty"`
}

func (h *Handler) simulationService() *simulator.Service {
	if h.Simulator != nil {
		return h.Simulator
	}
	return defaultSimulationService
}

// Simulate handles POST /api/v1/simulate.
//
// It accepts an XDR-encoded transaction, loads the current storage snapshot of
// every indexed contract the transaction touches, executes the invocation
// against that snapshot, and returns the events, return value, and metering.
// Identical simulations are served from cache for 30 seconds, and failures are
// reported with a structured diagnostic.
func (h *Handler) Simulate(w http.ResponseWriter, r *http.Request) {
	var req simulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}

	xdr := strings.TrimSpace(req.XDR)
	if xdr == "" {
		xdr = strings.TrimSpace(req.TransactionXDR)
	}
	if xdr == "" {
		writeError(w, r, http.StatusUnprocessableEntity, simulator.CodeInvalidXDR,
			"xdr is required and must be a base64-encoded Soroban transaction envelope")
		return
	}
	if req.Network != "" && !validNetworks[req.Network] {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
			"network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}

	raw, err := simulator.DecodeTransactionXDR(xdr)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, simulator.CodeInvalidXDR,
			"could not decode the transaction envelope")
		return
	}
	candidates := simulator.ExtractContractCandidates(raw)
	if len(candidates) == 0 {
		writeError(w, r, http.StatusUnprocessableEntity, simulator.CodeNoIndexedContract,
			"the transaction does not reference any contract address")
		return
	}

	ledger := uint32(0)
	if req.Ledger != nil {
		ledger = *req.Ledger
	}
	network := req.Network

	ctx := r.Context()
	contracts := make([]simulator.ContractState, 0, len(candidates))
	for _, id := range candidates {
		c, err := h.Store.GetContract(ctx, id)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			h.Logger.Error("simulate get contract", "err", err, "contract_id", id)
			writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load contract state")
			return
		}
		if network == "" {
			network = c.Network
		}
		if ledger == 0 {
			if ss, err := h.Store.GetSyncState(ctx, id); err == nil && ss.LastLedger > ledger {
				ledger = ss.LastLedger
			}
		}

		entries, err := h.snapshotFor(ctx, id, ledger)
		if err != nil {
			h.Logger.Error("simulate snapshot", "err", err, "contract_id", id)
			writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load storage snapshot")
			return
		}
		invocations, _, err := h.Store.ListInvocations(ctx, id, "", 50, store.InvocationFilters{})
		if err != nil {
			h.Logger.Error("simulate invocations", "err", err, "contract_id", id)
			writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load invocation history")
			return
		}
		events, err := h.Store.RecentEvents(ctx, id, 50)
		if err != nil {
			h.Logger.Error("simulate events", "err", err, "contract_id", id)
			writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load recent events")
			return
		}
		contracts = append(contracts, simulator.ContractState{
			ContractID:  id,
			Storage:     entries,
			Invocations: invocations,
			Events:      events,
		})
	}

	if len(contracts) == 0 {
		writeError(w, r, http.StatusUnprocessableEntity, simulator.CodeNoIndexedContract,
			"none of the contracts referenced by the transaction are indexed")
		return
	}

	svc := h.simulationService()
	res, err := svc.Simulate(ctx, simulator.Input{
		Key:           svc.Key(network, strconv.FormatUint(uint64(ledger), 10), xdr, strconv.FormatInt(req.BudgetCPUInsn, 10)),
		Ledger:        ledger,
		BudgetCPUInsn: req.BudgetCPUInsn,
		Contracts:     contracts,
	})
	if err != nil {
		var failure *simulator.Failure
		if errors.As(err, &failure) {
			writeError(w, r, http.StatusUnprocessableEntity, failure.Code, failure.Message)
			return
		}
		h.Logger.Error("simulate", "err", err)
		writeError(w, r, http.StatusBadGateway, simulator.CodeSimulationFailed, "the simulation host failed")
		return
	}

	writeJSON(w, http.StatusOK, simulateResponseFromResult(res, network))
}

// snapshotFor loads the storage snapshot for a contract. When no ledger is
// pinned it returns the current entries, which is what "the latest indexed
// state" means for a dry run.
func (h *Handler) snapshotFor(ctx context.Context, contractID string, ledger uint32) ([]store.StorageEntry, error) {
	if ledger == 0 {
		entries, _, err := h.Store.ListStorageEntries(ctx, contractID, "", 200, store.StorageFilters{})
		return entries, err
	}
	return h.Store.GetStorageSnapshot(ctx, contractID, ledger)
}

func simulateResponseFromResult(res simulator.Result, network string) simulateResponse {
	events := make([]simulateEventResponse, len(res.Events))
	for i, e := range res.Events {
		events[i] = simulateEventResponse{
			ContractID: e.ContractID,
			Ledger:     e.Ledger,
			TxHash:     e.TxHash,
			Type:       e.Type,
			TopicXDR:   e.TopicXDR,
			ValueXDR:   e.ValueXDR,
		}
	}
	var diag *simulateDiagnosticResponse
	if res.Diagnostic != nil {
		diag = &simulateDiagnosticResponse{Code: res.Diagnostic.Code, Message: res.Diagnostic.Message}
	}
	return simulateResponse{
		Status:      res.Status,
		Network:     network,
		Ledger:      res.Ledger,
		Contracts:   res.Contracts,
		Cached:      res.Cached,
		ReturnValue: res.ReturnValue,
		ReturnXDR:   res.ReturnXDR,
		Events:      events,
		Metering: simulateMeteringResponse{
			CPUInsn:            res.Metering.CPUInsn,
			MemByte:            res.Metering.MemByte,
			LedgerReadByte:     res.Metering.LedgerReadByte,
			LedgerWriteByte:    res.Metering.LedgerWriteByte,
			ResourceFeeCharged: res.Metering.ResourceFeeCharged,
		},
		ReplayedFrom: res.ReplayedFrom,
		Diagnostic:   diag,
	}
}
