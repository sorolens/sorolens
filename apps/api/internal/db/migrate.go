// Package db owns the schema migrations for the Sorolens API and a small
// runner that applies them in-process.
//
// The SQL files under migrations/ are embedded into the binary so a deployed
// container can apply pending migrations on startup without shipping the
// repository or a separate migration tool. The migrate files use the
// golang-migrate naming convention (<version>_<name>.up.sql / .down.sql) and
// the same files are used by `make migrate-up` locally, so the two paths stay
// in sync.
package db

import (
	"embed"
	"errors"
	"fmt"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	// Registers the "pgx5" database driver, which uses pgx's database/sql
	// adapter. A blank import is required because migrate looks the driver up
	// by URL scheme at runtime.
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// migrationsDir is the directory inside migrationsFS that holds the SQL files.
const migrationsDir = "migrations"

//go:embed migrations/*.sql
var migrationsFS embed.FS

// newSource builds the migration source from the embedded filesystem.
func newSource() (source.Driver, error) {
	src, err := iofs.New(migrationsFS, migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("open embedded migrations: %w", err)
	}
	return src, nil
}

// normalizeURL rewrites a postgres connection string so it selects migrate's
// pgx v5 driver. migrate picks its database driver from the URL scheme, and
// that driver registers itself as "pgx5", not "postgres".
func normalizeURL(databaseURL string) (string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return "", fmt.Errorf("parse database url: %w", err)
	}
	if u.Scheme == "" {
		return "", fmt.Errorf("parse database url: missing scheme in %q", databaseURL)
	}
	u.Scheme = "pgx5"
	return u.String(), nil
}

// Migrate applies every pending migration to the database at databaseURL. It is
// idempotent: when the schema is already current it returns nil. Callers should
// pass the direct (non-pooled) connection string so migrations are not run
// through a transaction-mode connection pooler.
func Migrate(databaseURL string) error {
	if databaseURL == "" {
		return errors.New("migrate: empty database url")
	}

	src, err := newSource()
	if err != nil {
		return err
	}

	normalized, err := normalizeURL(databaseURL)
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, normalized)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer func() {
		// Close reports source and database errors independently; neither
		// affects whether migrations were applied.
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
