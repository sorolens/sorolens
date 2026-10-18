// Package integration proves the Sorolens pipeline works end to end: the mock
// Soroban RPC serves one real event, the indexer indexes it once into
// Postgres, and the API returns exactly that data.
//
// The assertions are exact rather than status-code checks. A regression
// anywhere in the chain (RPC request shape, event insert, sync cursor, API
// serialisation) removes or alters the row and fails the suite.
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	contractID    = "CBIELTK6YBZJU5UP2WWQEUCYKLPU6AUNZ2BQ4WWFEIE3USCIHMXQDAMA"
	contractLabel = "integration-e2e"
	otherContract = "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCW"

	// Keep these in sync with tests/integration/mock_rpc/main.go.
	latestLedger = 1_000_000
	eventID      = "0000000000999999-0000000001"
	eventLedger  = 999_999
	txHash       = "9f4c0d5b2e8a7f6c1d3b5a7e9c2f4d6b8a1c3e5f7d9b2a4c6e8f1d3b5a7c9e2f"
	topicXDR     = "AAAADwAAAAh0cmFuc2Zlcg=="
	valueXDR     = "AAAACgAAAAAAAAAAAAAAAAAAA+g="
	resultXDR    = "AAAAAAAAAGQAAAAAAAAAAAAAAAA="
)

var (
	apiURL        = envOr("INTEGRATION_API_URL", "http://localhost:8080")
	apiHTTPClient = &http.Client{Timeout: 15 * time.Second}
	sessionToken  string
)

// ---- fixture ---------------------------------------------------------------

