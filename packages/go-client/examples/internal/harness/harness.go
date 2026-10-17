// Package harness holds the small helpers shared by the runnable examples in
// this directory: environment configuration and an offline stub of the
// Sorolens API used by the self-checks and tests. It is internal to the
// examples and is not part of the client's public surface.
package harness

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"time"
)

const (
	// DefaultBaseURL is the public Sorolens API. Examples target it unless
	// SOROLENS_BASE_URL overrides it.
	DefaultBaseURL = "https://api.sorolens.xyz"

	// ContractID is a valid contract id served by the stub.
	ContractID = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAH2"
	// MissingContractID is deliberately not served, so reading it yields a 404.
	MissingContractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"

	// RateLimitScenarioHeader/Value make the stub answer 429, standing in for
	// the API's real per-IP rate limit so the errors example can exercise it.
	RateLimitScenarioHeader = "X-Example-Scenario"
	RateLimitScenarioValue  = "rate-limited"

	// WebhookSecret, WebhookTimestamp, WebhookBody and WebhookSignature are the
	// published test vector from docs/webhooks.md, so the webhook example can be
	// verified offline and stays in lock-step with the documented algorithm.
	WebhookSecret    = "whsec_example_0123456789abcdef0123456789abcdef"
	WebhookTimestamp = "1700000000"
	WebhookBody      = `{"event":"watchdog.alert","severity":"Critical"}`
	WebhookSignature = "97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615"
)

// BaseURL returns the API base URL the examples should talk to.
func BaseURL() string {
	if v := os.Getenv("SOROLENS_BASE_URL"); v != "" {
		return v
	}
	return DefaultBaseURL
}

// APIKey returns the optional API key from the environment.
func APIKey() string { return os.Getenv("SOROLENS_API_KEY") }

type contract struct {
	AddedAt            time.Time  `json:"added_at"`
	BackfillCompleteAt *time.Time `json:"backfill_complete_at"`
	CreatedAtLedger    int64      `json:"created_at_ledger"`
	Id                 string     `json:"id"`
	Label              string     `json:"label"`
	Network            string     `json:"network"`
	Status             string     `json:"status"`
	Tags               []string   `json:"tags"`
	WasmHash           string     `json:"wasm_hash"`
}

var (
	page1 = []contract{
		{Id: ContractID, Label: "counter", Network: "testnet", Status: "active", WasmHash: "0x1", CreatedAtLedger: 100, AddedAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Tags: []string{}},
		{Id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAC3", Label: "token", Network: "mainnet", Status: "backfilling", WasmHash: "0x2", CreatedAtLedger: 200, AddedAt: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC), Tags: []string{"defi"}},
	}
	page2 = []contract{
		{Id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD4", Label: "vault", Network: "testnet", Status: "active", WasmHash: "0x3", CreatedAtLedger: 300, AddedAt: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), Tags: []string{}},
		{Id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAE5", Label: "oracle", Network: "mainnet", Status: "paused", WasmHash: "0x4", CreatedAtLedger: 400, AddedAt: time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC), Tags: []string{}},
	}
	allContracts = append(append([]contract{}, page1...), page2...)
)

// streamEvents is the SSE payload the stub replays for /api/v1/stream/events.
var streamEvents = []string{
	`{"id":"evt_1","contract_id":"` + ContractID + `","network":"testnet","ledger":101,"ledger_closed_at":"2026-09-01T00:00:00Z","tx_hash":"0xaaa","type":"transfer","topic_xdr":[],"value_xdr":"","topic_decoded":[],"value_decoded":null,"in_successful_call":true}`,
	`{"id":"evt_2","contract_id":"` + ContractID + `","network":"testnet","ledger":102,"ledger_closed_at":"2026-09-01T00:01:00Z","tx_hash":"0xbbb","type":"mint","topic_xdr":[],"value_xdr":"","topic_decoded":[],"value_decoded":null,"in_successful_call":true}`,
}

// NewServer starts an httptest.Server serving the endpoints the examples use.
// The caller owns the server and must Close it.
func NewServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/contracts", func(w http.ResponseWriter, r *http.Request) {
		page, next := page1, "cursor-page-2"
		if r.URL.Query().Get("cursor") == "cursor-page-2" {
			page, next = page2, ""
		}
		writeJSON(w, http.StatusOK, map[string]any{"contracts": page, "next_cursor": next})
	})

	mux.HandleFunc("GET /api/v1/contracts/{id}", func(w http.ResponseWriter, r *http.Request) {
		for _, c := range allContracts {
			if c.Id == r.PathValue("id") {
				writeJSON(w, http.StatusOK, c)
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{"code": "NOT_FOUND", "message": "contract " + r.PathValue("id") + " not found"},
		})
	})

	mux.HandleFunc("GET /api/v1/stats/global", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(RateLimitScenarioHeader) == RateLimitScenarioValue {
			w.Header().Set("Retry-After", "1")
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error": map[string]any{"code": "RATE_LIMITED", "message": "too many requests"},
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"tracked_contracts": 4, "total_events": 128, "total_invocations": 42, "total_storage_entries": 57,
		})
	})

	mux.HandleFunc("GET /api/v1/stream/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for _, event := range streamEvents {
			_, _ = w.Write([]byte("data: " + event + "\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}
	})

	return httptest.NewServer(mux)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
