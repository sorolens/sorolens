package store_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestStoreContractParity runs the same scenario against MockStore and the
// Postgres-backed store and requires identical observable results. MockStore
// substitutes for Postgres in handler and indexer tests, so silent divergence
// would make those suites assert behaviour the production store does not have.
//
// Each scenario truncates only the tables it touches after the mock run, so a
// failure names the store surface that drifted (issue #548).
func TestStoreContractParity(t *testing.T) {
	compareContractStores(t, []contractScenario{
		apiKeyContractScenario(),
		failedEventContractScenario(),
		subscriptionContractScenario(),
		watchdogContractScenario(),
		alertGroupContractScenario(),
		healthScoreContractScenario(),
		globalEventContractScenario(),
		contractAndLabelContractScenario(),
	})
}

type contractScenario struct {
	name   string
	tables []string
	run    func(context.Context, store.FullStore) []string
}

func compareContractStores(t *testing.T, scenarios []contractScenario) {
	t.Helper()
	pool := pgPool(t)
	ctx := context.Background()
	pg := store.NewFullStore(pool)
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			want := sc.run(ctx, store.NewMockStore())
			for _, table := range sc.tables {
				if _, err := pool.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE"); err != nil {
					t.Fatalf("truncate %s: %v", table, err)
				}
			}
			got := sc.run(ctx, pg)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("MockStore and PostgresStore diverge\n  mock:     %#v\n  postgres: %#v", want, got)
			}
		})
	}
}

// contractErr reduces an error to its sentinel class; driver-specific wrapping
// and messages legitimately differ between the two stores.
func contractErr(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, store.ErrNotFound):
		return "not_found"
	case errors.Is(err, store.ErrInvalidCursor):
		return "invalid_cursor"
	case errors.Is(err, store.ErrInvalidScope):
		return "invalid_scope"
	default:
		return "error"
	}
}

// contractCursor only reports whether more pages exist: Postgres cursor
// payloads embed BIGSERIAL ids that MockStore cannot reproduce.
func contractCursor(cursor string) string {
	if cursor == "" {
		return "-"
	}
	return "+"
}

func contractTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func contractTimePtr(t *time.Time) string {
	if t == nil {
		return "nil"
	}
	return contractTime(*t)
}

func contractJoin(values []string) string {
	return strings.Join(values, ",")
}

func monitoredIDs(contracts []store.MonitoredContract) []string {
	out := make([]string, len(contracts))
	for i, c := range contracts {
		out[i] = c.ContractID
	}
	return out
}

func healthCheckHashes(checks []store.HealthCheck) []string {
	out := make([]string, len(checks))
	for i, c := range checks {
		out[i] = c.TxHash + "/" + c.Status
	}
	return out
}

func apiKeyContractScenario() contractScenario {
	return contractScenario{
		name:   "api_keys",
		tables: []string{"api_keys"},
		run: func(ctx context.Context, s store.FullStore) []string {
			var obs []string
			created := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
			keys := []store.APIKey{
				{ID: "key-a", Name: "Alpha", KeyPrefix: "sk_a", KeyHash: store.HashKey("secret-a"),
					Scopes: []string{store.ScopeReadContracts}, CreatedAt: created},
				{ID: "key-b", Name: "Beta", KeyPrefix: "sk_b", KeyHash: store.HashKey("secret-b"),
					Scopes: []string{store.ScopeReadWatchdog, store.ScopeWriteContracts}, CreatedAt: created.Add(time.Minute)},
				{ID: "key-c", Name: "Gamma", KeyPrefix: "sk_c", KeyHash: store.HashKey("secret-c"),
					Scopes: []string{store.ScopeAdmin}, CreatedAt: created.Add(2 * time.Minute)},
			}
			for _, k := range keys {
				obs = append(obs, "create:"+k.ID+":"+contractErr(s.CreateAPIKey(ctx, k)))
			}
			invalid := store.APIKey{ID: "key-bad", Name: "Bad", KeyPrefix: "sk_x",
				KeyHash: store.HashKey("secret-bad"), Scopes: []string{"not:a:scope"}, CreatedAt: created}
			obs = append(obs, "create-invalid:"+contractErr(s.CreateAPIKey(ctx, invalid)))

			k, err := s.GetAPIKeyByHash(ctx, store.HashKey("secret-b"))
			obs = append(obs, fmt.Sprintf("by-hash:%s:%s:%s:%s:%s", contractErr(err), k.ID, k.Name,
				contractJoin(k.Scopes), contractTime(k.CreatedAt)))
			_, err = s.GetAPIKeyByHash(ctx, "unknown-hash")
			obs = append(obs, "by-hash-unknown:"+contractErr(err))

			obs = append(obs, "rotate:"+contractErr(s.RotateAPIKey(ctx, "key-b", store.HashKey("secret-b2"), "sk_b2")))
			_, err = s.GetAPIKeyByHash(ctx, store.HashKey("secret-b"))
			obs = append(obs, "old-hash-after-rotate:"+contractErr(err))
			k, err = s.GetAPIKeyByHash(ctx, store.HashKey("secret-b2"))
			obs = append(obs, "new-hash-after-rotate:"+contractErr(err)+":"+k.KeyPrefix)
			rotated, err := s.GetAPIKey(ctx, "key-b")
			obs = append(obs, "rotate-stamp:"+contractErr(err)+":"+fmt.Sprintf("%t", rotated.RotatedAt != nil))
			obs = append(obs, "rotate-unknown:"+contractErr(s.RotateAPIKey(ctx, "missing", "h", "p")))

			obs = append(obs, "revoke:"+contractErr(s.RevokeAPIKey(ctx, "key-c")))
			obs = append(obs, "revoke-again:"+contractErr(s.RevokeAPIKey(ctx, "key-c")))
			obs = append(obs, "rotate-revoked:"+contractErr(s.RotateAPIKey(ctx, "key-c", store.HashKey("x"), "sk_x")))
			_, err = s.GetAPIKeyByHash(ctx, store.HashKey("secret-c"))
			obs = append(obs, "revoked-by-hash:"+contractErr(err))
			revoked, err := s.GetAPIKey(ctx, "key-c")
			obs = append(obs, "revoked-get:"+contractErr(err)+":"+fmt.Sprintf("%t", revoked.RevokedAt != nil))

			obs = append(obs, "touch:"+contractErr(s.TouchAPIKey(ctx, "key-a")))
			touched, err := s.GetAPIKey(ctx, "key-a")
			obs = append(obs, "touched-stamp:"+contractErr(err)+":"+fmt.Sprintf("%t", touched.LastUsedAt != nil))

			page1, next1, err := s.ListAPIKeys(ctx, "", 2)
			obs = append(obs, "page1:"+contractErr(err)+":"+contractJoin(idsOfKeys(page1))+":"+contractCursor(next1))
			page2, next2, err := s.ListAPIKeys(ctx, next1, 2)
			obs = append(obs, "page2:"+contractErr(err)+":"+contractJoin(idsOfKeys(page2))+":"+contractCursor(next2))
			return obs
		},
	}
}

