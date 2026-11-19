package cmd

import (
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sorolens/sorolens/cli/internal/client"
)

var updateCompareGoldens = flag.Bool("update-compare", false, "update compare golden files")

const (
	compareIDAlpha = "C" + "A" + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	compareIDBeta  = "C" + "B" + "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
)

func compareEntryFor(id string) client.CompareContractEntry {
	score := int32(92)
	switch id {
	case compareIDAlpha:
		return client.CompareContractEntry{
			ID: id, Network: "mainnet", Label: "alpha", Status: "active",
			Tracked: true, HasData: true, EventCount: 1200, InvocationCount: 340,
			AvgCPU: 1530, AvgFee: 220, HealthScore: &score, LastSyncedLedger: 99,
			EventVolume: []client.CompareVolumePoint{
				{Timestamp: "2026-11-18T00:00:00Z", Count: 1},
				{Timestamp: "2026-11-18T01:00:00Z", Count: 4},
				{Timestamp: "2026-11-18T02:00:00Z", Count: 2},
				{Timestamp: "2026-11-18T03:00:00Z", Count: 8},
				{Timestamp: "2026-11-18T04:00:00Z", Count: 5},
				{Timestamp: "2026-11-18T05:00:00Z", Count: 0},
			},
		}
	case compareIDBeta:
		// Registered but nothing indexed yet: every metric is an em dash.
		return client.CompareContractEntry{ID: id, Status: "unknown", Tracked: false}
	}
	return client.CompareContractEntry{ID: id, HasData: true, EventCount: 7, InvocationCount: 3}
}

// compareTestServer serves /compare and per-contract invocation stats. The
// beta contract's stats endpoint fails so the partial-data path is covered.
func compareTestServer(t *testing.T, calls *[]string) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/compare":
			ids := strings.Split(r.URL.Query().Get("ids"), ",")
			mu.Lock()
			*calls = append(*calls, r.URL.Query().Get("ids"))
			mu.Unlock()
			resp := client.CompareResponse{Window: r.URL.Query().Get("window")}
			for _, id := range ids {
				resp.Contracts = append(resp.Contracts, compareEntryFor(id))
			}
			_ = json.NewEncoder(w).Encode(resp)
		case strings.HasSuffix(r.URL.Path, "/invocations/stats"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/contracts/"), "/invocations/stats")
			if id == compareIDBeta {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"boom"}}`))
				return
			}
			_ = json.NewEncoder(w).Encode(client.ContractInvocationStats{
				Windows: map[string]client.InvocationWindowStats{
					"24h": {InvocationCount: 340, FailureRate: 0.025},
					"7d":  {InvocationCount: 900, FailureRate: 0.01},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func checkCompareGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateCompareGoldens {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run with -update-compare): %v", path, err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestCompareCommandTableGolden(t *testing.T) {
	var calls []string
	srv := compareTestServer(t, &calls)
	defer srv.Close()

	rootCmd.SetArgs([]string{
		"compare", compareIDAlpha, compareIDBeta,
		"--window", "24h", "--width", "80", "--api-url", srv.URL, "--no-color",
	})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checkCompareGolden(t, "compare_success.txt", out)
	for _, want := range []string{"alpha", "Failure rate", "2.50%", "Event volume", "▂▅▃█▅▁"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestCompareCommandPartialDataGolden(t *testing.T) {
	var calls []string
	srv := compareTestServer(t, &calls)
	defer srv.Close()

	rootCmd.SetArgs([]string{
		"compare", compareIDAlpha, compareIDBeta,
		"--window", "7d", "--width", "60", "--api-url", srv.URL, "--no-color",
	})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("a contract without data must not fail the command: %v", err)
	}
	checkCompareGolden(t, "compare_partial.txt", out)
	if !strings.Contains(out, "—") {
		t.Errorf("partial output should use em dashes for missing data:\n%s", out)
	}
}

func TestCompareCommandJSONExposesFailureRate(t *testing.T) {
	var calls []string
	srv := compareTestServer(t, &calls)
	defer srv.Close()

	rootCmd.SetArgs([]string{
		"compare", compareIDAlpha, compareIDBeta,
		"--window", "24h", "--json", "--api-url", srv.URL,
	})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got struct {
		Window    string `json:"window"`
		Contracts []struct {
			ID          string   `json:"id"`
			FailureRate *float64 `json:"failure_rate"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if got.Window != "24h" || len(got.Contracts) != 2 {
		t.Fatalf("unexpected JSON: %+v", got)
	}
	if got.Contracts[0].FailureRate == nil || *got.Contracts[0].FailureRate != 0.025 {
		t.Errorf("alpha failure_rate: %+v", got.Contracts[0].FailureRate)
	}
	if got.Contracts[1].FailureRate != nil {
		t.Errorf("beta has no stats and must not carry a failure_rate")
	}
}

func TestCompareCommandBatchesFifthContract(t *testing.T) {
	var calls []string
	srv := compareTestServer(t, &calls)
	defer srv.Close()

	fifth := "C" + "D" + strings.Repeat("D", 54)
	rootCmd.SetArgs([]string{
		"compare", compareIDAlpha, compareIDBeta, compareIDBeta,
		compareIDAlpha, fifth, "--window", "24h", "--json", "--api-url", srv.URL,
	})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got struct {
		Contracts []struct {
			ID string `json:"id"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if len(got.Contracts) != 5 {
		t.Fatalf("expected 5 contracts, got %d", len(got.Contracts))
	}
	if len(calls) != 2 {
		t.Fatalf("expected the API to be called in 2 batches, got %d (%v)", len(calls), calls)
	}
}

func TestCompareCommandRejectsArgumentCountAndWindow(t *testing.T) {
	rootCmd.SetArgs([]string{"compare", compareIDAlpha})
	if _, err := captureOutput(t, rootCmd.Execute); err == nil {
		t.Error("expected an error for one contract")
	}

	rootCmd.SetArgs([]string{"compare", compareIDAlpha, compareIDBeta, "--window", "90d"})
	if _, err := captureOutput(t, rootCmd.Execute); err == nil {
		t.Error("expected an error for an unsupported window")
	}
}
