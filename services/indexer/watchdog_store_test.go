package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
	"github.com/sorolens/sorolens/services/indexer/internal/watchdog"
)

type mockWatchdogStore struct {
	upserted   []wdMonitoredContract
	deleted    []string
	healths    []wdHealthCheck
	alerts     []wdContractAlert
	upsertErr  error
	deleteErr  error
	healthErr  error
	alertErr   error
}

func (m *mockWatchdogStore) UpsertMonitoredContract(_ context.Context, c wdMonitoredContract) error {
	m.upserted = append(m.upserted, c)
	return m.upsertErr
}

func (m *mockWatchdogStore) DeleteMonitoredContract(_ context.Context, contractID string) error {
	m.deleted = append(m.deleted, contractID)
	return m.deleteErr
}

func (m *mockWatchdogStore) InsertHealthCheck(_ context.Context, h wdHealthCheck) error {
	m.healths = append(m.healths, h)
	return m.healthErr
}

func (m *mockWatchdogStore) InsertContractAlert(_ context.Context, a wdContractAlert) error {
	m.alerts = append(m.alerts, a)
	return m.alertErr
}

type mockRPCClient struct {
	eventsRes *poller.GetEventsResult
	eventsErr error
}

func (m *mockRPCClient) GetLatestLedger(context.Context) (*poller.LatestLedger, error) {
	return nil, nil
}

func (m *mockRPCClient) GetEvents(context.Context, uint32, uint32, []poller.EventFilter) (*poller.GetEventsResult, error) {
	return m.eventsRes, m.eventsErr
}

func (m *mockRPCClient) GetTransaction(context.Context, string) (*poller.TransactionResult, error) {
	return nil, nil
}

