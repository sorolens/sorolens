package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Identity is established only from a credential the caller can prove it
// holds:
//
//   - a scoped API key (optionally bound to a user),
//   - an HMAC-signed workspace session token minted by
//     POST /api/v1/auth/session, or
//   - identity headers signed by a trusted reverse proxy.
//
// Raw X-User-ID / X-GitHub-Login / X-User-Sub / X-GitHub-ID headers are never
// trusted on their own (issue #518).
const (
	// SessionTokenPrefix distinguishes session tokens from API keys. API keys
	// use the sl_ prefix and are looked up by hash; session tokens are
	// verified locally with the signing key.
	SessionTokenPrefix = "sls_"

	// IdentitySourceSession, IdentitySourceAPIKey and IdentitySourceProxy name
	// the credential that established an identity (audit logs, debugging).
	IdentitySourceSession = "session"
	IdentitySourceAPIKey  = "api_key"
	IdentitySourceProxy   = "trusted_proxy"

	// DefaultSessionTTL is how long a minted session token stays valid.
	DefaultSessionTTL = 30 * 24 * time.Hour

	// DefaultProxyMaxSkew bounds the age of a signed proxy identity. A wider
	// window would let a captured header be replayed for longer.
	DefaultProxyMaxSkew = 5 * time.Minute

	proxyUserIDHeader    = "X-Identity-User-ID"
	proxyTimestampHeader = "X-Identity-Timestamp"
	proxySignatureHeader = "X-Identity-Signature"
)

// VerifiedIdentity is the authenticated caller attached to a request context
// by the Identity middleware.
type VerifiedIdentity struct {
	// UserID is empty for an API key that is not bound to a user. Such a key
	// still authenticates scopes, but cannot act as a user.
	UserID string
	// Role is the stored role of UserID, empty when there is no user.
	Role string
	// Source is one of the IdentitySource* constants.
	Source string
	// APIKey is set when the caller presented a valid scoped API key.
	APIKey *store.APIKey
}

// IsAdminKey reports whether the caller holds a key with the admin:* scope.
// Admin-scoped keys satisfy role checks without being bound to a user row so
// that an operator can bootstrap the first admin key (and keep rotating
// keys) without relying on identity headers.
func (id VerifiedIdentity) IsAdminKey() bool {
	return id.APIKey != nil && id.APIKey.HasScope(store.ScopeAdmin)
}

type identityContextKey struct{}

// WithIdentity returns a context carrying id. It is exported so tests and
// non-HTTP callers can inject a verified identity without going through the
// middleware.
func WithIdentity(ctx context.Context, id VerifiedIdentity) context.Context {
	return context.WithValue(ctx, identityContextKey{}, id)
}

// IdentityFromContext returns the verified identity attached by Identity.
func IdentityFromContext(ctx context.Context) (VerifiedIdentity, bool) {
	id, ok := ctx.Value(identityContextKey{}).(VerifiedIdentity)
	return id, ok
}

// IdentityFromRequest returns the verified identity attached to r, if any.
func IdentityFromRequest(r *http.Request) (VerifiedIdentity, bool) {
	if r == nil {
		return VerifiedIdentity{}, false
	}
	return IdentityFromContext(r.Context())
}

// VerifiedUserID returns the authenticated user ID for r, or "".
func VerifiedUserID(r *http.Request) string {
	id, ok := IdentityFromRequest(r)
	if !ok {
		return ""
	}
	return id.UserID
}

// SessionSigner mints and verifies HMAC-SHA256 session tokens. A zero or
// empty key disables both operations; deployments that never configure
// IDENTITY_SIGNING_KEY therefore cannot mint sessions.
type SessionSigner struct {
	key []byte
	ttl time.Duration
	now func() time.Time
}

// NewSessionSigner returns a signer for secret. ttl <= 0 uses
// DefaultSessionTTL.
func NewSessionSigner(secret string, ttl time.Duration) *SessionSigner {
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return &SessionSigner{key: []byte(secret), ttl: ttl, now: time.Now}
}

// Enabled reports whether the signer has key material.
func (s *SessionSigner) Enabled() bool {
	return s != nil && len(s.key) > 0
}

// Issue mints a token binding userID until the returned expiry.
func (s *SessionSigner) Issue(userID string) (string, time.Time) {
	expiresAt := s.now().Add(s.ttl).UTC()
	payload := base64.RawURLEncoding.EncodeToString(
		[]byte(userID + "|" + strconv.FormatInt(expiresAt.Unix(), 10)),
	)
	return SessionTokenPrefix + payload + "." + s.sign(payload), expiresAt
}

// Verify checks a token's signature and expiry and returns the bound user ID.
func (s *SessionSigner) Verify(token string) (string, bool) {
	if !s.Enabled() || !strings.HasPrefix(token, SessionTokenPrefix) {
		return "", false
	}
	payload, signature, ok := strings.Cut(strings.TrimPrefix(token, SessionTokenPrefix), ".")
	if !ok {
		return "", false
	}
	got, err := hex.DecodeString(signature)
	if err != nil {
		return "", false
	}
	if !hmac.Equal(got, s.mac(payload)) {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", false
	}
	userID, exp, ok := strings.Cut(string(raw), "|")
	if !ok || userID == "" {
		return "", false
	}
	expiresUnix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || s.now().Unix() >= expiresUnix {
		return "", false
	}
	return userID, true
}

func (s *SessionSigner) sign(payload string) string {
	return hex.EncodeToString(s.mac(payload))
}

func (s *SessionSigner) mac(payload string) []byte {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}

// IdentityLookup is the store subset the identity middleware needs.
type IdentityLookup interface {
	GetUserByID(ctx context.Context, id string) (store.User, error)
	GetAPIKeyByHash(ctx context.Context, hash string) (store.APIKey, error)
	TouchAPIKey(ctx context.Context, id string) error
}

