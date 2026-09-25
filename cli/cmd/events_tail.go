package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
)

var eventsTailCmd = &cobra.Command{
	Use:   "tail --contract <contract-id>",
	Short: "Stream a contract's events live as they are indexed",
	Long: `Tail a contract's events using the Sorolens server-sent events endpoint.

Unlike "sorolens events <contract-id>", which returns a page of already-indexed
events, tail keeps the connection open and pretty-prints each event as it
arrives. Press Ctrl-C to close the stream.`,
	Args: cobra.NoArgs,
	RunE: runEventsTail,
}

var eventsTailContract string

func init() {
	eventsTailCmd.Flags().StringVar(
		&eventsTailContract,
		"contract",
		"",
		"Contract ID to stream events for (required)",
	)
	_ = eventsTailCmd.MarkFlagRequired("contract")
	eventsCmd.AddCommand(eventsTailCmd)
}

func runEventsTail(cmd *cobra.Command, _ []string) error {
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}

	c := client.New(globalConfig.APIURL, 0)

	// Ctrl-C / SIGTERM cancel the stream context so the SSE connection closes
	// cleanly instead of the process being killed mid-frame.
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if !globalConfig.JSON {
		fmt.Fprintf(os.Stderr, "Streaming events for %s (Ctrl-C to stop)...\n", eventsTailContract)
	}

	if err := c.StreamEvents(ctx, eventsTailContract, printStreamMessage); err != nil {
		return fmt.Errorf("stream events: %w", err)
	}

	if ctx.Err() != nil && !globalConfig.JSON {
		fmt.Fprintln(os.Stderr, "Stream closed.")
	}
	return nil
}

// printStreamMessage pretty-prints a single SSE frame. Heartbeats and any
// unknown frame types are ignored.
func printStreamMessage(msg client.StreamMessage) error {
	if globalConfig.JSON {
		return format.PrintJSON(msg)
	}

	switch msg.Type {
	case "event":
		if msg.Event == nil {
			return nil
		}
		e := msg.Event
		fmt.Printf("[%s] %-12s ledger=%d tx=%s %s\n",
			e.LedgerClosedAt.Format("15:04:05"),
			e.Type,
			e.Ledger,
			shortHash(e.TxHash),
			truncate(summarize(e.ValueDecoded), 80),
		)
	case "alert":
		fmt.Printf("[%s] %-12s contract=%s %v\n",
			time.Now().Format("15:04:05"),
			"ALERT",
			msg.ContractID,
			msg.Alert,
		)
	case "connected":
		fmt.Fprintln(os.Stderr, "Connected to event stream.")
	default:
		// Heartbeat ("ping") and future frame types are not user-facing.
	}
	return nil
}