func (m *mockRPCClient) GetLedgerEntries(context.Context, []string) (*poller.GetLedgerEntriesResult, error) {
	return nil, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestWatchdogInterceptor_Unwrap(t *testing.T) {
	inner := &mockRPCClient{}
	interceptor := &watchdogInterceptor{
		RPCClient: inner,
		log:       testLogger(),
	}
	if interceptor.Unwrap() != inner {
		t.Errorf("Unwrap() = %v, want %v", interceptor.Unwrap(), inner)
	}
}

func TestWatchdogInterceptor_DisabledOrUnconfigured(t *testing.T) {
	ctx := context.Background()
	store := &mockWatchdogStore{}
	inner := &mockRPCClient{
		eventsRes: &poller.GetEventsResult{
			Events: []poller.RPCEvent{
				{
					ContractID: "CWATCHDOG",
					Topic:      []string{watchdog.TopicContractDeregistered, "CTARGET"},
				},
			},
		},
	}

	tests := []struct {
		name        string
		enabled     bool
		contract    string
		store       watchdogStore
		rpcErr      error
		wantCall    bool
		wantErrPass bool
	}{
		{
			name:     "disabled",
			enabled:  false,
			contract: "CWATCHDOG",
			store:    store,
			wantCall: false,
		},
		{
			name:     "empty contract",
			enabled:  true,
			contract: "",
			store:    store,
			wantCall: false,
		},
		{
			name:     "nil store",
			enabled:  true,
			contract: "CWATCHDOG",
			store:    nil,
			wantCall: false,
		},
		{
			name:        "rpc error propagates",
			enabled:     true,
			contract:    "CWATCHDOG",
			store:       store,
			rpcErr:      errors.New("rpc failed"),
			wantCall:    false,
			wantErrPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store.deleted = nil
			rpc := &mockRPCClient{
				eventsRes: inner.eventsRes,
				eventsErr: tt.rpcErr,
			}
			wi := &watchdogInterceptor{
				RPCClient: rpc,
				store:     tt.store,
				enabled:   tt.enabled,
				contract:  tt.contract,
				log:       testLogger(),
			}

			res, err := wi.GetEvents(ctx, 1, 10, nil)
			if tt.wantErrPass {
				if !errors.Is(err, tt.rpcErr) {
					t.Errorf("expected rpcErr %v, got %v", tt.rpcErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res == nil {
				t.Fatal("expected non-nil result")
			}
			if len(store.deleted) > 0 && !tt.wantCall {
				t.Errorf("store was called unexpectedly: %v", store.deleted)
			}
		})
	}
}

func TestWatchdogInterceptor_GetEventsRouting(t *testing.T) {
	ctx := context.Background()
	const wdContract = "CWATCHDOG"
	const targetContract = "CTARGET123"

	nowStr := time.Now().UTC().Format(time.RFC3339)

	tests := []struct {
		name        string
		event       poller.RPCEvent
		verifyStore func(t *testing.T, s *mockWatchdogStore)
	}{
		{
			name: "contract registered",
			event: poller.RPCEvent{
				ContractID:     wdContract,
				Topic:          []string{watchdog.TopicContractRegistered, targetContract},
				Ledger:         100,
				TxHash:         "txreg",
				LedgerClosedAt: nowStr,
			},
			verifyStore: func(t *testing.T, s *mockWatchdogStore) {
				if len(s.upserted) != 1 {
					t.Fatalf("expected 1 upsert, got %d", len(s.upserted))
				}
				if s.upserted[0].ContractID != targetContract {
					t.Errorf("ContractID = %q, want %q", s.upserted[0].ContractID, targetContract)
				}
			},
		},
		{
			name: "contract deregistered",
			event: poller.RPCEvent{
				ContractID:     wdContract,
				Topic:          []string{watchdog.TopicContractDeregistered, targetContract},
				Ledger:         101,
				TxHash:         "txdereg",
				LedgerClosedAt: nowStr,
			},
			verifyStore: func(t *testing.T, s *mockWatchdogStore) {
				if len(s.deleted) != 1 {
					t.Fatalf("expected 1 delete, got %d", len(s.deleted))
				}
				if s.deleted[0] != targetContract {
					t.Errorf("ContractID = %q, want %q", s.deleted[0], targetContract)
				}
			},
		},
		{
			name: "health check",
			event: poller.RPCEvent{
				ContractID:     wdContract,
				Topic:          []string{watchdog.TopicHealthCheck, targetContract},
				Ledger:         102,
				TxHash:         "txhealth",
				LedgerClosedAt: nowStr,
			},
			verifyStore: func(t *testing.T, s *mockWatchdogStore) {
				if len(s.healths) != 1 {
					t.Fatalf("expected 1 health check, got %d", len(s.healths))
				}
				if s.healths[0].ContractID != targetContract {
					t.Errorf("ContractID = %q, want %q", s.healths[0].ContractID, targetContract)
				}
				if s.healths[0].Ledger != 102 {
					t.Errorf("Ledger = %d, want 102", s.healths[0].Ledger)
				}
				if s.healths[0].TxHash != "txhealth" {
					t.Errorf("TxHash = %q, want txhealth", s.healths[0].TxHash)
				}
			},
		},
		{
			name: "contract alert",
			event: poller.RPCEvent{
				ContractID:     wdContract,
				Topic:          []string{watchdog.TopicContractAlert, targetContract, "Warning"},
				Ledger:         103,
				TxHash:         "txalert",
				LedgerClosedAt: nowStr,
			},
			verifyStore: func(t *testing.T, s *mockWatchdogStore) {
				if len(s.alerts) != 1 {
					t.Fatalf("expected 1 alert, got %d", len(s.alerts))
				}
				if s.alerts[0].ContractID != targetContract {
					t.Errorf("ContractID = %q, want %q", s.alerts[0].ContractID, targetContract)
				}
				if s.alerts[0].Severity != "Warning" {
					t.Errorf("Severity = %q, want Warning", s.alerts[0].Severity)
				}
				if s.alerts[0].Ledger != 103 {
					t.Errorf("Ledger = %d, want 103", s.alerts[0].Ledger)
				}
				if s.alerts[0].TxHash != "txalert" {
					t.Errorf("TxHash = %q, want txalert", s.alerts[0].TxHash)
				}
			},
		},
		{
			name: "different contract ignored",
			event: poller.RPCEvent{
				ContractID:     "COTHER",
				Topic:          []string{watchdog.TopicContractAlert, targetContract, "Critical"},
				Ledger:         104,
				TxHash:         "txother",
				LedgerClosedAt: nowStr,
			},
			verifyStore: func(t *testing.T, s *mockWatchdogStore) {
				if len(s.upserted) != 0 || len(s.deleted) != 0 || len(s.healths) != 0 || len(s.alerts) != 0 {
					t.Errorf("store unexpectedly called for other contract")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockWatchdogStore{}
			wi := &watchdogInterceptor{
				RPCClient: &mockRPCClient{
					eventsRes: &poller.GetEventsResult{
						Events: []poller.RPCEvent{tt.event},
					},
				},
				store:    store,
				enabled:  true,
				contract: wdContract,
				log:      testLogger(),
			}

			res, err := wi.GetEvents(ctx, 1, 200, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res == nil {
				t.Fatal("expected result")
			}
			tt.verifyStore(t, store)
		})
	}
}

func TestDecodeWatchdogValue(t *testing.T) {
	t.Run("empty string returns empty map", func(t *testing.T) {
		m := decodeWatchdogValue("")
		if len(m) != 0 {
			t.Errorf("expected empty map, got %v", m)
		}
	})

	t.Run("invalid base64 returns empty map", func(t *testing.T) {
		m := decodeWatchdogValue("!invalid!")
		if len(m) != 0 {
			t.Errorf("expected empty map, got %v", m)
		}
	})

	t.Run("valid symbol xdr", func(t *testing.T) {
		// ScVal symbol "hello"
		buf := make([]byte, 4+4+5)
		binary.BigEndian.PutUint32(buf[:4], 15) // scvSymbol
		binary.BigEndian.PutUint32(buf[4:8], 5)
		copy(buf[8:], "hello")
		b64 := base64.StdEncoding.EncodeToString(buf)

		m := decodeWatchdogValue(b64)
		if m["type"] != "scvSymbol" {
			t.Errorf("type = %v, want scvSymbol", m["type"])
		}
		if m["human"] != "hello" {
			t.Errorf("human = %v, want hello", m["human"])
		}
	})
}
