package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
	rulelang "github.com/sorolens/sorolens/apps/api/rulelang"
)

// ---- shared types ----------------------------------------------------------

// ruleDiagnostic is the machine-readable form of a rulelang validation error.
// The dashboard editor renders Message next to the offending line and shows
// Hint as a coach mark.
type ruleDiagnostic struct {
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
}

// diagnosticsFromError converts a rulelang error into a one-element list. A
// non-*rulelang.Error (which should not happen) yields a positionless entry.
func diagnosticsFromError(err error) []ruleDiagnostic {
	var rerr *rulelang.Error
	if errors.As(err, &rerr) {
		line, col := rerr.LineCol()
		return []ruleDiagnostic{{Message: rerr.Msg, Hint: rerr.Hint, Line: line, Column: col}}
	}
	return []ruleDiagnostic{{Message: err.Error(), Line: 1, Column: 1}}
}

type ruleResponse struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Source     string    `json:"source"`
	Severity   string    `json:"severity"`
	ContractID string    `json:"contract_id,omitempty"`
	Network    string    `json:"network,omitempty"`
	Window     string    `json:"window,omitempty"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func ruleFromStore(r store.AlertRule) ruleResponse {
	out := ruleResponse{
		ID:         r.ID,
		Name:       r.Name,
		Source:     r.Source,
		Severity:   r.Severity,
		ContractID: r.ContractID,
		Network:    r.Network,
		Enabled:    r.Enabled,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
	if r.WindowSecs > 0 {
		out.Window = (time.Duration(r.WindowSecs) * time.Second).String()
	}
	return out
}

// ---- create ----------------------------------------------------------------

type createRuleRequest struct {
	Name       string `json:"name"`
	Source     string `json:"source"`
	Severity   string `json:"severity"`
	ContractID string `json:"contract_id"`
	Network    string `json:"network"`
}

type validateRuleResponse struct {
	Valid      bool             `json:"valid"`
	Normalized string           `json:"normalized,omitempty"`
	Metrics    []string         `json:"metrics,omitempty"`
	Window     string           `json:"window,omitempty"`
	Errors     []ruleDiagnostic `json:"errors,omitempty"`
}

// CreateRule handles POST /api/v1/rules. The rule is parsed and validated
// before it is stored, so a malformed rule can never reach the evaluator.
func (h *Handler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var req createRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Source = strings.TrimSpace(req.Source)
	req.Severity = strings.TrimSpace(req.Severity)
	if req.Severity == "" {
		req.Severity = "Warning"
	}

	if req.Name == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "name is required")
		return
	}
	if !validRuleSeverity(req.Severity) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "severity must be one of: Info, Warning, Critical")
		return
	}

	rule, err := rulelang.Validate(req.Source)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, validateRuleResponse{
			Valid:  false,
			Errors: diagnosticsFromError(err),
		})
		return
	}

	// Clauses in the source win over the request fields; the request fields are
	// the scope for sources that do not carry one.
	contractID := firstNonEmpty(rule.ContractID, strings.TrimSpace(req.ContractID))
	network := firstNonEmpty(rule.Network, strings.TrimSpace(req.Network))
	if contractID != "" {
		if ok, _ := isValidContractStrKey(contractID); !ok {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "contract_id is not a valid contract address")
			return
		}
	}
	if network != "" && !validNetworks[network] {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}

	created, err := h.Store.CreateAlertRule(r.Context(), store.AlertRule{
		Name:       req.Name,
		Source:     rule.Format(),
		Severity:   req.Severity,
		ContractID: contractID,
		Network:    network,
		WindowSecs: int64(rule.For.Seconds()),
		Enabled:    true,
	})
	if err != nil {
		if errors.Is(err, store.ErrRuleExists) {
			writeError(w, r, http.StatusConflict, CodeInvalidInput, "an identical rule already exists")
			return
		}
		h.Logger.Error("create alert rule", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create rule")
		return
	}
	writeJSON(w, http.StatusCreated, ruleFromStore(created))
}

// ---- list / get / delete ---------------------------------------------------

// ListRules handles GET /api/v1/rules.
func (h *Handler) ListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.Store.ListAlertRules(r.Context())
	if err != nil {
		h.Logger.Error("list alert rules", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list rules")
		return
	}
	out := make([]ruleResponse, len(rules))
	for i, rule := range rules {
		out[i] = ruleFromStore(rule)
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": out})
}

// DeleteRule handles DELETE /api/v1/rules/{id}.
func (h *Handler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := ruleIDParam(r)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "rule id must be an integer")
		return
	}
	if err := h.Store.DeleteAlertRule(r.Context(), id); err != nil {
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

// SetRuleEnabledRequest is the body for PATCH /api/v1/rules/{id}.
type SetRuleEnabledRequest struct {
	Enabled *bool `json:"enabled"`
}

// SetRuleEnabled handles PATCH /api/v1/rules/{id} so the dashboard can pause a
// noisy rule without deleting it.
func (h *Handler) SetRuleEnabled(w http.ResponseWriter, r *http.Request) {
	id, err := ruleIDParam(r)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "rule id must be an integer")
		return
	}
	var req SetRuleEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "body must be {\"enabled\": true|false}")
		return
	}
	updated, err := h.Store.SetAlertRuleEnabled(r.Context(), id, *req.Enabled)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "rule not found")
			return
		}
		h.Logger.Error("set alert rule enabled", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to update rule")
		return
	}
	writeJSON(w, http.StatusOK, ruleFromStore(updated))
}

// ---- validate --------------------------------------------------------------

// ValidateRule handles POST /api/v1/rules/validate. It is a pure function of
// the source text: no writes, no store access.
func (h *Handler) ValidateRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	writeJSON(w, http.StatusOK, validateRuleSource(req.Source))
}

// validateRuleSource is shared by the validate and preview endpoints.
func validateRuleSource(source string) validateRuleResponse {
	rule, err := rulelang.Validate(strings.TrimSpace(source))
	if err != nil {
		return validateRuleResponse{Valid: false, Errors: diagnosticsFromError(err)}
	}
	resp := validateRuleResponse{Valid: true, Normalized: rule.Format(), Metrics: rule.Metrics()}
	if rule.For > 0 {
		resp.Window = rule.For.String()
	}
	return resp
}

// ---- preview ---------------------------------------------------------------

type previewPoint struct {
	At    time.Time `json:"at"`
	Value *float64  `json:"value"`
	Fired bool      `json:"fired"`
}

type previewRuleResponse struct {
	Valid       bool             `json:"valid"`
	Fired       bool             `json:"fired"`
	Op          string           `json:"op,omitempty"`
	Value       *float64         `json:"value"`
	Threshold   *float64         `json:"threshold"`
	Reason      string           `json:"reason,omitempty"`
	Window      string           `json:"window,omitempty"`
	EvaluatedAt time.Time        `json:"evaluated_at"`
	Points      []previewPoint   `json:"points,omitempty"`
	Errors      []ruleDiagnostic `json:"errors,omitempty"`
}

// DefaultPreviewWindow is the window used when a rule has no `for` clause and
// the caller does not pass one. Without it a bare metric would be previewed
// against a single minute, which reads as noise in the editor.
const DefaultPreviewWindow = time.Hour

// PreviewRule handles POST /api/v1/rules/preview. Given a rule source and an
// optional contract id it fetches the contract's per-minute metrics and runs
// the real evaluator, so the editor shows exactly what the indexer would do.
func (h *Handler) PreviewRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source     string `json:"source"`
		ContractID string `json:"contract_id"`
		Window     string `json:"window"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}

	vr := validateRuleSource(req.Source)
	if !vr.Valid {
		writeJSON(w, http.StatusOK, previewRuleResponse{Valid: false, Errors: vr.Errors})
		return
	}
	rule, _ := rulelang.Validate(strings.TrimSpace(req.Source))
	contractID := firstNonEmpty(rule.ContractID, strings.TrimSpace(req.ContractID))
	if contractID == "" {
		writeJSON(w, http.StatusOK, previewRuleResponse{
			Valid: true,
			Errors: []ruleDiagnostic{{
				Message: "preview needs a contract: pass contract_id, or add `on contract <id>` to the rule",
				Line:    1, Column: 1,
			}},
		})
		return
	}

	window := evaluateWindow(rule, req.Window)
	now := time.Now().UTC()
	samples, err := h.Store.ContractMetricSamples(r.Context(), contractID, now.Add(-window), now)
	if err != nil {
		h.Logger.Error("preview rule samples", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load metric samples")
		return
	}

	res, err := rulelang.Evaluate(rule, toRuleWindow(now, samples))
	if err != nil {
		writeJSON(w, http.StatusOK, previewRuleResponse{Valid: true, Errors: diagnosticsFromError(err)})
		return
	}

	out := previewRuleResponse{
		Valid:       true,
		Fired:       res.Fired,
		Op:          res.Op,
		Reason:      res.Reason,
		Window:      window.String(),
		EvaluatedAt: now,
	}
	if res.HasValue {
		v := res.Value
		out.Value = &v
	}
	t := res.Threshold
	out.Threshold = &t
	for _, p := range res.Points {
		pt := previewPoint{At: p.At, Fired: p.Fired}
		if p.HasValue {
			v := p.Value
			pt.Value = &v
		}
		out.Points = append(out.Points, pt)
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- catalog + library -----------------------------------------------------

// ListRuleMetrics handles GET /api/v1/rules/metrics. The dashboard editor uses
// it for autocomplete and unit hints.
func (h *Handler) ListRuleMetrics(w http.ResponseWriter, r *http.Request) {
	type metricResponse struct {
		Name        string `json:"name"`
		Unit        string `json:"unit"`
		Description string `json:"description"`
	}
	metrics := rulelang.Catalog()
	out := make([]metricResponse, len(metrics))
	for i, m := range metrics {
		out[i] = metricResponse{Name: m.Name, Unit: m.Unit, Description: m.Description}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"metrics":      out,
		"aggregations": rulelang.AggregationFuncs,
		"networks":     rulelang.ValidNetworks,
	})
}

// ListRuleLibrary handles GET /api/v1/rules/library: the curated sample rules.
func (h *Handler) ListRuleLibrary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"rules": rulelang.Library()})
}

// ---- helpers ---------------------------------------------------------------

func ruleIDParam(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func validRuleSeverity(s string) bool {
	switch s {
	case "Info", "Warning", "Critical":
		return true
	}
	return false
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// evaluateWindow picks the preview window: the rule's own `for`, then an
// explicit request value, then the default.
func evaluateWindow(rule *rulelang.Rule, requested string) time.Duration {
	if rule.For > 0 {
		return rule.For
	}
	if requested != "" {
		if d, err := time.ParseDuration(requested); err == nil && d > 0 && d <= rulelang.MaxWindow {
			return d
		}
	}
	return DefaultPreviewWindow
}

// toRuleWindow converts store samples into the evaluator's window type.
func toRuleWindow(now time.Time, samples []store.MetricSample) rulelang.Window {
	w := rulelang.Window{To: now}
	if len(samples) > 0 {
		w.From = samples[0].At
	} else {
		w.From = now
	}
	for _, s := range samples {
		w.Samples = append(w.Samples, rulelang.Sample{At: s.At, Values: s.Values})
	}
	return w
}
