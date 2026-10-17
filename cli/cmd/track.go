package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/sorolens/sorolens/cli/internal/presets"
	"github.com/spf13/cobra"
)

var trackCmd = &cobra.Command{
	Use:   "track <contract-id>",
	Short: "Register a contract for tracking",
	Args:  cobra.ExactArgs(1),
	RunE:  runTrack,
}

var trackAlias string
var trackPreset string

func init() {
	trackCmd.Flags().StringVar(&trackAlias, "alias", "", "Optional human-readable label for the contract")
	trackCmd.Flags().StringVar(&trackPreset, "preset", "", "Contract-type preset: dex, lending, stablecoin (pre-configures event filters, function names, and suggested labels)")
	rootCmd.AddCommand(trackCmd)
	// Attach preset help to the track command's --help output.
	trackCmd.SetUsageTemplate(usageTemplate())
}

// usageTemplate extends the default cobra usage with the built-in preset list
// so that `sorolens track --help` surfaces the available presets without the
// user needing to read external docs.
func usageTemplate() string {
	var sb strings.Builder
	sb.WriteString(`Usage:
  `)
	sb.WriteString(os.Args[0])
	sb.WriteString(` [command] `)
	sb.WriteString(trackCmd.Name())
	sb.WriteString(` [arguments]\n\n`)
	sb.WriteString(`Available presets:\n`)
	for _, p := range presets.PresetsByName() {
		sb.WriteString(fmt.Sprintf("  %-12s  %s\n", p.Type, p.Description))
	}
	sb.WriteString(`\n`)
	return sb.String()
}

func runTrack(cmd *cobra.Command, args []string) error {
	contractID := args[0]

	// Validate the preset early so the user gets a clear error message before
	// any network call.
	if trackPreset != "" {
		if _, err := presets.Lookup(presets.PresetType(trackPreset)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			fmt.Fprintf(os.Stderr, "Run %s track --help for available presets.\n", os.Args[0])
			return nil
		}
	}

	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}

	c := client.New(globalConfig.APIURL, globalConfig.Timeout)
	contract, err := c.TrackContract(cmd.Context(), contractID, trackAlias, globalConfig.Network, trackPreset)
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
	if contract.TrackingPreset != "" {
		fmt.Printf("Preset:  %s\n", contract.TrackingPreset)
	}
	return nil
}
