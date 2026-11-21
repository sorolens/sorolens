package db

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// This file lints up migrations against the zero-downtime policy in
// docs/migrations.md (issue #845). It runs inside the existing Go CI job as
// part of `go test ./...`; `make migrate-lint` runs it on its own.
//
// A rule is skipped for a table the same migration creates: a brand-new table
// is empty and unused, so rewriting or locking it is harmless.
//
// A violation that is genuinely safe (or already applied everywhere) is
// accepted only through migrationLintAllowlist, one line per finding:
//
//	<file> <rule> <reason>
//
// The reason is required, so every exception is explained where reviewers see
// it, and applied migrations never need to be edited.

const migrationLintAllowlist = "migration_lint_allowlist.txt"

type migrationRule struct {
	id      string
	pattern *regexp.Regexp
	// skip reports whether a statement matching pattern is acceptable anyway.
	skip   func(stmt string) bool
	reason string
	safe   string
}

var (
	createTablePattern = regexp.MustCompile(`\bCREATE\s+(?:UNLOGGED\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([\w."]+)`)
	alterTableTarget   = regexp.MustCompile(`^ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?([\w."]+)`)
	indexTarget        = regexp.MustCompile(`\bON\s+(?:ONLY\s+)?([\w."]+)`)
	dollarQuoted       = regexp.MustCompile(`(?s)\$(\w*)\$.*?\$(\w*)\$`)
	blockComment       = regexp.MustCompile(`(?s)/\*.*?\*/`)
	lineComment        = regexp.MustCompile(`--[^\n]*`)
	whitespace         = regexp.MustCompile(`\s+`)
)

var migrationRules = []migrationRule{
	{
		id:      "index-not-concurrent",
		pattern: regexp.MustCompile(`^CREATE\s+(?:UNIQUE\s+)?INDEX\b`),
		skip:    func(stmt string) bool { return strings.Contains(stmt, " CONCURRENTLY ") },
		reason:  "CREATE INDEX without CONCURRENTLY blocks writes to the table for the whole build",
		safe:    "use CREATE INDEX CONCURRENTLY IF NOT EXISTS, alone in its own migration",
	},
	{
		id:      "add-column-not-null-without-default",
		pattern: regexp.MustCompile(`\bADD\s+(?:COLUMN\s+)?(?:IF\s+NOT\s+EXISTS\s+)?[\w"]+\s+[^,]*\bNOT\s+NULL\b`),
		skip: func(stmt string) bool {
			return regexp.MustCompile(`\bADD\s+(?:COLUMN\s+)?(?:IF\s+NOT\s+EXISTS\s+)?[\w"]+\s+[^,]*\bDEFAULT\b`).MatchString(stmt)
		},
		reason: "adding a NOT NULL column without a DEFAULT fails on a non-empty table",
		safe:   "add the column nullable or with a constant DEFAULT, backfill in batches, then add the constraint",
	},
	{
		id:      "alter-column-type",
		pattern: regexp.MustCompile(`\bALTER\s+(?:COLUMN\s+)?[\w"]+\s+(?:SET\s+DATA\s+)?TYPE\b`),
		reason:  "changing a column type rewrites the table under an ACCESS EXCLUSIVE lock",
		safe:    "expand and contract: add a new column, dual-write and backfill, switch reads, drop the old column later",
	},
	{
		id:      "set-not-null",
		pattern: regexp.MustCompile(`\bALTER\s+(?:COLUMN\s+)?[\w"]+\s+SET\s+NOT\s+NULL\b`),
		reason:  "SET NOT NULL scans the whole table while holding an ACCESS EXCLUSIVE lock",
		safe:    "add CHECK (col IS NOT NULL) NOT VALID, VALIDATE CONSTRAINT in a later migration, then SET NOT NULL",
	},
	{
		id:      "constraint-without-not-valid",
		pattern: regexp.MustCompile(`\bADD\s+(?:CONSTRAINT\s+[\w"]+\s+)?(?:FOREIGN\s+KEY|CHECK)\b`),
		skip:    func(stmt string) bool { return strings.Contains(stmt, " NOT VALID") },
		reason:  "adding a CHECK or FOREIGN KEY constraint validates every row while blocking writes",
		safe:    "add the constraint NOT VALID, then VALIDATE CONSTRAINT in a separate migration",
	},
	{
		id:      "rename",
		pattern: regexp.MustCompile(`^ALTER\s+TABLE\b.*\bRENAME\b`),
		reason:  "renaming a table or column breaks every running instance that still uses the old name",
		safe:    "expand and contract: add the new name, migrate readers and writers, drop the old name in a later release",
	},
	{
		id:      "drop-column",
		pattern: regexp.MustCompile(`^ALTER\s+TABLE\b.*\bDROP\s+(?:COLUMN\s+)?(?:IF\s+EXISTS\s+)?[\w"]+`),
		skip: func(stmt string) bool {
			return regexp.MustCompile(`\bDROP\s+(?:CONSTRAINT|DEFAULT|NOT\s+NULL|EXPRESSION|IDENTITY)\b`).MatchString(stmt)
		},
		reason: "dropping a column breaks running instances that still read or write it",
		safe:   "stop using the column in a release first, then drop it in a later contract migration",
	},
	{
		id:      "full-table-lock",
		pattern: regexp.MustCompile(`^(?:VACUUM\s+FULL|CLUSTER|LOCK\s+TABLE)\b`),
		reason:  "VACUUM FULL, CLUSTER and LOCK TABLE hold an ACCESS EXCLUSIVE lock on the table",
		safe:    "use plain VACUUM, pg_repack, or redesign the migration to avoid an explicit lock",
	},
}

