// Command errors shows how the client surfaces failures. Every response type
// carries the parsed success and error bodies (JSON200, JSON404, JSON429, ...),
// so callers can branch on the status without unmarshalling anything by hand.
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
	"net/http"
	"os"
	"strings"

	"github.com/sorolens/go-client/examples/internal/harness"
	"github.com/sorolens/go-client/internal/client"
)

// headerTransport injects a header on every request. The errors example uses it
// to ask the offline stub for a 429; against the real API the 429 arrives when
// the per-IP rate limit is actually exceeded.
type headerTransport struct {
	base  http.RoundTripper
	key   string
	value string
}

func (t headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set(t.key, t.value)
	return t.base.RoundTrip(req)
}

func run(ctx context.Context, baseURL, apiKey string, out io.Writer) error {
	c, err := client.New(baseURL, apiKey, nil)
	if err != nil {
		return err
	}

	// A resource that does not exist: the client maps 404 to JSON404.
	resp, err := c.GetContract(ctx, harness.MissingContractID)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		fmt.Fprintf(out, "found %s\n", resp.JSON200.Id)
	case http.StatusNotFound:
		fmt.Fprintf(out, "not found: %s\n", errorMessage(resp.JSON404))
	default:
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode(), resp.Body)
	}

	// A rate-limited request: 429 is mapped to JSON429 with a Retry-After hint.
	limited, err := client.New(baseURL, apiKey, &http.Client{
		Transport: headerTransport{base: http.DefaultTransport, key: harness.RateLimitScenarioHeader, value: harness.RateLimitScenarioValue},
	})
	if err != nil {
		return err
	}
	stats, err := limited.GetGlobalStats(ctx)
	if err != nil {
		return err
	}
	switch stats.StatusCode() {
	case http.StatusOK:
		fmt.Fprintf(out, "tracked contracts: %d\n", stats.JSON200.TrackedContracts)
	case http.StatusTooManyRequests:
		fmt.Fprintf(out, "rate limited: %s (retry after %s)\n", errorMessage(stats.JSON429), stats.HTTPResponse.Header.Get("Retry-After"))
	default:
		return fmt.Errorf("unexpected status %d: %s", stats.StatusCode(), stats.Body)
	}

	return nil
}

// errorMessage renders the nested {code, message} both error bodies share.
func errorMessage(e *client.Error) string {
	if e == nil {
		return "<no error body>"
	}
	if detail, err := e.Error.AsErrorError0(); err == nil {
		return fmt.Sprintf("%s (%s)", detail.Message, detail.Code)
	}
	raw, _ := e.Error.MarshalJSON()
	return strings.TrimSpace(string(raw))
}

func main() {
	if err := run(context.Background(), harness.BaseURL(), harness.APIKey(), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "errors:", err)
		os.Exit(1)
	}
}
