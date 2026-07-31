package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

// ListEventsOpts holds optional filters for listing events.
type ListEventsOpts struct {
	Type   string
	Limit  int
	Cursor string
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

// GetGlobalStats fetches network-wide aggregate statistics.
func (c *Client) GetGlobalStats(ctx context.Context) (GlobalStats, error) {
	var out GlobalStats
	err := c.do(ctx, http.MethodGet, "/api/v1/stats/global", nil, &out)
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
