package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/spf13/cobra"
)

var reportCmd = &cobra.Command{
	Use:   "report [contract-id]",
	Short: "Get monthly SLA and uptime report for a contract",
	Long: `Display the monthly service-level agreement (SLA) and uptime report for a
tracked contract, derived from watchdog health checks and alerts.

Validates the reporting month format (YYYY-MM). When omitted, the current UTC
month is used.`,
	Example: `  # View report for the current month
  sorolens report CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA

  # View report for a specific month
  sorolens report CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA --month 2026-02

  # Output in machine-readable JSON format
  sorolens report CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA --month 2026-02 --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReport,
}

var (
	reportMonth    string
	reportContract string
)

func init() {
	reportCmd.Flags().StringVar(&reportMonth, "month", "", "Reporting month in YYYY-MM format (UTC)")
	reportCmd.Flags().StringVar(&reportContract, "contract", "", "Contract ID (alternative to positional argument)")
	rootCmd.AddCommand(reportCmd)
}

func runReport(cmd *cobra.Command, args []string) error {
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}

	contractFlag, _ := cmd.Flags().GetString("contract")
	contractID := contractFlag
	if len(args) > 0 {
		contractID = args[0]
	}

	c := client.New(globalConfig.APIURL, globalConfig.Timeout)

	if contractID == "" {
		// Attempt to resolve from tracked contracts if only one exists.
		contracts, err := c.ListContracts(cmd.Context(), client.ListContractsOpts{Limit: 2})
		if err == nil && len(contracts.Contracts) == 1 {
			contractID = contracts.Contracts[0].ID
		} else if err == nil && len(contracts.Contracts) > 1 {
			return fmt.Errorf("contract ID is required when multiple contracts are tracked: specify sorolens report <contract-id>")
		} else {
			return fmt.Errorf("contract ID is required: specify sorolens report <contract-id>")
		}
	}

	month, _ := cmd.Flags().GetString("month")
	if month != "" {
		if _, err := time.Parse("2006-01", month); err != nil {
			return fmt.Errorf("invalid --month format %q: must be YYYY-MM", month)
		}
	} else {
		month = time.Now().UTC().Format("2006-01")
	}

	stop := startSpinner(fmt.Sprintf("Fetching report for %s (%s)...", contractID, month))
	report, err := c.GetContractReport(cmd.Context(), contractID, month)
	stop()

	if err != nil {
		return fmt.Errorf("get contract report: %w", err)
	}

	if globalConfig.JSON {
		return format.PrintJSON(report)
	}

	status := "Met (>=99.9%)"
	if report.TotalChecks == 0 {
		status = "No Data"
	} else if report.UptimePct < 99.9 {
		status = "Breached (<99.9%)"
	}

	uptimeStr := "N/A"
	if report.TotalChecks > 0 {
		uptimeStr = fmt.Sprintf("%.2f%%", report.UptimePct)
	}

	signedStr := "No"
	if report.Signed {
		signedStr = fmt.Sprintf("Yes (%s)", report.SignatureAlgorithm)
	}

	pairs := [][]string{
		{"Contract ID", report.ContractID},
		{"Month", report.Month},
		{"SLA Status", status},
		{"Uptime", uptimeStr},
		{"Total Checks", fmt.Sprintf("%d", report.TotalChecks)},
		{"Healthy Checks", fmt.Sprintf("%d", report.HealthyChecks)},
		{"Incidents", fmt.Sprintf("%d", report.Incidents)},
		{"MTTR", fmt.Sprintf("%.1fs", report.MTTRSeconds)},
		{"Total Downtime", fmt.Sprintf("%.1fs", report.TotalDowntimeSeconds)},
		{"Longest Outage", fmt.Sprintf("%.1fs", report.LongestOutageSeconds)},
		{"Critical Alerts", fmt.Sprintf("%d", report.CriticalAlerts)},
		{"Warning Alerts", fmt.Sprintf("%d", report.WarningAlerts)},
		{"Total Alerts", fmt.Sprintf("%d", report.TotalAlerts)},
		{"Signed", signedStr},
	}
	fmt.Println(format.RenderKeyValue(pairs))
	return nil
}