func failedEventContractScenario() contractScenario {
	return contractScenario{
		name:   "failed_events",
		tables: []string{"failed_events"},
		run: func(ctx context.Context, s store.FullStore) []string {
			var obs []string
			created := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
			payload := []byte(`{"id":"evt-1","contract_id":"CFAIL","network":"testnet","ledger":42,"tx_hash":"deadbeef","type":"contract"}`)
			base := store.FailedEvent{
				EventID: "evt-1", ContractID: "CFAIL", Network: "testnet",
				EventPayload: payload, ErrorMessage: "first failure", Attempts: 4, CreatedAt: created,
			}
			obs = append(obs, "insert:"+contractErr(s.InsertFailedEvent(ctx, base)))

			upsert := base
			upsert.ErrorMessage = "second failure"
			upsert.Attempts = 6
			upsert.Network = "mainnet"
			obs = append(obs, "upsert:"+contractErr(s.InsertFailedEvent(ctx, upsert)))

			for i := 2; i <= 4; i++ {
				fe := store.FailedEvent{
					EventID: fmt.Sprintf("evt-%d", i), ContractID: "CFAIL", Network: "testnet",
					EventPayload: []byte(fmt.Sprintf(`{"id":"evt-%d"}`, i)),
					ErrorMessage: "boom", Attempts: i, CreatedAt: created.Add(time.Duration(i) * time.Minute),
				}
				obs = append(obs, fmt.Sprintf("insert-%d:%s", i, contractErr(s.InsertFailedEvent(ctx, fe))))
			}

			page1, next, err := s.ListFailedEvents(ctx, "", 2)
			obs = append(obs, "page1:"+contractErr(err)+":"+contractJoin(eventIDs(page1))+":"+contractCursor(next))
			page2, next2, err := s.ListFailedEvents(ctx, next, 2)
			obs = append(obs, "page2:"+contractErr(err)+":"+contractJoin(eventIDs(page2))+":"+contractCursor(next2))

			first, err := s.GetFailedEvent(ctx, page1[0].ID)
			obs = append(obs, fmt.Sprintf("get:%s:%s:%s:%s:%d:%s:%s", contractErr(err), first.EventID,
				first.ContractID, first.Network, first.Attempts, first.ErrorMessage, contractTime(first.CreatedAt)))
			raw, err := s.GetFailedEvent(ctx, page1[1].ID)
			obs = append(obs, fmt.Sprintf("get-upserted:%s:%s:%s:%d", contractErr(err), raw.EventID, raw.Network, raw.Attempts))

			obs = append(obs, "delete:"+contractErr(s.DeleteFailedEvent(ctx, page2[0].ID)))
			obs = append(obs, "delete-again:"+contractErr(s.DeleteFailedEvent(ctx, page2[0].ID)))
			_, err = s.GetFailedEvent(ctx, page2[0].ID)
			obs = append(obs, "get-deleted:"+contractErr(err))
			_, err = s.GetFailedEvent(ctx, 987654)
			obs = append(obs, "get-unknown:"+contractErr(err))
			return obs
		},
	}
}

