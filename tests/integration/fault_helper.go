package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var (
	toxiproxyURL = envOr("TOXIPROXY_URL", "http://localhost:8474")
	mockRPCURL   = envOr("SOROBAN_RPC_URL", "http://localhost:8000")
)

// ToxiproxyClient manages upstream proxies and toxic injections.
type ToxiproxyClient struct {
	baseURL string
	client  *http.Client
}

// NewToxiproxyClient creates a ToxiproxyClient pointing to the configured endpoint.
func NewToxiproxyClient() *ToxiproxyClient {
	return &ToxiproxyClient{
		baseURL: toxiproxyURL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// CreateProxy registers a new proxy mapping an incoming port to an upstream service.
func (c *ToxiproxyClient) CreateProxy(name, listen, upstream string) error {
	payload := map[string]any{
		"name":     name,
		"listen":   listen,
		"upstream": upstream,
		"enabled":  true,
	}
	body, _ := json.Marshal(payload)
	resp, err := c.client.Post(fmt.Sprintf("%s/proxies", c.baseURL), "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create proxy failed: %s: %s", resp.Status, b)
	}
	return nil
}

// AddTimeoutToxic injects a timeout into downstream traffic.
func (c *ToxiproxyClient) AddTimeoutToxic(proxyName, toxicName string, timeoutMs int) error {
	payload := map[string]any{
		"name":       toxicName,
		"type":       "timeout",
		"stream":     "downstream",
		"attributes": map[string]any{"timeout": timeoutMs},
	}
	return c.addToxic(proxyName, payload)
}

// AddLatencyToxic adds latency and jitter to downstream traffic.
func (c *ToxiproxyClient) AddLatencyToxic(proxyName, toxicName string, latencyMs, jitterMs int) error {
	payload := map[string]any{
		"name":       toxicName,
		"type":       "latency",
		"stream":     "downstream",
		"attributes": map[string]any{"latency": latencyMs, "jitter": jitterMs},
	}
	return c.addToxic(proxyName, payload)
}

// AddResetPeerToxic forces socket closure / reset peer.
func (c *ToxiproxyClient) AddResetPeerToxic(proxyName, toxicName string) error {
	payload := map[string]any{
		"name":   toxicName,
		"type":   "reset_peer",
		"stream": "downstream",
	}
	return c.addToxic(proxyName, payload)
}

// AddSlicerToxic slices packets into small chunks with delay (partial/truncated response).
func (c *ToxiproxyClient) AddSlicerToxic(proxyName, toxicName string, avgSize, variation, delayUs int) error {
	payload := map[string]any{
		"name":   toxicName,
		"type":   "slicer",
		"stream": "downstream",
		"attributes": map[string]any{
			"average_size":   avgSize,
			"size_variation": variation,
			"delay":          delayUs,
		},
	}
	return c.addToxic(proxyName, payload)
}

func (c *ToxiproxyClient) addToxic(proxyName string, payload map[string]any) error {
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/proxies/%s/toxics", c.baseURL, proxyName)
	resp, err := c.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("add toxic failed: %s: %s", resp.Status, b)
	}
	return nil
}

// RemoveToxic removes a named toxic from a proxy.
func (c *ToxiproxyClient) RemoveToxic(proxyName, toxicName string) error {
	url := fmt.Sprintf("%s/proxies/%s/toxics/%s", c.baseURL, proxyName, toxicName)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Reset clears all configured toxics and proxies in Toxiproxy.
func (c *ToxiproxyClient) Reset() error {
	resp, err := c.client.Post(fmt.Sprintf("%s/reset", c.baseURL), "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// MockRPCFaultConfig holds options for the mock RPC fault generator.
type MockRPCFaultConfig struct {
	TimeoutMs      int    `json:"timeout_ms,omitempty"`
	StatusCode     int    `json:"status_code,omitempty"`
	Truncate       bool   `json:"truncate,omitempty"`
	DropConnection bool   `json:"drop_connection,omitempty"`
	FailCount      int    `json:"fail_count,omitempty"`
	Method         string `json:"method,omitempty"`
}

// MockRPCFaultClient configures faults on the mock RPC service.
type MockRPCFaultClient struct {
	baseURL string
	client  *http.Client
}

// NewMockRPCFaultClient creates a client to toggle mock-rpc faults.
func NewMockRPCFaultClient() *MockRPCFaultClient {
	return &MockRPCFaultClient{
		baseURL: mockRPCURL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// ConfigureFault sends a fault configuration to mock-rpc.
func (m *MockRPCFaultClient) ConfigureFault(cfg MockRPCFaultConfig) error {
	body, _ := json.Marshal(cfg)
	resp, err := m.client.Post(fmt.Sprintf("%s/fault/configure", m.baseURL), "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("configure fault failed: %s: %s", resp.Status, b)
	}
	return nil
}

// Reset clears any active fault on mock-rpc.
func (m *MockRPCFaultClient) Reset() error {
	resp, err := m.client.Post(fmt.Sprintf("%s/fault/reset", m.baseURL), "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// ResetAllFaults resets both toxiproxy and mock-rpc fault states.
func ResetAllFaults() {
	_ = NewToxiproxyClient().Reset()
	_ = NewMockRPCFaultClient().Reset()
}
