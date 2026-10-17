package store_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestWatchdogStorePostgres exercises the watchdog SQL surfaces: monitored
// contract upserts, health-check/alert deduplication, keyset pagination,
// dashboard stats, uptime windows and cascade deletes.
func TestWatchdogStorePostgres(t *testing.T) {
	pool := pgPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE monitored_contracts CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	s := store.NewFullStore(pool)

	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	lastCheck := base.Add(6 * time.Hour)
	contracts := []store.MonitoredContract{
		{ContractID: "CW-A", Network: "testnet", Name: "Alpha", Owner: "G1",
			Status: "Healthy", LastCheck: &lastCheck, CheckInterval: 60, RegisteredAt: base},
		{ContractID: "CW-B", Network: "testnet", Name: "Beta", Owner: "G2",
			Status: "Degraded", CheckInterval: 120, RegisteredAt: base},
		{ContractID: "CW-C", Network: "mainnet", Name: "Gamma", Owner: "G3",
			Status: "Unresponsive", CheckInterval: 30, RegisteredAt: base},
		{ContractID: "CW-UPTIME", Network: "testnet", Name: "Uptime", Owner: "G4",
			Status: "Healthy", RegisteredAt: base},
	}
	for _, mc := range contracts {
		if err := s.UpsertMonitoredContract(ctx, mc); err != nil {
			t.Fatalf("UpsertMonitoredContract(%s): %v", mc.ContractID, err)
		}
	}

	got, err := s.GetMonitoredContract(ctx, "CW-A")
	if err != nil {
		t.Fatalf("GetMonitoredContract: %v", err)
	}
	if got.Name != "Alpha" || got.LastCheck == nil || !got.LastCheck.Equal(lastCheck) {
		t.Errorf("GetMonitoredContract = %+v", got)
	}
	if _, err := s.GetMonitoredContract(ctx, "CW-MISSING"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetMonitoredContract(unknown): want ErrNotFound, got %v", err)
	}

	// A later upsert without a last_check must keep the recorded one.
	if err := s.UpsertMonitoredContract(ctx, store.MonitoredContract{
		ContractID: "CW-A", Network: "testnet", Name: "Alpha renamed", Owner: "G1",
		Status: "Healthy", CheckInterval: 60, RegisteredAt: base,
	}); err != nil {
		t.Fatalf("second UpsertMonitoredContract: %v", err)
	}
	got, err = s.GetMonitoredContract(ctx, "CW-A")
	if err != nil {
		t.Fatalf("GetMonitoredContract after upsert: %v", err)
	}
	if got.Name != "Alpha renamed" || got.LastCheck == nil || !got.LastCheck.Equal(lastCheck) {
		t.Errorf("upsert lost last_check: %+v", got)
	}

	// Health checks: newer-first listing, dedupe on (tx_hash, contract_id).
	for i, ts := range []time.Time{base.Add(time.Hour), base.Add(2 * time.Hour), base.Add(3 * time.Hour)} {
		if err := s.InsertHealthCheck(ctx, store.HealthCheck{
			ContractID: "CW-A", Status: []string{"Healthy", "Degraded", "Healthy"}[i],
			Metadata: fmt.Sprintf("check=%d", i), Ledger: int64(10 + i),
			TxHash: fmt.Sprintf("hcx-%d", i), Timestamp: ts,
		}); err != nil {
			t.Fatalf("InsertHealthCheck: %v", err)
		}
	}
	// Replayed on-chain event.
	if err := s.InsertHealthCheck(ctx, store.HealthCheck{
		ContractID: "CW-A", Status: "Degraded", Ledger: 12,
		TxHash: "hcx-2", Timestamp: base.Add(2 * time.Hour),
	}); err != nil {
		t.Fatalf("duplicate InsertHealthCheck: %v", err)
	}
	checks, err := s.ListHealthChecks(ctx, "CW-A", 2)
	if err != nil {
		t.Fatalf("ListHealthChecks: %v", err)
	}
	if len(checks) != 2 || checks[0].Ledger != 12 || checks[1].Ledger != 11 {
		t.Fatalf("ListHealthChecks limit 2 = %+v, want newest two", checks)
	}
	if checks[0].Status != "Healthy" {
		t.Errorf("duplicate replay overwrote the status: %q", checks[0].Status)
	}
	allChecks, err := s.ListHealthChecks(ctx, "CW-A", 0)
	if err != nil {
		t.Fatalf("ListHealthChecks default limit: %v", err)
	}
	if len(allChecks) != 3 {
		t.Errorf("dedupe: got %d checks, want 3", len(allChecks))
	}
	if allChecks[0].Metadata != "check=2" || allChecks[2].Metadata != "check=0" {
		t.Errorf("metadata not carried newest-first: %+v", allChecks)
	}

	// Alerts: four rows across two networks, newest first.
	alerts := []store.ContractAlert{
		{ContractID: "CW-A", Severity: "Critical", Message: "a3", Ledger: 3, TxHash: "alx-3", Timestamp: base.Add(3 * time.Hour)},
		{ContractID: "CW-A", Severity: "Warning", Message: "a2", Ledger: 2, TxHash: "alx-2", Timestamp: base.Add(2 * time.Hour)},
		{ContractID: "CW-B", Severity: "Critical", Message: "b1", Ledger: 1, TxHash: "alx-1", Timestamp: base.Add(time.Hour)},
		{ContractID: "CW-C", Severity: "Critical", Message: "c0", Ledger: 0, TxHash: "alx-0", Timestamp: base},
	}
	for _, alert := range alerts {
		if err := s.InsertContractAlert(ctx, alert); err != nil {
			t.Fatalf("InsertContractAlert: %v", err)
		}
	}
	if err := s.InsertContractAlert(ctx, alerts[0]); err != nil {
		t.Fatalf("duplicate InsertContractAlert: %v", err)
	}

	page1, cursor, err := s.ListAlerts(ctx, "", "", "", "", 2)
	if err != nil {
		t.Fatalf("ListAlerts page 1: %v", err)
	}
	if len(page1) != 2 || page1[0].Message != "a3" || page1[1].Message != "a2" || cursor == "" {
		t.Fatalf("page 1 = %v, cursor %q", alertMessages(page1), cursor)
	}
	page2, cursor2, err := s.ListAlerts(ctx, "", "", "", cursor, 2)
	if err != nil {
		t.Fatalf("ListAlerts page 2: %v", err)
	}
	if len(page2) != 2 || page2[0].Message != "b1" || page2[1].Message != "c0" || cursor2 != "" {
		t.Fatalf("page 2 = %v, cursor %q", alertMessages(page2), cursor2)
	}
	critical, _, err := s.ListAlerts(ctx, "", "Critical", "", "", 50)
	if err != nil {
		t.Fatalf("ListAlerts severity: %v", err)
	}
	if len(critical) != 3 {
		t.Errorf("critical alerts = %d, want 3 (duplicate ignored)", len(critical))
	}
	mainnet, _, err := s.ListAlerts(ctx, "", "", "mainnet", "", 50)
	if err != nil {
		t.Fatalf("ListAlerts network: %v", err)
	}
	if len(mainnet) != 1 || mainnet[0].ContractID != "CW-C" {
		t.Errorf("mainnet alerts = %+v", alertMessages(mainnet))
	}
	contractAlerts, _, err := s.ListAlerts(ctx, "CW-A", "", "", "", 50)
	if err != nil {
		t.Fatalf("ListAlerts contract: %v", err)
	}
	if len(contractAlerts) != 2 || contractAlerts[0].Message != "a3" {
		t.Errorf("CW-A alerts = %v", alertMessages(contractAlerts))
	}
	if _, _, err := s.ListAlerts(ctx, "", "", "", "not-a-cursor", 10); !errors.Is(err, store.ErrInvalidCursor) {
		t.Errorf("ListAlerts(invalid cursor): want ErrInvalidCursor, got %v", err)
	}

	// Dashboard stats, with and without the network filter.
	stats, err := s.GetWatchdogStats(ctx, "testnet")
	if err != nil {
		t.Fatalf("GetWatchdogStats(testnet): %v", err)
	}
	want := store.WatchdogStats{TotalMonitored: 3, Healthy: 2, Degraded: 1, TotalAlerts: 3, CriticalAlerts: 2}
	if stats != want {
		t.Errorf("testnet stats = %+v, want %+v", stats, want)
	}
	allStats, err := s.GetWatchdogStats(ctx, "")
	if err != nil {
		t.Fatalf("GetWatchdogStats(all): %v", err)
	}
	wantAll := store.WatchdogStats{TotalMonitored: 4, Healthy: 2, Degraded: 1, Unresponsive: 1, TotalAlerts: 4, CriticalAlerts: 3}
	if allStats != wantAll {
		t.Errorf("all-network stats = %+v, want %+v", allStats, wantAll)
	}

	// Uptime windows use NOW()-relative checks.
	now := time.Now().UTC()
	uptimeChecks := []store.HealthCheck{
		{ContractID: "CW-UPTIME", Status: "Healthy", Ledger: 1, TxHash: "up-1", Timestamp: now.Add(-1 * time.Hour)},
		{ContractID: "CW-UPTIME", Status: "Healthy", Ledger: 2, TxHash: "up-2", Timestamp: now.Add(-2 * time.Hour)},
		{ContractID: "CW-UPTIME", Status: "Degraded", Ledger: 3, TxHash: "up-3", Timestamp: now.Add(-3 * time.Hour)},
		{ContractID: "CW-UPTIME", Status: "Healthy", Ledger: 4, TxHash: "up-4", Timestamp: now.Add(-10 * 24 * time.Hour)},
	}
	for _, hc := range uptimeChecks {
		if err := s.InsertHealthCheck(ctx, hc); err != nil {
			t.Fatalf("InsertHealthCheck(uptime): %v", err)
		}
	}
	for _, tc := range []struct {
		window string
		want   float64
	}{
		{"24h", 200.0 / 3.0},
		{"7d", 200.0 / 3.0},
		{"30d", 75},
	} {
		res, err := s.GetContractUptime(ctx, "CW-UPTIME", tc.window)
		if err != nil {
			t.Fatalf("GetContractUptime(%s): %v", tc.window, err)
		}
		if res.ContractID != "CW-UPTIME" || res.Window != tc.window || math.Abs(res.Uptime-tc.want) > 0.0001 {
			t.Errorf("uptime(%s) = %+v, want %v", tc.window, res, tc.want)
		}
	}
	if _, err := s.GetContractUptime(ctx, "CW-UPTIME", "1y"); err == nil {
		t.Error("GetContractUptime accepted an invalid window")
	}

	// Deleting a monitored contract cascades to checks, alerts and groups.
	if _, err := pool.Exec(ctx, `INSERT INTO alert_groups
		(group_key, contract_id, severity, rule, count, dedupe_window_secs, first_seen, last_seen)
		VALUES ('CW-B|Critical|manual', 'CW-B', 'Critical', 'manual', 1, 300, $1, $1)`, base); err != nil {
		t.Fatalf("seed alert group: %v", err)
	}
	if err := s.DeleteMonitoredContract(ctx, "CW-B"); err != nil {
		t.Fatalf("DeleteMonitoredContract: %v", err)
	}
	if _, err := s.GetMonitoredContract(ctx, "CW-B"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("deleted contract still found: %v", err)
	}
	remainingChecks, err := s.ListHealthChecks(ctx, "CW-B", 50)
	if err != nil {
		t.Fatalf("ListHealthChecks after delete: %v", err)
	}
	if len(remainingChecks) != 0 {
		t.Errorf("health checks survived the cascade: %d", len(remainingChecks))
	}
	remainingAlerts, _, err := s.ListAlerts(ctx, "CW-B", "", "", "", 50)
	if err != nil {
		t.Fatalf("ListAlerts after delete: %v", err)
	}
	if len(remainingAlerts) != 0 {
		t.Errorf("alerts survived the cascade: %d", len(remainingAlerts))
	}
	groups, _, err := s.ListAlertGroups(ctx, "", 50, store.AlertGroupFilters{ContractID: "CW-B"})
	if err != nil {
		t.Fatalf("ListAlertGroups after delete: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("alert groups survived the cascade: %d", len(groups))
	}
}

func alertMessages(alerts []store.ContractAlert) []string {
	out := make([]string, len(alerts))
	for i, a := range alerts {
		out[i] = a.Message
	}
	return out
}
