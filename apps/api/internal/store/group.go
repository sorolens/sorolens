package store

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Group is a named portfolio of tracked contracts owned by a single user.
type Group struct {
	ID        string
	OwnerID   string
	Name      string
	CreatedAt time.Time
}

// GroupSummary is a group together with the aggregate statistics across all of
// its member contracts. It backs the group list page, which renders one set of
// stat cards per group.
type GroupSummary struct {
	Group
	ContractCount      int64
	EventCount         int64
	InvocationCount    int64
	StorageEntryCount  int64
	AverageHealthScore float64
}

// GroupContract is one member contract of a group plus the per-contract signals
// the group detail view renders: the cached composite health score and the
// timestamp of the contract's most recent indexed activity.
type GroupContract struct {
	ContractID     string
	Network        string
	Label          string
	Status         string
	HealthScore    *int32
	LastActivityAt *time.Time
}

// GroupStats is the aggregated view of a single group's member contracts.
type GroupStats struct {
	GroupID            string
	ContractCount      int64
	EventCount         int64
	InvocationCount    int64
	StorageEntryCount  int64
	AverageHealthScore float64
}

// ErrInvalidGroupName is returned when a group is created or renamed with an
// empty (or whitespace-only) name.
var ErrInvalidGroupName = errors.New("store: group name must not be empty")

// MaxGroupNameLen bounds group names so the portfolio pages render predictably.
const MaxGroupNameLen = 100

// GroupStore is the data-access surface for user-owned contract groups
// (portfolios, issue #326). Every method is scoped by ownerID so a caller can
// only read or mutate their own groups; a group owned by somebody else is
// indistinguishable from a missing one (ErrNotFound).
//
// Membership is many-to-many: a contract may belong to any number of groups,
// and adding the same contract twice is idempotent.
type GroupStore interface {
	// CreateGroup creates a group owned by ownerID. Returns the stored group
	// with its server-assigned ID and creation timestamp, or
	// ErrInvalidGroupName when name is empty.
	CreateGroup(ctx context.Context, ownerID, name string) (Group, error)

	// ListGroups returns every group owned by ownerID, newest first, each with
	// its aggregate statistics computed in a single query.
	ListGroups(ctx context.Context, ownerID string) ([]GroupSummary, error)

	// GetGroup returns one owned group, or ErrNotFound when it does not exist
	// or belongs to another user.
	GetGroup(ctx context.Context, ownerID, groupID string) (Group, error)

	// UpdateGroup renames an owned group, returning the updated group or
	// ErrNotFound.
	UpdateGroup(ctx context.Context, ownerID, groupID, name string) (Group, error)

	// DeleteGroup removes an owned group and all of its membership rows.
	DeleteGroup(ctx context.Context, ownerID, groupID string) error

	// AddContractToGroup adds a contract to an owned group. It is idempotent:
	// re-adding an existing member succeeds without error.
	AddContractToGroup(ctx context.Context, ownerID, groupID, contractID string) error

	// RemoveContractFromGroup removes a contract from an owned group. Removing
	// a contract that is not a member succeeds without error.
	RemoveContractFromGroup(ctx context.Context, ownerID, groupID, contractID string) error

	// ListGroupContracts returns the member contracts of an owned group with
	// their cached health score and last-activity timestamp, ordered by
	// contract ID. A contract with no cached score or no indexed activity
	// yields nil for the corresponding field.
	ListGroupContracts(ctx context.Context, ownerID, groupID string) ([]GroupContract, error)

	// GetGroupStats aggregates event count, invocation count, storage entry
	// count, and the average cached health score across an owned group's
	// contracts in a single query. Returns ErrNotFound for an unowned group.
	GetGroupStats(ctx context.Context, ownerID, groupID string) (GroupStats, error)
}

// ---- shared helpers ---------------------------------------------------------

// newGroupID returns a random RFC 4122 version 4 UUID. Group IDs are generated
// in the application so the in-memory mock and postgres backends share the
// same shape; the migration keeps a gen_random_uuid() default for callers that
// insert rows directly.
func newGroupID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand does not fail on supported platforms; fall back to a
		// time-derived ID rather than panicking on the request path.
		return fmt.Sprintf("00000000-0000-4000-8000-%012x", uint64(time.Now().UnixNano())&0xffffffffffff)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// validGroupName trims and length-checks a candidate group name.
func validGroupName(name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || len(trimmed) > MaxGroupNameLen {
		return "", false
	}
	return trimmed, true
}

// validUUID reports whether s is a canonical 8-4-4-4-12 UUID. Group IDs are
// UUIDs, and the postgres backend casts path parameters to uuid, so a malformed
// ID must be rejected before the query rather than surfacing as a 500 from the
// cast. The mock treats any unknown ID as ErrNotFound, so this keeps the two
// backends behaviourally identical.
func validUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// ---- postgres implementation ----------------------------------------------

