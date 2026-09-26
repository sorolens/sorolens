package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
)

var trackCmd = &cobra.Command{
	Use:   "track <contract-id>",
	Short: "Register a contract for tracking",
	Args:  cobra.ExactArgs(1),
	RunE:  runTrack,
}

var trackAlias string
var trackNetwork string

func init() {
	trackCmd.Flags().StringVar(&trackAlias, "alias", "", "Optional human-readable label for the contract")
	trackCmd.Flags().StringVar(&trackNetwork, "network", "testnet", "Stellar network (testnet, mainnet, futurenet, standalone)")
	rootCmd.AddCommand(trackCmd)
}

func runTrack(cmd *cobra.Command, args []string) error {
	contractID := args[0]
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}

	c := client.New(globalConfig.APIURL, globalConfig.Timeout)
	contract, err := c.TrackContract(cmd.Context(), contractID, trackAlias, trackNetwork)
	if err != nil {
		if se, ok := err.(*client.SorolensError); ok {
			fmt.Fprintf(os.Stderr, "Error (%s): %s\n", se.Code, se.Message)
			return nil
		}
		return err
	}

	if globalConfig.JSON {
		return format.PrintJSON(contract)
	}

	label := contract.ID
	if contract.Label != "" {
		label = fmt.Sprintf("%s (%s)", contract.Label, contract.ID)
	}
	fmt.Printf("Tracking %s\n", label)
	fmt.Printf("Status:  %s\n", contract.Status)
	fmt.Printf("Network: %s\n", contract.Network)
	return nil
}
