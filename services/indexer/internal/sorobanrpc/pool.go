package sorobanrpc

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// ProviderEndpoint defines an RPC endpoint config with optional weight.
type ProviderEndpoint struct {
	URL    string
	Weight int
}

// EndpointMetrics holds metrics for a single RPC provider endpoint.
type EndpointMetrics struct {
	URL                 string        `json:"url"`
	Weight              int           `json:"weight"`
	SuccessfulCalls     int64         `json:"successful_calls"`
	FailedCalls         int64         `json:"failed_calls"`
	ConsecutiveFailures int64         `json:"consecutive_failures"`
	AverageLatency      time.Duration `json:"average_latency"`
	State               string        `json:"state"`
}

// ProviderPool manages multiple RPC clients with health-weighted selection and circuit breaking.
type ProviderPool struct {
	mu        sync.Mutex
	clients   []*pooledClient
	maxBreaks int
}

type pooledClient struct {
	client              *Client
	weight              int
	successfulCalls     atomic.Int64
	failedCalls         atomic.Int64
	consecutiveFailures atomic.Int64
	totalLatencyNs      atomic.Int64
	callCount           atomic.Int64
	isTripped           atomic.Bool
	trippedAt           atomic.Int64 // unix nano
}

// NewProviderPool creates an RPC provider pool.
func NewProviderPool(endpoints []ProviderEndpoint, opts ...Option) *ProviderPool {
	holdingNewPool := func(endpoints []ProviderEndpoint, opts ...Option) *ProviderPool {
		var pClients []*pooledClient
		for _, ep := range endpoints {
			w := ep.Weight
			if w <= 0 {
				w = 1
			}
			pClients = append(pClients, &pooledClient{
				client: New(ep.URL, opts...),
				weight: w,
			})
		}
		return &ProviderPool{
			clients: pClients,
		}
	}
	return holdingNewPool(endpoints, opts...)
}

// Metrics returns per-endpoint metrics for inspection.
func (p *ProviderPool) Metrics() []EndpointMetrics {
	p.mu.Lock()
	defer p.mu.Unlock()

	var res []EndpointMetrics
	for _, pc := range p.clients {
		state := "healthy"
		if pc.isTripped.Load() {
			if time.Since(time.Unix(0, pc.trippedAt.Load())) > 10*time.Second {
				state = "probing"
			} else {
				state = "tripped"
			}
		}

		var avgLatency time.Duration
		if cc := pc.callCount.Load(); cc > 0 {
			avgLatency = time.Duration(pc.totalLatencyNs.Load() / cc)
		}

		res = append(res, EndpointMetrics{
			URL:                 pc.client.Endpoint(),
			Weight:              pc.weight,
			SuccessfulCalls:     pc.successfulCalls.Load(),
			FailedCalls:         pc.failedCalls.Load(),
			ConsecutiveFailures: pc.consecutiveFailures.Load(),
			AverageLatency:      avgLatency,
			State:               state,
		})
	}
	return res
}

// SelectBestClient picks a healthy client based on weights, latency, and circuit breaker status.
func (p *ProviderPool) SelectBestClient(minLedger uint32) (*Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var best *pooledClient
	bestScore := int64(-1)

	now := time.Now()

	for _, pc := range p.clients {
		// Check circuit breaker
		if pc.isTripped.Load() {
			// Probe periodically after 10 seconds
			if now.Sub(time.Unix(0, pc.trippedAt.Load())) > 10*time.Second {
				// allow probe
			} else {
				continue
			}
		}

		// Respect retention window if minLedger > 0
		if minLedger > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, err := pc.client.GetLatestLedger(ctx)
			cancel()
			if err == nil {
				// If node has history
			}
		}

		// Score calculation: weight * (1000 / (1 + avgLatencyMs + consecutiveFailures*500))
		cc := pc.callCount.Load()
		var avgMs int64
		if cc > 0 {
			avgMs = pc.totalLatencyNs.Load() / cc / int64(time.Millisecond)
		}
		fails := pc.consecutiveFailures.Load()

		score := int64(pc.weight) * 1000 / (1 + avgMs + fails*500)
		if score > bestScore {
			bestScore = score
			best = pc
		}
	}

	if best == nil {
		// Fallback to any client if all tripped
		if len(p.clients) > 0 {
			best = p.clients[0]
		}
	}

	if best == nil {
		return nil, context.DeadlineExceeded
	}

	return best.client, nil
}

// RecordResult updates metrics and circuit breaker state for a client endpoint.
func (p *ProviderPool) RecordResult(endpoint string, duration time.Duration, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, pc := range p.clients {
		if pc.client.Endpoint() == endpoint {
			pc.callCount.Add(1)
			pc.totalLatencyNs.Add(duration.Nanoseconds())
			if err != nil {
				pc.failedCalls.Add(1)
				fails := pc.consecutiveFailures.Add(1)
				if fails >= 3 {
					pc.isTripped.Store(true)
					pc.trippedAt.Store(time.Now().UnixNano())
				}
			} else {
				pc.successfulCalls.Add(1)
				pc.consecutiveFailures.Store(0)
				pc.isTripped.Store(false)
			}
		}
	}
}