func subscriptionContractScenario() contractScenario {
	return contractScenario{
		name:   "alert_subscriptions",
		tables: []string{"monitored_contracts"},
		run: func(ctx context.Context, s store.FullStore) []string {
			var obs []string
			base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
			for _, id := range []string{"CSUB-A", "CSUB-B"} {
				obs = append(obs, "monitor:"+id+":"+contractErr(s.UpsertMonitoredContract(ctx, store.MonitoredContract{
					ContractID: id, Network: "testnet", Name: id, Status: "Healthy", RegisteredAt: base,
				})))
			}
			subA := store.AlertSubscription{
				ID: "sub-a", ContractID: "CSUB-A", WebhookURL: "https://example.test/a",
				SeverityFilter: "Critical", ChannelType: "slack", RoutingKey: "route-1",
				SigningSecret: "whsec_a", SigningSecretHash: "hash-a",
				SigningSecretCreatedAt: base, CreatedAt: base, UpdatedAt: base,
			}
			subB := store.AlertSubscription{
				ID: "sub-b", ContractID: "CSUB-B", WebhookURL: "https://example.test/b",
				SeverityFilter: "", ChannelType: "", SigningSecret: "whsec_b",
				SigningSecretHash: "hash-b", SigningSecretCreatedAt: base.Add(time.Hour),
				CreatedAt: base.Add(time.Hour), UpdatedAt: base.Add(time.Hour),
			}
			obs = append(obs, "create-a:"+contractErr(s.Create(ctx, subA)))
			obs = append(obs, "create-b:"+contractErr(s.Create(ctx, subB)))

			got, err := s.GetSubscription(ctx, "sub-a")
			obs = append(obs, fmt.Sprintf("get:%s:%s:%s:%s:%s:%s:%s:%s:%s:%s:%s", contractErr(err), got.ID,
				got.ContractID, got.WebhookURL, got.SeverityFilter, got.ChannelType, got.RoutingKey,
				got.SigningSecret, got.SigningSecretHash, contractTime(got.SigningSecretCreatedAt), contractTime(got.CreatedAt)))
			_, err = s.GetSubscription(ctx, "missing")
			obs = append(obs, "get-unknown:"+contractErr(err))

			defaulted, err := s.GetSubscription(ctx, "sub-b")
			obs = append(obs, "channel-default:"+contractErr(err)+":"+defaulted.ChannelType)

			byContract, err := s.ListByContract(ctx, "CSUB-A")
			obs = append(obs, "by-contract:"+contractErr(err)+":"+contractJoin(subscriptionIDs(byContract)))
			byContract, err = s.ListByContract(ctx, "CSUB-B")
			obs = append(obs, "by-contract-b:"+contractErr(err)+":"+contractJoin(subscriptionIDs(byContract)))

			all, err := s.ListAll(ctx)
			obs = append(obs, "all:"+contractErr(err)+":"+contractJoin(subscriptionIDs(all)))

			rotatedAt := base.Add(2 * time.Hour)
			obs = append(obs, "rotate:"+contractErr(s.RotateSigningSecret(ctx, "sub-a", "whsec_a2", "hash-a2", rotatedAt)))
			rotated, err := s.GetSubscription(ctx, "sub-a")
			obs = append(obs, fmt.Sprintf("rotated:%s:%s:%s:%s:%s", contractErr(err), rotated.SigningSecret,
				rotated.SigningSecretHash, contractTimePtr(rotated.SigningSecretRotatedAt), contractTime(rotated.UpdatedAt)))
			obs = append(obs, "rotate-unknown:"+contractErr(s.RotateSigningSecret(ctx, "missing", "s", "h", rotatedAt)))

			obs = append(obs, "delete:"+contractErr(s.Delete(ctx, "sub-b")))
			obs = append(obs, "delete-again:"+contractErr(s.Delete(ctx, "sub-b")))
			obs = append(obs, "cascade-monitor:"+contractErr(s.DeleteMonitoredContract(ctx, "CSUB-A")))
			all, err = s.ListAll(ctx)
			obs = append(obs, "after-cascade:"+contractErr(err)+":"+contractJoin(subscriptionIDs(all)))
			return obs
		},
	}
}

