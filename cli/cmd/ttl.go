package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
)

var ttlCmd = &cobra.Command{
	Use:   "ttl <contract-id>",
	Short: "Show storage entry TTL health for a tracked contract",
	Args:  cobra.ExactArgs(1),
	RunE:  runTTL,
}

func init() {
	rootCmd.AddCommand(ttlCmd)
}

func runTTL(cmd *cobra.Command, args []string) error {
	contractID := args[0]
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}

	c := client.New(globalConfig.APIURL, globalConfig.Timeout)
	entries, err := c.GetStorage(cmd.Context(), contractID)
	if err != nil {
		return fmt.Errorf("get storage: %w", err)
	}

	if globalConfig.JSON {
		return format.PrintJSON(entries)
	}

	// Sort by live_until_ledger ascending (most urgent first).
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LiveUntilLedger < entries[j].LiveUntilLedger
	})

	headers := []string{"Key Hash", "Durability", "Live Until Ledger", "Ledgers Until Expiry"}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		keyHash := shortHash(e.KeyXDR)
		liveUntil := strconv.FormatInt(e.LiveUntilLedger, 10)
		// Ledgers until expiry is shown as the raw live_until_ledger when no current
		// ledger is available from the API response.
		ledgersLeft := int(e.LiveUntilLedger)
		urgency := format.ColorByUrgency(ledgersLeft)
		rows[i] = []string{
			urgency.Render(keyHash),
			urgency.Render(e.Durability),
			urgency.Render(liveUntil),
			urgency.Render(liveUntil),
		}
	}

	if len(entries) == 0 {
		fmt.Println("No storage entries found for", contractID)
		return nil
	}
	fmt.Println(format.RenderTable(headers, rows))
	return nil
}
