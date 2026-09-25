package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
)

// watchdogCmd is the parent command for watchdog-related subcommands.
var watchdogCmd = &cobra.Command{
	Use:   "watchdog",
	Short: "Interact with the Sorolens watchdog",
}

var watchdogStatusCmd = &cobra.Command{
	Use:   "status <contract-id>",
	Short: "Print the current on-chain health of a monitored contract",
	Args:  cobra.ExactArgs(1),
	RunE:  runWatchdogStatus,
}

func init() {
	watchdogCmd.AddCommand(watchdogStatusCmd)
	rootCmd.AddCommand(watchdogCmd)
}

func runWatchdogStatus(cmd *cobra.Command, args []string) error {
	contractID := args[0]
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}

	c := client.New(globalConfig.APIURL, globalConfig.Timeout)

	stop := startSpinner("Fetching watchdog status...")
	mc, err := c.GetMonitoredContract(cmd.Context(), contractID)
	stop()

	if err != nil {
		if se, ok := err.(*client.SorolensError); ok && se.Status == 404 {
			fmt.Fprintf(os.Stderr, "Monitored contract not found: %s\n", contractID)
			os.Exit(1)
		}
		return fmt.Errorf("get monitored contract: %w", err)
	}

	if globalConfig.JSON {
		return format.PrintJSON(mc)
	}

	lastCheck := "never"
	if mc.LastCheck != nil {
		lastCheck = mc.LastCheck.Format(time.RFC3339)
	}

	pairs := [][]string{
		{"Contract ID", mc.ContractID},
		{"Name", mc.Name},
		{"Network", mc.Network},
		{"Owner", mc.Owner},
		{"Status", mc.Status},
		{"Last Check", lastCheck},
		{"Check Interval (s)", fmt.Sprintf("%d", mc.CheckInterval)},
		{"Registered At", mc.RegisteredAt.Format(time.RFC3339)},
		{"Updated At", mc.UpdatedAt.Format(time.RFC3339)},
	}
	fmt.Println(format.RenderKeyValue(pairs))
	return nil
}
