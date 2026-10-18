package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIterator_ContractsMultiPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		w.Header().Set("Content-Type", "application/json")

		switch cursor {
		case "":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contracts": []any{
					map[string]any{"id": "C1", "network": "testnet", "status": "active", "wasm_hash": "0x1", "added_at": time.Now()},
					map[string]any{"id": "C2", "network": "testnet", "status": "active", "wasm_hash": "0x2", "added_at": time.Now()},
				},
				"next_cursor": "cur_page2",
			})
		case "cur_page2":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contracts": []any{
					map[string]any{"id": "C3", "network": "testnet", "status": "active", "wasm_hash": "0x3", "added_at": time.Now()},
					map[string]any{"id": "C4", "network": "testnet", "status": "active", "wasm_hash": "0x4", "added_at": time.Now()},
				},
				"next_cursor": "cur_page3",
			})
		case "cur_page3":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contracts": []any{
					map[string]any{"id": "C5", "network": "testnet", "status": "active", "wasm_hash": "0x5", "added_at": time.Now()},
				},
				"next_cursor": "",
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	it := c.IterateContracts(context.Background(), &ListContractsParams{})
	var ids []string
	for it.Next() {
		ids = append(ids, it.Item().Id)
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iteration error: %v", err)
	}

	wantIDs := []string{"C1", "C2", "C3", "C4", "C5"}
	if len(ids) != len(wantIDs) {
		t.Fatalf("got %d contracts, want %d: %v", len(ids), len(wantIDs), ids)
	}
	for i, want := range wantIDs {
		if ids[i] != want {
			t.Errorf("contract[%d] = %q, want %q", i, ids[i], want)
		}
	}
}

func TestIterator_NextPageBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		w.Header().Set("Content-Type", "application/json")

		if cursor == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contracts": []any{
					map[string]any{"id": "C1", "network": "testnet", "status": "active", "wasm_hash": "0x1", "added_at": time.Now()},
					map[string]any{"id": "C2", "network": "testnet", "status": "active", "wasm_hash": "0x2", "added_at": time.Now()},
				},
				"next_cursor": "cur_2",
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contracts": []any{
					map[string]any{"id": "C3", "network": "testnet", "status": "active", "wasm_hash": "0x3", "added_at": time.Now()},
				},
				"next_cursor": "",
			})
		}
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	it := c.IterateContracts(context.Background(), nil)

	// Fetch page 1
	p1, err := it.NextPage()
	if err != nil {
		t.Fatalf("page 1 err: %v", err)
	}
	if len(p1) != 2 {
		t.Fatalf("page 1 length = %d, want 2", len(p1))
	}
	if !it.HasMore() {
		t.Fatal("expected HasMore() to be true after page 1")
	}

	// Fetch page 2
	p2, err := it.NextPage()
	if err != nil {
		t.Fatalf("page 2 err: %v", err)
	}
	if len(p2) != 1 {
		t.Fatalf("page 2 length = %d, want 1", len(p2))
	}

	// Fetch beyond end
	p3, err := it.NextPage()
	if err != nil {
		t.Fatalf("page 3 err: %v", err)
	}
	if len(p3) != 0 {
		t.Fatalf("expected empty slice beyond end, got %d items", len(p3))
	}
}

func TestIterator_Collect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		w.Header().Set("Content-Type", "application/json")

		if cursor == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contracts": []any{
					map[string]any{"id": "C1", "network": "testnet", "status": "active", "wasm_hash": "0x1", "added_at": time.Now()},
				},
				"next_cursor": "p2",
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contracts": []any{
					map[string]any{"id": "C2", "network": "testnet", "status": "active", "wasm_hash": "0x2", "added_at": time.Now()},
				},
				"next_cursor": "",
			})
		}
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	it := c.IterateContracts(context.Background(), nil)
	all, err := it.Collect()
	if err != nil {
		t.Fatalf("Collect err: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("Collect got %d items, want 2", len(all))
	}
	if all[0].Id != "C1" || all[1].Id != "C2" {
		t.Errorf("unexpected IDs: %v", all)
	}
}

