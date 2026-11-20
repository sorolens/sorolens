package cmd

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/spf13/cobra"
)

var invocationsCmd = &cobra.Command{
	Use:   "invocations <contract-id>",
	Short: "List invocations of a tracked contract",
	Long: `List the most recent invocations of a contract: transaction hash, ledger,
status, function and resource usage.

Use --status and --fn to filter, --limit to cap the rows, and --json for
machine-readable output. Pass a transaction hash to "sorolens inspect --tx" to
see one invocation in detail.`,
	Example: `  sorolens invocations CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
  sorolens invocations <contract-id> --status FAILED --fn transfer
  sorolens invocations list <contract-id> --limit 10 --json`,
	Args: cobra.ExactArgs(1),
	RunE: runInvocations,
}

var invocationsListCmd = &cobra.Command{
	Use:   "list <contract-id>",
	Short: "List invocations of a tracked contract (same as \"invocations <contract-id>\")",
	Args:  cobra.ExactArgs(1),
	RunE:  runInvocations,
}

var (
	invocationsStatus string
	invocationsFn     string
	invocationsLimit  int
)

// validInvocationStatuses are the status filter values the API accepts.
var validInvocationStatuses = []string{"SUCCESS", "FAILED", "NOT_FOUND"}

// txHashPattern matches a Soroban transaction hash: 64 hex characters.
var txHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

func init() {
	invocationsCmd.AddCommand(invocationsListCmd)
	for _, c := range []*cobra.Command{invocationsCmd, invocationsListCmd} {
		c.Flags().StringVar(&invocationsStatus, "status", "", "Filter by status ("+strings.Join(validInvocationStatuses, ", ")+")")
		c.Flags().StringVar(&invocationsFn, "fn", "", "Filter by function name")
		c.Flags().IntVar(&invocationsLimit, "limit", 50, "Maximum number of invocations to list")
	}
	rootCmd.AddCommand(invocationsCmd)
}

func runInvocations(cmd *cobra.Command, args []string) error {
	contractID := args[0]
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}
	if invocationsLimit < 1 {
		return fmt.Errorf("invalid --limit %d: must be at least 1", invocationsLimit)
	}
	status := strings.ToUpper(strings.TrimSpace(invocationsStatus))
	if status != "" && !containsString(validInvocationStatuses, status) {
		return fmt.Errorf("invalid --status %q: must be one of %s", invocationsStatus, strings.Join(validInvocationStatuses, ", "))
	}

	c := newClient(globalConfig.Timeout)
	stop := startSpinner("Fetching invocations...")
	resp, err := c.ListInvocations(cmd.Context(), contractID, client.ListInvocationsOpts{
		Status: status,
		Fn:     strings.TrimSpace(invocationsFn),
		Limit:  invocationsLimit,
	})
	stop()
	if err != nil {
		var se *client.SorolensError
		if errors.As(err, &se) && se.Status == 404 {
			return fmt.Errorf("contract not found: %s", contractID)
		}
		return err
	}

	// The API caps pages on its own; never print more than requested.
	invocations := resp.Invocations
	if len(invocations) > invocationsLimit {
		invocations = invocations[:invocationsLimit]
	}

	if globalConfig.JSON {
		if invocations == nil {
			invocations = []client.Invocation{}
		}
		return format.PrintJSON(invocations)
	}

	if len(invocations) == 0 {
		fmt.Println("No invocations found.")
		return nil
	}

	rows := make([][]string, len(invocations))
	for i, inv := range invocations {
		rows[i] = []string{
			inv.TxHash,
			fmt.Sprintf("%d", inv.Ledger),
			inv.LedgerClosedAt.Format(time.RFC3339),
			inv.Status,
			inv.FunctionName,
			fmt.Sprintf("%d", inv.CPUInsn),
			fmt.Sprintf("%d", inv.ResourceFeeCharged),
		}
	}
	fmt.Println(format.RenderTable([]string{"Tx Hash", "Ledger", "Time", "Status", "Function", "CPU", "Fee"}, rows))
	return nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
