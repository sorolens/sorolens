package poller

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// countingRPC counts every RPC method call so tests can prove a wrapper both
// waits and delegates.
type countingRPC struct {
	mu    sync.Mutex
	calls int
}

func (c *countingRPC) add() {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
}

func (c *countingRPC) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func (c *countingRPC) GetLatestLedger(context.Context) (*LatestLedger, error) {
	c.add()
	return &LatestLedger{Sequence: 1}, nil
}

func (c *countingRPC) GetEvents(context.Context, uint32, uint32, []EventFilter) (*GetEventsResult, error) {
	c.add()
	return &GetEventsResult{}, nil
}

func (c *countingRPC) GetTransaction(context.Context, string) (*TransactionResult, error) {
	c.add()
	return &TransactionResult{}, nil
}

func (c *countingRPC) GetLedgerEntries(context.Context, []string) (*GetLedgerEntriesResult, error) {
	c.add()
	return &GetLedgerEntriesResult{}, nil
}

// ---- limiter configuration -------------------------------------------------

func TestBuildRateLimiters(t *testing.T) {
	t.Parallel()

	t.Run("nil config falls back to the default shared budget", func(t *testing.T) {
		limiters := buildRateLimiters(nil)
		l := limiterFor(limiters, "testnet")
		if l == nil {
			t.Fatal("expected a default limiter for an unconfigured network")
		}
		if l.Limit() != rate.Limit(DefaultRPCRateRPS) {
			t.Errorf("limit = %v, want %v", l.Limit(), rate.Limit(DefaultRPCRateRPS))
		}
	})

	t.Run("per-network settings override the shared budget", func(t *testing.T) {
		limiters := buildRateLimiters(map[string]float64{"": 10, "testnet": 5})
		if got := limiterFor(limiters, "testnet"); got == nil || got.Limit() != 5 {
			t.Errorf("testnet limit = %v, want 5", got)
		}
		if got := limiterFor(limiters, "mainnet"); got == nil || got.Limit() != 10 {
			t.Errorf("mainnet limit = %v, want the shared budget 10", got)
		}
	})

	t.Run("burst is one second of budget", func(t *testing.T) {
		limiters := buildRateLimiters(map[string]float64{"": 7.5})
		if got := limiterFor(limiters, ""); got == nil || got.Burst() != 8 {
			t.Errorf("burst = %v, want 8 (ceil of 7.5)", got)
		}
	})

	t.Run("zero disables limiting", func(t *testing.T) {
		if l := limiterFor(buildRateLimiters(map[string]float64{"": 0}), "mainnet"); l != nil {
			t.Errorf("limiter = %v, want nil when the shared budget is 0", l)
		}
		// An explicit per-network 0 wins over a non-zero shared budget.
		limiters := buildRateLimiters(map[string]float64{"": 10, "testnet": 0})
		if l := limiterFor(limiters, "testnet"); l != nil {
			t.Errorf("testnet limiter = %v, want nil (explicit 0)", l)
		}
		if l := limiterFor(limiters, "mainnet"); l == nil {
			t.Error("mainnet should inherit the shared budget of 10")
		}
	})
}

func TestRateLimitedClient_NilLimiterReturnsClient(t *testing.T) {
	t.Parallel()

	inner := &countingRPC{}
	if got := rateLimitedClient(inner, nil); got != RPCClient(inner) {
		t.Errorf("a nil limiter must not wrap the client, got %T", got)
	}
}

// TestNewWithRPCClients_AppliesPerNetworkLimiters proves the wiring: clients
// for limited networks are wrapped, networks explicitly set to 0 are not, and
// the map keys (i.e. routing) are preserved.
func TestNewWithRPCClients_AppliesPerNetworkLimiters(t *testing.T) {
	t.Parallel()

	cfg := Config{RPCRateLimits: map[string]float64{"": 10, "testnet": 0}}
	p := NewWithRPCClients(map[string]RPCClient{
		"testnet": &countingRPC{},
		"mainnet": &countingRPC{},
	}, newFakeStore(nil), newFakeRedis(), cfg, testLogger())

	if _, ok := p.rpcClients["testnet"].(*rateLimitedRPC); ok {
		t.Error("testnet has an explicit budget of 0 and must not be rate limited")
	}
	if _, ok := p.rpcClients["mainnet"].(*rateLimitedRPC); !ok {
		t.Error("mainnet should be wrapped in the shared rate limiter")
	}
}

// ---- limiter behavior ------------------------------------------------------

// TestRateLimitedRPC_SpacesEveryCallToTheBudget asserts that all four RPC
// methods go through the limiter (not just the ones processContract happens
// to call first) and that the wrapped client still receives the call.
func TestRateLimitedRPC_SpacesEveryCallToTheBudget(t *testing.T) {
	t.Parallel()

	calls := map[string]func(RPCClient) error{
		"GetLatestLedger": func(c RPCClient) error {
			_, err := c.GetLatestLedger(context.Background())
			return err
		},
		"GetEvents": func(c RPCClient) error {
			_, err := c.GetEvents(context.Background(), 1, 2, nil)
			return err
		},
		"GetTransaction": func(c RPCClient) error {
			_, err := c.GetTransaction(context.Background(), "abc")
			return err
		},
		"GetLedgerEntries": func(c RPCClient) error {
			_, err := c.GetLedgerEntries(context.Background(), []string{"key"})
			return err
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			inner := &countingRPC{}
			// 50 rps with burst 1: the second and third calls must each wait
			// ~20ms, so three calls take ~40ms end to end.
			client := rateLimitedClient(inner, rate.NewLimiter(50, 1))

			start := time.Now()
			for i := 0; i < 3; i++ {
				if err := call(client); err != nil {
					t.Fatalf("call %d: %v", i, err)
				}
			}
			if elapsed := time.Since(start); elapsed < 30*time.Millisecond {
				t.Errorf("3 calls at 50 rps with burst 1 took %v, want >= 30ms (limiter not applied)", elapsed)
			}
			if got := inner.count(); got != 3 {
				t.Errorf("inner RPC calls = %d, want 3 (the limiter must delegate, not swallow)", got)
			}
		})
	}
}

// TestRateLimitedRPC_HonoursCancelledContext keeps shutdown responsive: a
// request stuck in the limiter queue must fail fast on cancellation instead
// of blocking until its token comes free.
func TestRateLimitedRPC_HonoursCancelledContext(t *testing.T) {
	t.Parallel()

	inner := &countingRPC{}
	// 1 rps with burst 1: the first call eats the only token, so the second
	// would have to wait a full second.
	client := rateLimitedClient(inner, rate.NewLimiter(1, 1))

	if _, err := client.GetLatestLedger(context.Background()); err != nil {
		t.Fatalf("first call: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.GetLatestLedger(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if got := inner.count(); got != 1 {
		t.Errorf("inner RPC calls = %d, want 1 (a cancelled wait must not reach the RPC)", got)
	}
}