// groupSummariesQuery aggregates the portfolio statistics for every group
// owned by $1 in one statement. A non-NULL $2 restricts the result to a single
// group. Event/invocation/storage counts are pre-aggregated per contract (one
// hash aggregate each) and joined onto the group's membership, so the query
// stays O(rows) instead of issuing one count per contract.
const groupSummariesQuery = `
	WITH scoped AS (
		SELECT gc.group_id, gc.contract_id
		FROM group_contracts gc
		JOIN groups sg ON sg.id = gc.group_id
		WHERE sg.owner_id = $1
		  AND ($2::uuid IS NULL OR gc.group_id = $2::uuid)
	),
	ev AS (
		SELECT e.contract_id, COUNT(*) AS n
		FROM events e
		WHERE e.contract_id IN (SELECT contract_id FROM scoped)
		GROUP BY e.contract_id
	),
	inv AS (
		SELECT i.contract_id, COUNT(*) AS n
		FROM invocations i
		WHERE i.contract_id IN (SELECT contract_id FROM scoped)
		GROUP BY i.contract_id
	),
	st AS (
		SELECT se.contract_id, COUNT(*) AS n
		FROM storage_entries se
		WHERE se.contract_id IN (SELECT contract_id FROM scoped)
		GROUP BY se.contract_id
	)
	SELECT g.id::text, g.owner_id, g.name, g.created_at,
	       COUNT(s.contract_id)                    AS contract_count,
	       COALESCE(SUM(ev.n), 0)                  AS event_count,
	       COALESCE(SUM(inv.n), 0)                 AS invocation_count,
	       COALESCE(SUM(st.n), 0)                  AS storage_entry_count,
	       COALESCE(AVG(hs.score), 0)::float8      AS average_health_score
	FROM groups g
	LEFT JOIN scoped s ON s.group_id = g.id
	LEFT JOIN ev  ON ev.contract_id  = s.contract_id
	LEFT JOIN inv ON inv.contract_id = s.contract_id
	LEFT JOIN st  ON st.contract_id  = s.contract_id
	LEFT JOIN contract_health_scores hs ON hs.contract_id = s.contract_id
	WHERE g.owner_id = $1
	  AND ($2::uuid IS NULL OR g.id = $2::uuid)
	GROUP BY g.id, g.owner_id, g.name, g.created_at
	ORDER BY g.created_at DESC, g.id`

