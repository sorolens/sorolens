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

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for sorolens.

To load completions:

Bash:
  $ source <(sorolens completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ sorolens completion bash > /etc/bash_completion.d/sorolens
  # macOS:
  $ sorolens completion bash > $(brew --prefix)/etc/bash_completion.d/sorolens

Zsh:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  $ sorolens completion zsh > "${fpath[1]}/_sorolens"

Fish:
  $ sorolens completion fish | source

  # To load completions for each session, execute once:
  $ sorolens completion fish > ~/.config/fish/completions/sorolens.fish

PowerShell:
  PS> sorolens completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, add the output to your profile:
  PS> sorolens completion powershell > sorolens.ps1
  # and source this file from your PowerShell profile.`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	SilenceUsage:          true,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletionV2(out, true)
		case "zsh":
			return rootCmd.GenZshCompletion(out)
		case "fish":
			return rootCmd.GenFishCompletion(out, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(out)
		default:
			return fmt.Errorf("unsupported shell: %q", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)

	rootCmd.PersistentFlags().StringVar(
		&globalConfig.APIURL, "api-url",
		"",
		"Sorolens API base URL",
	)
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

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
