package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/spf13/cobra"
)

// maxComparePerRequest is the API's documented cap for GET /api/v1/compare.
// The command accepts up to five contracts (issue #820), so a fifth one is
// fetched in a second request and the responses are merged in request order.
const maxComparePerRequest = 4

var (
	compareWindow string
	compareWidth  int
)

var compareCmd = &cobra.Command{
	Use:   "compare <contract-id>...",
	Short: "Compare 2 to 5 contracts side by side",
	Long: `Render events, invocations, failure rate, average CPU and fees, health
score and hourly event-volume sparklines for 2 to 5 contracts side by side.

A contract with no indexed data yet still renders a column: its cells show an
em dash instead of failing the whole comparison. Only request errors (a bad
contract ID, an unreachable API) exit non-zero.`,
	Example: `  # Compare two contracts over the last 24 hours
  sorolens compare CAAA... CBBB...

  # Compare five contracts over 7 days
  sorolens compare CAAA... CBBB... CCCC... CDDD... CEEE... --window 7d

  # Machine-readable output for scripts
  sorolens compare CAAA... CBBB... --json`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 || len(args) > 5 {
			return fmt.Errorf("compare requires 2 to 5 contract IDs, got %d", len(args))
		}
		return nil
	},
	RunE: runCompare,
}

func init() {
	compareCmd.Flags().StringVar(&compareWindow, "window", "24h", "Window to compare over (24h, 7d or 30d)")
	compareCmd.Flags().IntVar(&compareWidth, "width", 0, "Table width in columns (0 detects the terminal, minimum 40)")
	rootCmd.AddCommand(compareCmd)
}

// compareEntry is one contract row plus the failure rate, which lives on the
// invocation-stats endpoint rather than the comparison response.
type compareEntry struct {
	client.CompareContractEntry
	FailureRate *float64 `json:"failure_rate"`
}

type compareOutput struct {
	Window    string         `json:"window"`
	Contracts []compareEntry `json:"contracts"`
}

func runCompare(cmd *cobra.Command, args []string) error {
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}

	window := strings.ToLower(strings.TrimSpace(compareWindow))
	switch window {
	case "":
		window = "24h"
	case "24h", "7d", "30d":
	default:
		return fmt.Errorf("invalid --window %q: must be 24h, 7d or 30d", compareWindow)
	}

	c := client.New(globalConfig.APIURL, globalConfig.Timeout)
	compare, err := fetchComparison(cmd.Context(), c, args, window)
	if err != nil {
		return err
	}

	failureRates := fetchFailureRates(cmd.Context(), c, args, window)

	out := compareOutput{Window: compare.Window, Contracts: make([]compareEntry, len(compare.Contracts))}
	for i, entry := range compare.Contracts {
		out.Contracts[i] = compareEntry{CompareContractEntry: entry, FailureRate: failureRates[entry.ID]}
	}

	if globalConfig.JSON || globalConfig.Output == "json" {
		return format.PrintJSON(out)
	}

	fmt.Println(renderCompare(out, resolveWidth(compareWidth)))
	printCompareNotes(out)
	return nil
}

// fetchComparison fetches every contract in request-sized batches and merges
// the entries, preserving the order the user typed.
func fetchComparison(ctx context.Context, c *client.Client, ids []string, window string) (client.CompareResponse, error) {
	merged := client.CompareResponse{Window: window}
	for start := 0; start < len(ids); start += maxComparePerRequest {
		end := start + maxComparePerRequest
		if end > len(ids) {
			end = len(ids)
		}
		part, err := c.CompareContracts(ctx, ids[start:end], window)
		if err != nil {
			return merged, fmt.Errorf("compare contracts: %w", err)
		}
		merged.Window = part.Window
		merged.Contracts = append(merged.Contracts, part.Contracts...)
	}
	return merged, nil
}

// fetchFailureRates reads the invocation stats for every contract in parallel.
// A failure is non-fatal: the failure-rate column falls back to an em dash
// while every other metric still renders.
func fetchFailureRates(ctx context.Context, c *client.Client, ids []string, window string) map[string]*float64 {
	rates := make(map[string]*float64, len(ids))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			stats, err := c.GetContractInvocationStats(ctx, id)
			if err != nil {
				return
			}
			w, ok := stats.Windows[window]
			if !ok {
				return
			}
			rate := w.FailureRate
			mu.Lock()
			rates[id] = &rate
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return rates
}

