package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/allowlist"
)

// Scope values understood by the API. A key may hold any combination.
// admin:* is a super-scope that satisfies every requirement.
const (
	ScopeReadContracts  = "read:contracts"
	ScopeWriteContracts = "write:contracts"
	ScopeReadWatchdog   = "read:watchdog"
	ScopeAdmin          = "admin:*"
)

// ValidScopes is the set of scopes a key may be created with.
var ValidScopes = map[string]bool{
	ScopeReadContracts:  true,
	ScopeWriteContracts: true,
	ScopeReadWatchdog:   true,
	ScopeAdmin:          true,
}

// APIKey is a scoped credential used to authenticate API requests.
// KeyHash is the SHA-256 hex digest of the plaintext token; the token
// itself is never stored.
type APIKey struct {
	ID         string
	Name       string
	KeyPrefix  string
	KeyHash    string
	Scopes     []string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	ExpiresAt  *time.Time
	// UserID binds the key to a user when non-nil. The key then carries that
	// user's identity (issue #518). Unbound keys authenticate scopes only.
	UserID *string
	// RotatedAt is when the key's secret was last regenerated in place,
	// or nil if it has never been rotated.
	RotatedAt *time.Time
	// MonthlyQuota is the optional hard quota on monthly requests (issue #792).
	MonthlyQuota *int
	// AllowedCIDRs restricts the key to these source networks (issue #793).
	// Empty means any address. Entries are normalized CIDR ranges; a bare IP
	// is stored as a single-host range.
	AllowedCIDRs []string
	// AllowedOrigins restricts the key to these browser Origins (issue #793).
	// Empty means any origin; a request without an Origin header is not a
	// browser cross-origin request and is unaffected.
	AllowedOrigins []string
}

// Revoked reports whether the key has been revoked.
func (k APIKey) Revoked() bool { return k.RevokedAt != nil }

// Expired reports whether the key has expired relative to now.
func (k APIKey) Expired(now time.Time) bool {
	return k.ExpiresAt != nil && !now.Before(*k.ExpiresAt)
}

// HasScope reports whether the key grants the required scope. admin:*
// always grants access; a trailing ":*" wildcard grants every scope in
// the same namespace (e.g. "read:*" grants "read:contracts").
func (k APIKey) HasScope(required string) bool {
	if required == "" {
		return true
	}
	for _, s := range k.Scopes {
		if s == required || s == ScopeAdmin || s == "*" {
			return true
		}
		if len(s) >= 2 && s[len(s)-2:] == ":*" && len(required) >= len(s)-1 &&
			required[:len(s)-1] == s[:len(s)-1] {
			return true
		}
	}
	return false
}

// HashKey returns the SHA-256 hex digest of a plaintext API token.
func HashKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ErrInvalidScope is returned when a key is created with an unknown scope.
var ErrInvalidScope = errors.New("store: invalid scope")

// ErrInvalidCIDR and ErrInvalidOrigin report an allowed_cidrs or
// allowed_origins entry that failed validation (issue #793). They wrap the
// allowlist package's sentinels so handlers can map them to a 422 without
// importing the validation package themselves.
var (
	ErrInvalidCIDR   = allowlist.ErrInvalidCIDR
	ErrInvalidOrigin = allowlist.ErrInvalidOrigin
)

// APIKeyStore is the data-access surface for scoped API keys.
type APIKeyStore interface {
	// CreateAPIKey persists a new key. It returns ErrInvalidScope if any
	// scope is not in ValidScopes.
	CreateAPIKey(ctx context.Context, k APIKey) error
	// GetAPIKeyByHash returns the key matching hash, or ErrNotFound.
	GetAPIKeyByHash(ctx context.Context, hash string) (APIKey, error)
	// GetAPIKey returns the key with the given id, or ErrNotFound. Unlike
	// GetAPIKeyByHash it also returns revoked keys, so callers can inspect a
	// key's metadata (including rotated_at) after it has been rotated.
	GetAPIKey(ctx context.Context, id string) (APIKey, error)
	// RotateAPIKey atomically replaces the key's secret: it stores newHash
	// and newPrefix, stamps rotated_at, and invalidates the previous hash in
	// a single statement. The old secret stops authenticating immediately.
	// Returns ErrNotFound if the key is absent or already revoked.
	RotateAPIKey(ctx context.Context, id, newHash, newPrefix string) error
	// ListAPIKeys returns a cursor-paginated list, newest first.
	ListAPIKeys(ctx context.Context, cursor string, limit int) ([]APIKey, string, error)
	// RevokeAPIKey marks a key as revoked. Returns ErrNotFound if absent.
	RevokeAPIKey(ctx context.Context, id string) error
	// TouchAPIKey records the last time a key authenticated a request.
	TouchAPIKey(ctx context.Context, id string) error
}
