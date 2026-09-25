package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- response types ---------------------------------------------------------

type monitoredContractResponse struct {
	ContractID    string     `json:"contract_id"`
	Network       string     `json:"network"`
	Name          string     `json:"name"`
	Owner         string     `json:"owner"`
	Status        string     `json:"status"`
	LastCheck     *time.Time `json:"last_check"`
	CheckInterval int64      `json:"check_interval"`
	RegisteredAt  time.Time  `json:"registered_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type healthCheckResponse struct {
	ContractID string    `json:"contract_id"`
	Status     string    `json:"status"`
	Metadata   string    `json:"metadata"`
	Ledger     int64     `json:"ledger"`
	TxHash     string    `json:"tx_hash"`
	Timestamp  time.Time `json:"timestamp"`
}

type contractAlertResponse struct {
	ContractID string    `json:"contract_id"`
	Severity   string    `json:"severity"`
	Message    string    `json:"message"`
	Ledger     int64     `json:"ledger"`
	TxHash     string    `json:"tx_hash"`
	Timestamp  time.Time `json:"timestamp"`
}

type watchdogStatsResponse struct {
	TotalMonitored int64 `json:"total_monitored"`
	Healthy        int64 `json:"healthy"`
	Degraded       int64 `json:"degraded"`
	Unresponsive   int64 `json:"unresponsive"`
	TotalAlerts    int64 `json:"total_alerts"`
	CriticalAlerts int64 `json:"critical_alerts"`
}

// ---- converters -------------------------------------------------------------

func monitoredFromStore(m store.MonitoredContract) monitoredContractResponse {
	return monitoredContractResponse{
		ContractID:    m.ContractID,
		Network:       m.Network,
		Name:          m.Name,
		Owner:         m.Owner,
		Status:        m.Status,
		LastCheck:     m.LastCheck,
		CheckInterval: m.CheckInterval,
		RegisteredAt:  m.RegisteredAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func healthCheckFromStore(h store.HealthCheck) healthCheckResponse {
	return healthCheckResponse{
		ContractID: h.ContractID,
		Status:     h.Status,
		Metadata:   h.Metadata,
		Ledger:     h.Ledger,
		TxHash:     h.TxHash,
		Timestamp:  h.Timestamp,
	}
}

func alertFromStore(a store.ContractAlert) contractAlertResponse {
	return contractAlertResponse{
		ContractID: a.ContractID,
		Severity:   a.Severity,
		Message:    a.Message,
		Ledger:     a.Ledger,
		TxHash:     a.TxHash,
		Timestamp:  a.Timestamp,
	}
}

// ---- handlers ---------------------------------------------------------------

// ListMonitoredContracts handles GET /api/v1/watchdog/contracts.
func (h *Handler) ListMonitoredContracts(w http.ResponseWriter, r *http.Request) {
	rawCursor, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	items, nextRaw, err := h.Store.ListMonitoredContracts(r.Context(), rawCursor, intQuery(r, "limit", 50), network)
	if err != nil {
		h.Logger.Error("list monitored contracts", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list monitored contracts")
		return
	}
	resp := make([]monitoredContractResponse, len(items))
	for i, m := range items {
		resp[i] = monitoredFromStore(m)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"contracts":   resp,
		"next_cursor": encodeCursor(nextRaw),
	})
}

// GetMonitoredContract handles GET /api/v1/watchdog/contracts/{id}.
func (h *Handler) GetMonitoredContract(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.Store.GetMonitoredContract(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "monitored contract not found")
		return
	}
	if err != nil {
		h.Logger.Error("get monitored contract", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch monitored contract")
		return
	}
	writeJSON(w, http.StatusOK, monitoredFromStore(m))
}

// ListHealthChecks handles GET /api/v1/watchdog/contracts/{id}/health.
func (h *Handler) ListHealthChecks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	checks, err := h.Store.ListHealthChecks(r.Context(), id, intQuery(r, "limit", 100))
	if err != nil {
		h.Logger.Error("list health checks", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list health checks")
		return
	}
	resp := make([]healthCheckResponse, len(checks))
	for i, c := range checks {
		resp[i] = healthCheckFromStore(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"health_checks": resp})
}

// ListWatchdogAlerts handles GET /api/v1/watchdog/contracts/{id}/alerts and
// GET /api/v1/watchdog/alerts.
//
// Pagination (issue #150): pass ?limit=<n>&cursor=<c> to continue from a
// previous page; the response carries next_cursor (empty when exhausted).
// The cursor is a base64 "timestamp|contract_id" keyset pair, so pages
// remain stable while new alerts arrive.
func (h *Handler) ListWatchdogAlerts(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	severity := r.URL.Query().Get("severity")
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	alerts, next, err := h.Store.ListAlerts(r.Context(), id, severity, network, r.URL.Query().Get("cursor"), intQuery(r, "limit", 100))
	if err != nil {
		if errors.Is(err, store.ErrInvalidCursor) {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
			return
		}
		h.Logger.Error("list alerts", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list alerts")
		return
	}
	resp := make([]contractAlertResponse, len(alerts))
	for i, a := range alerts {
		resp[i] = alertFromStore(a)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"alerts":      resp,
		"next_cursor": next,
	})
}

// WatchdogStats handles GET /api/v1/watchdog/stats.
func (h *Handler) WatchdogStats(w http.ResponseWriter, r *http.Request) {
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	s, err := h.Store.GetWatchdogStats(r.Context(), network)
	if err != nil {
		h.Logger.Error("watchdog stats", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch watchdog stats")
		return
	}
	writeJSON(w, http.StatusOK, watchdogStatsResponse{
		TotalMonitored: s.TotalMonitored,
		Healthy:        s.Healthy,
		Degraded:       s.Degraded,
		Unresponsive:   s.Unresponsive,
		TotalAlerts:    s.TotalAlerts,
		CriticalAlerts: s.CriticalAlerts,
	})
}
