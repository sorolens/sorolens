// Cursor pagination over GET /api/v1/contracts using the SDK's page iterator.
//
// Run against a live server:
//
//	SOROLENS_API_URL=https://api.sorolens.xyz go run ./examples/pagination
//
// Without SOROLENS_API_URL the example serves canned pages through
// httptest.Server and asserts the full walk offline (that is what CI runs).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/sorolens/go-client/internal/client"
)

func page(w http.ResponseWriter, contracts []client.Contract, nextCursor string) {
	writeJSON(w, http.StatusOK, map[string]any{
		"contracts":   contracts,
		"next_cursor": nextCursor,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write response: %v", err)
	}
}

func contract(id, network, label, status string) client.Contract {
	return client.Contract{
		Id:              id,
		Network:         network,
		Label:           label,
		WasmHash:        "9d3f2a1b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f7",
		CreatedAtLedger: 51234,
		Status:          client.ContractStatus(status),
		Tags:            []string{"prod"},
	}
}

func offlineServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/contracts", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("cursor") {
		case "page-2":
			page(w, []client.Contract{
				contract("CAQYAADZS2Y4XG4FWKF6MUNQCVKJXHVQEBDGV2Y4XG4FWKF6MUNQCWEB", "futurenet", "vault", "active"),
			}, "")
		default:
			page(w, []client.Contract{
				contract("CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC", "testnet", "counter", "active"),
				contract("CBECJ5JYFLYQZQWZMFIIWOJHTBLXEWQOGXFVN5BPOVQTQFSXJPWBBV6", "mainnet", "watchdog", "backfilling"),
			}, "page-2")
		}
	})
	return httptest.NewServer(mux)
}

func main() {
	ctx := context.Background()
	live := os.Getenv("SOROLENS_API_URL")

	baseURL := live
	if baseURL == "" {
		srv := offlineServer()
		defer srv.Close()
		baseURL = srv.URL
	}

	c, err := client.New(baseURL, os.Getenv("SOROLENS_API_KEY"), nil)
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	var ids []string
	limit := client.Limit(2)
	it := c.IterateContracts(ctx, &client.ListContractsParams{Limit: &limit})
	for it.Next() {
		contract := it.Item()
		ids = append(ids, contract.Id)
		fmt.Printf("%s\t%s\t%s\n", contract.Id, contract.Network, contract.Label)
	}
	if err := it.Err(); err != nil {
		log.Fatalf("iterate contracts: %v", err)
	}

	fmt.Printf("paginated through %d contracts\n", len(ids))
	if live == "" {
		if len(ids) != 3 {
			log.Fatalf("expected 3 contracts across 2 pages, got %d", len(ids))
		}
		if len(map[string]bool{ids[0]: true, ids[1]: true, ids[2]: true}) != 3 {
			log.Fatal("cursor pages repeated items")
		}
	}
}