type migrationFinding struct {
	file, rule, statement string
}

// sqlStatements returns the upper-cased, whitespace-normalized statements of
// a migration, with comments and dollar-quoted bodies removed.
func sqlStatements(sql string) []string {
	sql = dollarQuoted.ReplaceAllString(sql, "''")
	sql = blockComment.ReplaceAllString(sql, " ")
	sql = lineComment.ReplaceAllString(sql, " ")
	var out []string
	for _, stmt := range strings.Split(sql, ";") {
		stmt = strings.TrimSpace(whitespace.ReplaceAllString(strings.ToUpper(stmt), " "))
		if stmt != "" {
			out = append(out, stmt)
		}
	}
	return out
}

func normalizeTable(name string) string {
	name = strings.Trim(strings.ToLower(name), `"`)
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = strings.Trim(name[i+1:], `"`)
	}
	return name
}

// statementTable returns the table an ALTER TABLE or CREATE INDEX targets.
func statementTable(stmt string) string {
	if m := alterTableTarget.FindStringSubmatch(stmt); m != nil {
		return normalizeTable(m[1])
	}
	if strings.HasPrefix(stmt, "CREATE ") && strings.Contains(stmt, " INDEX ") {
		if m := indexTarget.FindStringSubmatch(stmt); m != nil {
			return normalizeTable(m[1])
		}
	}
	return ""
}

// lintMigration returns every rule violation in one up migration's SQL.
func lintMigration(file, sql string) []migrationFinding {
	stmts := sqlStatements(sql)
	created := map[string]bool{}
	for _, stmt := range stmts {
		for _, m := range createTablePattern.FindAllStringSubmatch(stmt, -1) {
			created[normalizeTable(m[1])] = true
		}
	}

	var findings []migrationFinding
	for _, stmt := range stmts {
		if table := statementTable(stmt); table != "" && created[table] {
			continue
		}
		for _, rule := range migrationRules {
			if !rule.pattern.MatchString(stmt) || (rule.skip != nil && rule.skip(stmt)) {
				continue
			}
			findings = append(findings, migrationFinding{file: file, rule: rule.id, statement: stmt})
		}
	}
	return findings
}

