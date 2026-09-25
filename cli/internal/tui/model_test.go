package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sorolens/sorolens/cli/internal/client"
)

// fakeAPI is an in-memory API used to drive the dashboard headlessly.
type fakeAPI struct {
	contracts   []client.Contract
	events      map[string][]client.Event
	invocations map[string][]client.Invocation
	storage     map[string][]client.StorageEntry
	alerts      []client.ContractAlert
	err         error

	contractCalls int
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{
		contracts: []client.Contract{
			{ID: "CTESTALPHA0000000000000000000000000000000000000000000000", Label: "alpha", Status: "active"},
			{ID: "CTESTBETA00000000000000000000000000000000000000000000000", Label: "beta", Status: "paused"},
			{ID: "CTESTGAMMA000000000000000000000000000000000000000000000000", Label: "gamma", Status: "error"},
		},
		events: map[string][]client.Event{
			"CTESTALPHA0000000000000000000000000000000000000000000000": {
				{ID: "evt-1", Type: "contract", LedgerClosedAt: time.Unix(0, 0), ValueDecoded: map[string]any{"amount": 10}},
				{ID: "evt-2", Type: "transfer", LedgerClosedAt: time.Unix(0, 0), ValueDecoded: "hello"},
			},
		},
		invocations: map[string][]client.Invocation{
			"CTESTALPHA0000000000000000000000000000000000000000000000": {
				{TxHash: "tx-1", FunctionName: "transfer", Status: "SUCCESS", LedgerClosedAt: time.Unix(0, 0)},
				{TxHash: "tx-2", FunctionName: "mint", Status: "FAILED", LedgerClosedAt: time.Unix(0, 0)},
			},
		},
		storage: map[string][]client.StorageEntry{
			"CTESTALPHA0000000000000000000000000000000000000000000000": {
				{KeyXDR: "key-one", Durability: "persistent", LiveUntilLedger: 500},
				{KeyXDR: "key-two", Durability: "temporary", LiveUntilLedger: 50000},
			},
		},
		alerts: []client.ContractAlert{
			{ContractID: "CTESTBETA00000000000000000000000000000000000000000000000", Severity: "Critical", Message: "ttl low"},
			{ContractID: "CTESTALPHA0000000000000000000000000000000000000000000000", Severity: "Warning", Message: "missed ledger"},
		},
	}
}

func (f *fakeAPI) ListContracts(_ context.Context, _ client.ListContractsOpts) (client.ContractsResponse, error) {
	f.contractCalls++
	if f.err != nil {
		return client.ContractsResponse{}, f.err
	}
	return client.ContractsResponse{Contracts: f.contracts}, nil
}

func (f *fakeAPI) ListEvents(_ context.Context, id string, _ client.ListEventsOpts) (client.EventsResponse, error) {
	if f.err != nil {
		return client.EventsResponse{}, f.err
	}
	return client.EventsResponse{Events: f.events[id]}, nil
}

func (f *fakeAPI) ListInvocations(_ context.Context, id string, _ client.ListInvocationsOpts) (client.InvocationsResponse, error) {
	if f.err != nil {
		return client.InvocationsResponse{}, f.err
	}
	return client.InvocationsResponse{Invocations: f.invocations[id]}, nil
}

func (f *fakeAPI) ListAlerts(_ context.Context, _ client.ListAlertsOpts) (client.AlertsResponse, error) {
	if f.err != nil {
		return client.AlertsResponse{}, f.err
	}
	return client.AlertsResponse{Alerts: f.alerts}, nil
}

func (f *fakeAPI) GetStorage(_ context.Context, id string) ([]client.StorageEntry, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.storage[id], nil
}

func newTestModel(f *fakeAPI) Model {
	m := New(f, Options{APIURL: "http://localhost:8080", RefreshInterval: time.Hour, RequestTimeout: time.Second})
	m.width, m.height = 160, 60
	m.applyData(fetch(context.Background(), f, ""))
	return m
}

func key(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func press(m Model, s string) Model {
	mm, _ := m.Update(key(s))
	return mm.(Model)
}

func TestViewRendersAllPanes(t *testing.T) {
	m := newTestModel(newFakeAPI())
	view := m.View()
	for _, want := range []string{"Contracts", "Events", "Invocations", "Alerts", "Storage TTL", "alpha", "transfer", "ttl low"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\n%s", want, view)
		}
	}
}

func TestFetchSelectsFirstActiveContract(t *testing.T) {
	m := newTestModel(newFakeAPI())
	if m.selectedID != "CTESTALPHA0000000000000000000000000000000000000000000000" {
		t.Fatalf("selectedID = %q, want alpha", m.selectedID)
	}
	if len(m.events) != 2 || len(m.invocations) != 2 || len(m.storage) != 2 || len(m.alerts) != 2 {
		t.Fatalf("child data not loaded: events=%d invocations=%d storage=%d alerts=%d",
			len(m.events), len(m.invocations), len(m.storage), len(m.alerts))
	}
}