func watchdogContractScenario() contractScenario {
	return contractScenario{
		name:   "watchdog",
		tables: []string{"monitored_contracts"},
		run: func(ctx context.Context, s store.FullStore) []string {
			var obs []string
			now := time.Now().UTC().Truncate(time.Second)
			base := now.Add(-48 * time.Hour)
			lastCheck := base.Add(6 * time.Hour)
			contracts := []store.MonitoredContract{
				{ContractID: "CW-A", Network: "testnet", Name: "Alpha", Owner: "team-a", Status: "Healthy",
					LastCheck: &lastCheck, CheckInterval: 60, RegisteredAt: base},
				{ContractID: "CW-B", Network: "testnet", Name: "Beta", Owner: "team-b", Status: "Degraded",
					CheckInterval: 120, RegisteredAt: base},
				{ContractID: "CW-C", Network: "mainnet", Name: "Gamma", Owner: "team-c", Status: "Unresponsive",
					CheckInterval: 300, RegisteredAt: base},
			}
			for _, mc := range contracts {
				obs = append(obs, "monitor:"+mc.ContractID+":"+contractErr(s.UpsertMonitoredContract(ctx, mc)))
			}
			// A nil LastCheck on update must preserve the stored value.
			obs = append(obs, "monitor-preserve:"+contractErr(s.UpsertMonitoredContract(ctx, store.MonitoredContract{
				ContractID: "CW-A", Network: "testnet", Name: "Alpha v2", Owner: "team-a", Status: "Healthy",
				CheckInterval: 90, RegisteredAt: base,
			})))
			got, err := s.GetMonitoredContract(ctx, "CW-A")
			obs = append(obs, fmt.Sprintf("get:%s:%s:%s:%s:%s:%d:%s", contractErr(err), got.ContractID,
				got.Name, got.Status, contractTimePtr(got.LastCheck), got.CheckInterval, contractTime(got.RegisteredAt)))
			_, err = s.GetMonitoredContract(ctx, "missing")
			obs = append(obs, "get-unknown:"+contractErr(err))

			page1, next, err := s.ListMonitoredContracts(ctx, "", 2, "")
			obs = append(obs, "page1:"+contractErr(err)+":"+contractJoin(monitoredIDs(page1))+":"+contractCursor(next))
			page2, next2, err := s.ListMonitoredContracts(ctx, next, 2, "")
			obs = append(obs, "page2:"+contractErr(err)+":"+contractJoin(monitoredIDs(page2))+":"+contractCursor(next2))
			mainnet, next3, err := s.ListMonitoredContracts(ctx, "", 10, "mainnet")
			obs = append(obs, "network:"+contractErr(err)+":"+contractJoin(monitoredIDs(mainnet))+":"+contractCursor(next3))

			checks := []store.HealthCheck{
				{ContractID: "CW-A", Status: "Healthy", Metadata: `{"cpu":1}`, Ledger: 100, TxHash: "tx-1", Timestamp: now.Add(-3 * time.Hour)},
				{ContractID: "CW-A", Status: "Degraded", Metadata: `{"cpu":2}`, Ledger: 101, TxHash: "tx-2", Timestamp: now.Add(-2 * time.Hour)},
				{ContractID: "CW-A", Status: "Healthy", Metadata: `{"cpu":3}`, Ledger: 102, TxHash: "tx-3", Timestamp: now.Add(-1 * time.Hour)},
			}
			for _, hc := range checks {
				obs = append(obs, "check:"+hc.TxHash+":"+contractErr(s.InsertHealthCheck(ctx, hc)))
			}
			obs = append(obs, "check-duplicate:"+contractErr(s.InsertHealthCheck(ctx, store.HealthCheck{
				ContractID: "CW-A", Status: "Unresponsive", Ledger: 99, TxHash: "tx-3", Timestamp: now,
			})))
			checksOut, err := s.ListHealthChecks(ctx, "CW-A", 2)
			obs = append(obs, "checks:"+contractErr(err)+":"+contractJoin(healthCheckHashes(checksOut)))
			checksOut, err = s.ListHealthChecks(ctx, "CW-A", 10)
			obs = append(obs, "checks-all:"+contractErr(err)+":"+contractJoin(healthCheckHashes(checksOut)))

			alerts := []store.ContractAlert{
				{ContractID: "CW-A", Severity: "Critical", Message: "a1", Ledger: 200, TxHash: "atx-1", Timestamp: now.Add(-4 * time.Hour)},
				{ContractID: "CW-A", Severity: "Info", Message: "a2", Ledger: 201, TxHash: "atx-2", Timestamp: now.Add(-3 * time.Hour)},
				{ContractID: "CW-B", Severity: "Critical", Message: "b1", Ledger: 202, TxHash: "btx-1", Timestamp: now.Add(-2 * time.Hour)},
				{ContractID: "CW-C", Severity: "Warning", Message: "c1", Ledger: 203, TxHash: "ctx-1", Timestamp: now.Add(-1 * time.Hour)},
			}
			for _, a := range alerts {
				obs = append(obs, "alert:"+a.TxHash+":"+contractErr(s.InsertContractAlert(ctx, a)))
			}
			obs = append(obs, "alert-duplicate:"+contractErr(s.InsertContractAlert(ctx, store.ContractAlert{
				ContractID: "CW-A", Severity: "Info", Message: "dup", Ledger: 1, TxHash: "atx-1", Timestamp: now,
			})))
			alertPage1, alertNext, err := s.ListAlerts(ctx, "", "", "", "", 2)
			obs = append(obs, "alerts-page1:"+contractErr(err)+":"+contractJoin(alertMessages(alertPage1))+":"+contractCursor(alertNext))
			alertPage2, alertNext2, err := s.ListAlerts(ctx, "", "", "", alertNext, 2)
			obs = append(obs, "alerts-page2:"+contractErr(err)+":"+contractJoin(alertMessages(alertPage2))+":"+contractCursor(alertNext2))
			critical, _, err := s.ListAlerts(ctx, "", "Critical", "", "", 10)
			obs = append(obs, "alerts-critical:"+contractErr(err)+":"+contractJoin(alertMessages(critical)))
			mainnetAlerts, _, err := s.ListAlerts(ctx, "", "", "mainnet", "", 10)
			obs = append(obs, "alerts-mainnet:"+contractErr(err)+":"+contractJoin(alertMessages(mainnetAlerts)))
			contractAlerts, _, err := s.ListAlerts(ctx, "CW-A", "", "", "", 10)
			obs = append(obs, "alerts-contract:"+contractErr(err)+":"+contractJoin(alertMessages(contractAlerts)))
			_, _, err = s.ListAlerts(ctx, "", "", "", "not-a-cursor", 10)
			obs = append(obs, "alerts-bad-cursor:"+contractErr(err))

			stats, err := s.GetWatchdogStats(ctx, "")
			obs = append(obs, fmt.Sprintf("stats-all:%s:%d/%d/%d/%d/%d/%d", contractErr(err), stats.TotalMonitored,
				stats.Healthy, stats.Degraded, stats.Unresponsive, stats.TotalAlerts, stats.CriticalAlerts))
			stats, err = s.GetWatchdogStats(ctx, "testnet")
			obs = append(obs, fmt.Sprintf("stats-testnet:%s:%d/%d/%d/%d/%d/%d", contractErr(err), stats.TotalMonitored,
				stats.Healthy, stats.Degraded, stats.Unresponsive, stats.TotalAlerts, stats.CriticalAlerts))

			for _, window := range []string{"24h", "7d", "30d", "bogus"} {
				up, err := s.GetContractUptime(ctx, "CW-A", window)
				if err != nil {
					obs = append(obs, "uptime:"+window+":"+contractErr(err))
					continue
				}
				obs = append(obs, fmt.Sprintf("uptime:%s:%s:%.2f", window, contractErr(err), up.Uptime))
			}

			obs = append(obs, "cascade:"+contractErr(s.DeleteMonitoredContract(ctx, "CW-B")))
			checksOut, err = s.ListHealthChecks(ctx, "CW-B", 10)
			obs = append(obs, "cascade-checks:"+contractErr(err)+":"+contractJoin(healthCheckHashes(checksOut)))
			alertPage, _, err := s.ListAlerts(ctx, "CW-B", "", "", "", 10)
			obs = append(obs, "cascade-alerts:"+contractErr(err)+":"+contractJoin(alertMessages(alertPage)))
			_, err = s.GetMonitoredContract(ctx, "CW-B")
			obs = append(obs, "cascade-get:"+contractErr(err))
			stats, err = s.GetWatchdogStats(ctx, "")
			obs = append(obs, fmt.Sprintf("stats-after-cascade:%s:%d/%d/%d/%d/%d/%d", contractErr(err), stats.TotalMonitored,
				stats.Healthy, stats.Degraded, stats.Unresponsive, stats.TotalAlerts, stats.CriticalAlerts))
			return obs
		},
	}
}

