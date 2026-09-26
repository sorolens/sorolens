package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
)

var eventsCmd = &cobra.Command{
	Use:   "events <contract-id>",
	Short: "List events emitted by a tracked contract",
	Args:  cobra.ExactArgs(1),
	RunE:  runEvents,
}

var (
	eventsFollow bool
	eventsType   string
	eventsLimit  int
	eventsCSV    bool
)

func init() {
	eventsCmd.Flags().BoolVar(&eventsFollow, "follow", false, "Poll every 3 seconds and append new events")
	eventsCmd.Flags().StringVar(&eventsType, "type", "", "Filter by event type")
	eventsCmd.Flags().IntVar(&eventsLimit, "limit", 50, "Number of events to fetch per request")
	eventsCmd.Flags().BoolVar(&eventsCSV, "csv", false, "Output RFC 4180 CSV with header row")
	rootCmd.AddCommand(eventsCmd)
}

func runEvents(cmd *cobra.Command, args []string) error {
	contractID := args[0]
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}

	c := client.New(globalConfig.APIURL, globalConfig.Timeout)
	opts := client.ListEventsOpts{
		Type:  eventsType,
		Limit: eventsLimit,
	}

	seen := make(map[string]bool)

	// CSV writer shared for follow mode.
	var csvWriter *csv.Writer
	if eventsCSV {
		csvWriter = csv.NewWriter(os.Stdout)
		_ = csvWriter.Write([]string{"time", "type", "tx_hash", "summary"})
	}

	printEvents := func(events []client.Event) {
		var newEvents []client.Event
		for _, e := range events {
			if !seen[e.ID] {
				seen[e.ID] = true
				newEvents = append(newEvents, e)
			}
		}
		if len(newEvents) == 0 {
			return
		}

		if globalConfig.JSON {
			_ = format.PrintJSON(newEvents)
			return
		}

		if eventsCSV {
			for _, e := range newEvents {
				_ = csvWriter.Write([]string{
					e.LedgerClosedAt.Format(time.RFC3339),
					e.Type,
					e.TxHash,
					summarize(e.ValueDecoded),
				})
			}
			csvWriter.Flush()
			return
		}

		headers := []string{"Time", "Type", "Tx Hash", "Summary"}
		rows := make([][]string, len(newEvents))
		for i, e := range newEvents {
			rows[i] = []string{
				e.LedgerClosedAt.Format("2006-01-02 15:04"),
				e.Type,
				shortHash(e.TxHash),
				truncate(summarize(e.ValueDecoded), 60),
			}
		}
		fmt.Println(format.RenderTable(headers, rows))
	}

	resp, err := c.ListEvents(cmd.Context(), contractID, opts)
	if err != nil {
		return fmt.Errorf("list events: %w", err)
	}
	printEvents(resp.Events)

	if !eventsFollow {
		return nil
	}

	cursor := resp.NextCursor
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cmd.Context().Done():
			return nil
		case <-ticker.C:
			pollOpts := opts
			if cursor != "" {
				pollOpts.Cursor = cursor
			}
			r, err := c.ListEvents(cmd.Context(), contractID, pollOpts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "poll error: %v\n", err)
				continue
			}
			printEvents(r.Events)
			if r.NextCursor != "" {
				cursor = r.NextCursor
			}
		}
	}
}

func shortHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:8] + "..." + h[len(h)-4:]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func summarize(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case map[string]any:
		var parts []string
		for k, mv := range val {
			parts = append(parts, fmt.Sprintf("%s=%v", k, mv))
		}
		return strings.Join(parts, " ")
	default:
		return fmt.Sprintf("%v", v)
	}
}