func TestMain(m *testing.M) {
	if err := waitForReady(2 * time.Minute); err != nil {
		fmt.Fprintf(os.Stderr, "integration stack not ready: %v\n", err)
		os.Exit(1)
	}
	if err := setupFixture(); err != nil {
		fmt.Fprintf(os.Stderr, "integration fixture: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// setupFixture registers a contract through the public API, activates it, and
// runs the indexer once.
//
// Registration currently leaves a contract in "pending" and nothing advances
// it (issue #509 owns the pending -> backfilling -> active lifecycle). The
// indexer only indexes active/backfilling contracts, so the fixture flips the
// row directly. Once #509 lands this step should become unnecessary.
func setupFixture() error {
	status, body, err := apiRequest("POST", "/api/v1/auth/session", map[string]any{})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	if status != http.StatusCreated {
		return fmt.Errorf("create session: status %d body %s", status, body)
	}
	var session struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &session); err != nil {
		return fmt.Errorf("decode session: %w", err)
	}
	sessionToken = session.Token

	status, body, err = apiRequest("POST", "/api/v1/contracts", map[string]any{
		"id":      contractID,
		"network": "testnet",
		"label":   contractLabel,
	})
	if err != nil {
		return fmt.Errorf("register contract: %w", err)
	}
	if status != http.StatusCreated {
		return fmt.Errorf("register contract: status %d body %s", status, body)
	}

	if err := activateContract(); err != nil {
		return err
	}
	if err := runIndexerOnce(); err != nil {
		return err
	}
	return waitForEvent(30 * time.Second)
}

// activateContract marks the fixture contract as indexable in Postgres.
func activateContract() error {
	sql := fmt.Sprintf("UPDATE contracts SET status = 'active' WHERE id = '%s'", contractID)
	out, err := composeOutput("exec", "-T", "postgres",
		"psql", "-U", "postgres", "-d", "sorolens_test", "-tAc", sql)
	if err != nil {
		return fmt.Errorf("activate contract: %w\n%s", err, out)
	}
	if strings.TrimSpace(out) != "UPDATE 1" {
		return fmt.Errorf("activate contract: expected UPDATE 1, got %q", strings.TrimSpace(out))
	}
	return nil
}

// runIndexerOnce runs a single indexing pass in the compose stack and fails on
// a non-zero exit, so a crash in the poller fails the suite instead of leaving
// stale rows behind.
func runIndexerOnce() error {
	out, err := composeOutput("run", "--rm", "indexer", "-mode", "once")
	if err != nil {
		return fmt.Errorf("indexer run failed: %w\n%s", err, out)
	}
	if !strings.Contains(out, "sorolens/indexer done") {
		return fmt.Errorf("indexer did not finish cleanly:\n%s", out)
	}
	return nil
}

func waitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		status, body, err := apiRequest("GET", "/readyz", nil)
		if err == nil && status == http.StatusOK {
			return nil
		}
		last = fmt.Sprintf("status=%d err=%v body=%s", status, err, body)
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timed out after %s: %s", timeout, last)
}

func waitForEvent(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		status, body, err := apiRequest("GET", "/api/v1/contracts/"+contractID+"/events", nil)
		if err == nil && status == http.StatusOK {
			var resp eventsResponse
			if json.Unmarshal(body, &resp) == nil && len(resp.Events) == 1 {
				return nil
			}
			last = string(body)
		} else {
			last = fmt.Sprintf("status=%d err=%v body=%s", status, err, body)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("indexed event never appeared: %s", last)
}

// ---- tests -----------------------------------------------------------------

func TestRegisteredContractIsServedByTheAPI(t *testing.T) {
	status, body, err := apiRequest("GET", "/api/v1/contracts/"+contractID, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body: %s", body)

	var c struct {
		ID      string `json:"id"`
		Network string `json:"network"`
		Label   string `json:"label"`
		Status  string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(body, &c))
	assert.Equal(t, contractID, c.ID)
	assert.Equal(t, "testnet", c.Network)
	assert.Equal(t, contractLabel, c.Label)
	assert.Equal(t, "active", c.Status)
}

func TestIndexedEventMatchesMockRPC(t *testing.T) {
	events, nextCursor := fetchEvents(t, "")
	require.Len(t, events, 1, "the mock RPC event must be indexed exactly once")
	assert.Empty(t, nextCursor)

	ev := events[0]
	assert.Equal(t, eventID, ev.ID)
	assert.Equal(t, contractID, ev.ContractID)
	assert.Equal(t, "testnet", ev.Network)
	assert.Equal(t, uint32(eventLedger), ev.Ledger)
	assert.Equal(t, txHash, ev.TxHash)
	assert.Equal(t, "contract", ev.Type)
	assert.Equal(t, []string{topicXDR}, ev.TopicXDR)
	assert.Equal(t, valueXDR, ev.ValueXDR)
	assert.True(t, ev.InSuccessfulCall)
	assert.WithinDuration(t, time.Now(), ev.LedgerClosedAt, 10*time.Minute)
}

func TestIndexedInvocationMatchesMockRPC(t *testing.T) {
	status, body, err := apiRequest("GET", "/api/v1/contracts/"+contractID+"/invocations", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body: %s", body)

	var resp invocationsResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	require.Len(t, resp.Invocations, 1, "the event's transaction must be indexed")
	assert.Empty(t, resp.NextCursor)

	inv := resp.Invocations[0]
	assert.Equal(t, txHash, inv.TxHash)
	assert.Equal(t, contractID, inv.ContractID)
	assert.Equal(t, "testnet", inv.Network)
	assert.Equal(t, uint32(eventLedger), inv.Ledger)
	assert.Equal(t, "SUCCESS", inv.Status)
	assert.Equal(t, resultXDR, inv.ResultXDR)
	assert.Equal(t, 1, inv.ApplicationOrder)
	assert.Empty(t, inv.FunctionName, "the indexer does not decode the function name yet")
	assert.WithinDuration(t, time.Now(), inv.LedgerClosedAt, 10*time.Minute)
}

func TestStatsReflectIndexedData(t *testing.T) {
	stats := fetchStats(t, "")
	assert.EqualValues(t, 1, stats.EventCount)
	assert.EqualValues(t, 1, stats.InvocationCount)
	assert.EqualValues(t, 0, stats.StorageCount)
	assert.EqualValues(t, latestLedger, stats.LastSyncedLedger)
	assert.EqualValues(t, 1, stats.WindowEventCount)
	assert.EqualValues(t, 1, stats.WindowInvocationCount)
	assert.Equal(t, "24h", stats.WindowDuration)

	weekly := fetchStats(t, "?window=7d")
	assert.Equal(t, "7d", weekly.WindowDuration)
	assert.EqualValues(t, 1, weekly.WindowEventCount)
	assert.EqualValues(t, 1, weekly.WindowInvocationCount)
}

func TestEventFiltersAndPagination(t *testing.T) {
	matching, _ := fetchEvents(t, "?type=contract")
	assert.Len(t, matching, 1, "type=contract must return the indexed event")

	other, _ := fetchEvents(t, "?type=transfer")
	assert.Empty(t, other, "a different event type must not match")

	page, next := fetchEvents(t, "?limit=1")
	assert.Len(t, page, 1)
	assert.Empty(t, next, "a single-row result is the last page")

	outOfRange, _ := fetchEvents(t, fmt.Sprintf("?from=%d", latestLedger))
	assert.Empty(t, outOfRange, "the event ledger is below the from bound")

	status, body, err := apiRequest("GET", "/api/v1/contracts/"+otherContract+"/events", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	var resp eventsResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Empty(t, resp.Events, "another contract must not see the fixture event")
}

func TestStorageIsEmpty(t *testing.T) {
	status, body, err := apiRequest("GET", "/api/v1/contracts/"+contractID+"/storage", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body: %s", body)

	var resp struct {
		Storage []json.RawMessage `json:"storage"`
	}
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Empty(t, resp.Storage)
}

func TestIndexerIsIdempotent(t *testing.T) {
	require.NoError(t, runIndexerOnce(), "second indexing pass must succeed")

	events, _ := fetchEvents(t, "")
	assert.Len(t, events, 1, "re-indexing must not duplicate the event")

	stats := fetchStats(t, "")
	assert.EqualValues(t, 1, stats.EventCount)
	assert.EqualValues(t, 1, stats.InvocationCount)
	assert.EqualValues(t, latestLedger, stats.LastSyncedLedger)
}

func TestRegistrationRejectsInvalidContractID(t *testing.T) {
	status, body, err := apiRequest("POST", "/api/v1/contracts", map[string]any{
		"id":      "not-a-contract",
		"network": "testnet",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusUnprocessableEntity, status, "body: %s", body)

	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	assert.Equal(t, "INVALID_INPUT", envelope.Error.Code)
}

// ---- API types -------------------------------------------------------------

type event struct {
	ID               string    `json:"id"`
	ContractID       string    `json:"contract_id"`
	Network          string    `json:"network"`
	Ledger           uint32    `json:"ledger"`
	LedgerClosedAt   time.Time `json:"ledger_closed_at"`
	TxHash           string    `json:"tx_hash"`
	Type             string    `json:"type"`
	TopicXDR         []string  `json:"topic_xdr"`
	ValueXDR         string    `json:"value_xdr"`
	InSuccessfulCall bool      `json:"in_successful_call"`
}

type eventsResponse struct {
	Events     []event `json:"events"`
	NextCursor string  `json:"next_cursor"`
}

type invocation struct {
	TxHash           string    `json:"tx_hash"`
	ContractID       string    `json:"contract_id"`
	Network          string    `json:"network"`
	Ledger           uint32    `json:"ledger"`
	LedgerClosedAt   time.Time `json:"ledger_closed_at"`
	Status           string    `json:"status"`
	FunctionName     string    `json:"function_name"`
	ResultXDR        string    `json:"result_xdr"`
	ApplicationOrder int       `json:"application_order"`
}

type invocationsResponse struct {
	Invocations []invocation `json:"invocations"`
	NextCursor  string       `json:"next_cursor"`
}

type statsResponse struct {
	EventCount            int64  `json:"event_count"`
	InvocationCount       int64  `json:"invocation_count"`
	StorageCount          int64  `json:"storage_count"`
	LastSyncedLedger      int64  `json:"last_synced_ledger"`
	WindowEventCount      int64  `json:"window_event_count"`
	WindowInvocationCount int64  `json:"window_invocation_count"`
	WindowDuration        string `json:"window_duration"`
}

func fetchEvents(t *testing.T, query string) ([]event, string) {
	t.Helper()
	status, body, err := apiRequest("GET", "/api/v1/contracts/"+contractID+"/events"+query, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body: %s", body)
	var resp eventsResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	return resp.Events, resp.NextCursor
}

func fetchStats(t *testing.T, query string) statsResponse {
	t.Helper()
	status, body, err := apiRequest("GET", "/api/v1/contracts/"+contractID+"/stats"+query, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body: %s", body)
	var resp statsResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	return resp
}

// ---- HTTP ------------------------------------------------------------------

func apiRequest(method, path string, payload any) (int, []byte, error) {
	var reader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, apiURL+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if sessionToken != "" {
		req.Header.Set("Authorization", "Bearer "+sessionToken)
	}
	resp, err := apiHTTPClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, body, err
}

// ---- compose ---------------------------------------------------------------

var (
	composeOnce sync.Once
	composeBase []string
)

// composeCommand resolves "docker compose" (v2 plugin) or the standalone
// docker-compose binary, mirroring the Makefile.
func composeCommand() []string {
	composeOnce.Do(func() {
		if override := os.Getenv("DOCKER_COMPOSE"); override != "" {
			composeBase = strings.Fields(override)
			return
		}
		for _, candidate := range [][]string{{"docker", "compose"}, {"docker-compose"}} {
			cmd := exec.Command(candidate[0], append(candidate[1:], "version")...)
			if err := cmd.Run(); err == nil {
				composeBase = candidate
				return
			}
		}
		composeBase = []string{"docker-compose"}
	})
	return composeBase
}

// locateCompose finds the integration compose file whether the suite is run
// from tests/integration or from the repository root.
func locateCompose() (file, dir string) {
	if override := os.Getenv("INTEGRATION_COMPOSE_FILE"); override != "" {
		abs, err := filepath.Abs(override)
		if err == nil {
			return abs, filepath.Dir(abs)
		}
	}
	wd, _ := os.Getwd()
	candidate := filepath.Join(wd, "tests", "integration", "docker-compose.yml")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, filepath.Dir(candidate)
	}
	candidate = filepath.Join(wd, "docker-compose.yml")
	if _, err := os.Stat(candidate); err == nil && filepath.Base(wd) == "integration" {
		return candidate, wd
	}
	return "", wd
}

func composeOutput(args ...string) (string, error) {
	base := composeCommand()
	full := append([]string{}, base...)
	file, dir := locateCompose()
	if file != "" {
		full = append(full, "-f", file)
	}
	full = append(full, args...)

	cmd := exec.Command(full[0], full[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
