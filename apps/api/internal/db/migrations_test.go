// Package db has no Go code of its own: migrations/ holds only SQL. This
// package exists to hold the guards that keep that directory loadable and to
// verify that the trigram indexes backing label and contract search are
// actually used by the query planner.
//
// Every deployment path in this repository (CI, docker-compose dev and prod,
// the integration compose file) applies migrations by looping over
// `ls migrations/*.up.sql | sort` with psql, and `make migrate-up` hands the
// same directory to golang-migrate. golang-migrate refuses to read a directory
// where two files share a version, and it then reports the *first* collision
// it finds, so a duplicated version number both breaks `make migrate-up` and
// hides every other collision behind it. That happened for real: 000002 landed
// twice, then 000009 landed eight times (issue #516).
//
// psql does not care about duplicate versions, so nothing in CI noticed. This
// test is the check CI was missing; it runs as part of `go test ./...`.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

// migrationFile is "<six digits>_<snake_case_name>.(up|down).sql".
var migrationFile = regexp.MustCompile(`^(\d{6})_([a-z0-9][a-z0-9_]*)\.(up|down)\.sql$`)

const migrationsDir = "migrations"

// trigramIndexes lists the GIN trigram indexes that must exist for the ILIKE
// substring searches in store/postgres.go to avoid full table scans. Each entry
// maps a table to the index name and the column it covers.
var trigramIndexes = []struct {
	table  string
	index  string
	column string
}{
	{"labels", "idx_labels_name_trgm", "name"},
	{"contracts", "idx_contracts_name_trgm", "name"},
}

// TestMigrationVersionsAreUniqueAndContiguous fails when two migrations share a
// version, when a version is missing from the sequence, or when an up/down pair
// is incomplete.
func TestMigrationVersionsAreUniqueAndContiguous(t *testing.T) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read %s: %v (the guard must run against the real migrations directory)", migrationsDir, err)
	}

	// version -> direction -> file names that claim it
	byVersion := map[int]map[string][]string{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		m := migrationFile.FindStringSubmatch(name)
		if m == nil {
			t.Errorf("%s: not a valid golang-migrate file name; want <00000N>_<snake_case>.{up,down}.sql", name)
			continue
		}
		version, err := strconv.Atoi(m[1])
		if err != nil {
			t.Errorf("%s: unreadable version %q: %v", name, m[1], err)
			continue
		}
		if byVersion[version] == nil {
			byVersion[version] = map[string][]string{}
		}
		byVersion[version][m[3]] = append(byVersion[version][m[3]], name)
	}

	versions := make([]int, 0, len(byVersion))
	for version := range byVersion {
		versions = append(versions, version)
	}
	sort.Ints(versions)

	for _, version := range versions {
		for direction, files := range byVersion[version] {
			if len(files) > 1 {
				sort.Strings(files)
				t.Errorf("duplicate migration version %06d (%s): %v shares it; "+
					"golang-migrate cannot read the directory and `make migrate-up` fails (issue #516). "+
					"Renumber the later file to the next free version, keeping the files in the order "+
					"they must be applied.", version, direction, files)
			}
		}
		if len(byVersion[version]["up"]) == 1 && len(byVersion[version]["down"]) != 1 {
			t.Errorf("migration %06d has %s but no single matching down file", version, byVersion[version]["up"])
		}
	}

	// The sequence must run 1..N with no gaps. `make migrate-new` derives the
	// next version from the highest one present, so a gap means the highest
	// version and the file count disagree and the tooling stops agreeing about
	// what the next migration is called.
	if len(versions) > 0 {
		if versions[0] != 1 {
			t.Errorf("migration sequence starts at %06d, want 000001", versions[0])
		}
		for i, version := range versions {
			if want := i + 1; version != want {
				t.Errorf("migration sequence has a gap: %06d follows %06d, want %06d", version, versions[i-1], want)
			}
		}
		if got, want := len(versions), versions[len(versions)-1]; got != want {
			t.Errorf("%d distinct versions but the highest is %06d; the sequence is not contiguous", got, want)
		}
	}
}

