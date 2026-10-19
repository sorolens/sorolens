package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
)

var contractsCmd = &cobra.Command{
	Use:   "contracts",
	Short: "List and inspect tracked contracts",
}

var contractsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tracked contracts",
	Long: `List the contracts tracked by the Sorolens API on the selected network.

Use --network to choose the network (default: testnet), --limit to cap the
number of rows, and --json for machine-readable output.`,
	Example: `  sorolens contracts list
  sorolens contracts list --network mainnet --limit 10
  sorolens contracts list --json`,
	Args: cobra.NoArgs,
	RunE: runContractsList,
}

var contractsGetCmd = &cobra.Command{
	Use:   "get <contract-id>",
	Short: "Show a tracked contract",
	Long:  "Show the registration details of a single tracked contract.",
	Example: `  sorolens contracts get CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
  sorolens contracts get <contract-id> --json`,
	Args: cobra.ExactArgs(1),
	RunE: runContractsGet,
}

var contractsLimit int

func init() {
	contractsListCmd.Flags().IntVar(&contractsLimit, "limit", 50, "Maximum number of contracts to list")
	contractsCmd.AddCommand(contractsListCmd, contractsGetCmd)
	rootCmd.AddCommand(contractsCmd)
}

func runContractsList(cmd *cobra.Command, _ []string) error {
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}
	if contractsLimit < 1 {
		return fmt.Errorf("invalid --limit %d: must be at least 1", contractsLimit)
	}

	c := client.New(globalConfig.APIURL, globalConfig.Timeout)
	stop := startSpinner("Fetching contracts...")
	resp, err := c.ListContracts(cmd.Context(), client.ListContractsOpts{
		Network: globalConfig.Network,
		Limit:   contractsLimit,
	})
	stop()
	if err != nil {
		return err
	}

	// The API caps pages on its own; never print more than requested.
	contracts := resp.Contracts
	if len(contracts) > contractsLimit {
		contracts = contracts[:contractsLimit]
	}

	if globalConfig.JSON {
		if contracts == nil {
			contracts = []client.Contract{}
		}
		return format.PrintJSON(contracts)
	}

	if len(contracts) == 0 {
		fmt.Println("No tracked contracts found.")
		return nil
	}

	rows := make([][]string, len(contracts))
	for i, ct := range contracts {
		rows[i] = []string{ct.ID, ct.Label, ct.Network, ct.Status, ct.AddedAt.Format(time.RFC3339)}
	}
	fmt.Println(format.RenderTable([]string{"ID", "Label", "Network", "Status", "Added"}, rows))
	return nil
}

func runContractsGet(cmd *cobra.Command, args []string) error {
	contractID := args[0]
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}

	c := client.New(globalConfig.APIURL, globalConfig.Timeout)
	stop := startSpinner("Fetching contract...")
	contract, err := c.GetContract(cmd.Context(), contractID)
	stop()
	if err != nil {
		var se *client.SorolensError
		if errors.As(err, &se) && se.Status == 404 {
			return fmt.Errorf("contract not found: %s", contractID)
		}
		return err
	}

	if globalConfig.JSON {
		return format.PrintJSON(contract)
	}

	backfill := "pending"
	if contract.BackfillCompleteAt != nil {
		backfill = contract.BackfillCompleteAt.Format(time.RFC3339)
	}
	pairs := [][]string{
		{"Contract ID", contract.ID},
		{"Label", contract.Label},
		{"Network", contract.Network},
		{"Status", contract.Status},
		{"WASM Hash", contract.WasmHash},
		{"Created At Ledger", fmt.Sprintf("%d", contract.CreatedAtLedger)},
		{"Added", contract.AddedAt.Format(time.RFC3339)},
		{"Backfill Complete", backfill},
	}
	fmt.Println(format.RenderKeyValue(pairs))
	return nil
}
