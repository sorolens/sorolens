package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <contract-id>",
	Short: "Show detailed information about a tracked contract",
	Args:  cobra.ExactArgs(1),
	RunE:  runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}

func runInspect(cmd *cobra.Command, args []string) error {
	contractID := args[0]
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}

	c := client.New(globalConfig.APIURL, globalConfig.Timeout)

	stop := startSpinner("Fetching contract...")
	contract, err := c.GetContract(cmd.Context(), contractID)
	stop()

	if err != nil {
		if se, ok := err.(*client.SorolensError); ok && se.Status == 404 {
			fmt.Fprintf(os.Stderr, "Contract not found: %s\n", contractID)
			os.Exit(1)
		}
		return err
	}

	// Fetch 24h stats (best-effort; display N/A on error).
	stats, _ := c.GetContractStats(cmd.Context(), contractID, "24h")

	if globalConfig.JSON {
		out := map[string]any{
			"contract": contract,
			"stats":    stats,
		}
		return format.PrintJSON(out)
	}

	firstSeen := contract.AddedAt.Format(time.RFC3339)
	backfill := "pending"
	if contract.BackfillCompleteAt != nil {
		backfill = contract.BackfillCompleteAt.Format(time.RFC3339)
	}

	pairs := [][]string{
		{"Alias / Label", contract.Label},
		{"Contract ID", contract.ID},
		{"Network", contract.Network},
		{"Status", contract.Status},
		{"First Seen", firstSeen},
		{"Backfill Complete", backfill},
		{"Last Synced Ledger", fmt.Sprintf("%d", stats.LastSyncedLedger)},
		{"Events (24h)", fmt.Sprintf("%d", stats.WindowEventCount)},
		{"Invocations (24h)", fmt.Sprintf("%d", stats.WindowInvocationCount)},
		{"Storage Entries", fmt.Sprintf("%d", stats.StorageCount)},
	}
	fmt.Println(format.RenderKeyValue(pairs))
	return nil
}

// startSpinner displays a simple spinner goroutine until the returned stop function is called.
func startSpinner(msg string) func() {
	if !format.IsColorEnabled() {
		return func() {}
	}
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	stop := make(chan struct{})
	go func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				fmt.Print("\r\033[K")
				return
			default:
				fmt.Printf("\r%s %s", frames[i%len(frames)], msg)
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
	return func() {
		close(stop)
		time.Sleep(10 * time.Millisecond)
	}
}
