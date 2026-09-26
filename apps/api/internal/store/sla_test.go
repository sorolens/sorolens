package store

import (
	"fmt"
	"testing"
	"time"
)

func at(month string, day, hour int) time.Time {
	t, err := time.Parse("2006-01-02 15:04", fmt.Sprintf("%s-%02d %02d:00", month, day, hour))
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

func check(status string, t time.Time) HealthCheck {
	return HealthCheck{ContractID: "CTEST", Status: status, Timestamp: t}
}

func alert(severity string, t time.Time) ContractAlert {
	return ContractAlert{ContractID: "CTEST", Severity: severity, Timestamp: t}
}

func TestMonthRangeRejectsMalformedMonth(t *testing.T) {
	for _, bad := range []string{"2026-13", "2026", "26-01", "2026-1", ""} {
		if _, _, err := MonthRange(bad); err == nil {
			t.Fatalf("MonthRange(%q) accepted an invalid month", bad)
		}
	}
	from, to, err := MonthRange("2026-02")
	if err != nil {
		t.Fatalf("MonthRange(2026-02): %v", err)
	}
	if got := from.Format(time.RFC3339); got != "2026-02-01T00:00:00Z" {
		t.Fatalf("from = %s", got)
	}
	// February has 28 days in 2026; the range must be half-open at 1 March.
	if got := to.Format(time.RFC3339); got != "2026-03-01T00:00:00Z" {
		t.Fatalf("to = %s", got)
	}
}

func TestComputeMonthlySLANoData(t *testing.T) {
	m := ComputeMonthlySLA("CTEST", "2026-02", nil, nil)
	if m.HasData() {
		t.Fatal("expected HasData false for a month with no checks")
	}
	if m.TotalChecks != 0 || m.Incidents != 0 || m.UptimePct != 0 {
		t.Fatalf("unexpected metrics: %+v", m)
	}
	// No checks means no outage was ever observed: zero incidents, not one.
	if m.OngoingOutage {
		t.Fatal("a month with no data must not report an ongoing outage")
	}
	if m.FirstCheck != nil || m.LastCheck != nil {
		t.Fatal("expected nil check bounds")
	}
}

func TestComputeMonthlySLAFullUptime(t *testing.T) {
	checks := []HealthCheck{
		check("Healthy", at("2026-02", 1, 0)),
		check("Healthy", at("2026-02", 2, 0)),
		check("Healthy", at("2026-02", 3, 0)),
	}
	m := ComputeMonthlySLA("CTEST", "2026-02", checks, nil)

	if m.TotalChecks != 3 || m.HealthyChecks != 3 {
		t.Fatalf("checks = %d/%d, want 3/3", m.HealthyChecks, m.TotalChecks)
	}
	if m.UptimePct != 100 {
		t.Fatalf("uptime = %v, want 100", m.UptimePct)
	}
	if m.Incidents != 0 || m.MTTRSeconds != 0 || m.TotalDowntimeSeconds != 0 {
		t.Fatalf("expected no incidents: %+v", m)
	}
	if !m.Healthy() {
		t.Fatal("100 percent uptime should pass the 99.9 percent bar")
	}
	if m.FirstCheck == nil || m.LastCheck == nil {
		t.Fatal("expected check bounds to be set")
	}
}

func TestComputeMonthlySLACountsOneIncidentAndMTTR(t *testing.T) {
	// Healthy at 00:00 and 01:00, unhealthy at 02:00 and 03:00, healthy at
	// 04:00. That is ONE incident lasting two hours, not two incidents.
	checks := []HealthCheck{
		check("Healthy", at("2026-02", 1, 0)),
		check("Healthy", at("2026-02", 1, 1)),
		check("Degraded", at("2026-02", 1, 2)),
		check("Unresponsive", at("2026-02", 1, 3)),
		check("Healthy", at("2026-02", 1, 4)),
	}
	m := ComputeMonthlySLA("CTEST", "2026-02", checks, nil)

	if m.Incidents != 1 {
		t.Fatalf("incidents = %d, want 1", m.Incidents)
	}
	if m.MTTRSeconds != 7200 {
		t.Fatalf("MTTR = %v, want 7200", m.MTTRSeconds)
	}
	if m.TotalDowntimeSeconds != 7200 {
		t.Fatalf("downtime = %v, want 7200", m.TotalDowntimeSeconds)
	}
	if m.LongestOutageSeconds != 7200 {
		t.Fatalf("longest = %v, want 7200", m.LongestOutageSeconds)
	}
	if m.OngoingOutage {
		t.Fatal("the incident recovered, so OngoingOutage must be false")
	}
	if m.HealthyChecks != 3 || m.TotalChecks != 5 {
		t.Fatalf("checks = %d/%d, want 3/5", m.HealthyChecks, m.TotalChecks)
	}
	if m.UptimePct != 60 {
		t.Fatalf("uptime = %v, want 60", m.UptimePct)
	}
	if m.Healthy() {
		t.Fatal("60 percent uptime must not pass the 99.9 percent bar")
	}
}

func TestComputeMonthlySLAOngoingOutageExcludedFromMTTR(t *testing.T) {
	// One recovered incident, then a second that is still open at month end.
	checks := []HealthCheck{
		check("Healthy", at("2026-02", 1, 0)),
		check("Degraded", at("2026-02", 1, 1)),
		check("Healthy", at("2026-02", 1, 2)), // recovered after 1h
		check("Degraded", at("2026-02", 1, 5)), // second incident, never recovers
	}
	m := ComputeMonthlySLA("CTEST", "2026-02", checks, nil)

	if m.Incidents != 2 {
		t.Fatalf("incidents = %d, want 2", m.Incidents)
	}
	if !m.OngoingOutage {
		t.Fatal("expected OngoingOutage true")
	}
	// MTTR averages only the recovered incident, so it is 1h and not diluted by
	// the open one.
	if m.MTTRSeconds != 3600 {
		t.Fatalf("MTTR = %v, want 3600 (only the recovered incident)", m.MTTRSeconds)
	}
	// Downtime still includes the elapsed part of the open incident: 1h + 0s
	// between 05:00 and the last check at 05:00.
	if m.TotalDowntimeSeconds != 3600 {
		t.Fatalf("downtime = %v, want 3600", m.TotalDowntimeSeconds)
	}
}

func TestComputeMonthlySLAIgnoresChecksOutsideTheMonth(t *testing.T) {
	checks := []HealthCheck{
		check("Unresponsive", at("2026-01", 31, 23)), // previous month
		check("Healthy", at("2026-02", 1, 0)),
		check("Healthy", at("2026-02", 28, 23)),
		check("Unresponsive", at("2026-03", 1, 0)), // next month, half-open bound
	}
	m := ComputeMonthlySLA("CTEST", "2026-02", checks, nil)

	if m.TotalChecks != 2 {
		t.Fatalf("total checks = %d, want only the 2 in February", m.TotalChecks)
	}
	if m.Incidents != 0 {
		t.Fatalf("incidents = %d, want 0 (out-of-month outages must not leak in)", m.Incidents)
	}
	if m.UptimePct != 100 {
		t.Fatalf("uptime = %v, want 100", m.UptimePct)
	}
}

func TestComputeMonthlySLACountsAlertsBySeverity(t *testing.T) {
	alerts := []ContractAlert{
		alert("Critical", at("2026-02", 1, 0)),
		alert("critical", at("2026-02", 1, 1)), // case-insensitive
		alert("Warning", at("2026-02", 1, 2)),
		alert("Info", at("2026-02", 1, 3)),
		alert("Critical", at("2026-01", 5, 0)), // outside the month
	}
	m := ComputeMonthlySLA("CTEST", "2026-02", nil, alerts)

	if m.TotalAlerts != 4 {
		t.Fatalf("total alerts = %d, want 4", m.TotalAlerts)
	}
	if m.CriticalAlerts != 2 || m.WarningAlerts != 1 || m.InfoAlerts != 1 {
		t.Fatalf("severity split = %d/%d/%d, want 2/1/1",
			m.CriticalAlerts, m.WarningAlerts, m.InfoAlerts)
	}
}

func TestComputeMonthlySLAOutOfOrderInput(t *testing.T) {
	// The same data as the incident test, shuffled. Ordering must not change
	// the answer: callers should not have to sort.
	ordered := []HealthCheck{
		check("Healthy", at("2026-02", 1, 0)),
		check("Degraded", at("2026-02", 1, 2)),
		check("Healthy", at("2026-02", 1, 4)),
	}
	shuffled := []HealthCheck{ordered[2], ordered[0], ordered[1]}

	want := ComputeMonthlySLA("CTEST", "2026-02", ordered, nil)
	got := ComputeMonthlySLA("CTEST", "2026-02", shuffled, nil)

	if got.Incidents != want.Incidents || got.MTTRSeconds != want.MTTRSeconds ||
		got.UptimePct != want.UptimePct || got.TotalDowntimeSeconds != want.TotalDowntimeSeconds {
		t.Fatalf("order changed the result:\nwant %+v\ngot  %+v", want, got)
	}
}

func TestComputeMonthlySLAInvalidMonth(t *testing.T) {
	m := ComputeMonthlySLA("CTEST", "not-a-month", []HealthCheck{check("Healthy", time.Now())}, nil)
	if m.ContractID != "CTEST" || m.Month != "not-a-month" {
		t.Fatalf("unexpected echo: %+v", m)
	}
	if m.TotalChecks != 0 {
		t.Fatalf("an invalid month must not count checks, got %d", m.TotalChecks)
	}
}

func TestComputeMonthlySLAHistoryBucketsOldestFirst(t *testing.T) {
	now := at("2026-04", 15, 12)
	checks := []HealthCheck{
		check("Healthy", at("2026-02", 1, 0)),
		check("Degraded", at("2026-03", 1, 0)),
		check("Healthy", at("2026-04", 1, 0)),
	}
	hist := ComputeMonthlySLAHistory("CTEST", 3, now, checks, nil)

	if len(hist) != 3 {
		t.Fatalf("got %d buckets, want 3", len(hist))
	}
	if hist[0].Month != "2026-02" || hist[2].Month != "2026-04" {
		t.Fatalf("buckets not oldest-first: %s .. %s", hist[0].Month, hist[2].Month)
	}
	if hist[0].UptimePct != 100 {
		t.Fatalf("February uptime = %v, want 100", hist[0].UptimePct)
	}
	if hist[1].UptimePct != 0 || !hist[1].HasData() {
		t.Fatalf("March should have data and 0%% uptime, got %+v", hist[1])
	}
	if hist[2].UptimePct != 100 {
		t.Fatalf("April uptime = %v, want 100", hist[2].UptimePct)
	}
}

func TestHistoryRangeSpansRequestedMonths(t *testing.T) {
	now := at("2026-04", 15, 12)
	from, to := HistoryRange(3, now)

	if got := from.Format("2006-01"); got != "2026-02" {
		t.Fatalf("from month = %s, want 2026-02", got)
	}
	if got := to.Format(time.RFC3339); got != "2026-05-01T00:00:00Z" {
		t.Fatalf("to = %s", got)
	}
}
