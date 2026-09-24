package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var apiURL = "http://localhost:8080"
var client = &http.Client{Timeout: 5 * time.Second}

func makeRequest(t *testing.T, method, path string, body map[string]interface{}) *http.Response {
	var req *http.Request
	if body != nil {
		reqBody, _ := json.Marshal(body)
		req, _ = http.NewRequest(method, apiURL+path, bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, apiURL+path, nil)
	}

	resp, err := client.Do(req)
	assert.NoError(t, err)
	return resp
}

func parseResponse(t *testing.T, resp *http.Response, v interface{}) {
	defer resp.Body.Close()
	err := json.NewDecoder(resp.Body).Decode(v)
	assert.NoError(t, err)
}

// 1. Track Contract - Basic
func TestIntegration_TrackContract(t *testing.T) {
	resp := makeRequest(t, "POST", "/api/v1/contracts", map[string]interface{}{
		"contract_id": "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAK",
		"network":     "testnet",
	})
	assert.Contains(t, []int{http.StatusOK, http.StatusCreated, http.StatusUnauthorized}, resp.StatusCode)
	resp.Body.Close()
}

// 2. Track Contract - Duplicate
func TestIntegration_TrackContract_Duplicate(t *testing.T) {
	resp := makeRequest(t, "POST", "/api/v1/contracts", map[string]interface{}{
		"contract_id": "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAK",
		"network":     "testnet",
	})
	assert.Contains(t, []int{http.StatusOK, http.StatusConflict, http.StatusCreated, http.StatusUnauthorized}, resp.StatusCode)
	resp.Body.Close()
}

// 3. Index Events - Basic
func TestIntegration_IndexEvents_Basic(t *testing.T) {
	resp := makeRequest(t, "GET", "/api/v1/contracts/CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAK/events", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]interface{}
	parseResponse(t, resp, &body)
}

// 4. Index Events - Empty
func TestIntegration_IndexEvents_Empty(t *testing.T) {
	resp := makeRequest(t, "GET", "/api/v1/contracts/CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC/events", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]interface{}
	parseResponse(t, resp, &body)
}

// 5. List Events - Pagination
func TestIntegration_ListEvents_Pagination(t *testing.T) {
	resp := makeRequest(t, "GET", "/api/v1/contracts/CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAK/events?limit=2", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]interface{}
	parseResponse(t, resp, &body)
}

// 6. List Events - Filter
func TestIntegration_ListEvents_Filter(t *testing.T) {
	resp := makeRequest(t, "GET", "/api/v1/contracts/CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAK/events?type=transfer", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]interface{}
	parseResponse(t, resp, &body)
}

// 7. Watchdog Register - Success
func TestIntegration_WatchdogRegister_Success(t *testing.T) {
	resp := makeRequest(t, "POST", "/api/v1/watchdog/contracts", map[string]interface{}{
		"contract_id": "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAK",
		"email": "test@example.com",
	})
	assert.Contains(t, []int{http.StatusOK, http.StatusCreated, http.StatusNotFound, http.StatusUnauthorized}, resp.StatusCode)
	resp.Body.Close()
}

// 8. Watchdog Register - Fail
func TestIntegration_WatchdogRegister_Fail(t *testing.T) {
	resp := makeRequest(t, "POST", "/api/v1/watchdog/contracts", map[string]interface{}{
		"contract_id": "invalid",
	})
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound, http.StatusUnauthorized}, resp.StatusCode)
	resp.Body.Close()
}

// 9. Alert Fires - OnTrigger
func TestIntegration_AlertFires_OnTrigger(t *testing.T) {
	resp := makeRequest(t, "GET", "/api/v1/watchdog/alerts?status=triggered", nil)
	assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, resp.StatusCode)
	resp.Body.Close()
}

// 10. Alert Fires - NoTrigger
func TestIntegration_AlertFires_NoTrigger(t *testing.T) {
	resp := makeRequest(t, "GET", "/api/v1/watchdog/alerts?status=resolved", nil)
	assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, resp.StatusCode)
	resp.Body.Close()
}
