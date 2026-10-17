package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot <contract-id>",
	Short: "Show a storage and state snapshot for a contract",
	Long: `Display a snapshot of a contract's storage entries and state.

By default, fetches the current state snapshot export. Supply --ledger to view
a historical storage snapshot as of a specific ledger sequence number.`,
	Example: `  # View current contract snapshot
  sorolens snapshot CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA

  # View snapshot at a historical ledger
  sorolens snapshot CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA --ledger 123456

  # Output in machine-readable JSON format
  sorolens snapshot CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA --json`,
	Args: cobra.ExactArgs(1),
	RunE: runSnapshot,
}

var snapshotLedger uint32

func init() {
	snapshotCmd.Flags().Uint32Var(&snapshotLedger, "ledger", 0, "Ledger sequence number to snapshot at")
	rootCmd.AddCommand(snapshotCmd)
}

func truncateXDR(xdr string) string {
	if len(xdr) <= 12 {
		return xdr
	}
	return xdr[:6] + "..." + xdr[len(xdr)-6:]
}

func runSnapshot(cmd *cobra.Command, args []string) error {
	contractID := args[0]
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}

	ledger, _ := cmd.Flags().GetUint32("ledger")
	c := client.New(globalConfig.APIURL, globalConfig.Timeout)

	if ledger > 0 {
		stop := startSpinner(fmt.Sprintf("Fetching snapshot for %s at ledger %d...", contractID, ledger))
		snap, err := c.GetContractSnapshot(cmd.Context(), contractID, ledger)
		stop()

		if err != nil {
			return fmt.Errorf("get contract snapshot: %w", err)
		}

		if globalConfig.JSON {
			return format.PrintJSON(snap)
		}

		pairs := [][]string{
			{"Contract ID", snap.ContractID},
			{"Network", snap.Network},
			{"Ledger", fmt.Sprintf("%d", snap.Ledger)},
			{"First Tracked Ledger", fmt.Sprintf("%d", snap.FirstTrackedLedger)},
			{"Storage Entries", fmt.Sprintf("%d", len(snap.Storage))},
		}
		if snap.LastEvent != nil {
			pairs = append(pairs, []string{"Last Event", fmt.Sprintf("%s (ledger %d)", snap.LastEvent.Type, snap.LastEvent.Ledger)})
		}
		fmt.Println(format.RenderKeyValue(pairs))

		if len(snap.Storage) > 0 {
			fmt.Println("\nStorage Entries:")
			headers := []string{"Key Hash", "Durability", "Live Until Ledger"}
			rows := make([][]string, len(snap.Storage))
			for i, e := range snap.Storage {
				rows[i] = []string{
					truncateXDR(e.KeyXDR),
					e.Durability,
					strconv.FormatInt(e.LiveUntilLedger, 10),
				}
			}
			fmt.Println(format.RenderTable(headers, rows))
		}
		return nil
	}

	stop := startSpinner(fmt.Sprintf("Fetching snapshot for %s...", contractID))
	snapExport, err := c.GetContractSnapshotExport(cmd.Context(), contractID)
	stop()

	if err != nil {
		return fmt.Errorf("get contract snapshot: %w", err)
	}

	if globalConfig.JSON {
		return format.PrintJSON(snapExport)
	}

	pairs := [][]string{
		{"Contract ID", snapExport.ContractID},
		{"Network", snapExport.Network},
		{"Ledger", fmt.Sprintf("%d", snapExport.Ledger)},
		{"Storage Entries", fmt.Sprintf("%d", len(snapExport.Storage))},
		{"Recent Events", fmt.Sprintf("%d", len(snapExport.Events))},
	}
	fmt.Println(format.RenderKeyValue(pairs))

	if len(snapExport.Storage) > 0 {
		fmt.Println("\nStorage Entries:")
		headers := []string{"Key Hash", "Durability", "Live Until Ledger"}
		rows := make([][]string, len(snapExport.Storage))
		for i, e := range snapExport.Storage {
			rows[i] = []string{
				truncateXDR(e.KeyXDR),
				e.Durability,
				strconv.FormatInt(e.LiveUntilLedger, 10),
			}
		}
		fmt.Println(format.RenderTable(headers, rows))
	}
	return nil
}
