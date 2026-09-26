package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ---- models ----------------------------------------------------------------

// MonthlySLA is one contract's service-level summary for a calendar month.
//
// Every field is derived from the health checks and alerts already recorded by
// the watchdog, so generating a report never re-queries the chain.
type MonthlySLA struct {
	ContractID string `json:"contract_id"`
	// Month is the reporting period in YYYY-MM form (UTC).
	Month string `json:"month"`

	// UptimePct is healthy checks / total checks * 100 over the month. It is
	// 0 when there were no checks, which is reported alongside TotalChecks so
	// a consumer can tell "0% up" from "no data".
	UptimePct     float64 `json:"uptime_pct"`
	TotalChecks   int64   `json:"total_checks"`
	HealthyChecks int64   `json:"healthy_checks"`

	// Incidents counts outages: a transition from Healthy into any other
	// status. A month with no health checks has zero incidents, not one.
	Incidents int64 `json:"incidents"`
	// MTTRSeconds is the mean time to recovery across incidents that recovered
	// within the month. Incidents still open at the last check contribute to
	// TotalDowntimeSeconds but not to MTTR.
	MTTRSeconds float64 `json:"mttr_seconds"`
	// TotalDowntimeSeconds sums every outage's duration inside the month.
	TotalDowntimeSeconds float64 `json:"total_downtime_seconds"`
	// LongestOutageSeconds is the longest single outage.
	LongestOutageSeconds float64 `json:"longest_outage_seconds"`
	// OngoingOutage is true when the month ends mid-incident. MTTR then
	// understates reality, and the report says so rather than hiding it.
	OngoingOutage bool `json:"ongoing_outage"`

	// Alert counts by severity for the month.
	CriticalAlerts int64 `json:"critical_alerts"`
	WarningAlerts  int64 `json:"warning_alerts"`
	InfoAlerts     int64 `json:"info_alerts"`
	TotalAlerts    int64 `json:"total_alerts"`

	FirstCheck *time.Time `json:"first_check"`
	LastCheck  *time.Time `json:"last_check"`
}

// Healthy is true when uptime is at or above the 99.9% "three nines" bar most
// SLAs are written against. Reports and badges use it as the pass/fail line.
func (m MonthlySLA) Healthy() bool {
	return m.TotalChecks > 0 && m.UptimePct >= 99.9
}

// HasData reports whether the month has any health checks at all.
func (m MonthlySLA) HasData() bool { return m.TotalChecks > 0 }

// ---- interface -------------------------------------------------------------

// SLAStore is the read surface the reporting handlers need on top of the
// watchdog data. It is separate from WatchdogStore so the reporting feature can
// be wired — or omitted — without disturbing the existing watchdog endpoint
// surface.
type SLAStore interface {
	// ListHealthChecksInRange returns a contract's checks with
	// from <= timestamp < to, oldest first.
	ListHealthChecksInRange(ctx context.Context, contractID string, from, to time.Time) ([]HealthCheck, error)
	// ListAlertsInRange returns a contract's alerts in the same window,
	// oldest first.
	ListAlertsInRange(ctx context.Context, contractID string, from, to time.Time) ([]ContractAlert, error)
}

// ---- period helpers --------------------------------------------------------

// MonthRange converts a YYYY-MM period into its half-open UTC bounds
// [from, to), which is what every query and aggregation in this file uses.
func MonthRange(month string) (time.Time, time.Time, error) {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("month must be formatted YYYY-MM")
	}
	from := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	return from, from.AddDate(0, 1, 0), nil
}

// CurrentMonth is the UTC reporting period containing now.
func CurrentMonth(now time.Time) string { return now.UTC().Format("2006-01") }

// ---- computation -----------------------------------------------------------