func alertGroupContractScenario() contractScenario {
	return contractScenario{
		name:   "alert_groups",
		tables: []string{"monitored_contracts"},
		run: func(ctx context.Context, s store.FullStore) []string {
			var obs []string
			base := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
			for _, mc := range []store.MonitoredContract{
				{ContractID: "CAG-A", Network: "testnet", Name: "A", Status: "Degraded", RegisteredAt: base},
				{ContractID: "CAG-B", Network: "testnet", Name: "B", Status: "Healthy", RegisteredAt: base},
				{ContractID: "CAG-C", Network: "mainnet", Name: "C", Status: "Healthy", RegisteredAt: base},
			} {
				obs = append(obs, "monitor:"+mc.ContractID+":"+contractErr(s.UpsertMonitoredContract(ctx, mc)))
			}

			groups := []store.AlertGroup{
				{GroupKey: "CAG-A|Critical|ttl", ContractID: "CAG-A", Severity: "Critical", Rule: "ttl",
					Count: 1, DedupeWindowSecs: 300, FirstSeen: base, LastSeen: base.Add(4 * time.Hour), LastMessage: "first"},
				{GroupKey: "CAG-A|Warning|fee", ContractID: "CAG-A", Severity: "Warning", Rule: "fee",
					Count: 2, DedupeWindowSecs: 300, FirstSeen: base, LastSeen: base.Add(3 * time.Hour), LastMessage: "fee"},
				{GroupKey: "CAG-B|Info|noop", ContractID: "CAG-B", Severity: "Info", Rule: "noop",
					Count: 1, DedupeWindowSecs: 60, FirstSeen: base, LastSeen: base.Add(2 * time.Hour), LastMessage: "info"},
				{GroupKey: "CAG-C|Critical|ttl", ContractID: "CAG-C", Severity: "Critical", Rule: "ttl",
					Count: 5, DedupeWindowSecs: 300, FirstSeen: base, LastSeen: base.Add(time.Hour), LastMessage: "c"},
			}
			for _, g := range groups {
				_, err := s.UpsertAlertGroup(ctx, g)
				obs = append(obs, "group:"+g.GroupKey+":"+contractErr(err))
			}

			merged, err := s.UpsertAlertGroup(ctx, store.AlertGroup{
				GroupKey: "CAG-A|Critical|ttl", ContractID: "CAG-A", Severity: "Critical", Rule: "ttl",
				Count: 2, DedupeWindowSecs: 300, FirstSeen: base.Add(5 * time.Hour),
				LastSeen: base.Add(6 * time.Hour), LastMessage: "merged",
			})
			obs = append(obs, fmt.Sprintf("merge:%s:%d:%s:%s", contractErr(err), merged.Count,
				contractTime(merged.FirstSeen), contractTime(merged.LastSeen)))

			newest, next, err := s.ListAlertGroups(ctx, "", 2, store.AlertGroupFilters{})
			obs = append(obs, "page1:"+contractErr(err)+":"+contractJoin(groupKeys(newest))+":"+contractCursor(next))
			rest, next2, err := s.ListAlertGroups(ctx, next, 2, store.AlertGroupFilters{})
			obs = append(obs, "page2:"+contractErr(err)+":"+contractJoin(groupKeys(rest))+":"+contractCursor(next2))
			_, _, err = s.ListAlertGroups(ctx, "bad-cursor", 2, store.AlertGroupFilters{})
			obs = append(obs, "bad-cursor:"+contractErr(err))

			byContract, _, err := s.ListAlertGroups(ctx, "", 10, store.AlertGroupFilters{ContractID: "CAG-A"})
			obs = append(obs, "by-contract:"+contractErr(err)+":"+contractJoin(groupKeys(byContract)))
			critical, _, err := s.ListAlertGroups(ctx, "", 10, store.AlertGroupFilters{Severity: "Critical"})
			obs = append(obs, "by-severity:"+contractErr(err)+":"+contractJoin(groupKeys(critical)))
			mainnet, _, err := s.ListAlertGroups(ctx, "", 10, store.AlertGroupFilters{Network: "mainnet"})
			obs = append(obs, "by-network:"+contractErr(err)+":"+contractJoin(groupKeys(mainnet)))
			combined, _, err := s.ListAlertGroups(ctx, "", 10, store.AlertGroupFilters{Network: "testnet", Severity: "Warning"})
			obs = append(obs, "combined:"+contractErr(err)+":"+contractJoin(groupKeys(combined)))

			obs = append(obs, "cascade:"+contractErr(s.DeleteMonitoredContract(ctx, "CAG-B")))
			after, _, err := s.ListAlertGroups(ctx, "", 10, store.AlertGroupFilters{})
			obs = append(obs, "after-cascade:"+contractErr(err)+":"+contractJoin(groupKeys(after)))
			return obs
		},
	}
}

