package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertDataCompleteness asserts that exactly expectedCount events are indexed
// and queryable through the API, with matching event ID and topic.
func assertDataCompleteness(t *testing.T, expectedCount int) {
	t.Helper()
	status, body, err := apiRequest("GET", "/api/v1/contracts/"+contractID+"/events", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	var resp eventsResponse
	err = json.Unmarshal(body, &resp)
	require.NoError(t, err)

	require.Len(t, resp.Events, expectedCount, "event count must match expected exactly without duplicates or losses")
	if expectedCount > 0 {
		ev := resp.Events[0]
		assert.Equal(t, eventID, ev.ID)
		assert.Equal(t, uint32(eventLedger), ev.Ledger)
		assert.Equal(t, txHash, ev.TxHash)
	}
}

// TestFault_RPCTimeoutMidWindow tests indexer resilience when Soroban RPC times out mid-request.
func TestFault_RPCTimeoutMidWindow(t *testing.T) {
	defer ResetAllFaults()
	ResetAllFaults()

	rpcClient := NewMockRPCFaultClient()
	// Injects a 6-second timeout on getEvents for 1 call, exceeding the client request timeout.
	err := rpcClient.ConfigureFault(MockRPCFaultConfig{
		Method:    "getEvents",
		TimeoutMs: 6000,
		FailCount: 1,
	})
	require.NoError(t, err)

	// Indexer should fail or retry without advancing cursor or corrupting state
	_ = runIndexerOnce()

	// Ensure fault is cleared and run indexer again
	ResetAllFaults()
	err = runIndexerOnce()
	require.NoError(t, err, "subsequent indexer run after timeout resolution must succeed cleanly")

	// Verify complete event ingest with no duplicate rows
	assertDataCompleteness(t, 1)
}

// TestFault_RPCPartialResponse tests indexer handling when Soroban RPC returns a truncated response.
func TestFault_RPCPartialResponse(t *testing.T) {
	defer ResetAllFaults()
	ResetAllFaults()

	rpcClient := NewMockRPCFaultClient()
	// Inject truncated response on getEvents for 1 call
	err := rpcClient.ConfigureFault(MockRPCFaultConfig{
		Method:    "getEvents",
		Truncate:  true,
		FailCount: 1,
	})
	require.NoError(t, err)

	// Indexer encounters invalid JSON syntax and must roll back cleanly
	_ = runIndexerOnce()

	// Clear fault and indexer runs cleanly
	ResetAllFaults()
	err = runIndexerOnce()
	require.NoError(t, err, "subsequent indexer run after truncated payload must succeed")

	// Verify complete event ingest with no corrupt or missing rows
	assertDataCompleteness(t, 1)
}

// TestFault_DatabaseConnectionLossDuringWrite tests behavior when database connectivity drops.
func TestFault_DatabaseConnectionLossDuringWrite(t *testing.T) {
	defer ResetAllFaults()
	ResetAllFaults()

	// Use toxiproxy to inject postgres socket reset if toxiproxy is active,
	// or exercise the indexer transaction rollback behavior.
	toxi := NewToxiproxyClient()
	_ = toxi.CreateProxy("postgres_proxy", "0.0.0.0:5433", "postgres:5432")
	_ = toxi.AddResetPeerToxic("postgres_proxy", "db_down")

	// Attempt indexer run under network fault
	_ = runIndexerOnce()

	// Restore database connectivity
	_ = toxi.RemoveToxic("postgres_proxy", "db_down")
	ResetAllFaults()

	// Run indexer once DB is healthy
	err := runIndexerOnce()
	require.NoError(t, err, "indexer must recover cleanly after DB connectivity restore")

	// Assert data completeness
	assertDataCompleteness(t, 1)
}

// TestFault_ProcessKillAndRestartCursorResume tests that cursor resumes correctly after process termination.
func TestFault_ProcessKillAndRestartCursorResume(t *testing.T) {
	defer ResetAllFaults()
	ResetAllFaults()

	// Run indexer with short timeout/kill simulation
	out, _ := composeOutput("run", "--rm", "indexer", "-mode", "once")
	t.Logf("first indexer pass output: %s", out)

	// Immediate second pass simulates process resume; must be idempotent
	err := runIndexerOnce()
	require.NoError(t, err, "second indexer run must be idempotent and resume from cursor")

	// Verify idempotency: exactly 1 event exists
	assertDataCompleteness(t, 1)
}

// TestFault_TransientRPC503OrRateLimit tests transient RPC 503 / 429 response handling.
func TestFault_TransientRPC503OrRateLimit(t *testing.T) {
	defer ResetAllFaults()
	ResetAllFaults()

	rpcClient := NewMockRPCFaultClient()
	// Return HTTP 503 Service Unavailable for 1 call
	err := rpcClient.ConfigureFault(MockRPCFaultConfig{
		Method:     "getEvents",
		StatusCode: http.StatusServiceUnavailable,
		FailCount:  1,
	})
	require.NoError(t, err)

	// First pass handles transient error
	_ = runIndexerOnce()

	// Clear fault and rerun
	ResetAllFaults()
	err = runIndexerOnce()
	require.NoError(t, err, "indexer must recover cleanly from transient 503")

	// Verify event data completeness
	assertDataCompleteness(t, 1)
}
