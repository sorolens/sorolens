package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// ApiClient is an opinionated, typed wrapper over the generated oapi-codegen
// client. It injects the API key into every outgoing request and exposes the
// high-level endpoints as Go functions returning strongly-typed models.
type ApiClient struct {
	generated *ClientWithResponses
	apiKey    string
}

// New constructs an ApiClient for a Sorolens server URL.
//
// The server URL can be a full base like "https://api.sorolens.xyz" or
// "http://localhost:8080". If apiKey is non-empty it is sent as a bearer
// token on every request. Pass an optional *http.Client to override the
// default transport.
func New(server, apiKey string, httpClient *http.Client) (*ApiClient, error) {
	opts := []ClientOption{}
	if apiKey != "" {
		opts = append(opts, withBearerToken(apiKey))
	}
	if httpClient != nil {
		opts = append(opts, WithHTTPClient(httpClient))
	}
	c, err := NewClientWithResponses(server, opts...)
	if err != nil {
		return nil, fmt.Errorf("go-client: init: %w", err)
	}
	return &ApiClient{generated: c, apiKey: apiKey}, nil
}

// Generated exposes the full generated client (all endpoints including the
// admin and role routes) for advanced callers.
func (c *ApiClient) Generated() *ClientWithResponses {
	return c.generated
}

// APIKey returns the configured bearer token (empty if none).
func (c *ApiClient) APIKey() string {
	return c.apiKey
}

// ---- typed convenience methods ---------------------------------------------

// ListContracts retrieves a page of tracked contracts.
func (c *ApiClient) ListContracts(ctx context.Context, params *ListContractsParams) (*ListContractsResponse, error) {
	return c.generated.ListContractsWithResponse(ctx, params)
}

// GetContract fetches a single contract.
func (c *ApiClient) GetContract(ctx context.Context, id string) (*GetContractResponse, error) {
	return c.generated.GetContractWithResponse(ctx, id)
}

// GetGlobalStats fetches global index statistics.
func (c *ApiClient) GetGlobalStats(ctx context.Context) (*GetGlobalStatsResponse, error) {
	return c.generated.GetGlobalStatsWithResponse(ctx)
}

// GetContractForecast fetches the usage forecast for a contract.
func (c *ApiClient) GetContractForecast(ctx context.Context, id string, params *GetContractForecastParams) (*GetContractForecastResponse, error) {
	return c.generated.GetContractForecastWithResponse(ctx, id, params)
}

// ListContractEvents fetches a page of contract events.
func (c *ApiClient) ListContractEvents(ctx context.Context, id string, params *ListContractEventsParams) (*ListContractEventsResponse, error) {
	return c.generated.ListContractEventsWithResponse(ctx, id, params)
}

// ListContractInvocations fetches a page of contract invocations.
func (c *ApiClient) ListContractInvocations(ctx context.Context, id string, params *ListContractInvocationsParams) (*ListContractInvocationsResponse, error) {
	return c.generated.ListContractInvocationsWithResponse(ctx, id, params)
}

// ListContractStorage fetches a page of storage entries for a contract.
func (c *ApiClient) ListContractStorage(ctx context.Context, id string, params *ListContractStorageParams) (*ListContractStorageResponse, error) {
	return c.generated.ListContractStorageWithResponse(ctx, id, params)
}

// ListWatchdogAlerts fetches watchdog alerts.
func (c *ApiClient) ListWatchdogAlerts(ctx context.Context, params *ListWatchdogAlertsParams) (*ListWatchdogAlertsResponse, error) {
	return c.generated.ListWatchdogAlertsWithResponse(ctx, params)
}

// GetContractSnapshot fetches the storage snapshot at a historical ledger.
func (c *ApiClient) GetContractSnapshot(ctx context.Context, id string, params *GetContractSnapshotParams) (*GetContractSnapshotResponse, error) {
	return c.generated.GetContractSnapshotWithResponse(ctx, id, params)
}

// withBearerToken returns a ClientOption that injects the API key as a bearer
// token on every request.
func withBearerToken(token string) ClientOption {
	return WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	})
}

// decodeError is a convenience helper that maps a non-2xx response to a
// descriptive error carrying the HTTP status. Many callers only care about
// the overall outcome, so this keeps the generated verbose types out of
// their way.
func decodeError(status string, body []byte) error {
	if status == "" {
		return errors.New("go-client: empty HTTP response")
	}
	return fmt.Errorf("go-client: API returned %s: %s", status, truncate(body, 200))
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
