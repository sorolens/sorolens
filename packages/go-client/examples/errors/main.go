// Error handling: non-2xx responses and automatic rate-limit backoff.
//
// Run against a live server (expect a 404 for the missing contract):
//
//	SOROLENS_API_URL=https://api.sorolens.xyz go run ./examples/errors
//
// Without SOROLENS_API_URL the example replays canned 404 and 429 responses
// through httptest.Server and asserts the behaviour offline.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/sorolens/go-client/internal/client"
)

const missingID = "CMISSINGCONTRACTIDENTIFIER00000000000000000000000000000000"

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":       code,
			"message":    message,
			"request_id": "0f8b0f0e-6a1f-4f0f-9f4e-2b1c9f0e6a1f",
		},
	})
}

func offlineServer(listCalls *int) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/contracts/"+missingID, func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "contract not found")
	})
	mux.HandleFunc("GET /api/v1/contracts", func(w http.ResponseWriter, r *http.Request) {
		*listCalls++
		if *listCalls == 1 {
			// First attempt is rate limited; the SDK retries with backoff.
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"contracts": []map[string]any{
				{
					"id":                "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
					"network":           "testnet",
					"label":             "counter",
					"wasm_hash":         "9d3f2a1b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f7",
					"created_at_ledger": 51234,
					"status":            "active",
					"tags":              []string{"prod"},
				},
			},
			"next_cursor": "",
		})
	})
	return httptest.NewServer(mux)
}

func main() {
	ctx := context.Background()
	live := os.Getenv("SOROLENS_API_URL")

	baseURL := live
	listCalls := 0
	if baseURL == "" {
		srv := offlineServer(&listCalls)
		defer srv.Close()
		baseURL = srv.URL
	}

	c, err := client.New(baseURL, os.Getenv("SOROLENS_API_KEY"), nil,
		client.WithOnRetry(func(attempt int, req *http.Request, resp *http.Response, err error, wait time.Duration) {
			status := 0
			if resp != nil {
				status = resp.StatusCode
			}
			fmt.Printf("  retrying %s (attempt %d, waiting %v): status=%d err=%v\n",
				req.URL.Path, attempt, wait, status, err)
		}),
	)
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	fmt.Println("missing contract:")
	resp, err := c.GetContract(ctx, missingID)
	if err != nil {
		log.Fatalf("get contract: %v", err)
	}
	if resp.StatusCode() == http.StatusNotFound {
		fmt.Printf("  %s\n", resp.Status())
		fmt.Printf("  body: %s\n", truncate(resp.Body, 160))
	} else if live != "" {
		// A live server may answer 200 for a contract it knows.
		fmt.Printf("  contract found (status %d)\n", resp.StatusCode())
	} else {
		log.Fatalf("expected 404 for a missing contract, got %d", resp.StatusCode())
	}

	fmt.Println("rate limit:")
	limit := client.Limit(10)
	list, err := c.ListContracts(ctx, &client.ListContractsParams{Limit: &limit})
	if err != nil {
		log.Fatalf("list contracts: %v", err)
	}
	if list.StatusCode() != http.StatusOK {
		log.Fatalf("expected the retry to recover to 200, got %d", list.StatusCode())
	}
	if list.JSON200 == nil {
		log.Fatal("expected a parsed contracts page")
	}
	fmt.Printf("  recovered after backoff: %d contracts\n", len(list.JSON200.Contracts))
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
