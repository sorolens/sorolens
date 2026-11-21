package store

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// migration762Tokens names what migration 000046 must create, extension
// statement first, then the seven index names.
var migration762Tokens = []string{
	"CREATE EXTENSION IF NOT EXISTS pg_trgm;",
	"idx_contracts_id_trgm",
	"idx_contracts_label_trgm",
	"idx_labels_public_label_trgm",
	"idx_labels_public_value_trgm",
	"idx_labels_workspace_label_trgm",
	"idx_labels_workspace_value_trgm",
	"idx_events_contract_time_id",
}

// migration762Indexes are the CREATE INDEX statements, which is also how many
// table targets the existence guard has to resolve.
var migration762Indexes = migration762Tokens[1:]

// TestTrigramAndCompositeIndexesMigration guards that the indexes required
// by Issue #762 are present in the migration files and properly structured.
func TestTrigramAndCompositeIndexesMigration(t *testing.T) {
	migrationPath := filepath.Join("..", "db", "migrations", "000046_trigram_and_composite_indexes.up.sql")
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("failed to read migration file %s: %v", migrationPath, err)
	}

	sql := string(content)

	for _, idx := range migration762Tokens {
		if !strings.Contains(sql, idx) {
			t.Errorf("migration missing expected index definition %q", idx)
		}
	}

	downPath := filepath.Join("..", "db", "migrations", "000046_trigram_and_composite_indexes.down.sql")
	downContent, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatalf("failed to read down migration file %s: %v", downPath, err)
	}

	downSql := string(downContent)
	for _, idx := range migration762Tokens[1:] {
		if !strings.Contains(downSql, idx) {
			t.Errorf("down migration missing rollback for index %q", idx)
		}
	}
}

// TestTrigramIndexTargetsExist guards that every table the #762 migration
// indexes is actually created by the migration sequence. CI applies the
// migrations to an empty database, so an index on a table that no migration
// creates fails the whole Go job at the migration step.
func TestTrigramIndexTargetsExist(t *testing.T) {
	dir := filepath.Join("..", "db", "migrations")
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}

	created := map[string]bool{}
	createTable := regexp.MustCompile(`(?i)CREATE\s+(?:UNLOGGED\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([\w."]+)`)
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range createTable.FindAllStringSubmatch(string(content), -1) {
			created[strings.ToLower(m[1])] = true
		}
	}

	up, err := os.ReadFile(filepath.Join(dir, "000046_trigram_and_composite_indexes.up.sql"))
	if err != nil {
		t.Fatalf("failed to read #762 migration: %v", err)
	}
	// Statement by statement, with comments removed, so prose that happens to
	// contain "on the" cannot be mistaken for an index target.
	indexTarget := regexp.MustCompile(`(?is)CREATE\s+(?:UNIQUE\s+)?INDEX\s+.*?\bON\s+(?:ONLY\s+)?([\w."]+)`)
	indexed := 0
	for _, stmt := range strings.Split(stripSQLComments(string(up)), ";") {
		m := indexTarget.FindStringSubmatch(stmt)
		if m == nil {
			continue
		}
		indexed++
		table := strings.ToLower(strings.Trim(m[1], `"`))
		if !created[table] {
			t.Errorf("#762 migration indexes %q, which no migration creates", table)
		}
	}
	if want := len(migration762Indexes); indexed != want {
		t.Errorf("#762 migration indexes %d tables, expected %d: the guard regex no longer matches the migration", indexed, want)
	}
}

// stripSQLComments removes `--` line comments from SQL so that structural
// regular expressions only see executable text.
func stripSQLComments(sql string) string {
	lines := strings.Split(sql, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}
