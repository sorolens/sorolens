package poller

import (
	"context"
	"math"

	"golang.org/x/time/rate"
)

// DefaultRPCRateRPS is the per-network Soroban RPC budget (requests per
// second) used when Config.RPCRateLimits is nil. It keeps a parallel pass
// from multiplying its request rate by Concurrency against a public RPC
// provider. Set INDEXER_RPC_RPS=0 to opt out.
const DefaultRPCRateRPS = 10.0

// buildRateLimiters turns the per-network RPS settings into one shared
// limiter per network key. A nil map falls back to DefaultRPCRateRPS on the
// "" key, so a Poller built with a zero-value Config still honors rate
// limits. An RPS of 0 stores a nil limiter, which means "unlimited" for
// exactly that network.
func buildRateLimiters(limits map[string]float64) map[string]*rate.Limiter {
	if limits == nil {
		limits = map[string]float64{"": DefaultRPCRateRPS}
	}
	limiters := make(map[string]*rate.Limiter, len(limits))
	for network, rps := range limits {
		if rps <= 0 {
			limiters[network] = nil // explicitly unlimited
			continue
		}
		// Burst is one second of budget, so short RPC bursts that fit in
		// the average rate are never delayed.
		burst := int(math.Ceil(rps))
		if burst < 1 {
			burst = 1
		}
		limiters[network] = rate.NewLimiter(rate.Limit(rps), burst)
	}
	return limiters
}

// limiterFor returns the shared limiter for a network. An explicit entry
// wins (including an explicit nil meaning unlimited); otherwise the "" budget
// applies, and with no "" entry the network is unlimited.
func limiterFor(limiters map[string]*rate.Limiter, network string) *rate.Limiter {
	if l, ok := limiters[network]; ok {
		return l
	}
	return limiters[""]
}

// rateLimitedClient wraps c so every RPC method waits on the shared limiter.
// A nil limiter returns c unchanged, so networks configured as unlimited pay
// no indirection.
func rateLimitedClient(c RPCClient, l *rate.Limiter) RPCClient {
	if c == nil || l == nil {
		return c
	}
	return &rateLimitedRPC{RPCClient: c, limiter: l}
}

// rateLimitedRPC gates each RPC method on a shared limiter before delegating
// to the wrapped client. Wrapping the client (rather than sprinkling waits
// through the poller) keeps every call site covered, including per-transaction
// GetTransaction calls, without touching processContract.
//
// A single limiter instance is shared by all concurrent workers on the
// network, which is what makes Concurrency > 1 safe against per-network RPC
// rate limits. Limiter.Wait honours cancellation, so shutdown and once-mode
// deadlines are not held up by a queued request.
type rateLimitedRPC struct {
	RPCClient
	limiter *rate.Limiter
}

// wait blocks until the limiter admits one request or ctx is done.
func (r *rateLimitedRPC) wait(ctx context.Context) error {
	return r.limiter.Wait(ctx)
}

func (r *rateLimitedRPC) GetLatestLedger(ctx context.Context) (*LatestLedger, error) {
	if err := r.wait(ctx); err != nil {
		return nil, err
	}
	return r.RPCClient.GetLatestLedger(ctx)
}

func (r *rateLimitedRPC) GetEvents(ctx context.Context, startLedger, endLedger uint32, filters []EventFilter) (*GetEventsResult, error) {
	if err := r.wait(ctx); err != nil {
		return nil, err
	}
	return r.RPCClient.GetEvents(ctx, startLedger, endLedger, filters)
}

func (r *rateLimitedRPC) GetTransaction(ctx context.Context, hash string) (*TransactionResult, error) {
	if err := r.wait(ctx); err != nil {
		return nil, err
	}
	return r.RPCClient.GetTransaction(ctx, hash)
}

func (r *rateLimitedRPC) GetLedgerEntries(ctx context.Context, keys []string) (*GetLedgerEntriesResult, error) {
	if err := r.wait(ctx); err != nil {
		return nil, err
	}
	return r.RPCClient.GetLedgerEntries(ctx, keys)
}