func healthScoreContractScenario() contractScenario {
	now := time.Now().UTC().Truncate(time.Second)
	return contractScenario{
		name: "health_score",
		tables: []string{
			"contracts", "monitored_contracts", "invocations", "events",
			"storage_entries", "sync_state", "contract_health_scores",
		},
		run: func(ctx context.Context, s store.FullStore) []string {
			var obs []string
			base := now.Add(-72 * time.Hour)
			obs = append(obs, "contract:"+contractErr(s.UpsertContract(ctx, store.Contract{
				ID: "CHS", Network: "testnet", Label: "health", Status: "active", AddedAt: base,
			})))
			obs = append(obs, "monitor:"+contractErr(s.UpsertMonitoredContract(ctx, store.MonitoredContract{
				ContractID: "CHS", Network: "testnet", Name: "CHS", Status: "Degraded", RegisteredAt: base,
			})))

			for i := 0; i < 105; i++ {
				status := "Healthy"
				if i < 5 {
					status = "Unresponsive"
				}
				obs = append(obs, "check:"+contractErr(s.InsertHealthCheck(ctx, store.HealthCheck{
					ContractID: "CHS", Status: status, Ledger: int64(i), TxHash: fmt.Sprintf("hc-%03d", i),
					Timestamp: base.Add(time.Duration(i) * time.Minute),
				})))
			}
			invocations := []store.Invocation{
				{TxHash: "inv-1", ContractID: "CHS", Network: "testnet", Ledger: 10, LedgerClosedAt: now.Add(-2 * time.Hour), Status: "SUCCESS"},
				{TxHash: "inv-2", ContractID: "CHS", Network: "testnet", Ledger: 11, LedgerClosedAt: now.Add(-3 * time.Hour), Status: "FAILED"},
				{TxHash: "inv-3", ContractID: "CHS", Network: "testnet", Ledger: 12, LedgerClosedAt: now.Add(-4 * time.Hour), Status: "SUCCESS"},
				{TxHash: "inv-old", ContractID: "CHS", Network: "testnet", Ledger: 13, LedgerClosedAt: now.Add(-48 * time.Hour), Status: "FAILED"},
			}
			obs = append(obs, "invocations:"+contractErr(s.BatchInsertInvocations(ctx, invocations)))
			obs = append(obs, "events:"+contractErr(s.BatchInsertEvents(ctx, []store.Event{
				{ID: "evt-health", ContractID: "CHS", Network: "testnet", Ledger: 10, LedgerClosedAt: now.Add(-90 * time.Minute), TxHash: "inv-1", Type: "contract"},
			})))
			obs = append(obs, "sync:"+contractErr(s.UpsertSyncState(ctx, store.SyncState{ContractID: "CHS", LastLedger: 1000})))

			storage := []store.StorageEntry{
				{ContractID: "CHS", Network: "testnet", KeyXDR: "k-live-expiring", ValueXDR: "v", Durability: "persistent",
					LiveUntilLedger: 500, LastModifiedLedger: 10, Status: "live", LastSeenAt: now},
				{ContractID: "CHS", Network: "testnet", KeyXDR: "k-live-far", ValueXDR: "v", Durability: "persistent",
					LiveUntilLedger: 20000, LastModifiedLedger: 10, Status: "live", LastSeenAt: now},
				{ContractID: "CHS", Network: "testnet", KeyXDR: "k-live-horizon", ValueXDR: "v", Durability: "temporary",
					LiveUntilLedger: 18280, LastModifiedLedger: 10, Status: "live", LastSeenAt: now},
				{ContractID: "CHS", Network: "testnet", KeyXDR: "k-archived", ValueXDR: "v", Durability: "persistent",
					LiveUntilLedger: 1, LastModifiedLedger: 10, Status: "archived", LastSeenAt: now},
			}
			obs = append(obs, "storage:"+contractErr(s.UpsertStorageEntries(ctx, storage)))

			in, err := s.ContractHealthInputs(ctx, "CHS")
			var events, invokes int64
			for _, a := range in.Activity {
				events += a.EventCount
				invokes += a.InvokeCount
			}
			obs = append(obs, fmt.Sprintf("inputs:%s:%d/%d:%s:%d/%d:%d/%d:%d/%d", contractErr(err),
				in.HealthyChecks, in.TotalChecks, in.WatchdogStatus, in.TotalInvocations,
				in.FailedInvocations, in.TotalStorage, in.ExpiringStorage, events, invokes))

			empty, err := s.ContractHealthInputs(ctx, "CHS-EMPTY")
			obs = append(obs, fmt.Sprintf("empty:%s:%d/%d:%d/%d:%d/%d", contractErr(err),
				empty.HealthyChecks, empty.TotalChecks, empty.TotalInvocations, empty.FailedInvocations,
				empty.TotalStorage, empty.ExpiringStorage))

			score := store.ContractHealthScore{
				ContractID: "CHS", Score: 82, ComponentUptime: 90, ComponentErrorRate: 80,
				ComponentPerformance: 70, ComponentStorageTTL: 60, ComputedAt: base.Add(24 * time.Hour),
			}
			obs = append(obs, "score-upsert:"+contractErr(s.UpsertContractHealthScore(ctx, score)))
			got, err := s.GetContractHealthScore(ctx, "CHS")
			obs = append(obs, fmt.Sprintf("score-get:%s:%d:%d/%d/%d/%d:%s", contractErr(err), got.Score,
				got.ComponentUptime, got.ComponentErrorRate, got.ComponentPerformance,
				got.ComponentStorageTTL, contractTime(got.ComputedAt)))
			score.Score = 64
			score.ComputedAt = base.Add(48 * time.Hour)
			obs = append(obs, "score-replace:"+contractErr(s.UpsertContractHealthScore(ctx, score)))
			got, err = s.GetContractHealthScore(ctx, "CHS")
			obs = append(obs, fmt.Sprintf("score-after-replace:%s:%d:%s", contractErr(err), got.Score, contractTime(got.ComputedAt)))
			_, err = s.GetContractHealthScore(ctx, "missing")
			obs = append(obs, "score-unknown:"+contractErr(err))
			return obs
		},
	}
}

