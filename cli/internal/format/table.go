package format

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = func() lipgloss.Style {
		return lipgloss.NewStyle().Bold(true)
	}

	colorGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	colorYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	colorRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	noStyle     = lipgloss.NewStyle()
)

// RenderTable renders headers and rows as a plain aligned table.
// Column widths are computed from the widest cell in each column.
func RenderTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && lipgloss.Width(cell) > widths[i] {
				widths[i] = lipgloss.Width(cell)
			}
		}
	}

	hs := headerStyle()
	var lines []string

	// Header row
	headerCells := make([]string, len(headers))
	for i, h := range headers {
		headerCells[i] = hs.Width(widths[i]).Render(h)
	}
	lines = append(lines, strings.TrimRight(strings.Join(headerCells, "  "), " "))

	// Separator
	seps := make([]string, len(headers))
	for i, w := range widths {
		seps[i] = strings.Repeat("─", w) // "─"
	}
	lines = append(lines, strings.TrimRight(strings.Join(seps, "  "), " "))

	// Data rows
	for _, row := range rows {
		cells := make([]string, len(headers))
		for i := range headers {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			cells[i] = noStyle.Width(widths[i]).Render(cell)
		}
		lines = append(lines, strings.TrimRight(strings.Join(cells, "  "), " "))
	}

	return strings.Join(lines, "\n")
}

// RenderKeyValue renders a list of key-value pairs aligned at the colon.
func RenderKeyValue(pairs [][]string) string {
	maxKey := 0
	for _, p := range pairs {
		if len(p) > 0 && len(p[0]) > maxKey {
			maxKey = len(p[0])
		}
	}

	ks := headerStyle().Width(maxKey)
	var lines []string
	for _, p := range pairs {
		key, val := "", ""
		if len(p) > 0 {
			key = p[0]
		}
		if len(p) > 1 {
			val = p[1]
		}
		lines = append(lines, ks.Render(key)+"  "+val)
	}
	return strings.Join(lines, "\n")
}

// ColorByUrgency returns a lipgloss style colored by TTL urgency.
// green if > 10000 ledgers, yellow if > 1000, red if <= 1000.
func ColorByUrgency(ledgersLeft int) lipgloss.Style {
	if !IsColorEnabled() {
		return noStyle
	}
	if ledgersLeft <= 1000 {
		return colorRed
	}
	if ledgersLeft <= 10000 {
		return colorYellow
	}
	return colorGreen
}
