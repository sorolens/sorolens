package graph

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// DefaultComplexityLimit is the default per-operation complexity budget. List
// fields cost their child complexity times their page size (see
// complexityRoot), so e.g. contracts(first: 50) { events(first: 20) { type } }
// costs about 1,000.
const DefaultComplexityLimit = 5000

// Options configures the GraphQL handler.
type Options struct {
	// ComplexityLimit rejects operations whose estimated cost exceeds it.
	// Zero means DefaultComplexityLimit.
	ComplexityLimit int
	// PersistedOnly rejects any operation that is not in the embedded
	// allowlist (persisted/*.graphql) and disables introspection. Clients
	// then send only {"extensions":{"persistedQuery":{"version":1,
	// "sha256Hash":"<hash>"}}}.
	PersistedOnly bool
}

//go:embed persisted/*.graphql
var persistedFS embed.FS

// PersistedQueries returns the embedded allowlist keyed by the SHA-256 hex of
// each query's exact text, which is the automatic persisted query hash.
func PersistedQueries() (map[string]string, error) {
	out := map[string]string{}
	err := fs.WalkDir(persistedFS, "persisted", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := persistedFS.ReadFile(path)
		if err != nil {
			return err
		}
		out[queryHash(string(b))] = string(b)
		return nil
	})
	return out, err
}

func queryHash(q string) string {
	sum := sha256.Sum256([]byte(q))
	return hex.EncodeToString(sum[:])
}

// NewHandler returns the /graphql HTTP handler: POST-only, with per-request
// dataloaders, automatic persisted queries, and a complexity limit.
func NewHandler(s Store, opts Options) (http.Handler, error) {
	limit := opts.ComplexityLimit
	if limit <= 0 {
		limit = DefaultComplexityLimit
	}

	srv := handler.New(NewExecutableSchema(Config{
		Resolvers:  &Resolver{Store: s},
		Complexity: complexityRoot(),
	}))
	srv.AddTransport(transport.POST{})
	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	allowlist, err := PersistedQueries()
	if err != nil {
		return nil, fmt.Errorf("load persisted queries: %w", err)
	}
	apqCache := lru.New[string](1000)
	for hash, q := range allowlist {
		apqCache.Add(context.Background(), hash, q)
	}
	srv.Use(extension.AutomaticPersistedQuery{Cache: apqCache})
	srv.Use(extension.FixedComplexityLimit(limit))

	if opts.PersistedOnly {
		srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
			oc := graphql.GetOperationContext(ctx)
			if _, ok := allowlist[queryHash(oc.RawQuery)]; !ok {
				return graphql.OneShot(&graphql.Response{Errors: gqlerror.List{{
					Message:    "only persisted queries are allowed",
					Extensions: map[string]any{"code": "PERSISTED_QUERY_REQUIRED"},
				}}})
			}
			return next(ctx)
		})
	} else {
		srv.Use(extension.Introspection{})
	}

	return withLoaders(s, srv), nil
}

// listCost is the complexity of a list field: its per-item child cost times
// the page size it will actually fetch.
func listCost(childComplexity int, first *int, def int) int {
	return 1 + childComplexity*limit(first, def)
}

// complexityRoot prices list fields by page size so wide or deeply nested
// queries hit the complexity limit instead of the database.
func complexityRoot() ComplexityRoot {
	var c ComplexityRoot
	c.Query.Contracts = func(child int, _ *string, _ *string, first *int, _ *string) int {
		return listCost(child, first, 50)
	}
	c.Contract.Events = func(child int, first *int, _ *string) int { return listCost(child, first, 20) }
	c.Contract.Invocations = func(child int, first *int, _ *string, _ *string) int {
		return listCost(child, first, 20)
	}
	c.Contract.Storage = func(child int, first *int, _ *string, _ *string) int {
		return listCost(child, first, 20)
	}
	c.Contract.Alerts = func(child int, first *int, _ *string) int { return listCost(child, first, 20) }
	c.MonitoredContract.HealthChecks = func(child int, first *int) int { return listCost(child, first, 20) }
	c.MonitoredContract.Alerts = func(child int, first *int, _ *string) int { return listCost(child, first, 20) }
	c.Watchdog.Alerts = func(child int, _ *string, _ *string, first *int) int { return listCost(child, first, 50) }
	c.Watchdog.Contracts = func(child int, _ *string, first *int, _ *string) int {
		return listCost(child, first, 50)
	}
	return c
}