// loadMigrationAllowlist reads "<file> <rule> <reason>" lines. Blank lines
// and lines starting with # are ignored; a line without a reason is an error.
func loadMigrationAllowlist(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open(migrationLintAllowlist)
	if err != nil {
		t.Fatalf("open %s: %v", migrationLintAllowlist, err)
	}
	defer f.Close()

	allowed := map[string]string{}
	scanner := bufio.NewScanner(f)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) < 3 {
			t.Errorf("%s:%d: want \"<file> <rule> <reason>\"; every exception needs a reason", migrationLintAllowlist, line)
			continue
		}
		allowed[fields[0]+" "+fields[1]] = strings.Join(fields[2:], " ")
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read %s: %v", migrationLintAllowlist, err)
	}
	return allowed
}

// TestMigrationLint fails when an up migration breaks the zero-downtime
// migration policy without an allow-list entry explaining why.
func TestMigrationLint(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatalf("no up migrations found in %s", migrationsDir)
	}

	rules := map[string]migrationRule{}
	for _, r := range migrationRules {
		rules[r.id] = r
	}

	allowed := loadMigrationAllowlist(t)
	used := map[string]bool{}
	for _, path := range files {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, f := range lintMigration(filepath.Base(path), string(sql)) {
			key := f.file + " " + f.rule
			if _, ok := allowed[key]; ok {
				used[key] = true
				continue
			}
			r := rules[f.rule]
			stmt := f.statement
			if len(stmt) > 120 {
				stmt = stmt[:120] + "..."
			}
			t.Errorf("%s: unsafe migration (%s): %s.\n  statement: %s\n  safe alternative: %s.\n  See docs/migrations.md. If this is safe here, add\n    %s %s <reason>\n  to apps/api/internal/db/%s.",
				f.file, f.rule, r.reason, stmt, r.safe, f.file, f.rule, migrationLintAllowlist)
		}
	}

	for key := range allowed {
		if !used[key] {
			t.Errorf("%s: stale entry %q matches no current finding; remove it", migrationLintAllowlist, key)
		}
	}
}

func TestLintMigrationRules(t *testing.T) {
	cases := []struct {
		name, sql string
		want      []string
	}{
		{"plain index", "CREATE INDEX events_ledger_idx ON events (ledger);", []string{"index-not-concurrent"}},
		{"concurrent index", "CREATE INDEX CONCURRENTLY IF NOT EXISTS events_ledger_idx ON events (ledger);", nil},
		{"index on new table", "CREATE TABLE t (id int); CREATE INDEX t_idx ON t (id);", nil},
		{"not null without default", "ALTER TABLE events ADD COLUMN kind text NOT NULL;", []string{"add-column-not-null-without-default"}},
		{"not null with default", "ALTER TABLE events ADD COLUMN kind text NOT NULL DEFAULT 'x';", nil},
		{"nullable column", "ALTER TABLE events ADD COLUMN kind text;", nil},
		{"type change", "ALTER TABLE events ALTER COLUMN ledger TYPE bigint;", []string{"alter-column-type"}},
		{"set not null", "ALTER TABLE events ALTER COLUMN ledger SET NOT NULL;", []string{"set-not-null"}},
		{"fk", "ALTER TABLE events ADD CONSTRAINT fk FOREIGN KEY (c) REFERENCES contracts (id);", []string{"constraint-without-not-valid"}},
		{"fk not valid", "ALTER TABLE events ADD CONSTRAINT fk FOREIGN KEY (c) REFERENCES contracts (id) NOT VALID;", nil},
		{"rename", "ALTER TABLE events RENAME COLUMN ledger TO ledger_seq;", []string{"rename"}},
		{"drop column", "ALTER TABLE events DROP COLUMN legacy;", []string{"drop-column"}},
		{"drop constraint", "ALTER TABLE events DROP CONSTRAINT fk;", nil},
		{"lock", "LOCK TABLE events IN ACCESS EXCLUSIVE MODE;", []string{"full-table-lock"}},
		{"comment ignored", "-- CREATE INDEX x ON events (a);\nSELECT 1;", nil},
		{"function body ignored", "CREATE FUNCTION f() RETURNS void AS $$ BEGIN ALTER TABLE events DROP COLUMN x; END $$ LANGUAGE plpgsql;", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, f := range lintMigration("x.up.sql", tc.sql) {
				got = append(got, f.rule)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("rules = %v, want %v", got, tc.want)
			}
		})
	}
}