func (s *postgresStore) groupSummaries(ctx context.Context, ownerID, groupID string) ([]GroupSummary, error) {
	if groupID != "" && !validUUID(groupID) {
		return []GroupSummary{}, nil
	}
	var groupFilter any
	if groupID != "" {
		groupFilter = groupID
	}
	rows, err := s.pool.Query(ctx, groupSummariesQuery, ownerID, groupFilter)
	if err != nil {
		return nil, fmt.Errorf("group summaries: %w", err)
	}
	defer rows.Close()

	out := make([]GroupSummary, 0)
	for rows.Next() {
		var gs GroupSummary
		if err := rows.Scan(
			&gs.ID, &gs.OwnerID, &gs.Name, &gs.CreatedAt,
			&gs.ContractCount, &gs.EventCount, &gs.InvocationCount,
			&gs.StorageEntryCount, &gs.AverageHealthScore,
		); err != nil {
			return nil, fmt.Errorf("group summaries scan: %w", err)
		}
		out = append(out, gs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("group summaries: %w", err)
	}
	return out, nil
}

func (s *postgresStore) CreateGroup(ctx context.Context, ownerID, name string) (Group, error) {
	trimmed, ok := validGroupName(name)
	if !ok {
		return Group{}, ErrInvalidGroupName
	}
	// Identity is the self-asserted X-User-ID header (same contract as the
	// watchlist), so provision a viewer row on first use to satisfy the
	// owner_id foreign key.
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO users (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`, ownerID); err != nil {
		return Group{}, fmt.Errorf("create group owner: %w", err)
	}
	var g Group
	err := s.pool.QueryRow(ctx, `
		INSERT INTO groups (owner_id, name)
		VALUES ($1, $2)
		RETURNING id::text, owner_id, name, created_at`,
		ownerID, trimmed,
	).Scan(&g.ID, &g.OwnerID, &g.Name, &g.CreatedAt)
	if err != nil {
		return Group{}, fmt.Errorf("create group: %w", err)
	}
	return g, nil
}

func (s *postgresStore) ListGroups(ctx context.Context, ownerID string) ([]GroupSummary, error) {
	return s.groupSummaries(ctx, ownerID, "")
}

func (s *postgresStore) GetGroup(ctx context.Context, ownerID, groupID string) (Group, error) {
	if !validUUID(groupID) {
		return Group{}, ErrNotFound
	}
	var g Group
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, owner_id, name, created_at
		FROM groups
		WHERE id = $1::uuid AND owner_id = $2`,
		groupID, ownerID,
	).Scan(&g.ID, &g.OwnerID, &g.Name, &g.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	if err != nil {
		return Group{}, fmt.Errorf("get group: %w", err)
	}
	return g, nil
}

func (s *postgresStore) UpdateGroup(ctx context.Context, ownerID, groupID, name string) (Group, error) {
	trimmed, ok := validGroupName(name)
	if !ok {
		return Group{}, ErrInvalidGroupName
	}
	if !validUUID(groupID) {
		return Group{}, ErrNotFound
	}
	var g Group
	err := s.pool.QueryRow(ctx, `
		UPDATE groups
		SET name = $3
		WHERE id = $1::uuid AND owner_id = $2
		RETURNING id::text, owner_id, name, created_at`,
		groupID, ownerID, trimmed,
	).Scan(&g.ID, &g.OwnerID, &g.Name, &g.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	if err != nil {
		return Group{}, fmt.Errorf("update group: %w", err)
	}
	return g, nil
}

func (s *postgresStore) DeleteGroup(ctx context.Context, ownerID, groupID string) error {
	if !validUUID(groupID) {
		return ErrNotFound
	}
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM groups WHERE id = $1::uuid AND owner_id = $2`,
		groupID, ownerID,
	)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// groupOwned reports whether groupID exists and belongs to ownerID.
func (s *postgresStore) groupOwned(ctx context.Context, ownerID, groupID string) (bool, error) {
	if !validUUID(groupID) {
		return false, nil
	}
	var owned bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM groups WHERE id = $1::uuid AND owner_id = $2
		)`, groupID, ownerID).Scan(&owned)
	if err != nil {
		return false, fmt.Errorf("check group owner: %w", err)
	}
	return owned, nil
}

func (s *postgresStore) AddContractToGroup(ctx context.Context, ownerID, groupID, contractID string) error {
	owned, err := s.groupOwned(ctx, ownerID, groupID)
	if err != nil {
		return err
	}
	if !owned {
		return ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO group_contracts (group_id, contract_id)
		VALUES ($1::uuid, $2)
		ON CONFLICT (group_id, contract_id) DO NOTHING`,
		groupID, contractID,
	)
	if err != nil {
		return fmt.Errorf("add contract to group: %w", err)
	}
	return nil
}

func (s *postgresStore) RemoveContractFromGroup(ctx context.Context, ownerID, groupID, contractID string) error {
	owned, err := s.groupOwned(ctx, ownerID, groupID)
	if err != nil {
		return err
	}
	if !owned {
		return ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `
		DELETE FROM group_contracts
		WHERE group_id = $1::uuid AND contract_id = $2`,
		groupID, contractID,
	)
	if err != nil {
		return fmt.Errorf("remove contract from group: %w", err)
	}
	return nil
}

func (s *postgresStore) ListGroupContracts(ctx context.Context, ownerID, groupID string) ([]GroupContract, error) {
	owned, err := s.groupOwned(ctx, ownerID, groupID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, ErrNotFound
	}
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.network, COALESCE(c.label, ''), c.status,
		       hs.score,
		       (
		           SELECT MAX(t.ts) FROM (
		               SELECT MAX(e.ledger_closed_at) AS ts
		               FROM events e WHERE e.contract_id = c.id
		               UNION ALL
		               SELECT MAX(i.ledger_closed_at)
		               FROM invocations i WHERE i.contract_id = c.id
		           ) t
		       ) AS last_activity_at
		FROM group_contracts gc
		JOIN contracts c ON c.id = gc.contract_id
		LEFT JOIN contract_health_scores hs ON hs.contract_id = c.id
		WHERE gc.group_id = $1::uuid
		ORDER BY c.id`,
		groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("list group contracts: %w", err)
	}
	defer rows.Close()

	out := make([]GroupContract, 0)
	for rows.Next() {
		var gc GroupContract
		if err := rows.Scan(
			&gc.ContractID, &gc.Network, &gc.Label, &gc.Status,
			&gc.HealthScore, &gc.LastActivityAt,
		); err != nil {
			return nil, fmt.Errorf("list group contracts scan: %w", err)
		}
		out = append(out, gc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list group contracts: %w", err)
	}
	return out, nil
}

func (s *postgresStore) GetGroupStats(ctx context.Context, ownerID, groupID string) (GroupStats, error) {
	summaries, err := s.groupSummaries(ctx, ownerID, groupID)
	if err != nil {
		return GroupStats{}, err
	}
	if len(summaries) == 0 {
		return GroupStats{}, ErrNotFound
	}
	gs := summaries[0]
	return GroupStats{
		GroupID:            gs.ID,
		ContractCount:      gs.ContractCount,
		EventCount:         gs.EventCount,
		InvocationCount:    gs.InvocationCount,
		StorageEntryCount:  gs.StorageEntryCount,
		AverageHealthScore: gs.AverageHealthScore,
	}, nil
}
