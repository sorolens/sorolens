package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
	"github.com/sorolens/sorolens/packages/rules"
)

// ---- request/response types ------------------------------------------------

// createRuleRequest is the body of POST /api/v1/rules. expression is the only
// required field; contract_id is resolved from the expression's "on" clause
// when omitted.
type createRuleRequest struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	ContractID string `json:"contract_id"`
	Network    string `json:"network"`
	Severity   string `json:"severity"`
	Enabled    *bool  `json:"enabled"`
}

// updateRuleRequest is the body of PUT /api/v1/rules/{id}. Every field is
// replaced wholesale, matching the store's UpdateRule semantics.
type updateRuleRequest struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	ContractID string `json:"contract_id"`
	Network    string `json:"network"`
	Severity   string `json:"severity"`
	Enabled    *bool  `json:"enabled"`
}

// ruleResponse is the wire shape of one alert rule, enriched with the parsed
// fields so the dashboard can render the rule without re-lexing the DSL.
type ruleResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Expression string    `json:"expression"`
	ContractID string    `json:"contract_id"`
	Network    string    `json:"network"`
	Severity   string    `json:"severity"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Parsed fields (empty when the stored expression no longer parses, which
	// should not happen because writes validate, but is tolerated on read).
	Aggregation string `json:"aggregation,omitempty"`
	Metric      string `json:"metric,omitempty"`
	Comparison  string `json:"comparison,omitempty"`
	Threshold   string `json:"threshold,omitempty"`
	Unit        string `json:"unit,omitempty"`
	Window      string `json:"window,omitempty"`
}

// previewRequest is the body of POST /api/v1/rules/preview. contract_id is
// optional when the expression carries an "on <contract>" clause.
type previewRequest struct {
	Expression string `json:"expression"`
	ContractID string `json:"contract_id"`
}

// previewResponse is the live evaluation result for the dashboard editor.
type previewResponse struct {
	Valid          bool    `json:"valid"`
	Error          string  `json:"error,omitempty"`
	ContractID     string  `json:"contract_id"`
	Value          float64 `json:"value"`
	Threshold      float64 `json:"threshold"`
	Fired          bool    `json:"fired"`
	NoData         bool    `json:"no_data"`
	Samples        int     `json:"samples"`
	Events         int64   `json:"events"`
	Description    string  `json:"description"`
	WindowSeconds  int64   `json:"window_seconds"`
	Canonical      string  `json:"canonical,omitempty"`
}

type rulesResponse struct {
	Rules      []ruleResponse `json:"rules"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

type samplesResponse struct {
	Samples []rules.Sample `json:"samples"`
}

// validSeverities is the set accepted by create/update.
var validSeverities = map[string]bool{
	"Info": true, "Warning": true, "Critical": true,
}

// ---- converters -------------------------------------------------------------