func TestIterator_AllRangeLoop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		w.Header().Set("Content-Type", "application/json")

		if cursor == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contracts": []any{
					map[string]any{"id": "C1", "network": "testnet", "status": "active", "wasm_hash": "0x1", "added_at": time.Now()},
				},
				"next_cursor": "end",
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contracts": []any{
					map[string]any{"id": "C2", "network": "testnet", "status": "active", "wasm_hash": "0x2", "added_at": time.Now()},
				},
				"next_cursor": "",
			})
		}
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	it := c.IterateContracts(context.Background(), nil)
	var count int
	for contract, err := range it.All() {
		if err != nil {
			t.Fatalf("unexpected range error: %v", err)
		}
		count++
		if count == 1 && contract.Id != "C1" {
			t.Errorf("item 1 id = %q, want C1", contract.Id)
		}
		if count == 2 && contract.Id != "C2" {
			t.Errorf("item 2 id = %q, want C2", contract.Id)
		}
	}
	if count != 2 {
		t.Fatalf("iterated %d items, want 2", count)
	}
}

func TestIterator_CursorResumption(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		w.Header().Set("Content-Type", "application/json")

		if cursor != "resume_token" {
			t.Errorf("expected cursor 'resume_token', got %q", cursor)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"contracts": []any{
				map[string]any{"id": "C_RESUMED", "network": "testnet", "status": "active", "wasm_hash": "0x1", "added_at": time.Now()},
			},
			"next_cursor": "",
		})
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resumeCursor := "resume_token"
	it := c.IterateContracts(context.Background(), &ListContractsParams{
		Cursor: &resumeCursor,
	})
	if !it.Next() {
		t.Fatal("expected Next() = true")
	}
	if it.Item().Id != "C_RESUMED" {
		t.Errorf("got id = %q, want C_RESUMED", it.Item().Id)
	}
	if it.Next() {
		t.Fatal("expected Next() = false at end")
	}
}

func TestIterator_EmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"contracts":   []any{},
			"next_cursor": "",
		})
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	it := c.IterateContracts(context.Background(), nil)
	if it.Next() {
		t.Fatal("expected Next() = false for empty list")
	}
	if err := it.Err(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestIterator_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"contracts": []any{
				map[string]any{"id": "C1", "network": "testnet", "status": "active", "wasm_hash": "0x1", "added_at": time.Now()},
			},
			"next_cursor": "cur_next",
		})
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	it := c.IterateContracts(ctx, nil)

	if !it.Next() {
		t.Fatal("expected first Next() to succeed")
	}
	cancel()

	if it.Next() {
		t.Fatal("expected Next() to return false after context cancel")
	}
	if !errors.Is(it.Err(), context.Canceled) {
		t.Fatalf("expected context.Canceled error, got %v", it.Err())
	}
}

func TestIterator_ServerError(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contracts": []any{
					map[string]any{"id": "C1", "network": "testnet", "status": "active", "wasm_hash": "0x1", "added_at": time.Now()},
				},
				"next_cursor": "cur_err",
			})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal database error"))
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil,
		WithBackoff(1*time.Millisecond, 2*time.Millisecond),
		WithoutRetries(),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	it := c.IterateContracts(context.Background(), nil)
	if !it.Next() {
		t.Fatal("expected first item to succeed")
	}
	if it.Next() {
		t.Fatal("expected Next() to fail on page 2")
	}
	if it.Err() == nil {
		t.Fatal("expected non-nil error after 500 response")
	}
}