// TestTrigramIndexesExist fails when the pg_trgm extension or one of the GIN
// trigram indexes backing label and contract search is missing from the
// migrations. Without them the ILIKE '%term%' queries in store/postgres.go do
// full table scans.
func TestTrigramIndexesExist(t *testing.T) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read %s: %v", migrationsDir, err)
	}

	var upSQL strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		body, err := os.ReadFile(migrationsDir + "/" + entry.Name())
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		upSQL.Write(body)
		upSQL.WriteByte('\n')
	}

	all := strings.ToLower(upSQL.String())
	if !strings.Contains(all, "create extension if not exists pg_trgm") &&
		!strings.Contains(all, "create extension pg_trgm") {
		t.Errorf("no migration enables the pg_trgm extension; substring search cannot use a trigram index")
	}

	for _, idx := range trigramIndexes {
		if !strings.Contains(all, idx.index) {
			t.Errorf("no migration creates trigram index %q on %s(%s)", idx.index, idx.table, idx.column)
		}
	}
}

// TestTrigramIndexesUsedByPlanner is an EXPLAIN-based check that the planner
// picks the trigram index for the ILIKE substring searches. It needs a live
// Postgres with the migrations applied and is skipped unless
// SOROLENS_TEST_DATABASE_URL is set (CI sets it for the integration job).
func TestTrigramIndexesUsedByPlanner(t *testing.T) {
	dsn := os.Getenv("SOROLENS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SOROLENS_TEST_DATABASE_URL not set; skipping EXPLAIN-based index check")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	for _, idx := range trigramIndexes {
		// Seed enough rows that the planner prefers the index over a seq scan.
		seed := fmt.Sprintf(
			"INSERT INTO %s (%s) SELECT 'seed-' || g FROM generate_series(1, 5000) g ON CONFLICT DO NOTHING",
			idx.table, idx.column)
		if _, err := db.Exec(seed); err != nil {
			t.Fatalf("seed %s: %v", idx.table, err)
		}

		query := fmt.Sprintf("EXPLAIN SELECT * FROM %s WHERE %s ILIKE '%%seed-42%%'", idx.table, idx.column)
		rows, err := db.Query(query)
		if err != nil {
			t.Fatalf("explain %s: %v", idx.table, err)
		}
		var plan strings.Builder
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				rows.Close()
				t.Fatalf("scan plan: %v", err)
			}
			plan.WriteString(line)
			plan.WriteByte('\n')
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatalf("explain rows: %v", err)
		}

		if !strings.Contains(plan.String(), idx.index) {
			t.Errorf("query plan for %s(%s) does not use %s:\n%s", idx.table, idx.column, idx.index, plan.String())
		}
	}
}

// TestMigrationsApplyInSortedOrder documents the contract every deployment
// path relies on: the order golang-migrate applies migrations in (version
// order) must equal the order the psql loops apply them in (sorted file name).
// A renumbering that preserves the sorted order therefore cannot change which
// statement runs first.
func TestMigrationsApplyInSortedOrder(t *testing.T) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read %s: %v", migrationsDir, err)
	}
	var ups []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			ups = append(ups, entry.Name())
		}
	}
	sorted := append([]string(nil), ups...)
	sort.Strings(sorted)

	// Version order: sort by the leading number only.
	byVersion := append([]string(nil), ups...)
	sort.Slice(byVersion, func(i, j int) bool {
		vi, _ := strconv.Atoi(byVersion[i][:6])
		vj, _ := strconv.Atoi(byVersion[j][:6])
		return vi < vj
	})

	if fmt.Sprint(sorted) != fmt.Sprint(byVersion) {
		t.Errorf("sorted file order and version order disagree:\n sorted: %v\n version: %v", sorted, byVersion)
	}
}