// APIKeyLookup is the subset of the store the rate limiter needs. It is a
// narrower view of IdentityLookup, kept for callers that only validate keys.
type APIKeyLookup interface {
	GetAPIKeyByHash(ctx context.Context, hash string) (store.APIKey, error)
	TouchAPIKey(ctx context.Context, id string) error
}

// IdentityOptions configures Identity.
type IdentityOptions struct {
	// Sessions verifies session bearer tokens. Nil disables them.
	Sessions *SessionSigner
	// ProxySecret enables signed trusted-proxy headers when non-empty. The
	// proxy must send identity-user-id, identity-timestamp and
	// identity-signature headers (see ProxySignature).
	ProxySecret string
	// ProxyMaxSkew bounds the accepted clock difference for proxy headers.
	// Zero uses DefaultProxyMaxSkew.
	ProxyMaxSkew time.Duration
	// Now overrides the clock in tests. Nil uses time.Now.
	Now func() time.Time
}

// Identity attaches a VerifiedIdentity to the request context when the
// request carries a credential this service can verify. It never rejects a
// request: routes decide separately whether they need scopes or a role, and
// unverified headers simply produce no identity.
func Identity(lookup IdentityLookup, opts IdentityOptions, logger *slog.Logger) func(http.Handler) http.Handler {
	if opts.ProxyMaxSkew <= 0 {
		opts.ProxyMaxSkew = DefaultProxyMaxSkew
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, exists := IdentityFromContext(r.Context()); exists {
				next.ServeHTTP(w, r)
				return
			}
			if id, ok := resolveIdentity(r, lookup, opts, logger); ok {
				r = r.WithContext(WithIdentity(r.Context(), id))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func resolveIdentity(r *http.Request, lookup IdentityLookup, opts IdentityOptions, logger *slog.Logger) (VerifiedIdentity, bool) {
	token := extractAPIKey(r)
	if token != "" {
		if strings.HasPrefix(token, SessionTokenPrefix) {
			return resolveSession(r, lookup, opts, token)
		}
		return resolveAPIKey(r, lookup, token, logger)
	}
	if opts.ProxySecret != "" {
		return resolveTrustedProxy(r, lookup, opts)
	}
	return VerifiedIdentity{}, false
}

func resolveSession(r *http.Request, lookup IdentityLookup, opts IdentityOptions, token string) (VerifiedIdentity, bool) {
	if opts.Sessions == nil {
		return VerifiedIdentity{}, false
	}
	userID, ok := opts.Sessions.Verify(token)
	if !ok {
		return VerifiedIdentity{}, false
	}
	user, err := lookup.GetUserByID(r.Context(), userID)
	if err != nil {
		return VerifiedIdentity{}, false
	}
	return VerifiedIdentity{
		UserID: user.ID,
		Role:   user.Role,
		Source: IdentitySourceSession,
	}, true
}

func resolveAPIKey(r *http.Request, lookup IdentityLookup, token string, logger *slog.Logger) (VerifiedIdentity, bool) {
	key, err := lookup.GetAPIKeyByHash(r.Context(), store.HashKey(token))
	if err != nil || key.Revoked() {
		return VerifiedIdentity{}, false
	}
	if err := lookup.TouchAPIKey(r.Context(), key.ID); err != nil && logger != nil {
		logger.Warn("touch api key", "err", err, "key_id", key.ID)
	}

	id := VerifiedIdentity{Source: IdentitySourceAPIKey, APIKey: &key}
	if key.UserID != nil && *key.UserID != "" {
		user, err := lookup.GetUserByID(r.Context(), *key.UserID)
		if err != nil {
			// The key authenticates scopes, but its bound user is gone.
			return id, true
		}
		id.UserID = user.ID
		id.Role = user.Role
	}
	return id, true
}

func resolveTrustedProxy(r *http.Request, lookup IdentityLookup, opts IdentityOptions) (VerifiedIdentity, bool) {
	userID := strings.TrimSpace(r.Header.Get(proxyUserIDHeader))
	timestamp := strings.TrimSpace(r.Header.Get(proxyTimestampHeader))
	signature := strings.TrimSpace(r.Header.Get(proxySignatureHeader))
	if userID == "" || timestamp == "" || signature == "" {
		return VerifiedIdentity{}, false
	}
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return VerifiedIdentity{}, false
	}
	skew := opts.Now().Sub(time.Unix(unix, 0))
	if skew < 0 {
		skew = -skew
	}
	if skew > opts.ProxyMaxSkew {
		return VerifiedIdentity{}, false
	}
	got, err := hex.DecodeString(signature)
	if err != nil {
		return VerifiedIdentity{}, false
	}
	if !hmac.Equal(got, ProxyMAC(opts.ProxySecret, r.Method, r.URL.RequestURI(), userID, timestamp)) {
		return VerifiedIdentity{}, false
	}
	user, err := lookup.GetUserByID(r.Context(), userID)
	if err != nil {
		return VerifiedIdentity{}, false
	}
	return VerifiedIdentity{
		UserID: user.ID,
		Role:   user.Role,
		Source: IdentitySourceProxy,
	}, true
}

// ProxyMAC returns the HMAC-SHA256 a trusted proxy must send in
// X-Identity-Signature for the given request and identity. The canonical
// string is method, escaped request URI, user id and unix timestamp, each
// separated by a newline. Exporting it keeps the proxy's implementation and
// this verifier from drifting.
func ProxyMAC(secret, method, requestURI, userID, timestamp string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(method + "\n" + requestURI + "\n" + userID + "\n" + timestamp))
	return mac.Sum(nil)
}
