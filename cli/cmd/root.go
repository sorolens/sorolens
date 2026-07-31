package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// Config holds global CLI configuration derived from persistent flags and env vars.
type Config struct {
	APIURL  string
	JSON    bool
	NoColor bool
	Timeout time.Duration
}

var globalConfig Config

// rootCmd is the root cobra command for the sorolens CLI.
var rootCmd = &cobra.Command{
	Use:   "sorolens",
	Short: "Sorolens - Soroban smart contract observability CLI",
	Long: `sorolens is the command-line interface for the Sorolens observability
platform. It lets you track, inspect, and monitor Soroban smart contracts
on the Stellar network.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(
		&globalConfig.APIURL, "api-url",
		envOrDefault("SOROLENS_API_URL", "http://localhost:8080"),
		"Sorolens API base URL",
	)
	rootCmd.PersistentFlags().BoolVar(&globalConfig.JSON, "json", false, "Output machine-readable JSON")
	rootCmd.PersistentFlags().BoolVar(&globalConfig.NoColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().DurationVar(&globalConfig.Timeout, "timeout", 10*time.Second, "Request timeout")
}

// Execute runs the root command. Call this from main().
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
