package graph

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
	"github.com/vikstrous/dataloadgen"
)

// loaderWait is how long a loader collects keys before fetching. Sibling
// resolvers run concurrently, so a list of N parents lands in one batch.
const loaderWait = 2 * time.Millisecond

// maxConcurrentFetches bounds how many store calls one batch runs in
// parallel, so a wide query cannot exhaust the connection pool.
const maxConcurrentFetches = 8

type statsKey struct{ ContractID, Window string }

type eventsKey struct {
	ContractID, Type string
	Limit            int
}

type invocationsKey struct {
	ContractID, Status, FunctionName string
	Limit                            int
}

type storageKey struct {
	ContractID, Durability, Status string
	Limit                          int
}

type alertsKey struct {
	ContractID, Severity string
	Limit                int
}

type healthChecksKey struct {
	ContractID string
	Limit      int
}

// Loaders batches and caches store reads for the lifetime of one request, so
// resolving the same relationship for N parents costs one batch (and one
// store call per distinct key) instead of N sequential round trips.
//
// The store has no multi-key read methods, and this layer deliberately adds
// no new DB code, so each batch fans out to the existing single-key methods
// with bounded concurrency. The batch functions are the single place to
// swap in a multi-key query later.
type Loaders struct {
	Contract     *dataloadgen.Loader[string, *store.Contract]
	Monitor      *dataloadgen.Loader[string, *store.MonitoredContract]
	Stats        *dataloadgen.Loader[statsKey, store.ContractStats]
	Events       *dataloadgen.Loader[eventsKey, []store.Event]
	Invocations  *dataloadgen.Loader[invocationsKey, []store.Invocation]
	Storage      *dataloadgen.Loader[storageKey, []store.StorageEntry]
	Alerts       *dataloadgen.Loader[alertsKey, []store.ContractAlert]
	HealthChecks *dataloadgen.Loader[healthChecksKey, []store.HealthCheck]
}

// NewLoaders returns a fresh set of loaders backed by s.
func NewLoaders(s Store) *Loaders {
	return &Loaders{
		Contract: newLoader(func(ctx context.Context, id string) (*store.Contract, error) {
			c, err := s.GetContract(ctx, id)
			if errors.Is(err, store.ErrNotFound) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			return &c, nil
		}),
		Monitor: newLoader(func(ctx context.Context, id string) (*store.MonitoredContract, error) {
			m, err := s.GetMonitoredContract(ctx, id)
			if errors.Is(err, store.ErrNotFound) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			return &m, nil
		}),
		Stats: newLoader(func(ctx context.Context, k statsKey) (store.ContractStats, error) {
			return s.GetContractStats(ctx, k.ContractID, k.Window)
		}),
		Events: newLoader(func(ctx context.Context, k eventsKey) ([]store.Event, error) {
			events, _, err := s.ListEvents(ctx, k.ContractID, "", k.Limit, store.EventFilters{Type: k.Type})
			return events, err
		}),
		Invocations: newLoader(func(ctx context.Context, k invocationsKey) ([]store.Invocation, error) {
			invs, _, err := s.ListInvocations(ctx, k.ContractID, "", k.Limit, store.InvocationFilters{
				Status:       k.Status,
				FunctionName: k.FunctionName,
			})
			return invs, err
		}),
		Storage: newLoader(func(ctx context.Context, k storageKey) ([]store.StorageEntry, error) {
			entries, _, err := s.ListStorageEntries(ctx, k.ContractID, "", k.Limit, store.StorageFilters{
				Durability: k.Durability,
				Status:     k.Status,
			})
			return entries, err
		}),
		Alerts: newLoader(func(ctx context.Context, k alertsKey) ([]store.ContractAlert, error) {
			return s.ListAlerts(ctx, k.ContractID, k.Severity, "", k.Limit)
		}),
		HealthChecks: newLoader(func(ctx context.Context, k healthChecksKey) ([]store.HealthCheck, error) {
			return s.ListHealthChecks(ctx, k.ContractID, k.Limit)
		}),
	}
}

// newLoader builds a batching loader from a single-key fetch.
func newLoader[K comparable, V any](fetch func(ctx context.Context, key K) (V, error)) *dataloadgen.Loader[K, V] {
	return dataloadgen.NewLoader(func(ctx context.Context, keys []K) ([]V, []error) {
		vals := make([]V, len(keys))
		errs := make([]error, len(keys))
		sem := make(chan struct{}, maxConcurrentFetches)
		var wg sync.WaitGroup
		for i, k := range keys {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				vals[i], errs[i] = fetch(ctx, k)
			}()
		}
		wg.Wait()
		return vals, errs
	}, dataloadgen.WithWait(loaderWait))
}

type loadersKey struct{}

// For returns the request's loaders. It panics if the loader middleware was
// not installed, which is a wiring bug.
func For(ctx context.Context) *Loaders {
	return ctx.Value(loadersKey{}).(*Loaders)
}

// withLoaders installs a fresh set of loaders on every request so cached
// values never leak across requests.
func withLoaders(s Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), loadersKey{}, NewLoaders(s))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
