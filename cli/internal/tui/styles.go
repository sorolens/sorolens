package tui

import "github.com/charmbracelet/lipgloss"

var (
	activeStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	inactiveStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().Bold(true)
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

	criticalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	infoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
)

func severityStyle(severity string) lipgloss.Style {
	switch severity {
	case "Critical":
		return criticalStyle
	case "Warning":
		return warningStyle
	default:
		return infoStyle
	}
}

func statusStyle(status string) lipgloss.Style {
	switch status {
	case "active", "SUCCESS":
		return okStyle
	case "error", "FAILED":
		return criticalStyle
	case "backfilling", "pending":
		return warningStyle
	default:
		return dimStyle
	}
}

// ttlStyle colors a storage entry by how many ledgers remain before expiry.
func ttlStyle(ledgersLeft int64) lipgloss.Style {
	switch {
	case ledgersLeft <= 1000:
		return criticalStyle
	case ledgersLeft <= 10000:
		return warningStyle
	default:
		return okStyle
	}
}
