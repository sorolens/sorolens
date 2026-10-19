package client

import (
	"context"
	"fmt"
	"net/http"
)

// ApiClient is an opinionated, typed wrapper over the generated oapi-codegen
// client. It injects the API key into every outgoing request and exposes the
// high-level endpoints as Go functions returning strongly-typed models.
// Outgoing requests are automatically retried with context-aware backoff and
// Retry-After handling on transient errors (429, 502, 503, 504).
type ApiClient struct {
	generated *ClientWithResponses
	apiKey    string
	retryCfg  RetryConfig
}

// New constructs an ApiClient for a Sorolens server URL.
//
// The server URL can be a full base like "https://api.sorolens.xyz" or
// "http://localhost:8080". If apiKey is non-empty it is sent as a bearer
// token on every request. Pass an optional *http.Client to override or
// configure the HTTP transport (by default, requests are automatically retried
// on transient 429 and 5xx errors with context-aware backoff and Retry-After
// support). Additional ClientOptions (e.g. WithMaxRetries, WithoutRetries,
// WithBackoff) can be passed to tune client behaviour.
func New(server, apiKey string, httpClient *http.Client, opts ...ClientOption) (*ApiClient, error) {
	retryCfg := DefaultRetryConfig()
	clientOpts := []ClientOption{}

	if apiKey != "" {
		clientOpts = append(clientOpts, withBearerToken(apiKey))
	}

	// Set up HTTP transport with automatic retries by default
	var hc *http.Client
	if httpClient != nil {
		copyClient := *httpClient
		hc = &copyClient
		if hc.Transport == nil {
			hc.Transport = http.DefaultTransport
		}
		hc.Transport = NewRetryRoundTripper(hc.Transport, retryCfg)
	} else {
		hc = &http.Client{
			Transport: NewRetryRoundTripper(http.DefaultTransport, retryCfg),
		}
	}
	clientOpts = append(clientOpts, WithHTTPClient(hc))

	// Append caller-provided options (may override retries, headers, etc.)
	clientOpts = append(clientOpts, opts...)

	c, err := NewClientWithResponses(server, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("go-client: init: %w", err)
	}

	// Capture active retry config if RetryRoundTripper is present
	if cl, ok := c.ClientInterface.(*Client); ok {
		if hc, ok := cl.Client.(*http.Client); ok {
			if rt, ok := hc.Transport.(*RetryRoundTripper); ok {
				rt.mu.RLock()
				retryCfg = rt.Config
				rt.mu.RUnlock()
			}
		}
	}

	return &ApiClient{generated: c, apiKey: apiKey, retryCfg: retryCfg}, nil
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

// RetryConfig returns the active retry configuration for the client.
func (c *ApiClient) RetryConfig() RetryConfig {
	return c.retryCfg
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

// RegisterContract registers a contract for tracking.
func (c *ApiClient) RegisterContract(ctx context.Context, body RegisterContractJSONRequestBody) (*RegisterContractResponse, error) {
	return c.generated.RegisterContractWithResponse(ctx, body)
}

// GetGlobalStats fetches global index statistics.
func (c *ApiClient) GetGlobalStats(ctx context.Context) (*GetGlobalStatsResponse, error) {
	return c.generated.GetGlobalStatsWithResponse(ctx)
}

// GetContractStats fetches per-contract statistics over a time window.
func (c *ApiClient) GetContractStats(ctx context.Context, id string, params *GetContractStatsParams) (*GetContractStatsResponse, error) {
	return c.generated.GetContractStatsWithResponse(ctx, id, params)
}

// GetContractForecast fetches the usage forecast for a contract.
func (c *ApiClient) GetContractForecast(ctx context.Context, id string, params *GetContractForecastParams) (*GetContractForecastResponse, error) {
	return c.generated.GetContractForecastWithResponse(ctx, id, params)
}

// ListContractEvents fetches a page of contract events.
func (c *ApiClient) ListContractEvents(ctx context.Context, id string, params *ListContractEventsParams) (*ListContractEventsResponse, error) {
	return c.generated.ListContractEventsWithResponse(ctx, id, params)
}

// ListAllEvents fetches a page of events across all contracts.
func (c *ApiClient) ListAllEvents(ctx context.Context, params *ListAllEventsParams) (*ListAllEventsResponse, error) {
	return c.generated.ListAllEventsWithResponse(ctx, params)
}

// ListContractInvocations fetches a page of contract invocations.
func (c *ApiClient) ListContractInvocations(ctx context.Context, id string, params *ListContractInvocationsParams) (*ListContractInvocationsResponse, error) {
	return c.generated.ListContractInvocationsWithResponse(ctx, id, params)
}

// ListContractStorage fetches a page of storage entries for a contract.
func (c *ApiClient) ListContractStorage(ctx context.Context, id string, params *ListContractStorageParams) (*ListContractStorageResponse, error) {
	return c.generated.ListContractStorageWithResponse(ctx, id, params)
}

// ListWatchdogAlerts fetches watchdog alerts across monitored contracts.
func (c *ApiClient) ListWatchdogAlerts(ctx context.Context, params *ListWatchdogAlertsParams) (*ListWatchdogAlertsResponse, error) {
	return c.generated.ListWatchdogAlertsWithResponse(ctx, params)
}

// ListMonitoredContracts fetches a page of contracts tracked by the watchdog.
func (c *ApiClient) ListMonitoredContracts(ctx context.Context, params *ListMonitoredContractsParams) (*ListMonitoredContractsResponse, error) {
	return c.generated.ListMonitoredContractsWithResponse(ctx, params)
}

// GetMonitoredContract fetches the watchdog monitoring status for a single contract.
func (c *ApiClient) GetMonitoredContract(ctx context.Context, id string) (*GetMonitoredContractResponse, error) {
	return c.generated.GetMonitoredContractWithResponse(ctx, id)
}

// GetWatchdogStats fetches global watchdog aggregate statistics.
func (c *ApiClient) GetWatchdogStats(ctx context.Context, params *GetWatchdogStatsParams) (*GetWatchdogStatsResponse, error) {
	return c.generated.GetWatchdogStatsWithResponse(ctx, params)
}

// ListContractAlerts fetches watchdog alerts for a specific contract.
func (c *ApiClient) ListContractAlerts(ctx context.Context, id string, params *ListContractAlertsParams) (*ListContractAlertsResponse, error) {
	return c.generated.ListContractAlertsWithResponse(ctx, id, params)
}

// ListContractHealthChecks fetches watchdog health check records for a contract.
func (c *ApiClient) ListContractHealthChecks(ctx context.Context, id string, params *ListContractHealthChecksParams) (*ListContractHealthChecksResponse, error) {
	return c.generated.ListContractHealthChecksWithResponse(ctx, id, params)
}

// GetContractSnapshot fetches the storage snapshot at a historical ledger.
func (c *ApiClient) GetContractSnapshot(ctx context.Context, id string, params *GetContractSnapshotParams) (*GetContractSnapshotResponse, error) {
	return c.generated.GetContractSnapshotWithResponse(ctx, id, params)
}

// Health performs a liveness probe on the Sorolens server.
func (c *ApiClient) Health(ctx context.Context) (*HealthResponse, error) {
	return c.generated.HealthWithResponse(ctx)
}

// Readyz performs a readiness probe on the Sorolens server.
func (c *ApiClient) Readyz(ctx context.Context) (*ReadyzResponse, error) {
	return c.generated.ReadyzWithResponse(ctx)
}

// ListApiKeys retrieves API keys for the authenticated workspace.
func (c *ApiClient) ListApiKeys(ctx context.Context, params *ListApiKeysParams) (*ListApiKeysResponse, error) {
	return c.generated.ListApiKeysWithResponse(ctx, params)
}

// CreateApiKey mints a new API key with the given name and scopes.
func (c *ApiClient) CreateApiKey(ctx context.Context, body CreateApiKeyJSONRequestBody) (*CreateApiKeyResponse, error) {
	return c.generated.CreateApiKeyWithResponse(ctx, body)
}

// RevokeApiKey revokes the specified API key.
func (c *ApiClient) RevokeApiKey(ctx context.Context, id string) (*RevokeApiKeyResponse, error) {
	return c.generated.RevokeApiKeyWithResponse(ctx, id)
}

// ListWatchlist returns contracts on the user's watchlist.
func (c *ApiClient) ListWatchlist(ctx context.Context, params *ListWatchlistParams) (*ListWatchlistResponse, error) {
	return c.generated.ListWatchlistWithResponse(ctx, params)
}

// AddToWatchlist adds a contract to the user's watchlist.
func (c *ApiClient) AddToWatchlist(ctx context.Context, params *AddToWatchlistParams, body AddToWatchlistJSONRequestBody) (*AddToWatchlistResponse, error) {
	return c.generated.AddToWatchlistWithResponse(ctx, params, body)
}

// RemoveFromWatchlist removes a contract from the user's watchlist.
func (c *ApiClient) RemoveFromWatchlist(ctx context.Context, id string, params *RemoveFromWatchlistParams) (*RemoveFromWatchlistResponse, error) {
	return c.generated.RemoveFromWatchlistWithResponse(ctx, id, params)
}

// GetWatchlistStatus checks whether a contract is on the user's watchlist.
func (c *ApiClient) GetWatchlistStatus(ctx context.Context, id string, params *GetWatchlistStatusParams) (*GetWatchlistStatusResponse, error) {
	return c.generated.GetWatchlistStatusWithResponse(ctx, id, params)
}

// withBearerToken returns a ClientOption that injects the API key as a bearer
// token on every request.
func withBearerToken(token string) ClientOption {
	return WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	})
}
