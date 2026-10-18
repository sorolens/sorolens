package store

import (
	"context"
	"fmt"
)

// ---- contract tags ---------------------------------------------------------

// AddContractTag adds a tag to a contract. The (contract_id, tag) primary key
// makes a repeated add a no-op, so callers can treat the operation as
// idempotent.
func (s *postgresStore) AddContractTag(ctx context.Context, contractID, tag string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contract_tags (contract_id, tag)
		VALUES ($1, $2)
		ON CONFLICT (contract_id, tag) DO NOTHING`,
		contractID, tag,
	)
	if err != nil {
		return fmt.Errorf("add contract tag: %w", err)
	}
	return nil
}

// RemoveContractTag removes a tag from a contract. Deleting a tag that is not
// present is a no-op.
func (s *postgresStore) RemoveContractTag(ctx context.Context, contractID, tag string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM contract_tags WHERE contract_id = $1 AND tag = $2`,
		contractID, tag,
	)
	if err != nil {
		return fmt.Errorf("remove contract tag: %w", err)
	}
	return nil
}

// ListContractTags returns a contract's tags in ascending order. A contract
// with no tags yields an empty, non-nil slice.
func (s *postgresStore) ListContractTags(ctx context.Context, contractID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT tag FROM contract_tags
		WHERE contract_id = $1
		ORDER BY tag ASC`, contractID)
	if err != nil {
		return nil, fmt.Errorf("list contract tags: %w", err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		out = append(out, tag)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

// contractTagsByContract batch-loads tags for the given contract IDs and
// returns them keyed by contract ID. It is used by the list/read paths to
// attach tags without an N+1 query per contract.
func (s *postgresStore) contractTagsByContract(ctx context.Context, ids []string) (map[string][]string, error) {
	out := make(map[string][]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT contract_id, tag FROM contract_tags
		WHERE contract_id = ANY($1)
		ORDER BY contract_id ASC, tag ASC`, ids)
	if err != nil {
		return nil, fmt.Errorf("batch contract tags: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var contractID, tag string
		if err := rows.Scan(&contractID, &tag); err != nil {
			return nil, err
		}
		out[contractID] = append(out[contractID], tag)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	for id, tags := range out {
		if tags == nil {
			out[id] = []string{}
		}
	}
	return out, nil
}
