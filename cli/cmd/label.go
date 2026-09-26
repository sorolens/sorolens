package cmd

import (
	"fmt"

	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/spf13/cobra"
)

var labelScope string

var labelCmd = &cobra.Command{
	Use:   "label <name> <account-or-contract-id>",
	Short: "Register a human-readable account or contract label",
	Args:  cobra.ExactArgs(2),
	RunE:  runLabel,
}

func init() {
	labelCmd.Flags().StringVar(&labelScope, "scope", "workspace", "Label scope (workspace or public)")
	rootCmd.AddCommand(labelCmd)
}

func runLabel(cmd *cobra.Command, args []string) error {
	if labelScope != "workspace" && labelScope != "public" {
		return fmt.Errorf("scope must be workspace or public")
	}
	label, err := client.New(globalConfig.APIURL, globalConfig.Timeout).SaveLabel(cmd.Context(), args[0], args[1], labelScope)
	if err != nil { return err }
	if globalConfig.JSON { return format.PrintJSON(label) }
	fmt.Printf("Registered %s -> %s (%s)\n", label.Label, label.Value, label.Scope)
	return nil
}