// ComputeMonthlySLA derives a month's SLA numbers from raw watchdog data.
//
// It is deliberately a pure function over slices: the arithmetic that decides
// whether an SLA was met is the part worth testing, and keeping it free of the
// database means it can be tested directly with fixtures.
//
// Either slice may be in any order; both are sorted defensively. Checks outside
// the month are ignored, so a caller may pass a wider range without corrupting
// the figures.
func ComputeMonthlySLA(contractID, month string, checks []HealthCheck, alerts []ContractAlert) MonthlySLA {
	from, to, err := MonthRange(month)
	if err != nil {
		// An unparseable month cannot be filtered against; report the month as
		// given with zero metrics rather than silently using the wrong window.
		return MonthlySLA{ContractID: contractID, Month: month}
	}

	inWindow := make([]HealthCheck, 0, len(checks))
	for _, c := range checks {
		if c.ContractID != "" && c.ContractID != contractID {
			continue
		}
		if c.Timestamp.Before(from) || !c.Timestamp.Before(to) {
			continue
		}
		inWindow = append(inWindow, c)
	}
	sort.SliceStable(inWindow, func(i, j int) bool {
		return inWindow[i].Timestamp.Before(inWindow[j].Timestamp)
	})

	out := MonthlySLA{ContractID: contractID, Month: month, TotalChecks: int64(len(inWindow))}

	// Uptime: the share of checks that reported Healthy. Sampling-based uptime
	// (rather than time-weighted) is what the existing /uptime endpoint uses,
	// so the monthly report stays consistent with it.
	for _, c := range inWindow {
		if isHealthy(c.Status) {
			out.HealthyChecks++
		}
	}
	if out.TotalChecks > 0 {
		out.UptimePct = float64(out.HealthyChecks) / float64(out.TotalChecks) * 100
		first := inWindow[0].Timestamp
		last := inWindow[len(inWindow)-1].Timestamp
		out.FirstCheck = &first
		out.LastCheck = &last
	}

	// Incidents: walk the ordered checks and bracket each Healthy -> unhealthy
	// transition with the next Healthy check. An outage is one incident
	// regardless of how many unhealthy checks it spans, so a contract that is
	// Degraded for a week is one incident, not 2,016 of them.
	var (
		outageStart  *time.Time
		recovered    int64
		recoveredSum float64
	)
	for _, c := range inWindow {
		healthy := isHealthy(c.Status)
		switch {
		case !healthy && outageStart == nil:
			start := c.Timestamp
			outageStart = &start
			out.Incidents++
		case healthy && outageStart != nil:
			d := c.Timestamp.Sub(*outageStart).Seconds()
			out.TotalDowntimeSeconds += d
			if d > out.LongestOutageSeconds {
				out.LongestOutageSeconds = d
			}
			recoveredSum += d
			recovered++
			outageStart = nil
		}
	}
	if outageStart != nil {
		// Still down at the last check: count the elapsed time so far, and flag
		// it so a reader knows MTTR excludes an unresolved incident.
		d := inWindow[len(inWindow)-1].Timestamp.Sub(*outageStart).Seconds()
		out.TotalDowntimeSeconds += d
		if d > out.LongestOutageSeconds {
			out.LongestOutageSeconds = d
		}
		out.OngoingOutage = true
	}
	if recovered > 0 {
		out.MTTRSeconds = recoveredSum / float64(recovered)
	}

	for _, a := range alerts {
		if a.ContractID != "" && a.ContractID != contractID {
			continue
		}
		if a.Timestamp.Before(from) || !a.Timestamp.Before(to) {
			continue
		}
		out.TotalAlerts++
		switch {
		case strings.EqualFold(a.Severity, "Critical"):
			out.CriticalAlerts++
		case strings.EqualFold(a.Severity, "Warning"):
			out.WarningAlerts++
		default:
			out.InfoAlerts++
		}
	}

	return out
}

func isHealthy(status string) bool { return strings.EqualFold(status, "Healthy") }

// ---- postgres implementation -----------------------------------------------

func (s *postgresStore) ListHealthChecksInRange(ctx context.Context, contractID string, from, to time.Time) ([]HealthCheck, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT contract_id, status, metadata, ledger, tx_hash, timestamp
		FROM health_checks
		WHERE contract_id = $1
		  AND timestamp >= $2
		  AND timestamp <  $3
		ORDER BY timestamp ASC`, contractID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list health checks in range: %w", err)
	}
	defer rows.Close()

	var out []HealthCheck
	for rows.Next() {
		var h HealthCheck
		var meta *string
		if err := rows.Scan(&h.ContractID, &h.Status, &meta, &h.Ledger, &h.TxHash, &h.Timestamp); err != nil {
			return nil, err
		}
		if meta != nil {
			h.Metadata = *meta
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *postgresStore) ListAlertsInRange(ctx context.Context, contractID string, from, to time.Time) ([]ContractAlert, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT contract_id, severity, message, ledger, tx_hash, timestamp
		FROM contract_alerts
		WHERE contract_id = $1
		  AND timestamp >= $2
		  AND timestamp <  $3
		ORDER BY timestamp ASC`, contractID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list alerts in range: %w", err)
	}
	defer rows.Close()

	var out []ContractAlert
	for rows.Next() {
		var a ContractAlert
		if err := rows.Scan(&a.ContractID, &a.Severity, &a.Message, &a.Ledger, &a.TxHash, &a.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
