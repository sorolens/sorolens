package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTrigramAndCompositeIndexesMigration guards that the indexes required
// by Issue #762 are present in the migration files and properly structured.
func TestTrigramAndCompositeIndexesMigration(t *testing.T) {
	migrationPath := filepath.Join("..", "db", "migrations", "000046_trigram_and_composite_indexes.up.sql")
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("failed to read migration file %s: %v", migrationPath, err)
	}

	sql := string(content)

	requiredIndexes := []string{
		"CREATE EXTENSION IF NOT EXISTS pg_trgm;",
		"idx_contracts_id_trgm",
		"idx_contracts_label_trgm",
		"idx_account_labels_label_trgm",
		"idx_monitored_contracts_name_trgm",
		"idx_monitored_contracts_id_trgm",
		"idx_events_contract_time_id",
	}

	for _, idx := range requiredIndexes {
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
	for _, idx := range requiredIndexes[1:] {
		if !strings.Contains(downSql, idx) {
			t.Errorf("down migration missing rollback for index %q", idx)
		}
	}
}