func globalEventContractScenario() contractScenario {
	return contractScenario{
		name:   "global_events",
		tables: []string{"contracts", "events", "invocations", "storage_entries"},
		run: func(ctx context.Context, s store.FullStore) []string {
			var obs []string
			base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
			for _, id := range []string{"CG-1", "CG-2"} {
				obs = append(obs, "contract:"+id+":"+contractErr(s.UpsertContract(ctx, store.Contract{
					ID: id, Network: "testnet", Label: id, Status: "active", AddedAt: base,
				})))
			}
			events := []store.Event{
				{ID: "gev-1", ContractID: "CG-1", Network: "testnet", Ledger: 1, LedgerClosedAt: base, TxHash: "deadbeef01", Type: "contract"},
				{ID: "gev-2", ContractID: "CG-1", Network: "testnet", Ledger: 2, LedgerClosedAt: base.Add(time.Hour), TxHash: "deadbeef02", Type: "contract"},
				{ID: "gev-3", ContractID: "CG-2", Network: "mainnet", Ledger: 3, LedgerClosedAt: base.Add(2 * time.Hour), TxHash: "cafebabe03", Type: "diagnostic"},
				{ID: "gev-4", ContractID: "CG-2", Network: "testnet", Ledger: 4, LedgerClosedAt: base.Add(3 * time.Hour), TxHash: "deadbeef04", Type: "contract"},
				{ID: "gev-5", ContractID: "CG-1", Network: "testnet", Ledger: 5, LedgerClosedAt: base.Add(4 * time.Hour), TxHash: "cafebabe05", Type: "contract"},
			}
			obs = append(obs, "events:"+contractErr(s.BatchInsertEvents(ctx, events)))
			invocations := []store.Invocation{
				{TxHash: "itx-1", ContractID: "CG-1", Network: "testnet", Ledger: 4, LedgerClosedAt: base.Add(3 * time.Hour),
					Status: "SUCCESS", FunctionName: "transfer", CPUInsn: 10},
				{TxHash: "itx-2", ContractID: "CG-2", Network: "mainnet", Ledger: 3, LedgerClosedAt: base.Add(2 * time.Hour),
					Status: "FAILED", FunctionName: "transfer_from", CPUInsn: 20},
			}
			obs = append(obs, "invocations:"+contractErr(s.BatchInsertInvocations(ctx, invocations)))
			obs = append(obs, "storage:"+contractErr(s.UpsertStorageEntries(ctx, []store.StorageEntry{
				{ContractID: "CG-1", Network: "testnet", KeyXDR: "k1", ValueXDR: "v1", Durability: "persistent",
					LastModifiedLedger: 4, Status: "live", LastSeenAt: base.Add(3 * time.Hour)},
				{ContractID: "CG-2", Network: "mainnet", KeyXDR: "k2", ValueXDR: "v2", Durability: "persistent",
					LastModifiedLedger: 3, Status: "live", LastSeenAt: base.Add(2 * time.Hour)},
			})))

			page1, next, err := s.ListAllEvents(ctx, "", 2, store.GlobalEventFilters{})
			obs = append(obs, "page1:"+contractErr(err)+":"+contractJoin(searchEventIDs(page1))+":"+contractCursor(next))
			page2, next2, err := s.ListAllEvents(ctx, next, 2, store.GlobalEventFilters{})
			obs = append(obs, "page2:"+contractErr(err)+":"+contractJoin(searchEventIDs(page2))+":"+contractCursor(next2))
			byPrefix, _, err := s.ListAllEvents(ctx, "", 10, store.GlobalEventFilters{ContractID: "CG-2"})
			obs = append(obs, "prefix:"+contractErr(err)+":"+contractJoin(searchEventIDs(byPrefix)))
			byType, _, err := s.ListAllEvents(ctx, "", 10, store.GlobalEventFilters{Type: "contract"})
			obs = append(obs, "type:"+contractErr(err)+":"+contractJoin(searchEventIDs(byType)))
			byNetwork, _, err := s.ListAllEvents(ctx, "", 10, store.GlobalEventFilters{Network: "mainnet"})
			obs = append(obs, "network:"+contractErr(err)+":"+contractJoin(searchEventIDs(byNetwork)))
			inRange, _, err := s.ListAllEvents(ctx, "", 10, store.GlobalEventFilters{
				Since: base.Add(time.Hour), Until: base.Add(3 * time.Hour),
			})
			obs = append(obs, "range:"+contractErr(err)+":"+contractJoin(searchEventIDs(inRange)))

			searched, err := s.SearchEvents(ctx, "deadbeef", 10)
			obs = append(obs, "search-events:"+contractErr(err)+":"+contractJoin(searchEventIDs(searched)))
			searched, err = s.SearchEvents(ctx, "cafebabe", 10)
			obs = append(obs, "search-events-2:"+contractErr(err)+":"+contractJoin(searchEventIDs(searched)))
			functions, err := s.SearchFunctions(ctx, "transfer", 10)
			obs = append(obs, "search-functions:"+contractErr(err)+":"+contractJoin(searchFunctionNames(functions)))
			functions, err = s.SearchFunctions(ctx, "zzz", 10)
			obs = append(obs, "search-functions-empty:"+contractErr(err)+":"+contractJoin(searchFunctionNames(functions)))

			stats, err := s.GetGlobalStats(ctx)
			obs = append(obs, fmt.Sprintf("stats:%s:%d/%d/%d/%d", contractErr(err), stats.TrackedContracts,
				stats.TotalEvents, stats.TotalInvocations, stats.TotalStorageEntries))
			return obs
		},
	}
}