// renderCompare builds the side-by-side table. Columns are contracts, rows are
// metrics, and the sparkline row is sized to whatever space is left over so
// narrow terminals still line up.
func renderCompare(out compareOutput, width int) string {
	headers := make([]string, 0, len(out.Contracts)+1)
	headers = append(headers, "Metric")
	for _, entry := range out.Contracts {
		headers = append(headers, compareColumnName(entry.CompareContractEntry))
	}

	labelWidth := 18
	sparkWidth := sparklineWidth(width, labelWidth, len(out.Contracts))
	rows := [][]string{
		compareRow("Events", out.Contracts, func(e compareEntry) string {
			return compareCount(e, e.EventCount)
		}),
		compareRow("Invocations", out.Contracts, func(e compareEntry) string {
			return compareCount(e, e.InvocationCount)
		}),
		compareRow("Failure rate", out.Contracts, compareFailureRate),
		compareRow("Health score", out.Contracts, compareHealth),
		compareRow("Avg CPU", out.Contracts, func(e compareEntry) string {
			if !e.HasData || e.Error != "" {
				return "—"
			}
			return fmt.Sprintf("%.0f", e.AvgCPU)
		}),
		compareRow("Avg fee (stroops)", out.Contracts, func(e compareEntry) string {
			if !e.HasData || e.Error != "" {
				return "—"
			}
			return fmt.Sprintf("%.0f", e.AvgFee)
		}),
		compareRow("Event volume", out.Contracts, func(e compareEntry) string {
			if !e.HasData || e.Error != "" || len(e.EventVolume) == 0 {
				return "—"
			}
			counts := make([]int64, len(e.EventVolume))
			for i, point := range e.EventVolume {
				counts[i] = point.Count
			}
			return format.Sparkline(counts, sparkWidth)
		}),
	}

	return format.RenderTable(headers, rows)
}

func compareRow(label string, entries []compareEntry, cell func(compareEntry) string) []string {
	row := make([]string, 0, len(entries)+1)
	row = append(row, label)
	for _, entry := range entries {
		row = append(row, cell(entry))
	}
	return row
}

func compareCount(entry compareEntry, value int64) string {
	if entry.Error != "" || !entry.HasData {
		return "—"
	}
	return strconv.FormatInt(value, 10)
}

func compareFailureRate(entry compareEntry) string {
	if entry.Error != "" || !entry.HasData || entry.FailureRate == nil {
		return "—"
	}
	return fmt.Sprintf("%.2f%%", *entry.FailureRate*100)
}

func compareHealth(entry compareEntry) string {
	if entry.Error != "" || entry.HealthScore == nil {
		return "—"
	}
	return strconv.FormatInt(int64(*entry.HealthScore), 10)
}

func compareColumnName(entry client.CompareContractEntry) string {
	if entry.Label != "" {
		return entry.Label
	}
	return shortenContractID(entry.ID)
}

func shortenContractID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:6] + "…" + id[len(id)-4:]
}

func printCompareNotes(out compareOutput) {
	for _, entry := range out.Contracts {
		if entry.Error == "" {
			continue
		}
		fmt.Fprintf(os.Stderr, "note: %s: %s\n", shortenContractID(entry.ID), entry.Error)
	}
}

// sparklineWidth divides the free space between contract columns, clamped so a
// sparkline is still readable on a narrow terminal without forcing the table
// wider than the window.
func sparklineWidth(width, labelWidth, columns int) int {
	if columns <= 0 {
		return 12
	}
	free := width - labelWidth - columns*2
	per := free / columns
	if per < 6 {
		return 6
	}
	if per > 28 {
		return 28
	}
	return per
}

// resolveWidth uses --width when set, then $COLUMNS, then a 100-column default.
func resolveWidth(flagValue int) int {
	width := flagValue
	if width <= 0 {
		if raw := os.Getenv("COLUMNS"); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil {
				width = parsed
			}
		}
	}
	if width < 40 {
		width = 40
	}
	return width
}
