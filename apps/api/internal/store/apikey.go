package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
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
}

// Revoked reports whether the key has been revoked.
func (k APIKey) Revoked() bool { return k.RevokedAt != nil }

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

// APIKeyStore is the data-access surface for scoped API keys.
type APIKeyStore interface {
	// CreateAPIKey persists a new key. It returns ErrInvalidScope if any
	// scope is not in ValidScopes.
	CreateAPIKey(ctx context.Context, k APIKey) error
	// GetAPIKeyByHash returns the key matching hash, or ErrNotFound.
	GetAPIKeyByHash(ctx context.Context, hash string) (APIKey, error)
	// ListAPIKeys returns a cursor-paginated list, newest first.
	ListAPIKeys(ctx context.Context, cursor string, limit int) ([]APIKey, string, error)
	// RevokeAPIKey marks a key as revoked. Returns ErrNotFound if absent.
	RevokeAPIKey(ctx context.Context, id string) error
	// TouchAPIKey records the last time a key authenticated a request.
	TouchAPIKey(ctx context.Context, id string) error
}
