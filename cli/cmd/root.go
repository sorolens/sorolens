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
	APIKey  string
	Network string
	Output  string
	Query   string
	Profile string
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
	SilenceUsage:      true,
	SilenceErrors:     true,
	PersistentPreRunE: applyConfig,
}

// RootCommand returns the root command for documentation generation.
func RootCommand() *cobra.Command {
	return rootCmd
}

func init() {
	rootCmd.PersistentFlags().StringVar(
		&globalConfig.APIURL, "api-url",
		"",
		"Sorolens API base URL",
	)
	// --api-key is a persistent flag so every command that talks to the API
	// (track, label, tui, ...) can authenticate. It takes precedence over
	// SOROLENS_API_KEY, the api_key config-file entry, and the active
	// profile's stored key (see applyConfig and resolvedAPIKey).
	rootCmd.PersistentFlags().StringVar(&globalConfig.APIKey, "api-key", "", "API key for authenticating with the Sorolens API")
	rootCmd.PersistentFlags().StringVar(&globalConfig.Profile, "profile", "", "Configuration profile name (default: $SOROLENS_PROFILE, then the default profile)")
	rootCmd.PersistentFlags().StringVar(&globalConfig.Network, "network", "", "Stellar network (testnet, mainnet, futurenet, standalone)")
	rootCmd.PersistentFlags().StringVar(&globalConfig.Output, "output", "", "Output format (table, json, yaml or jsonl)")
	rootCmd.PersistentFlags().StringVar(&globalConfig.Query, "query", "", "jq-style path filter applied to structured output (e.g. '.items[].name')")
	rootCmd.PersistentFlags().BoolVar(&globalConfig.JSON, "json", false, "Output machine-readable JSON")
	rootCmd.PersistentFlags().BoolVar(&globalConfig.NoColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().DurationVar(&globalConfig.Timeout, "timeout", 10*time.Second, "Request timeout")
}

// Execute runs the root command. Call this from main().
func Execute() {
	if handled, code, err := tryPlugin(os.Args[1:], os.Stdout, os.Stderr); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		os.Exit(code)
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
