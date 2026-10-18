// Command seedload populates the Sorolens Postgres database with synthetic but
// schema-valid data so the k6 load-test suite (tests/load) has something to
// hammer. It is the seed generator from issue #551.
//
// It is deterministic: with the same -seed flag every run produces identical
// ids, ledgers and timestamps, so smoke runs are reproducible and comparable
// across CI jobs. Row counts default to the "~1M events" target in the issue,
// but are overridable so a CI smoke pass can seed a few thousand rows in
// seconds. math/rand (not crypto/rand) is used deliberately — this is
// reproducible fixture data, not security-sensitive material.
//
// Usage:
//
//	DATABASE_URL=postgres://sorolens:sorolens@localhost:5432/sorolens?sslmode=disable \
//	go run ./cmd/seedload -events 1000000 -invocations 250000 -contracts 50 -seed 42
//
// Columns and their NOT NULL requirements follow apps/api/internal/db/
// migrations/000001_init.up.sql (contracts, events, invocations).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// Contract ids are Stellar account-style: "C" followed by 55 alphanumeric
	// characters (the API pattern is ^C[A-Za-z0-9]{55}$).
	contractIDChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	contractIDLen   = 55
	networks        = "testnet,mainnet,futurenet,standalone"
	eventTypes      = "transfer,mint,burn,approve,swap,withdraw,deposit,governance"
	functionNames   = "transfer,mint,burn,approve,swap,withdraw,deposit,governance,upgrade,set_admin"
	invocStatuses   = "SUCCESS,FAILED,NOT_FOUND"
)

func main() {
	var (
		events      = flag.Int("events", 1_000_000, "number of event rows to generate")
		invocations = flag.Int("invocations", 250_000, "number of invocation rows to generate")
		contracts   = flag.Int("contracts", 50, "number of contracts to create (events/invocations are spread across them)")
		seed        = flag.Int64("seed", 42, "deterministic RNG seed")
		batch       = flag.Int("batch", 2000, "rows per multi-VALUES insert")
		startLedger = flag.Int64("start-ledger", 50_000_000, "ledger height at which generated rows begin")
	)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}
	if *events < 0 || *invocations < 0 || *contracts <= 0 || *batch <= 0 {
		logger.Error("invalid flags", "events", *events, "invocations", *invocations,
			"contracts", *contracts, "batch", *batch)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Error("postgres connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		logger.Error("postgres ping", "err", err)
		os.Exit(1)
	}

	rng := rand.New(rand.NewSource(*seed))
	start := time.Now()
	base := time.Now().UTC().Add(-24 * time.Hour)

	contractIDs := buildContracts(rng, *contracts)
	if err := insertContracts(ctx, pool, contractIDs, rng, logger); err != nil {
		logger.Error("seed contracts", "err", err)
		os.Exit(1)
	}
	if err := insertEvents(ctx, pool, contractIDs, *events, *batch, *startLedger, base, rng, logger); err != nil {
		logger.Error("seed events", "err", err)
		os.Exit(1)
	}
	if err := insertInvocations(ctx, pool, contractIDs, *invocations, *batch, *startLedger, base, rng, logger); err != nil {
		logger.Error("seed invocations", "err", err)
		os.Exit(1)
	}

	logger.Info("seed complete",
		"contracts", len(contractIDs),
		"events", *events,
		"invocations", *invocations,
		"seed", *seed,
		"duration", time.Since(start).String(),
	)
}

// pick chooses one element from a comma-separated constant list.
func pick(rng *rand.Rand, csv string) string {
	items := strings.Split(csv, ",")
	return items[rng.Intn(len(items))]
}

