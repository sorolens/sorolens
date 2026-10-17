// Command pagination walks every page of GET /api/v1/contracts using the
// opaque `next_cursor`, which is the pattern to follow for any list endpoint.
//
// Point it at a deployment with:
//
//	SOROLENS_BASE_URL=http://localhost:8080 SOROLENS_API_KEY=sl_... go run .
//
// It defaults to the public API at https://api.sorolens.xyz.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/sorolens/go-client/examples/internal/harness"
	"github.com/sorolens/go-client/internal/client"
)

func run(ctx context.Context, baseURL, apiKey string, out io.Writer) error {
	c, err := client.New(baseURL, apiKey, nil)
	if err != nil {
		return err
	}

	var (
		cursor string
		total  int
	)
	for {
		params := &client.ListContractsParams{}
		if cursor != "" {
			params.Cursor = &cursor
		}
		page, err := c.ListContracts(ctx, params)
		if err != nil {
			return err
		}
		if page.JSON200 == nil {
			return fmt.Errorf("unexpected status %d: %s", page.StatusCode(), page.Body)
		}

		for _, contract := range page.JSON200.Contracts {
			fmt.Fprintf(out, "%s\t%s\t%s\n", contract.Id, contract.Network, contract.Status)
			total++
		}

		if page.JSON200.NextCursor == "" {
			break
		}
		cursor = page.JSON200.NextCursor
	}

	fmt.Fprintf(out, "fetched %d contract(s)\n", total)
	return nil
}

func main() {
	if err := run(context.Background(), harness.BaseURL(), harness.APIKey(), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "pagination:", err)
		os.Exit(1)
	}
}
