package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// UpsertUser inserts or updates the user row. On conflict it refreshes the
// role when the caller supplied one; GitHub ID and role are only overwritten
// when non-empty so a partial update cannot wipe existing data.
func (s *postgresStore) UpsertUser(ctx context.Context, u User) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, github_id, role, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (id) DO UPDATE SET
			github_id = COALESCE($2, users.github_id),
			role      = CASE WHEN $3 = '' THEN users.role ELSE $3 END`,
		u.ID, u.GitHubID, u.Role,
	)
	return err
}

// GetUserByID returns the user with the given ID, or ErrNotFound.
func (s *postgresStore) GetUserByID(ctx context.Context, id string) (User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, github_id, role, created_at FROM users WHERE id = $1`, id)
	var u User
	if err := row.Scan(&u.ID, &u.GitHubID, &u.Role, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// GetUserByGitHubID returns the user whose GitHub ID matches, or ErrNotFound.
func (s *postgresStore) GetUserByGitHubID(ctx context.Context, githubID string) (User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, github_id, role, created_at FROM users WHERE github_id = $1`, githubID)
	var u User
	if err := row.Scan(&u.ID, &u.GitHubID, &u.Role, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("get user by github id: %w", err)
	}
	return u, nil
}
