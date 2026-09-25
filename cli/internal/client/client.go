package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	Version   = "dev"
	userAgent = "sorolens-cli/" + Version
)

// Client is a typed HTTP client for the Sorolens API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
	apiKey     string
}

// New creates a Client targeting baseURL with the given request timeout.
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// WithAPIKey returns the client with an API key attached to every request.
// The key is sent as both X-API-Key and Authorization: Bearer, matching the
// authentication schemes accepted by the Sorolens API.
func (c *Client) WithAPIKey(key string) *Client {
	c.apiKey = key
	return c
}

// ListEventsOpts holds optional filters for listing events.
type ListEventsOpts struct {
	Type   string
	Limit  int
	Cursor string
}

// ListContractsOpts holds optional filters for listing contracts.
type ListContractsOpts struct {
	Status string
	Limit  int
	Cursor string
}

// ListInvocationsOpts holds optional filters for listing invocations.
type ListInvocationsOpts struct {
	Status string
	Fn     string
	Limit  int
	Cursor string
}

// ListAlertsOpts holds optional filters for listing watchdog alerts.
type ListAlertsOpts struct {
	Severity string
	Limit    int
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var buf *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		buf = bytes.NewBuffer(b)
	}

	var reqBody *bytes.Reader
	if buf != nil {
		reqBody = bytes.NewReader(buf.Bytes())
	}

	var req *http.Request
	var err error
	if reqBody != nil {
		req, err = http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	}
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var env struct {
			Error struct {
				Code      string `json:"code"`
				Message   string `json:"message"`
				RequestID string `json:"request_id"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&env)
		return &SorolensError{
			Code:      env.Error.Code,
			Message:   env.Error.Message,
			RequestID: env.Error.RequestID,
			Status:    resp.StatusCode,
		}
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// GetContract fetches a single contract by ID.
func (c *Client) GetContract(ctx context.Context, contractID string) (Contract, error) {
	var out Contract
	err := c.do(ctx, http.MethodGet, "/api/v1/contracts/"+contractID, nil, &out)
	return out, err
}

// ListEvents fetches a paginated list of events for a contract.
func (c *Client) ListEvents(ctx context.Context, contractID string, opts ListEventsOpts) (EventsResponse, error) {
	q := url.Values{}
	if opts.Type != "" {
		q.Set("type", opts.Type)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Cursor != "" {
		q.Set("cursor", opts.Cursor)
	}
	path := "/api/v1/contracts/" + contractID + "/events"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out EventsResponse
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

// GetStorage fetches all storage entries for a contract (first page, limit 200).
func (c *Client) GetStorage(ctx context.Context, contractID string) ([]StorageEntry, error) {
	path := "/api/v1/contracts/" + contractID + "/storage?limit=200"
	var out StorageResponse
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out.Storage, err
}

// ListContracts fetches a paginated list of tracked contracts.
func (c *Client) ListContracts(ctx context.Context, opts ListContractsOpts) (ContractsResponse, error) {
	q := url.Values{}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Cursor != "" {
		q.Set("cursor", opts.Cursor)
	}
	path := "/api/v1/contracts"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out ContractsResponse
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

// ListInvocations fetches a paginated list of invocations for a contract.
func (c *Client) ListInvocations(ctx context.Context, contractID string, opts ListInvocationsOpts) (InvocationsResponse, error) {
	q := url.Values{}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.Fn != "" {
		q.Set("fn", opts.Fn)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Cursor != "" {
		q.Set("cursor", opts.Cursor)
	}
	path := "/api/v1/contracts/" + contractID + "/invocations"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out InvocationsResponse
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

// ListAlerts fetches watchdog alerts, optionally filtered by severity.
func (c *Client) ListAlerts(ctx context.Context, opts ListAlertsOpts) (AlertsResponse, error) {
	q := url.Values{}
	if opts.Severity != "" {
		q.Set("severity", opts.Severity)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	path := "/api/v1/watchdog/alerts"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out AlertsResponse
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

// TrackContract registers a contract for tracking.
func (c *Client) TrackContract(ctx context.Context, contractID, alias, network string) (Contract, error) {
	body := map[string]string{
		"id":      contractID,
		"network": network,
		"label":   alias,
	}
	var out Contract
	err := c.do(ctx, http.MethodPost, "/api/v1/contracts", body, &out)
	return out, err
}

// SaveLabel creates or updates a public or workspace label.
func (c *Client) SaveLabel(ctx context.Context, label, value, scope string) (Label, error) {
	var out Label
	err := c.do(ctx, http.MethodPost, "/api/v1/labels", map[string]string{"label": label, "value": value, "scope": scope}, &out)
	return out, err
}

// GetGlobalStats fetches network-wide aggregate statistics.
func (c *Client) GetGlobalStats(ctx context.Context) (GlobalStats, error) {
	var out GlobalStats
	err := c.do(ctx, http.MethodGet, "/api/v1/stats/global", nil, &out)
	return out, err
}

// GetMonitoredContract fetches the watchdog's on-chain health status for a monitored contract.
func (c *Client) GetMonitoredContract(ctx context.Context, contractID string) (MonitoredContract, error) {
	var out MonitoredContract
	err := c.do(ctx, http.MethodGet, "/api/v1/watchdog/contracts/"+contractID, nil, &out)
	return out, err
}

// GetContractStats fetches per-contract statistics for the given window ("24h", "7d", "30d").
func (c *Client) GetContractStats(ctx context.Context, contractID, window string) (ContractStats, error) {
	if window == "" {
		window = "24h"
	}
	path := "/api/v1/contracts/" + contractID + "/stats?window=" + window
	var out ContractStats
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

// StreamEvents opens the server-sent events stream at
// /api/v1/stream/events and invokes onMessage for every data frame received.
// When contractID is empty the stream carries events for all contracts.
//
// StreamEvents blocks until ctx is cancelled, the server closes the stream, or
// onMessage returns an error. A cancellation through ctx is reported as a nil
// error so callers can treat Ctrl-C as a clean shutdown. SSE comment frames
// (heartbeats) and malformed frames are skipped rather than terminating the
// stream.
func (c *Client) StreamEvents(ctx context.Context, contractID string, onMessage func(StreamMessage) error) error {
	q := url.Values{}
	if contractID != "" {
		q.Set("contract_id", contractID)
	}
	path := "/api/v1/stream/events"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/event-stream")

	// The stream is long-lived, so it must not inherit the shared client's
	// request timeout. The request context governs the connection lifetime.
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var env struct {
			Error struct {
				Code      string `json:"code"`
				Message   string `json:"message"`
				RequestID string `json:"request_id"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&env)
		return &SorolensError{
			Code:      env.Error.Code,
			Message:   env.Error.Message,
			RequestID: env.Error.RequestID,
			Status:    resp.StatusCode,
		}
	}

	scanner := bufio.NewScanner(resp.Body)
	// SSE frames can exceed bufio.Scanner's 64 KiB default; allow up to 1 MiB.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			// Comment frames such as ": ping" and other SSE fields keep the
			// connection alive but carry no payload for the CLI.
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var msg StreamMessage
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			continue
		}
		if err := onMessage(msg); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("read stream: %w", err)
	}
	return nil
}
