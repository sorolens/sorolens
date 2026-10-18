// Package tui implements the interactive, k9s-style terminal dashboard that
// backs the `sorolens tui` subcommand. It renders tracked contracts, their
// live events, invocations, watchdog alerts and storage TTL warnings from the
// Sorolens REST API and refreshes them on a timer.
package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sorolens/sorolens/cli/internal/client"
)

// API is the subset of the Sorolens REST client the dashboard needs. Keeping
// it behind an interface lets tests drive the UI with a headless fake.
type API interface {
	ListContracts(ctx context.Context, opts client.ListContractsOpts) (client.ContractsResponse, error)
	ListEvents(ctx context.Context, contractID string, opts client.ListEventsOpts) (client.EventsResponse, error)
	ListInvocations(ctx context.Context, contractID string, opts client.ListInvocationsOpts) (client.InvocationsResponse, error)
	ListAlerts(ctx context.Context, opts client.ListAlertsOpts) (client.AlertsResponse, error)
	GetStorage(ctx context.Context, contractID string) ([]client.StorageEntry, error)
}

// Options configures a dashboard Model.
type Options struct {
	// APIURL is only used for display in the header.
	APIURL string
	// RefreshInterval controls polling. Defaults to 5s when zero.
	RefreshInterval time.Duration
	// RequestTimeout bounds each refresh. Defaults to 10s when zero.
	RequestTimeout time.Duration
}

// pane identifies a focusable region of the dashboard.
type pane int

const (
	paneContracts pane = iota
	paneEvents
	paneInvocations
	paneAlerts
	paneStorage
	paneCount
)

var paneTitles = [paneCount]string{"Contracts", "Events", "Invocations", "Alerts", "Storage TTL"}

// rowsPerPane is the number of data rows each pane renders.
const rowsPerPane = 8

type tickMsg time.Time

// dataMsg carries the result of a refresh cycle.
type dataMsg struct {
	contracts   []client.Contract
	events      []client.Event
	invocations []client.Invocation
	alerts      []client.ContractAlert
	storage     []client.StorageEntry
	selected    string
	err         error
	at          time.Time
}

// Model is the bubbletea model for the dashboard.
type Model struct {
	api      API
	apiURL   string
	interval time.Duration
	timeout  time.Duration
	now      func() time.Time

	width  int
	height int

	contracts   []client.Contract
	contractIdx int
	selectedID  string

	events       []client.Event
	eventsIdx    int
	invocations  []client.Invocation
	invIdx       int
	alerts       []client.ContractAlert
	alertsIdx    int
	storage      []client.StorageEntry
	storageIdx   int
	lastRefresh  time.Time
	err          error
	errBannerFor string

	active    pane
	filter    string
	filtering bool
}

// New returns a dashboard model backed by api.
func New(api API, opts Options) Model {
	if opts.RefreshInterval <= 0 {
		opts.RefreshInterval = 5 * time.Second
	}
	if opts.RequestTimeout <= 0 {
		opts.RequestTimeout = 10 * time.Second
	}
	return Model{
		api:      api,
		apiURL:   opts.APIURL,
		interval: opts.RefreshInterval,
		timeout:  opts.RequestTimeout,
		now:      time.Now,
	}
}

// Init kicks off the first refresh and the polling timer.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchCmd(), m.tickCmd())
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(m.interval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// fetchCmd returns a command that loads the dashboard's data set.
func (m Model) fetchCmd() tea.Cmd {
	api := m.api
	selected := m.selectedID
	timeout := m.timeout
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return fetch(ctx, api, selected)
	}
}

// fetch loads contracts plus the selected contract's child data and the
// (global) alerts feed. A failure to list contracts is treated as fatal for
// the cycle and surfaced to the user; failures of the child feeds are not.
func fetch(ctx context.Context, api API, selected string) dataMsg {
	msg := dataMsg{at: time.Now()}

	contracts, err := api.ListContracts(ctx, client.ListContractsOpts{Limit: 100})
	if err != nil {
		msg.err = err
		return msg
	}
	msg.contracts = contracts.Contracts
	msg.selected = pickContract(msg.contracts, selected)

	if msg.selected != "" {
		if events, err := api.ListEvents(ctx, msg.selected, client.ListEventsOpts{Limit: 50}); err == nil {
			msg.events = events.Events
		}
		if inv, err := api.ListInvocations(ctx, msg.selected, client.ListInvocationsOpts{Limit: 50}); err == nil {
			msg.invocations = inv.Invocations
		}
		if storage, err := api.GetStorage(ctx, msg.selected); err == nil {
			msg.storage = storage
		}
	}
	if alerts, err := api.ListAlerts(ctx, client.ListAlertsOpts{Limit: 100}); err == nil {
		msg.alerts = alerts.Alerts
	}
	return msg
}

