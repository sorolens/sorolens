package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
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
	// Empty or nil means the key is not restricted by address.
	AllowedCIDRs []string
	// AllowedOrigins restricts browser use of the key to these origins.
	// Empty or nil means the key is not restricted by Origin header.
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

// InvalidAllowlistError reports a malformed CIDR range or origin in a key's
// allowlist. Its message is safe to return to an API caller.
type InvalidAllowlistError struct{ Reason string }

func (e *InvalidAllowlistError) Error() string { return e.Reason }

// NormalizeAllowedCIDRs trims, validates, de-duplicates and sorts a CIDR
// allowlist. Empty entries are ignored and an all-empty input returns nil,
// which the store persists as NULL and the middleware treats as unrestricted.
// Each range is reduced to its network form (203.0.113.7/24 becomes
// 203.0.113.0/24) so stored values compare predictably.
func NormalizeAllowedCIDRs(raw []string) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, entry := range raw {
		s := strings.TrimSpace(entry)
		if s == "" {
			continue
		}
		_, network, err := net.ParseCIDR(s)
		if err != nil {
			return nil, &InvalidAllowlistError{Reason: fmt.Sprintf("invalid CIDR range %q", s)}
		}
		normalized := network.String()
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// NormalizeAllowedOrigins trims, validates, de-duplicates and sorts an origin
// allowlist. An origin is scheme://host[:port] with an http or https scheme;
// paths, queries, fragments and embedded credentials are rejected. Empty
// entries are ignored and an all-empty input returns nil (unrestricted).
func NormalizeAllowedOrigins(raw []string) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, entry := range raw {
		s := strings.TrimSpace(entry)
		if s == "" {
			continue
		}
		origin, err := normalizeOrigin(s)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		out = append(out, origin)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// normalizeOrigin canonicalises one origin to lowercase scheme and host.
func normalizeOrigin(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	u, err := url.Parse(s)
	if err != nil || u.Opaque != "" {
		return "", &InvalidAllowlistError{Reason: fmt.Sprintf("invalid origin %q: want scheme://host[:port]", s)}
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "http" && scheme != "https") || u.Host == "" {
		return "", &InvalidAllowlistError{Reason: fmt.Sprintf("invalid origin %q: want an http or https origin", s)}
	}
	if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", &InvalidAllowlistError{Reason: fmt.Sprintf("invalid origin %q: must not contain credentials, a path, a query or a fragment", s)}
	}
	host := strings.ToLower(u.Host)
	// Browsers omit the default port from the Origin header, so an allowlist
	// entry written with it must still match.
	if port := u.Port(); (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		host = strings.TrimSuffix(host, ":"+port)
	}
	return scheme + "://" + host, nil
}

// AllowsClientIP reports whether ip is permitted by the key's allowlist. An
// empty allowlist permits every address; a non-empty one fails closed when the
// address cannot be resolved.
func (k APIKey) AllowsClientIP(ip net.IP) bool {
	if len(k.AllowedCIDRs) == 0 {
		return true
	}
	if ip == nil {
		return false
	}
	for _, entry := range k.AllowedCIDRs {
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// AllowsOrigin reports whether origin is permitted by the key's allowlist. An
// empty allowlist permits every origin (including no Origin header); a
// non-empty one requires an exact, case-normalised origin match.
func (k APIKey) AllowsOrigin(origin string) bool {
	if len(k.AllowedOrigins) == 0 {
		return true
	}
	got, err := normalizeOrigin(origin)
	if err != nil {
		return false
	}
	for _, allowed := range k.AllowedOrigins {
		want, err := normalizeOrigin(allowed)
		if err != nil {
			continue
		}
		if got == want {
			return true
		}
	}
	return false
}

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
	// UpdateAPIKeyAllowlists replaces a key's allowlists. Nil or empty
	// slices clear the restriction. Returns ErrNotFound if the key is absent.
	UpdateAPIKeyAllowlists(ctx context.Context, id string, allowedCIDRs, allowedOrigins []string) error
}