func ruleFromStore(r store.AlertRule) ruleResponse {
	out := ruleResponse{
		ID:         r.ID,
		Name:       r.Name,
		Expression: r.Expression,
		ContractID: r.ContractID,
		Network:    r.Network,
		Severity:   r.Severity,
		Enabled:    r.Enabled,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
	if parsed, err := rules.ParseExpression(r.Expression); err == nil {
		out.Aggregation = string(parsed.Aggregation)
		out.Metric = string(parsed.Metric)
		out.Comparison = string(parsed.Comparison)
		out.Threshold = strconv.FormatFloat(parsed.Threshold, 'f', -1, 64)
		out.Unit = string(parsed.Unit)
		if parsed.Window > 0 {
			out.Window = formatDur(parsed.Window)
		}
	}
	return out
}

// resolveContractID reconciles the request contract_id with the expression's
// "on" clause. Returns an error when both are present and disagree.
func resolveContractID(contractID string, parsed rules.Rule) (string, error) {
	if parsed.Contract != "" && contractID != "" && contractID != parsed.Contract {
		return "", fmt.Errorf("contract_id %q conflicts with the expression's \"on %s\" clause", contractID, parsed.Contract)
	}
	if contractID != "" {
		return contractID, nil
	}
	return parsed.Contract, nil
}

// ---- handlers ---------------------------------------------------------------

// CreateRule handles POST /api/v1/rules.
func (h *Handler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var req createRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "name is required")
		return
	}
	if strings.TrimSpace(req.Expression) == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "expression is required")
		return
	}
	severity := req.Severity
	if severity == "" {
		severity = "Warning"
	}
	if !validSeverities[severity] {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
			fmt.Sprintf("severity must be one of Info, Warning, Critical (got %q)", severity))
		return
	}
	parsed, err := rules.ParseExpression(req.Expression)
	if err != nil {
		// Friendly DSL errors already carry line/column + suggestion.
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}
	contractID, err := resolveContractID(req.ContractID, parsed)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}

	rule := store.AlertRule{
		ID:         fmt.Sprintf("rule_%d", time.Now().UnixNano()),
		Name:       req.Name,
		Expression: req.Expression,
		ContractID: contractID,
		Network:    networkOrDefaultValue(req.Network),
		Severity:   severity,
		Enabled:    req.Enabled == nil || *req.Enabled,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := h.Store.CreateRule(r.Context(), rule); err != nil {
		h.Logger.Error("create alert rule", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create rule")
		return
	}
	writeJSON(w, http.StatusCreated, ruleFromStore(rule))
}

// ListRules handles GET /api/v1/rules.
func (h *Handler) ListRules(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	var enabled *bool
	if v := q.Get("enabled"); v == "true" || v == "false" {
		b := v == "true"
		enabled = &b
	}
	rulesList, next, err := h.Store.ListRules(r.Context(), q.Get("cursor"), limit, enabled, q.Get("contract_id"))
	if err != nil {
		h.Logger.Error("list alert rules", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list rules")
		return
	}
	out := make([]ruleResponse, len(rulesList))
	for i, rule := range rulesList {
		out[i] = ruleFromStore(rule)
	}
	writeJSON(w, http.StatusOK, rulesResponse{Rules: out, NextCursor: next})
}

// GetRule handles GET /api/v1/rules/{id}.
func (h *Handler) GetRule(w http.ResponseWriter, r *http.Request) {
	rule, err := h.Store.GetRule(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "rule not found")
			return
		}
		h.Logger.Error("get alert rule", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to get rule")
		return
	}
	writeJSON(w, http.StatusOK, ruleFromStore(rule))
}

// UpdateRule handles PUT /api/v1/rules/{id}.
func (h *Handler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "name is required")
		return
	}
	severity := req.Severity
	if severity == "" {
		severity = "Warning"
	}
	if !validSeverities[severity] {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
			fmt.Sprintf("severity must be one of Info, Warning, Critical (got %q)", severity))
		return
	}
	parsed, err := rules.ParseExpression(req.Expression)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}
	contractID, err := resolveContractID(req.ContractID, parsed)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}

	existing, err := h.Store.GetRule(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "rule not found")
			return
		}
		h.Logger.Error("get alert rule for update", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to update rule")
		return
	}

	existing.Name = req.Name
	existing.Expression = req.Expression
	existing.ContractID = contractID
	existing.Network = networkOrDefaultValue(req.Network)
	existing.Severity = severity
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	existing.UpdatedAt = time.Now().UTC()

	if err := h.Store.UpdateRule(r.Context(), existing); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "rule not found")
			return
		}
		h.Logger.Error("update alert rule", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to update rule")
		return
	}
	writeJSON(w, http.StatusOK, ruleFromStore(existing))
}

// DeleteRule handles DELETE /api/v1/rules/{id}.
func (h *Handler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.DeleteRule(r.Context(), chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "rule not found")
			return
		}
		h.Logger.Error("delete alert rule", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete rule")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListRuleSamples handles GET /api/v1/rules/samples.
func (h *Handler) ListRuleSamples(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, samplesResponse{Samples: rules.Samples})
}

// PreviewRule handles POST /api/v1/rules/preview. It parses the expression and
// evaluates it against the trailing window of real indexed data, which is what
// powers the editor's "live evaluation" panel.
func (h *Handler) PreviewRule(w http.ResponseWriter, r *http.Request) {
	var req previewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	parsed, err := rules.ParseExpression(req.Expression)
	if err != nil {
		writeJSON(w, http.StatusOK, previewResponse{Valid: false, Error: err.Error()})
		return
	}
	contractID := req.ContractID
	if contractID == "" {
		contractID = parsed.Contract
	}
	if contractID == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
			"contract_id is required when the expression has no \"on <contract>\" clause (preview evaluates one contract at a time)")
		return
	}

	stats, err := h.Store.RuleWindowStats(r.Context(), contractID, parsed.Window)
	if err != nil {
		h.Logger.Error("preview rule window stats", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch evaluation data")
		return
	}
	res := rules.Evaluate(parsed, stats)

	writeJSON(w, http.StatusOK, previewResponse{
		Valid:         true,
		ContractID:    contractID,
		Value:         res.Value,
		Threshold:     parsed.Threshold,
		Fired:         res.Fired,
		NoData:        res.NoData,
		Samples:       res.Samples,
		Events:        stats.Events,
		Description:   res.Description,
		WindowSeconds: int64(parsed.Window.Seconds()),
		Canonical:     parsed.String(),
	})
}

// ---- helpers ----------------------------------------------------------------

// networkOrDefaultValue defaults an empty network to "testnet", matching the
// store-side default so a rule row is always comparable across networks.
func networkOrDefaultValue(network string) string {
	if network == "" {
		return "testnet"
	}
	return network
}

// formatDur renders a duration the same compact way the DSL uses in "for".
func formatDur(d time.Duration) string {
	switch {
	case d%(24*time.Hour) == 0 && d > 0:
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	case d%time.Hour == 0 && d > 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0 && d > 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	default:
		return d.String()
	}
}