func TestNavigationClampsAtBoundaries(t *testing.T) {
	m := newTestModel(newFakeAPI())

	m = press(m, "down")
	if m.contractIdx != 1 {
		t.Fatalf("contractIdx after one down = %d, want 1", m.contractIdx)
	}
	m = press(m, "down")
	m = press(m, "down")
	if m.contractIdx != 2 {
		t.Fatalf("contractIdx should clamp at 2, got %d", m.contractIdx)
	}
	m = press(m, "up")
	m = press(m, "up")
	m = press(m, "up")
	m = press(m, "up")
	if m.contractIdx != 0 {
		t.Fatalf("contractIdx should clamp at 0, got %d", m.contractIdx)
	}
}

func TestTabAndNumberKeysCyclePanes(t *testing.T) {
	m := newTestModel(newFakeAPI())
	if m.active != paneContracts {
		t.Fatalf("active = %v, want paneContracts", m.active)
	}
	m = press(m, "tab")
	if m.active != paneEvents {
		t.Fatalf("active after tab = %v, want paneEvents", m.active)
	}
	m = press(m, "shift+tab")
	if m.active != paneContracts {
		t.Fatalf("active after shift+tab = %v, want paneContracts", m.active)
	}
	m = press(m, "4")
	if m.active != paneAlerts {
		t.Fatalf("active after '4' = %v, want paneAlerts", m.active)
	}
}

func TestFilterNarrowsContractList(t *testing.T) {
	m := newTestModel(newFakeAPI())
	m = press(m, "/")
	if !m.filtering {
		t.Fatal("expected filter mode after '/'")
	}
	for _, r := range "beta" {
		m = press(m, string(r))
	}
	if got := m.visibleContracts(); len(got) != 1 || got[0].Label != "beta" {
		t.Fatalf("filtered contracts = %+v, want only beta", got)
	}
	m = press(m, "esc")
	if m.filter != "" || m.filtering {
		t.Fatalf("esc should clear filter, got filter=%q filtering=%v", m.filter, m.filtering)
	}
}

func TestDrillDownSelectsContractAndSwitchesPane(t *testing.T) {
	m := newTestModel(newFakeAPI())
	m = press(m, "down") // beta
	mm, cmd := m.Update(key("enter"))
	m = mm.(Model)

	if m.selectedID != "CTESTBETA00000000000000000000000000000000000000000000000" {
		t.Fatalf("selectedID = %q, want beta", m.selectedID)
	}
	if m.active != paneEvents {
		t.Fatalf("active = %v, want paneEvents after drill-down", m.active)
	}
	if cmd == nil {
		t.Fatal("drill-down should trigger a refresh command")
	}
}

func TestDrillDownFromAlertsJumpsToContract(t *testing.T) {
	m := newTestModel(newFakeAPI())
	m = press(m, "4") // alerts pane
	mm, cmd := m.Update(key("enter"))
	m = mm.(Model)
	if m.selectedID != "CTESTBETA00000000000000000000000000000000000000000000000" {
		t.Fatalf("selectedID = %q, want beta alert target", m.selectedID)
	}
	if cmd == nil {
		t.Fatal("alert drill-down should trigger a refresh command")
	}
}

func TestGracefulAPIError(t *testing.T) {
	f := newFakeAPI()
	f.err = errors.New("connection refused")
	m := New(f, Options{APIURL: "http://localhost:8080", RefreshInterval: time.Hour})
	m.width, m.height = 160, 60
	m.applyData(fetch(context.Background(), f, ""))

	if m.err == nil {
		t.Fatal("expected error to be recorded")
	}
	view := m.View()
	if !strings.Contains(view, "API unreachable") {
		t.Errorf("view should surface unreachable API, got:\n%s", view)
	}
	if !strings.Contains(view, "waiting for API") {
		t.Errorf("panes should show a waiting message, got:\n%s", view)
	}
}

func TestTickSchedulesRefresh(t *testing.T) {
	m := newTestModel(newFakeAPI())
	_, cmd := m.Update(tickMsg(time.Now()))
	if cmd == nil {
		t.Fatal("tick should schedule a refresh command")
	}
}

func TestQuitOnQ(t *testing.T) {
	m := newTestModel(newFakeAPI())
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("q should return a command")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("q command should produce a quit message")
	}
}

func TestStorageSortedByUrgency(t *testing.T) {
	m := newTestModel(newFakeAPI())
	m = press(m, "5")
	lines := m.paneLines(paneStorage, 80, 5)
	if len(lines) != 2 {
		t.Fatalf("expected 2 storage rows, got %d", len(lines))
	}
	// key-one expires at ledger 500 and must sort before key-two at 50000.
	if !strings.Contains(lines[0], "key-one") {
		t.Errorf("first storage row should be most urgent, got %q", lines[0])
	}
}