func TestIterator_AllEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		switch p {
		case "/api/v1/contracts/C1/events":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"events": []any{
					map[string]any{"id": "E1", "contract_id": "C1", "type": "transfer", "ledger": 100, "tx_hash": "0x1", "ledger_closed_at": time.Now()},
				},
				"next_cursor": "",
			})
		case "/api/v1/events":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"events": []any{
					map[string]any{"id": "E2", "contract_id": "C2", "type": "mint", "ledger": 101, "tx_hash": "0x2", "ledger_closed_at": time.Now()},
				},
				"next_cursor": "",
			})
		case "/api/v1/contracts/C1/invocations":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"invocations": []any{
					map[string]any{"id": "I1", "contract_id": "C1", "function_name": "transfer", "ledger": 100, "status": "success", "tx_hash": "0x1", "ledger_closed_at": time.Now()},
				},
				"next_cursor": "",
			})
		case "/api/v1/contracts/C1/storage":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"storage": []any{
					map[string]any{"contract_id": "C1", "key_xdr": "balance", "durability": "persistent", "status": "live", "last_modified_ledger": 100},
				},
				"next_cursor": "",
			})
		case "/api/v1/watchdog/alerts":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"alerts": []any{
					map[string]any{"contract_id": "C1", "severity": "Warning", "message": "TTL low", "ledger": 100, "tx_hash": "0x1", "timestamp": time.Now()},
				},
				"next_cursor": "",
			})
		case "/api/v1/watchdog/contracts":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contracts": []any{
					map[string]any{"contract_id": "C1", "network": "testnet", "status": "Healthy", "last_check_ledger": 100, "last_check_time": time.Now()},
				},
				"next_cursor": "",
			})
		case "/api/v1/watchdog/contracts/C1/alerts":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"alerts": []any{
					map[string]any{"contract_id": "C1", "severity": "Warning", "message": "Archive risk", "ledger": 100, "tx_hash": "0x1", "timestamp": time.Now()},
				},
				"next_cursor": "",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintf(w, "unknown path: %s", p)
		}
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	// 1. IterateContractEvents
	eventsIt := c.IterateContractEvents(ctx, "C1", nil)
	events, err := eventsIt.Collect()
	if err != nil {
		t.Fatalf("IterateContractEvents: %v", err)
	}
	if len(events) != 1 || events[0].Id != "E1" {
		t.Errorf("events: %+v", events)
	}

	// 2. IterateAllEvents
	allEventsIt := c.IterateAllEvents(ctx, nil)
	allEvents, err := allEventsIt.Collect()
	if err != nil {
		t.Fatalf("IterateAllEvents: %v", err)
	}
	if len(allEvents) != 1 || allEvents[0].Id != "E2" {
		t.Errorf("allEvents: %+v", allEvents)
	}

	// 3. IterateContractInvocations
	invIt := c.IterateContractInvocations(ctx, "C1", nil)
	invs, err := invIt.Collect()
	if err != nil {
		t.Fatalf("IterateContractInvocations: %v", err)
	}
	if len(invs) != 1 || invs[0].ContractId != "C1" {
		t.Errorf("invocations: %+v", invs)
	}

	// 4. IterateContractStorage
	storIt := c.IterateContractStorage(ctx, "C1", nil)
	storage, err := storIt.Collect()
	if err != nil {
		t.Fatalf("IterateContractStorage: %v", err)
	}
	if len(storage) != 1 || storage[0].KeyXdr != "balance" {
		t.Errorf("storage: %+v", storage)
	}

	// 5. IterateWatchdogAlerts
	alertsIt := c.IterateWatchdogAlerts(ctx, nil)
	alerts, err := alertsIt.Collect()
	if err != nil {
		t.Fatalf("IterateWatchdogAlerts: %v", err)
	}
	if len(alerts) != 1 || alerts[0].ContractId != "C1" {
		t.Errorf("alerts: %+v", alerts)
	}

	// 6. IterateMonitoredContracts
	monIt := c.IterateMonitoredContracts(ctx, nil)
	monContracts, err := monIt.Collect()
	if err != nil {
		t.Fatalf("IterateMonitoredContracts: %v", err)
	}
	if len(monContracts) != 1 || monContracts[0].ContractId != "C1" {
		t.Errorf("monitored contracts: %+v", monContracts)
	}

	// 7. IterateContractAlerts
	cAlertsIt := c.IterateContractAlerts(ctx, "C1", nil)
	cAlerts, err := cAlertsIt.Collect()
	if err != nil {
		t.Fatalf("IterateContractAlerts: %v", err)
	}
	if len(cAlerts) != 1 || cAlerts[0].Severity != ContractAlertSeverityWarning {
		t.Errorf("contract alerts: %+v", cAlerts)
	}
}

func TestIterator_CursorAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"contracts": []any{
				map[string]any{"id": "C1", "network": "testnet", "status": "active", "wasm_hash": "0x1", "added_at": time.Now()},
			},
			"next_cursor": "page_token_2",
		})
	}))
	defer server.Close()

	c, err := New(server.URL, "sl_test", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	it := c.IterateContracts(context.Background(), nil)
	if !it.Next() {
		t.Fatal("Next() failed")
	}
	if it.Cursor() != "" {
		t.Errorf("initial page cursor was %q, want empty", it.Cursor())
	}
	if it.NextCursor() != "page_token_2" {
		t.Errorf("NextCursor = %q, want page_token_2", it.NextCursor())
	}
}

