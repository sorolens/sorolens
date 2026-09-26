package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Postgres-backed implementation of ContractNoteStore.

// CreateContractNote inserts a note row. The caller supplies the id and
// timestamps so the API can echo the created row without a follow-up read.
func (s *postgresStore) CreateContractNote(ctx context.Context, n ContractNote) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contract_notes
			(id, contract_id, author, body, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		n.ID, n.ContractID, n.Author, n.Body, n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create contract note: %w", err)
	}
	return nil
}

// ListContractNotes returns a contract's notes, newest first.
func (s *postgresStore) ListContractNotes(ctx context.Context, contractID string, limit int) ([]ContractNote, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, contract_id, author, body, created_at, updated_at
		FROM contract_notes
		WHERE contract_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2`, contractID, limit)
	if err != nil {
		return nil, fmt.Errorf("list contract notes: %w", err)
	}
	defer rows.Close()

	var out []ContractNote
	for rows.Next() {
		var n ContractNote
		if err := rows.Scan(&n.ID, &n.ContractID, &n.Author, &n.Body, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// GetContractNote returns a single note by id, or ErrNotFound.
func (s *postgresStore) GetContractNote(ctx context.Context, id string) (ContractNote, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, contract_id, author, body, created_at, updated_at
		FROM contract_notes WHERE id = $1`, id)
	var n ContractNote
	err := row.Scan(&n.ID, &n.ContractID, &n.Author, &n.Body, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractNote{}, ErrNotFound
	}
	return n, err
}

// DeleteContractNote deletes a note scoped to its contract. It returns
// ErrNotFound when no row matched, so a note belonging to another contract
// cannot be deleted through this contract's path.
func (s *postgresStore) DeleteContractNote(ctx context.Context, contractID, id string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM contract_notes WHERE id = $1 AND contract_id = $2`, id, contractID)
	if err != nil {
		return fmt.Errorf("delete contract note: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