// pickContract keeps the current selection when it still exists, otherwise
// prefers the first active contract, then the first contract overall.
func pickContract(contracts []client.Contract, selected string) string {
	if selected != "" {
		for _, c := range contracts {
			if c.ID == selected {
				return selected
			}
		}
	}
	for _, c := range contracts {
		if c.Status == "active" {
			return c.ID
		}
	}
	if len(contracts) > 0 {
		return contracts[0].ID
	}
	return ""
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		return m, tea.Batch(m.fetchCmd(), m.tickCmd())
	case dataMsg:
		m.applyData(msg)
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.filtering {
		switch key {
		case "esc":
			m.filtering = false
			m.filter = ""
		case "enter":
			m.filtering = false
		case "ctrl+c":
			return m, tea.Quit
		case "backspace":
			if m.filter != "" {
				r := []rune(m.filter)
				m.filter = string(r[:len(r)-1])
			}
		default:
			if len(msg.Runes) > 0 {
				m.filter += string(msg.Runes)
			}
		}
		m.clampSelection()
		return m, nil
	}

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab":
		m.active = (m.active + 1) % paneCount
	case "shift+tab":
		m.active = (m.active + paneCount - 1) % paneCount
	case "1", "2", "3", "4", "5":
		m.active = pane(key[0] - '1')
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "/":
		m.filtering = true
		m.active = paneContracts
	case "esc":
		m.filter = ""
		m.clampSelection()
	case "enter":
		return m.drillDown()
	case "r":
		return m, m.fetchCmd()
	}
	return m, nil
}

func (m *Model) move(delta int) {
	switch m.active {
	case paneContracts:
		m.contractIdx = clamp(m.contractIdx+delta, len(m.visibleContracts()))
	case paneEvents:
		m.eventsIdx = clamp(m.eventsIdx+delta, len(m.events))
	case paneInvocations:
		m.invIdx = clamp(m.invIdx+delta, len(m.invocations))
	case paneAlerts:
		m.alertsIdx = clamp(m.alertsIdx+delta, len(m.alerts))
	case paneStorage:
		m.storageIdx = clamp(m.storageIdx+delta, len(m.storage))
	}
}

// drillDown selects the highlighted contract (or alert target) and reloads.
func (m Model) drillDown() (tea.Model, tea.Cmd) {
	switch m.active {
	case paneContracts:
		if vis := m.visibleContracts(); len(vis) > 0 {
			m.selectedID = vis[m.contractIdx].ID
			m.active = paneEvents
			m.resetChildSelection()
			return m, m.fetchCmd()
		}
	case paneAlerts:
		if m.alertsIdx < len(m.alerts) {
			if id := m.alerts[m.alertsIdx].ContractID; id != "" {
				m.selectedID = id
				m.active = paneEvents
				m.resetChildSelection()
				return m, m.fetchCmd()
			}
		}
	}
	return m, nil
}

func (m *Model) resetChildSelection() {
	m.eventsIdx, m.invIdx, m.alertsIdx, m.storageIdx = 0, 0, 0, 0
}

func (m *Model) applyData(msg dataMsg) {
	if msg.err != nil {
		m.err = msg.err
		m.errBannerFor = msg.err.Error()
		return
	}
	m.err = nil
	m.contracts = msg.contracts
	if msg.selected != "" {
		m.selectedID = msg.selected
	}
	m.events = msg.events
	m.invocations = msg.invocations
	m.alerts = msg.alerts
	m.storage = msg.storage
	m.lastRefresh = msg.at
	m.clampSelection()

	// Point the contracts cursor at the selected contract when visible.
	for i, c := range m.visibleContracts() {
		if c.ID == m.selectedID {
			m.contractIdx = i
			break
		}
	}
}

func (m *Model) clampSelection() {
	m.contractIdx = clamp(m.contractIdx, len(m.visibleContracts()))
	m.eventsIdx = clamp(m.eventsIdx, len(m.events))
	m.invIdx = clamp(m.invIdx, len(m.invocations))
	m.alertsIdx = clamp(m.alertsIdx, len(m.alerts))
	m.storageIdx = clamp(m.storageIdx, len(m.storage))
}

func clamp(i, n int) int {
	if n <= 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

// visibleContracts applies the active filter to the contract list.
func (m Model) visibleContracts() []client.Contract {
	if m.filter == "" {
		return m.contracts
	}
	f := strings.ToLower(m.filter)
	out := make([]client.Contract, 0, len(m.contracts))
	for _, c := range m.contracts {
		if strings.Contains(strings.ToLower(c.ID), f) ||
			strings.Contains(strings.ToLower(c.Label), f) {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) selectedContract() (client.Contract, bool) {
	for _, c := range m.contracts {
		if c.ID == m.selectedID {
			return c, true
		}
	}
	return client.Contract{}, false
}
