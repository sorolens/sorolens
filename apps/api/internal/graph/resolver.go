// Package graph serves the read-only GraphQL API at /graphql (issue #125).
//
// The schema (schema.graphqls) is schema-first gqlgen: generated.go and
// models_gen.go are generated, schema.resolvers.go holds the resolver bodies.
// Resolvers only call existing store methods; relationship fields go through
// the per-request dataloaders in loaders.go.
package graph

//go:generate go tool gqlgen generate --config gqlgen.yml

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Store is the read surface the resolvers need. handler.APIStore and
// store.FullStore both satisfy it.
type Store interface {
	GetContract(ctx context.Context, contractID string) (store.Contract, error)
	ListContracts(ctx context.Context, cursor string, limit int, f store.ContractFilters) ([]store.Contract, string, error)
	ListEvents(ctx context.Context, contractID, cursor string, limit int, f store.EventFilters) ([]store.Event, string, error)
	ListInvocations(ctx context.Context, contractID, cursor string, limit int, f store.InvocationFilters) ([]store.Invocation, string, error)
	ListStorageEntries(ctx context.Context, contractID, cursor string, limit int, f store.StorageFilters) ([]store.StorageEntry, string, error)
	GetContractStats(ctx context.Context, contractID, window string) (store.ContractStats, error)
	GetMonitoredContract(ctx context.Context, contractID string) (store.MonitoredContract, error)
	ListMonitoredContracts(ctx context.Context, cursor string, limit int, network string) ([]store.MonitoredContract, string, error)
	ListHealthChecks(ctx context.Context, contractID string, limit int) ([]store.HealthCheck, error)
	ListAlerts(ctx context.Context, contractID, severity, network string, limit int) ([]store.ContractAlert, error)
	GetWatchdogStats(ctx context.Context, network string) (store.WatchdogStats, error)
}

// Resolver is the gqlgen resolver root.
type Resolver struct {
	Store Store
}

// Watchdog is the root of the watchdog query namespace. Its fields take
// arguments, so they are all resolved on demand.
type Watchdog struct{}

// MaxListLimit caps every `first` argument, mirroring the REST page caps.
const MaxListLimit = 100

// limit resolves an optional `first` argument to a value in [1, MaxListLimit].
func limit(first *int, def int) int {
	if first == nil || *first < 1 {
		return def
	}
	if *first > MaxListLimit {
		return MaxListLimit
	}
	return *first
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Cursors use the same opaque base64 encoding as the REST API.

func encodeCursor(raw string) *string {
	if raw == "" {
		return nil
	}
	c := base64.StdEncoding.EncodeToString([]byte(raw))
	return &c
}

func decodeCursor(encoded *string) (string, error) {
	if encoded == nil || *encoded == "" {
		return "", nil
	}
	b, err := base64.StdEncoding.DecodeString(*encoded)
	if err != nil {
		return "", fmt.Errorf("invalid cursor")
	}
	return string(b), nil
}

var validNetworks = map[string]bool{
	"testnet":    true,
	"mainnet":    true,
	"futurenet":  true,
	"standalone": true,
}

// resolveNetwork validates an optional network filter ("" and "all" mean any).
func resolveNetwork(n *string) (string, error) {
	v := deref(n)
	if v == "" || v == "all" {
		return "", nil
	}
	if !validNetworks[v] {
		return "", fmt.Errorf("network must be one of: testnet, mainnet, futurenet, standalone")
	}
	return v, nil
}
