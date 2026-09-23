package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *postgresStore) CreateAPIKey(ctx context.Context, k APIKey) error {
	for _, scope := range k.Scopes {
		if !ValidScopes[scope] {
			return ErrInvalidScope
		}
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO api_keys (id, name, key_prefix, key_hash, scopes, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		k.ID, k.Name, k.KeyPrefix, k.KeyHash, k.Scopes, k.CreatedAt,
	)
	return err
}

func (s *postgresStore) GetAPIKeyByHash(ctx context.Context, hash string) (APIKey, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, key_prefix, key_hash, scopes, created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE key_hash = $1 AND revoked_at IS NULL`, hash)
	var k APIKey
	err := row.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.Scopes,
		&k.CreatedAt, &k.LastUsedAt, &k.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return APIKey{}, ErrNotFound
	}
	return k, err
}

func (s *postgresStore) ListAPIKeys(ctx context.Context, cursor string, limit int) ([]APIKey, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, key_prefix, key_hash, scopes, created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE ($1 = '' OR id > $1)
		ORDER BY id ASC
		LIMIT $2`, cursor, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var out []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.Scopes,
			&k.CreatedAt, &k.LastUsedAt, &k.RevokedAt); err != nil {
			return nil, "", err
		}
		out = append(out, k)
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}
	var next string
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}

func (s *postgresStore) RevokeAPIKey(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET revoked_at = $2
		WHERE id = $1 AND revoked_at IS NULL`, id, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *postgresStore) TouchAPIKey(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE api_keys SET last_used_at = $2 WHERE id = $1`, id, time.Now().UTC())
	return err
}