// buildContracts deterministically mints unique contract ids.
func buildContracts(rng *rand.Rand, n int) []string {
	ids := make([]string, 0, n)
	seen := make(map[string]struct{}, n)
	for len(ids) < n {
		var b strings.Builder
		b.WriteByte('C')
		for i := 0; i < contractIDLen; i++ {
			b.WriteByte(contractIDChars[rng.Intn(len(contractIDChars))])
		}
		id := b.String()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func insertContracts(ctx context.Context, pool *pgxpool.Pool, ids []string, rng *rand.Rand, logger *slog.Logger) error {
	const sql = `INSERT INTO contracts (id, network, label, status, created_at_ledger)
VALUES %s
ON CONFLICT (id) DO NOTHING`
	for start := 0; start < len(ids); start += 200 {
		end := min(start+200, len(ids))
		chunk := ids[start:end]
		var sb strings.Builder
		args := make([]any, 0, len(chunk)*5)
		for i, id := range chunk {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(tuple(i*5, 5, nil))
			n := start + i
			args = append(args, id, pick(rng, networks), fmt.Sprintf("load-contract-%d", n), "active", int64(50_000_000+n))
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(sql, sb.String()), args...); err != nil {
			return fmt.Errorf("insert contracts batch %d-%d: %w", start, end, err)
		}
	}
	logger.Info("contracts seeded", "count", len(ids))
	return nil
}

func insertEvents(ctx context.Context, pool *pgxpool.Pool, contractIDs []string, total, batch int, startLedger int64, base time.Time, rng *rand.Rand, logger *slog.Logger) error {
	const cols = "(id, contract_id, ledger, ledger_closed_at, tx_hash, type, topic_xdr, value_xdr, in_successful_call)"
	const sql = `INSERT INTO events ` + cols + `
VALUES %s
ON CONFLICT (id) DO NOTHING`
	if len(contractIDs) == 0 {
		return errors.New("no contracts to attach events to")
	}
	// Column 6 (0-based) of each row is the jsonb topic_xdr and needs a cast.
	jsonbCols := map[int]string{6: "jsonb"}
	for done := 0; done < total; done += batch {
		n := min(batch, total-done)
		var sb strings.Builder
		args := make([]any, 0, n*9)
		for i := 0; i < n; i++ {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(tuple(i*9, 9, jsonbCols))
			row := int64(done + i)
			args = append(args,
				fmt.Sprintf("evt-%d", row),
				contractIDs[rng.Intn(len(contractIDs))],
				startLedger+row,
				base.Add(time.Duration(row)*time.Second),
				randHex(rng, 64),
				pick(rng, eventTypes),
				`["AAAA","AAAAA"]`, // topic_xdr (jsonb)
				`AgAAAAQAAAAZ`,     // value_xdr (text)
				true,               // in_successful_call
			)
		}
		query := fmt.Sprintf(sql, sb.String())
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("insert events at row %d: %w", done, err)
		}
		if (done/batch)%25 == 0 && done > 0 {
			logger.Info("events seeded", "rows", done+n, "of", total)
		}
	}
	logger.Info("events seeded", "count", total)
	return nil
}

func insertInvocations(ctx context.Context, pool *pgxpool.Pool, contractIDs []string, total, batch int, startLedger int64, base time.Time, rng *rand.Rand, logger *slog.Logger) error {
	const cols = "(tx_hash, contract_id, ledger, ledger_closed_at, status, function_name, resource_fee_charged, cpu_insn, mem_byte)"
	const sql = `INSERT INTO invocations ` + cols + `
VALUES %s
ON CONFLICT (tx_hash) DO NOTHING`
	if len(contractIDs) == 0 {
		return errors.New("no contracts to attach invocations to")
	}
	for done := 0; done < total; done += batch {
		n := min(batch, total-done)
		var sb strings.Builder
		args := make([]any, 0, n*9)
		for i := 0; i < n; i++ {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(tuple(i*9, 9, nil))
			row := int64(done + i)
			args = append(args,
				randHex(rng, 64),
				contractIDs[rng.Intn(len(contractIDs))],
				startLedger+row,
				base.Add(time.Duration(row)*time.Second),
				pick(rng, invocStatuses),
				pick(rng, functionNames),
				rng.Int63n(100_000),
				rng.Int63n(5_000_000),
				rng.Int63n(500_000),
			)
		}
		query := fmt.Sprintf(sql, sb.String())
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("insert invocations at row %d: %w", done, err)
		}
		if (done/batch)%25 == 0 && done > 0 {
			logger.Info("invocations seeded", "rows", done+n, "of", total)
		}
	}
	logger.Info("invocations seeded", "count", total)
	return nil
}

// tuple renders a ($offset+1,...,$offset+count) VALUES group, adding a Postgres
// type cast to any column index present in the casts map.
func tuple(offset, count int, casts map[int]string) string {
	parts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		p := fmt.Sprintf("$%d", offset+i+1)
		if cast, ok := casts[i]; ok {
			p += "::" + cast
		}
		parts = append(parts, p)
	}
	return "(" + strings.Join(parts, ",") + ")"
}

// randHex returns a lowercase hex string of the requested length.
func randHex(rng *rand.Rand, length int) string {
	const digits = "0123456789abcdef"
	var b strings.Builder
	b.Grow(length)
	for i := 0; i < length; i++ {
		b.WriteByte(digits[rng.Intn(16)])
	}
	return b.String()
}