func contractAndLabelContractScenario() contractScenario {
	return contractScenario{
		name:   "contracts_and_labels",
		tables: []string{"contracts", "labels_public", "labels_workspace", "users"},
		run: func(ctx context.Context, s store.FullStore) []string {
			var obs []string
			base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

			for _, id := range []string{"user-1", "user-2"} {
				_ = s.UpsertUser(ctx, store.User{ID: id})
			}

			// Stellar account IDs are 56 characters; the labels tables enforce
			// that length with a CHECK constraint.
			labels := []store.Label{
				{Label: "alpha", Value: "GALPHA" + strings.Repeat("0", 50), Public: true},
				{Label: "beta", Value: "GBETA" + strings.Repeat("0", 51), Public: true},
				{Label: "gamma", Value: "GGAMMA" + strings.Repeat("0", 50), WorkspaceID: "user-1"},
				{Label: "delta", Value: "GDELTA" + strings.Repeat("0", 50), WorkspaceID: "user-1"},
				{Label: "alpha", Value: "GALPHAWS" + strings.Repeat("0", 48), WorkspaceID: "user-1"},
				{Label: "epsilon", Value: "GEPSILON" + strings.Repeat("0", 48), WorkspaceID: "user-2"},
			}
			for _, l := range labels {
				obs = append(obs, "label:"+l.Label+":"+contractErr(s.UpsertLabel(ctx, l)))
			}
			listed, err := s.ListLabels(ctx, "user-1", "")
			obs = append(obs, "labels:"+contractErr(err)+":"+contractJoin(labelNames(listed)))
			filtered, err := s.ListLabels(ctx, "user-1", "GAMMA")
			obs = append(obs, "labels-query-value:"+contractErr(err)+":"+contractJoin(labelNames(filtered)))
			filtered, err = s.ListLabels(ctx, "user-1", "alp")
			obs = append(obs, "labels-query-label:"+contractErr(err)+":"+contractJoin(labelNames(filtered)))
			noWorkspace, err := s.ListLabels(ctx, "user-3", "alp")
			obs = append(obs, "labels-other-workspace:"+contractErr(err)+":"+contractJoin(labelNames(noWorkspace)))

			resolved, err := s.ResolveLabel(ctx, "user-1", "alpha")
			obs = append(obs, "resolve-workspace:"+contractErr(err)+":"+resolved.Label+":"+resolved.Value+":"+fmt.Sprintf("%t", resolved.Public))
			resolved, err = s.ResolveLabel(ctx, "user-2", "ALPHA")
			obs = append(obs, "resolve-public:"+contractErr(err)+":"+resolved.Label+":"+resolved.Value+":"+fmt.Sprintf("%t", resolved.Public))
			_, err = s.ResolveLabel(ctx, "user-1", "missing")
			obs = append(obs, "resolve-unknown:"+contractErr(err))

			obs = append(obs, "label-update:"+contractErr(s.UpsertLabel(ctx, store.Label{Label: "beta", Value: "GBETA2" + strings.Repeat("0", 50), Public: true})))
			resolved, err = s.ResolveLabel(ctx, "user-2", "beta")
			obs = append(obs, "resolve-updated:"+contractErr(err)+":"+resolved.Value)

			contracts := []store.Contract{
				{ID: "CC-1", Network: "testnet", Label: "one", WasmHash: "w1", CreatedAtLedger: 1, Status: "active", AddedAt: base},
				{ID: "CC-2", Network: "testnet", Label: "two", WasmHash: "w2", CreatedAtLedger: 2, Status: "paused", AddedAt: base.Add(time.Hour)},
				{ID: "CC-3", Network: "mainnet", Label: "three", WasmHash: "w3", CreatedAtLedger: 3, Status: "active", AddedAt: base.Add(2 * time.Hour)},
			}
			for _, c := range contracts {
				obs = append(obs, "contract:"+c.ID+":"+contractErr(s.UpsertContract(ctx, c)))
			}
			obs = append(obs, "contract-update:"+contractErr(s.UpsertContract(ctx, store.Contract{
				ID: "CC-1", Network: "testnet", Label: "one-updated", WasmHash: "w1", CreatedAtLedger: 1,
				Status: "active", AddedAt: base,
			})))
			got, err := s.GetContract(ctx, "CC-1")
			obs = append(obs, "contract-get:"+contractErr(err)+":"+got.Label+":"+got.Network+":"+contractTime(got.AddedAt))
			_, err = s.GetContract(ctx, "missing")
			obs = append(obs, "contract-get-unknown:"+contractErr(err))

			obs = append(obs, "tag-defi:"+contractErr(s.AddContractTag(ctx, "CC-1", "defi")))
			obs = append(obs, "tag-core:"+contractErr(s.AddContractTag(ctx, "CC-1", "core")))
			obs = append(obs, "tag-defi-again:"+contractErr(s.AddContractTag(ctx, "CC-1", "defi")))
			obs = append(obs, "tag-remove:"+contractErr(s.RemoveContractTag(ctx, "CC-1", "core")))
			obs = append(obs, "tag-remove-absent:"+contractErr(s.RemoveContractTag(ctx, "CC-1", "nope")))
			tags, err := s.ListContractTags(ctx, "CC-1")
			obs = append(obs, "tags:"+contractErr(err)+":"+contractJoin(tags))

			byNetwork, next, err := s.ListContracts(ctx, "", 10, store.ContractFilters{Network: "testnet"})
			obs = append(obs, "list-network:"+contractErr(err)+":"+contractJoin(contractIDs(byNetwork))+":"+contractCursor(next))
			byStatus, _, err := s.ListContracts(ctx, "", 10, store.ContractFilters{Status: "active"})
			obs = append(obs, "list-status:"+contractErr(err)+":"+contractJoin(contractIDs(byStatus)))
			sorted, _, err := s.ListContracts(ctx, "", 10, store.ContractFilters{Sort: "added_at", SortDir: "desc"})
			obs = append(obs, "list-sorted:"+contractErr(err)+":"+contractJoin(contractIDs(sorted)))
			page1, next, err := s.ListContracts(ctx, "", 2, store.ContractFilters{})
			obs = append(obs, "list-page1:"+contractErr(err)+":"+contractJoin(contractIDs(page1))+":"+contractCursor(next))
			page2, next2, err := s.ListContracts(ctx, next, 2, store.ContractFilters{})
			obs = append(obs, "list-page2:"+contractErr(err)+":"+contractJoin(contractIDs(page2))+":"+contractCursor(next2))

			searched, err := s.SearchContracts(ctx, "three", 10)
			obs = append(obs, "search:"+contractErr(err)+":"+contractJoin(contractIDs(searched)))
			searched, err = s.SearchContracts(ctx, "CC-", 10)
			obs = append(obs, "search-id:"+contractErr(err)+":"+contractJoin(contractIDs(searched)))
			searched, err = s.SearchContracts(ctx, "nothing", 10)
			obs = append(obs, "search-empty:"+contractErr(err)+":"+contractJoin(contractIDs(searched)))

			updated, err := s.SetContractLabel(ctx, []string{"CC-1", "CC-2", "missing"}, "bulk")
			obs = append(obs, fmt.Sprintf("set-label:%s:%d", contractErr(err), updated))
			cleared, err := s.SetContractLabel(ctx, []string{"CC-1"}, "")
			obs = append(obs, fmt.Sprintf("clear-label:%s:%d", contractErr(err), cleared))

			deleted, err := s.DeleteContracts(ctx, []string{"CC-2", "missing"})
			obs = append(obs, fmt.Sprintf("delete:%s:%d", contractErr(err), deleted))
			_, err = s.GetContract(ctx, "CC-2")
			obs = append(obs, "deleted-get:"+contractErr(err))
			remaining, _, err := s.ListContracts(ctx, "", 10, store.ContractFilters{})
			obs = append(obs, "after-delete:"+contractErr(err)+":"+contractJoin(contractIDs(remaining)))
			return obs
		},
	}
}
