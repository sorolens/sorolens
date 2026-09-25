package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sorolens/sorolens/cli/internal/client"
)

// View implements tea.Model.
func (m Model) View() string {
	width := m.width
	if width <= 0 {
		width = 100
	}

	var b strings.Builder
	b.WriteString(m.headerView(width))
	b.WriteString("\n")

	col := (width - 3) / 2
	boxHeight := rowsPerPane + 2

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		m.paneView(paneContracts, col, boxHeight), " ",
		m.paneView(paneEvents, col, boxHeight)))
	b.WriteString("\n")

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		m.paneView(paneInvocations, col, boxHeight), " ",
		m.paneView(paneAlerts, col, boxHeight)))
	b.WriteString("\n")

	b.WriteString(m.paneView(paneStorage, width, boxHeight))
	b.WriteString("\n")
	b.WriteString(m.footerView())
	return b.String()
}

func (m Model) headerView(width int) string {
	title := titleStyle.Render("sorolens tui")
	line := fmt.Sprintf("%s  %s", title, dimStyle.Render(m.apiURL))

	if m.err != nil {
		line += "\n" + errorStyle.Render(truncate(
			fmt.Sprintf("API unreachable at %s: %v — retrying every %s", m.apiURL, m.err, m.interval),
			width))
	}
	return line
}

// paneView renders one bordered pane, highlighting it when focused.
func (m Model) paneView(p pane, width, height int) string {
	if width < 12 {
		width = 12
	}
	inner := width - 4
	if inner < 4 {
		inner = 4
	}

	title := paneTitles[p]
	switch {
	case p == paneContracts && m.filtering:
		title = fmt.Sprintf("Contracts — filter: %s▏", m.filter)
	case p == paneContracts && m.filter != "":
		title = fmt.Sprintf("Contracts — filter: %s", m.filter)
	case p == paneContracts:
		if c, ok := m.selectedContract(); ok {
			title = fmt.Sprintf("Contracts — %s", contractLabel(c))
		}
	case p == paneStorage && m.selectedID != "":
		title = "Storage TTL — " + shortID(m.selectedID)
	}

	lines := m.paneLines(p, inner, height-3)
	content := titleStyle.Render(title) + "\n" + strings.Join(lines, "\n")

	style := inactiveStyle
	if p == m.active {
		style = activeStyle
	}
	return style.Width(width).Height(height).Render(content)
}

func (m Model) paneLines(p pane, inner, rows int) []string {
	if m.err != nil {
		// Keep the data we already have visible; the header carries the error.
		if len(m.contracts) == 0 {
			return []string{dimStyle.Render("waiting for API…")}
		}
	}

	switch p {
	case paneContracts:
		vis := m.visibleContracts()
		if len(vis) == 0 {
			return []string{dimStyle.Render("no tracked contracts")}
		}
		start := windowStart(m.contractIdx, len(vis), rows)
		out := make([]string, 0, rows)
		for i := start; i < len(vis) && i < start+rows; i++ {
			c := vis[i]
			marker := "  "
			if i == m.contractIdx && m.active == paneContracts {
				marker = "▸ "
			}
			label := c.Label
			if label == "" {
				label = shortID(c.ID)
			}
			line := fmt.Sprintf("%s%s  %s", marker, truncate(label, inner-14),
				statusStyle(c.Status).Render(c.Status))
			out = append(out, truncate(line, inner))
		}
		return out

	case paneEvents:
		if len(m.events) == 0 {
			return []string{dimStyle.Render("no events")}
		}
		start := windowStart(m.eventsIdx, len(m.events), rows)
		out := make([]string, 0, rows)
		for i := start; i < len(m.events) && i < start+rows; i++ {
			e := m.events[i]
			marker := "  "
			if i == m.eventsIdx && m.active == paneEvents {
				marker = "▸ "
			}
			line := fmt.Sprintf("%s%s  %s  %s", marker,
				e.LedgerClosedAt.Format("15:04:05"), truncate(e.Type, 14),
				truncate(summarize(e.ValueDecoded), inner-28))
			out = append(out, truncate(line, inner))
		}
		return out

	case paneInvocations:
		if len(m.invocations) == 0 {
			return []string{dimStyle.Render("no invocations")}
		}
		start := windowStart(m.invIdx, len(m.invocations), rows)
		out := make([]string, 0, rows)
		for i := start; i < len(m.invocations) && i < start+rows; i++ {
			inv := m.invocations[i]
			marker := "  "
			if i == m.invIdx && m.active == paneInvocations {
				marker = "▸ "
			}
			line := fmt.Sprintf("%s%s  %s  %s", marker,
				inv.LedgerClosedAt.Format("15:04:05"), truncate(inv.FunctionName, 18),
				statusStyle(inv.Status).Render(inv.Status))
			out = append(out, truncate(line, inner))
		}
		return out

	case paneAlerts:
		if len(m.alerts) == 0 {
			return []string{dimStyle.Render("no alerts")}
		}
		start := windowStart(m.alertsIdx, len(m.alerts), rows)
		out := make([]string, 0, rows)
		for i := start; i < len(m.alerts) && i < start+rows; i++ {
			a := m.alerts[i]
			marker := "  "
			if i == m.alertsIdx && m.active == paneAlerts {
				marker = "▸ "
			}
			line := fmt.Sprintf("%s%s  %s  %s", marker,
				severityStyle(a.Severity).Render(truncate(a.Severity, 8)),
				shortID(a.ContractID), truncate(a.Message, inner-32))
			out = append(out, truncate(line, inner))
		}
		return out

	case paneStorage:
		if len(m.storage) == 0 {
			return []string{dimStyle.Render("no storage entries")}
		}
		sorted := make([]client.StorageEntry, len(m.storage))
		copy(sorted, m.storage)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].LiveUntilLedger < sorted[j].LiveUntilLedger
		})
		start := windowStart(m.storageIdx, len(sorted), rows)
		out := make([]string, 0, rows)
		for i := start; i < len(sorted) && i < start+rows; i++ {
			e := sorted[i]
			marker := "  "
			if i == m.storageIdx && m.active == paneStorage {
				marker = "▸ "
			}
			line := fmt.Sprintf("%s%s  %s  live until %d", marker,
				truncate(shortID(e.KeyXDR), 20), e.Durability, e.LiveUntilLedger)
			out = append(out, ttlStyle(e.LiveUntilLedger).Render(truncate(line, inner)))
		}
		return out
	}
	return nil
}

func (m Model) footerView() string {
	keys := "tab/shift+tab: pane  1-5: jump  j/k: move  enter: drill down  /: filter  r: refresh  q: quit"
	status := "ready"
	if m.err != nil {
		status = "api error"
	} else if m.lastRefresh.IsZero() {
		status = "loading…"
	} else {
		status = fmt.Sprintf("updated %s", m.lastRefresh.Format("15:04:05"))
	}
	return dimStyle.Render(keys) + "\n" + dimStyle.Render("status: "+status)
}

// windowStart keeps the selected row inside the visible window.
func windowStart(idx, total, rows int) int {
	if rows <= 0 || total <= rows {
		return 0
	}
	start := idx - rows/2
	if start < 0 {
		return 0
	}
	if start+rows > total {
		return total - rows
	}
	return start
}

func contractLabel(c client.Contract) string {
	if c.Label != "" {
		return c.Label
	}
	return shortID(c.ID)
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:8] + "…" + id[len(id)-4:]
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

func summarize(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%v", k, val[k]))
		}
		return strings.Join(parts, " ")
	default:
		return fmt.Sprintf("%v", v)
	}
}
