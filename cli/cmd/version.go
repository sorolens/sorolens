package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is injected at build time via:
// go build -ldflags "-X github.com/sorolens/sorolens/cli/cmd.Version=v0.1.0"
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the sorolens CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("sorolens", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
