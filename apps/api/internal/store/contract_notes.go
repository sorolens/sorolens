package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// MaxContractNoteBodyLen bounds a note body in characters. It is long enough
// for a migration write-up and short enough that the table cannot be turned
// into bulk storage.
const MaxContractNoteBodyLen = 8192

// ContractNote is a markdown note attached to a tracked contract (issue #164).
// Body holds markdown source; rendering is the caller's job.
type ContractNote struct {
	ID         string    `json:"id"`
	ContractID string    `json:"contract_id"`
	Author     string    `json:"author"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// scanContractNote materialises one contract_notes row. The select lists in
// this file share this column order; id is cast to text because the column is
// a uuid.
func scanContractNote(row pgx.Row) (ContractNote, error) {
	var n ContractNote
	if err := row.Scan(&n.ID, &n.ContractID, &n.Author, &n.Body, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return ContractNote{}, err
	}
	return n, nil
}

// ---- store.ContractNoteStore (postgres) -------------------------------------

// AddContractNote appends a note to a contract. The id and timestamps are
// assigned by the database and returned with the stored row, so the caller
// never has to guess them.
func (s *postgresStore) AddContractNote(ctx context.Context, contractID, author, body string) (ContractNote, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO contract_notes (contract_id, author, body)
		VALUES ($1, $2, $3)
		RETURNING id::text, contract_id, author, body, created_at, updated_at`,
		contractID, author, body,
	)
	note, err := scanContractNote(row)
	if err != nil {
		return ContractNote{}, fmt.Errorf("add contract note: %w", err)
	}
	return note, nil
}

// GetContractNote returns a single note scoped to its contract, or ErrNotFound
// when the contract has no such note. Scoping the lookup by contract_id keeps a
// note id from another contract from being readable through this contract's
// path.
func (s *postgresStore) GetContractNote(ctx context.Context, contractID, noteID string) (ContractNote, error) {
	if !validUUID(noteID) {
		return ContractNote{}, ErrNotFound
	}
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, contract_id, author, body, created_at, updated_at
		FROM contract_notes
		WHERE id = $1::uuid AND contract_id = $2`, noteID, contractID)
	note, err := scanContractNote(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ContractNote{}, ErrNotFound
		}
		return ContractNote{}, fmt.Errorf("get contract note: %w", err)
	}
	return note, nil
}

// ListContractNotes returns a contract's notes, newest first. A contract with no
// notes yields an empty, non-nil slice.
func (s *postgresStore) ListContractNotes(ctx context.Context, contractID string) ([]ContractNote, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, contract_id, author, body, created_at, updated_at
		FROM contract_notes
		WHERE contract_id = $1
		ORDER BY created_at DESC, id DESC`, contractID)
	if err != nil {
		return nil, fmt.Errorf("list contract notes: %w", err)
	}
	defer rows.Close()

	out := []ContractNote{}
	for rows.Next() {
		var n ContractNote
		if err := rows.Scan(&n.ID, &n.ContractID, &n.Author, &n.Body, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

// UpdateContractNote replaces a note body and bumps updated_at, returning the
// updated row or ErrNotFound when the contract has no such note. The author is
// deliberately not part of the statement: editing a note never reassigns it.
func (s *postgresStore) UpdateContractNote(ctx context.Context, contractID, noteID, body string) (ContractNote, error) {
	if !validUUID(noteID) {
		return ContractNote{}, ErrNotFound
	}
	row := s.pool.QueryRow(ctx, `
		UPDATE contract_notes
		SET body = $3, updated_at = NOW()
		WHERE id = $1::uuid AND contract_id = $2
		RETURNING id::text, contract_id, author, body, created_at, updated_at`,
		noteID, contractID, body,
	)
	note, err := scanContractNote(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ContractNote{}, ErrNotFound
		}
		return ContractNote{}, fmt.Errorf("update contract note: %w", err)
	}
	return note, nil
}

// DeleteContractNote removes a note. Deleting one that is not present is a
// no-op, so a retried delete is safe.
func (s *postgresStore) DeleteContractNote(ctx context.Context, contractID, noteID string) error {
	if !validUUID(noteID) {
		return nil
	}
	if _, err := s.pool.Exec(ctx, `
		DELETE FROM contract_notes
		WHERE id = $1::uuid AND contract_id = $2`, noteID, contractID); err != nil {
		return fmt.Errorf("delete contract note: %w", err)
	}
	return nil
}
