package cmd

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/tui"
)

var (
	tuiAPIKey  string
	tuiRefresh time.Duration
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch an interactive terminal dashboard for tracked contracts",
	Long: `Launch a full-screen, k9s-style dashboard for the Sorolens API.

The dashboard shows tracked contracts alongside their live event stream,
invocations, watchdog alerts and storage TTL warnings. Data is refreshed on a
timer (default every 5s) and can be refreshed on demand with "r".

Key bindings:
  tab / shift+tab   cycle panes
  1-5               jump to contracts / events / invocations / alerts / storage
  j / k, up / down  move the selection
  enter             drill down into the selected contract (or alert target)
  /                 filter the contract list
  r                 refresh now
  q, ctrl+c          quit

The dashboard talks to the API at --api-url and authenticates with an API key
passed via --api-key or the SOROLENS_API_KEY environment variable.`,
	Args: cobra.NoArgs,
	RunE: runTUI,
}

func init() {
	tuiCmd.Flags().StringVar(
		&tuiAPIKey, "api-key",
		envOrDefault("SOROLENS_API_KEY", ""),
		"API key for authenticating with the Sorolens API",
	)
	tuiCmd.Flags().DurationVar(&tuiRefresh, "refresh", 5*time.Second, "Polling interval for live data")
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}

	c := client.New(globalConfig.APIURL, globalConfig.Timeout)
	if tuiAPIKey != "" {
		c = c.WithAPIKey(tuiAPIKey)
	}

	model := tui.New(c, tui.Options{
		APIURL:          globalConfig.APIURL,
		RefreshInterval: tuiRefresh,
		RequestTimeout:  globalConfig.Timeout,
	})

	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}
	return nil
}
