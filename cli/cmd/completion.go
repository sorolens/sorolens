package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// completionCmd generates shell completion scripts for the sorolens CLI
// using Cobra's generators. It replaces Cobra's implicit default completion
// command so the command is a deliberate, documented part of the CLI (it
// shows up in `sorolens --help` and in the generated command reference and
// man pages) instead of a hidden default.
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
  # If shell completion is not already enabled in your environment,
  # enable it first:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ sorolens completion zsh > "${fpath[1]}/_sorolens"

  # Then start a new shell for this to take effect.

Fish:
  $ sorolens completion fish | source

  # To load completions for each session, execute once:
  $ sorolens completion fish > ~/.config/fish/completions/sorolens.fish

PowerShell:
  PS> sorolens completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> sorolens completion powershell > sorolens.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletionV2(out, true)
		case "zsh":
			return cmd.Root().GenZshCompletion(out)
		case "fish":
			return cmd.Root().GenFishCompletion(out, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(out)
		default:
			return fmt.Errorf("unsupported shell: %q", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
