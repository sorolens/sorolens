// Command sse consumes the /api/v1/stream/events Server-Sent Events stream.
//
// Point it at a deployment with:
//
//	SOROLENS_BASE_URL=http://localhost:8080 SOROLENS_API_KEY=sl_... go run .
//
// It defaults to the public API at https://api.sorolens.xyz. The stream stays
// open until the server or the context closes it; the offline stub used by the
// examples' self-checks closes it after a couple of events so this terminates.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/sorolens/go-client/examples/internal/harness"
	"github.com/sorolens/go-client/internal/client"
)

func run(ctx context.Context, baseURL, apiKey string, out io.Writer) error {
	c, err := client.New(baseURL, apiKey, nil)
	if err != nil {
		return err
	}

	// StreamAllEvents returns the live *http.Response so the body can be read
	// incrementally as events arrive. The WithResponse variant buffers the whole
	// body instead, which never returns for an endless stream.
	resp, err := c.Generated().StreamAllEvents(ctx, &client.StreamAllEventsParams{})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		data, ok := strings.CutPrefix(strings.TrimSpace(scanner.Text()), "data:")
		if !ok {
			continue // comment, event name, blank separator...
		}

		var event struct {
			Id         string `json:"id"`
			ContractId string `json:"contract_id"`
			Type       string `json:"type"`
			Ledger     int32  `json:"ledger"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &event); err != nil {
			fmt.Fprintf(out, "skipping malformed event: %v\n", err)
			continue
		}
		fmt.Fprintf(out, "%s %s %s ledger=%d\n", event.Id, event.ContractId, event.Type, event.Ledger)
	}
	return scanner.Err()
}

func main() {
	if err := run(context.Background(), harness.BaseURL(), harness.APIKey(), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "sse:", err)
		os.Exit(1)
	}
}
