package cmd

import (
	"fmt"
	"os"

	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/spf13/cobra"
)

var alertsCmd = &cobra.Command{
	Use:   "alerts",
	Short: "View and manage watchdog alerts",
	Long: `alerts commands allow you to query and inspect alerts emitted by the
Sorolens watchdog system for monitored contracts.`,
	Example: `  # List all recent alerts
  sorolens alerts list

  # Filter alerts by severity
  sorolens alerts list --severity critical

  # Output in machine-readable JSON format
  sorolens alerts list --json`,
}

var alertsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List watchdog alerts",
	Long: `List watchdog alerts across monitored contracts, optionally filtered by severity.`,
	Example: `  # List recent alerts
  sorolens alerts list

  # Filter by severity
  sorolens alerts list --severity critical

  # Limit number of alerts returned
  sorolens alerts list --limit 20

  # Output in machine-readable JSON format
  sorolens alerts list --json`,
	Args: cobra.NoArgs,
	RunE: runAlertsList,
}

var (
	alertsSeverity string
	alertsLimit    int
)

func init() {
	alertsListCmd.Flags().StringVarP(&alertsSeverity, "severity", "s", "", "Filter alerts by severity (e.g. critical, warning, info)")
	alertsListCmd.Flags().IntVarP(&alertsLimit, "limit", "l", 50, "Maximum number of alerts to fetch")
	alertsCmd.AddCommand(alertsListCmd)
	rootCmd.AddCommand(alertsCmd)
}

func runAlertsList(cmd *cobra.Command, _ []string) error {
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}

	severity, _ := cmd.Flags().GetString("severity")
	limit, _ := cmd.Flags().GetInt("limit")

	c := client.New(globalConfig.APIURL, globalConfig.Timeout)

	stop := startSpinner("Fetching alerts...")
	resp, err := c.ListAlerts(cmd.Context(), client.ListAlertsOpts{
		Severity: severity,
		Limit:    limit,
	})
	stop()

	if err != nil {
		return fmt.Errorf("list alerts: %w", err)
	}

	if globalConfig.JSON {
		alerts := resp.Alerts
		if alerts == nil {
			alerts = []client.ContractAlert{}
		}
		return format.PrintJSON(alerts)
	}

	if len(resp.Alerts) == 0 {
		fmt.Println("No alerts found.")
		return nil
	}

	headers := []string{"Time", "Severity", "Contract ID", "Ledger", "Message"}
	rows := make([][]string, len(resp.Alerts))
	for i, a := range resp.Alerts {
		tStr := "-"
		if !a.Timestamp.IsZero() {
			tStr = a.Timestamp.Format("2006-01-02 15:04:05")
		}
		ledgerStr := fmt.Sprintf("%d", a.Ledger)
		rows[i] = []string{
			tStr,
			a.Severity,
			a.ContractID,
			ledgerStr,
			a.Message,
		}
	}
	fmt.Println(format.RenderTable(headers, rows))
	return nil
}
