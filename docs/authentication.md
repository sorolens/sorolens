# Authentication and API Keys Guide

Sorolens uses a unified, provable authentication and authorization model designed to support both anonymous public dashboard browsing and secure, fine-grained access for automated services, scripts, and administrators.

This guide explains:
- **Authentication**: How requests prove their identity.
- **Scopes**: Permission boundaries attached to API keys.
- **Roles (RBAC)**: User privileges (`viewer`, `contributor`, `admin`).
- **API Key Lifecycle**: Creating, listing, inspecting, rotating, and revoking API keys with `curl`.
- **Workspace Sessions**: Self-service ephemeral credentials for interactive dashboards.
- **Error Responses**: Standard HTTP status codes and error payloads.

---

## 1. Authentication Model

Callers prove their identity through credentials verified by the server-side `Identity` middleware (`apps/api/internal/middleware/identity.go`). Unverifiable headers are ignored.

> [!IMPORTANT]
> **Raw identity headers are never trusted:** As of issue #518, bare headers such as `X-User-ID`, `X-GitHub-Login`, `X-User-Sub`, or `X-GitHub-ID` are ignored by the server. Callers must present either a signed session token, a verified API key, or cryptographic proxy signatures.

Sorolens recognizes three valid credential sources:

| Source | Prefix / Format | Description |
|---|---|---|
| **Scoped API Key** | `sl_...` | High-entropy random token (approx. 256 bits). Looked up by its SHA-256 hash in the database. Can be unbound (scopes only) or bound to a user ID. |
| **Workspace Session** | `sls_...` | HMAC-SHA256 signed bearer token minted by `POST /api/v1/auth/session`. Bound to an ephemeral workspace user with the `contributor` role. Default TTL is 30 days. |
| **Trusted Proxy** | Signed Headers | Used when deployed behind an authenticating gateway. Requires `X-Identity-User-ID`, `X-Identity-Timestamp`, and `X-Identity-Signature` (HMAC-SHA256 verified within a 5-minute skew window). |

---

## 2. Authorization: Scopes vs. Roles

Sorolens enforces two orthogonal access-control layers:

```
Request ---> [Identity Middleware]  (Extracts & verifies credential: API key, session, or proxy)
                    |
                    v
             [RequireScopes]        (Validates API key has required per-route scope)
                    |
                    v
             [RequireRole]          (Validates user role meets or exceeds route minimum)
                    |
                    v
             [Route Handler]
```

### Scopes (API Key Permissions)

Scopes define what actions an API key is authorized to perform. Scopes are attached directly to API keys when they are created.

| Scope | Description | Typical Endpoints |
|---|---|---|
| `read:contracts` | Read-only access to contracts, events, invocations, storage snapshots, global statistics, search, and traces. | `GET /api/v1/contracts`, `GET /api/v1/events`, `GET /api/v1/stats/*` |
| `write:contracts` | Mutating actions on contracts, tags, alert rules, and watched accounts. | `POST /api/v1/contracts`, `POST /api/v1/rules`, `POST /api/v1/contracts/{id}/tags` |
| `read:watchdog` | Read-only access to on-chain watchdog health checks, monitored contracts, alerts, and watchdog statistics. | `GET /api/v1/watchdog/*` |
| `admin:*` | Super-scope granting full access across all namespaces and satisfying all role requirements. | `GET /api/v1/api-keys`, `POST /api/v1/api-keys`, `GET /api/v1/admin/*` |

#### Scope Wildcards
The scope checker supports namespace wildcards:
- `read:*` satisfies `read:contracts` and `read:watchdog`.
- `admin:*` or `*` satisfies every scope requirement across the entire API.

#### Public Surface Policy
To enable open public dashboard browsing without requiring sign-in:
- **Read & Write Routes**: Unauthenticated requests (requests without any credential) are allowed through on standard read and write routes.
- **Admin Routes**: Endpoints under `/api/v1/api-keys` and `/api/v1/admin/*` **always** require an authenticated credential with `admin:*` scope and `admin` role.
- **Invalid Credentials**: If a caller presents a token that is invalid, expired, or revoked, the request is rejected with `401 Unauthorized` rather than falling back to anonymous access.

---

### Roles (Role-Based Access Control)

Roles define the privilege level of an authenticated user. Role ranking is strictly hierarchical:

$$\text{viewer (rank 1)} < \text{contributor (rank 2)} < \text{admin (rank 3)}$$

| Role | Rank | Capabilities |
|---|:---:|---|
| **`viewer`** | 1 | Read-only access to dashboard data and public feeds. |
| **`contributor`** | 2 | Everything in `viewer`, plus creating contract notes, registering contracts, managing personal watchlists and groups, creating alert rules, and configuring webhook subscriptions. |
| **`admin`** | 3 | Full administrative power: creating, inspecting, rotating, and revoking API keys, revealing/rotating webhook signing secrets, accessing `/api/v1/admin/audit`, and querying `/debug/pprof/*`. |

#### How Keys and Roles Interact
1. **Unbound API Key**: An API key created without a `user_id` authenticates scopes only. It cannot act as a user on routes that require personal user context (such as contract notes or user watchlists).
2. **User-Bound API Key**: An API key created with `user_id` inherits that user's role in addition to its own scopes.
3. **Admin Keys**: Any key holding the `admin:*` scope automatically satisfies every role requirement (including `admin`), even if it is not bound to a user row. This allows operators to bootstrap the first admin key and rotate keys without external user dependencies.

---

## 3. How to Send Authenticated Requests

You can supply credentials using HTTP headers or query parameters:

### Method 1: Bearer Token Header (Recommended)

Include the token in the `Authorization` header:

```bash
curl -H "Authorization: Bearer sl_live_your_api_key_here" \
  https://api.sorolens.xyz/api/v1/contracts
```

### Method 2: X-API-Key Header

Send the key in the `X-API-Key` header:

```bash
curl -H "X-API-Key: sl_live_your_api_key_here" \
  https://api.sorolens.xyz/api/v1/contracts
```

### Method 3: Query Parameter

Pass the token in the `api_key` query parameter (useful for quick testing or webhook callbacks):

```bash
curl "https://api.sorolens.xyz/api/v1/contracts?api_key=sl_live_your_api_key_here"
```

> [!TIP]
> If both an `Authorization: Bearer` header and an `api_key` query parameter are present, the `Authorization` header takes precedence.

---

## 4. Managing API Keys

Managing API keys requires an authenticated caller holding the `admin:*` scope and `admin` role.

### 4.1 Create an API Key

```http
POST /api/v1/api-keys
Content-Type: application/json
Authorization: Bearer <admin_token>
```

#### Request Payload
```json
{
  "name": "monitoring-bot",
  "scopes": ["read:contracts", "read:watchdog"],
  "user_id": "optional_user_id",
  "allowed_cidrs": ["203.0.113.0/24"],
  "allowed_origins": ["https://app.example.com"]
}
```

The two allowlist fields are optional; see [4.5](#45-restrict-a-key-by-source-address-and-origin).

#### curl Example
```bash
curl -X POST https://api.sorolens.xyz/api/v1/api-keys \
  -H "Authorization: Bearer sl_admin_secret_token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ci-indexer-worker",
    "scopes": ["read:contracts", "write:contracts"]
  }'
```

#### Response (`201 Created`)
```json
{
  "id": "a1b2c3d4e5f60718293a4b5c6d7e8f90",
  "name": "ci-indexer-worker",
  "key_prefix": "sl_a1b2c3d4",
  "scopes": [
    "read:contracts",
    "write:contracts"
  ],
  "user_id": null,
  "created_at": "2026-10-18T12:00:00Z",
  "last_used_at": null,
  "revoked_at": null,
  "rotated_at": null,
  "key": "sl_AbCdEf1234567890-_xyzAbCdEf1234567890-_xyz"
}
```

> [!CAUTION]
> **Save the `key` field immediately.** The plaintext key is returned **only once** upon creation or rotation. The server only stores the SHA-256 hash (`key_hash`) and cannot recover the plaintext token.

---

### 4.2 List API Keys

Retrieve a cursor-paginated list of all API keys (newest first). Plaintext tokens are omitted from listing responses.

```http
GET /api/v1/api-keys?limit=10&cursor=<cursor>
Authorization: Bearer <admin_token>
```

#### curl Example
```bash
curl -H "Authorization: Bearer sl_admin_secret_token" \
  "https://api.sorolens.xyz/api/v1/api-keys?limit=2"
```

#### Response (`200 OK`)
```json
{
  "api_keys": [
    {
      "id": "a1b2c3d4e5f60718293a4b5c6d7e8f90",
      "name": "ci-indexer-worker",
      "key_prefix": "sl_a1b2c3d4",
      "scopes": ["read:contracts", "write:contracts"],
      "user_id": null,
      "created_at": "2026-10-18T12:00:00Z",
      "last_used_at": "2026-10-18T12:30:00Z",
      "revoked_at": null,
      "rotated_at": null
    }
  ],
  "next_cursor": "ZXhhbXBsZS1jdXJzb3I="
}
```

---

### 4.3 Inspect an API Key

Retrieve metadata for a specific key by its ID:

```http
GET /api/v1/api-keys/{id}
Authorization: Bearer <admin_token>
```

#### curl Example
```bash
curl -H "Authorization: Bearer sl_admin_secret_token" \
  https://api.sorolens.xyz/api/v1/api-keys/a1b2c3d4e5f60718293a4b5c6d7e8f90
```

#### Response (`200 OK`)
```json
{
  "id": "a1b2c3d4e5f60718293a4b5c6d7e8f90",
  "name": "ci-indexer-worker",
  "key_prefix": "sl_a1b2c3d4",
  "scopes": ["read:contracts", "write:contracts"],
  "user_id": null,
  "created_at": "2026-10-18T12:00:00Z",
  "last_used_at": "2026-10-18T12:30:00Z",
  "revoked_at": null,
  "rotated_at": null
}
```

---

### 4.4 Rotate an API Key

Regenerate a key's secret token in-place without changing its ID, name, or scopes. The old token is invalidated immediately and the new token is returned once.

```http
POST /api/v1/api-keys/{id}/rotate
Authorization: Bearer <admin_token>
```

#### curl Example
```bash
curl -X POST https://api.sorolens.xyz/api/v1/api-keys/a1b2c3d4e5f60718293a4b5c6d7e8f90/rotate \
  -H "Authorization: Bearer sl_admin_secret_token"
```

#### Response (`200 OK`)
```json
{
  "id": "a1b2c3d4e5f60718293a4b5c6d7e8f90",
  "name": "ci-indexer-worker",
  "key_prefix": "sl_f9e8d7c6",
  "scopes": ["read:contracts", "write:contracts"],
  "user_id": null,
  "created_at": "2026-10-18T12:00:00Z",
  "last_used_at": "2026-10-18T12:30:00Z",
  "revoked_at": null,
  "rotated_at": "2026-10-18T13:45:00Z",
  "key": "sl_NewRotatedSecretToken123456789-_xyzABC"
}
```

---

### 4.5 Restrict a Key by Source Address and Origin

A key can be limited to a list of CIDR ranges and, for browser use, a list of
origins (issue #793). A leaked key that only works from the owner's servers is
far less dangerous than an unrestricted one.

Both lists are optional and start empty. An empty list means unrestricted;
this is the default and keeps existing keys unchanged.

- `allowed_cidrs` — CIDR ranges (IPv4 or IPv6). A bare address is rejected;
  each range is stored in network form (`203.0.113.7/24` becomes
  `203.0.113.0/24`).
- `allowed_origins` — browser origins as `scheme://host[:port]` with an
  `http` or `https` scheme. Paths, queries, fragments and embedded credentials
  are rejected. Matching is case-insensitive on scheme and host, and an entry
  written with the scheme's default port (`https://app.example.com:443`,
  `http://dev.example.com:80`) also matches the browser's port-less origin.
  Non-default ports stay distinct, and the scheme is significant.

Pass either list on `POST /api/v1/api-keys` (or `POST /api/v1/me/api-keys`),
or change them later with `PATCH /api/v1/api-keys/{id}` (or
`PATCH /api/v1/me/api-keys/{id}` for a key the session user owns). On PATCH,
an omitted field is left unchanged and `[]` or `null` clears the restriction.

```bash
curl -X PATCH https://api.sorolens.xyz/api/v1/api-keys/a1b2c3d4e5f60718293a4b5c6d7e8f90 \
  -H "Authorization: Bearer sl_admin_secret_token" \
  -H "Content-Type: application/json" \
  -d '{
    "allowed_cidrs": ["203.0.113.0/24", "2001:db8::/32"],
    "allowed_origins": ["https://app.example.com"]
  }'
```

A request that presents the key from outside its allowlist is rejected before
the route runs, with the standard error envelope and a distinct code:

| Code | Meaning |
|---|---|
| `API_KEY_IP_NOT_ALLOWED` | The resolved client address is outside `allowed_cidrs`. |
| `API_KEY_ORIGIN_NOT_ALLOWED` | The request's `Origin` is missing from or not in `allowed_origins`. |

```http
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "error": {
    "code": "API_KEY_IP_NOT_ALLOWED",
    "message": "API key is not allowed from this IP address",
    "request_id": "req_5f7c3e1b8a9d0246"
  }
}
```

#### What the Origin restriction does and does not do

An origin allowlist is a browser-enforced restriction, not an authentication
factor. Browsers set `Origin` themselves and refuse to let page script forge
it, so the check keeps a key embedded in a web app from being replayed by a
page on another origin. `Origin` is still just a request header: any
non-browser client (curl, a script, a stolen key used from a server) can send
whatever `Origin` value it likes. Origin restrictions therefore do **not**
protect a leaked key on their own, and they are not a substitute for keeping
the key secret. Use `allowed_cidrs` for a network-level restriction, which is
enforced against the resolved connection address rather than a header the
caller controls.

An origin-restricted key is rejected when the request carries no `Origin`
header at all, because the check fails closed. That is deliberate: a
header-less request cannot prove it came from an allowed browser origin.
Consequently an origin-restricted key cannot be used for ordinary
server-to-server requests, which normally send no `Origin`; give such a key no
`allowed_origins` and restrict it by `allowed_cidrs` instead.

#### Resolving the client address behind a proxy

The address is the transport peer by default. `X-Forwarded-For` is only
believed when the peer is inside `TRUSTED_PROXY_CIDRS` (comma-separated
CIDRs); every forwarded line is joined and the rightmost hop that is not
itself a trusted proxy is used. Repeated header lines are therefore treated as
one chain, and a malformed entry to the right of the first untrusted hop makes
the address unresolvable: the request is rejected rather than falling back to
a value the caller may have chosen. A trusted-proxy list that is invalid or
covers every address (`0.0.0.0/0`, `::/0`) stops the API at startup, and an
empty list is the safe default: the transport peer is used and
`X-Forwarded-For` is ignored. Set the variable to your load balancer or CDN
ranges when the API runs behind one; otherwise leave it empty. Hosted
deployments read the same variable, so it must be set wherever the API is
deployed (for Vercel, in the project's environment variables).

#### Who may change a key's allowlist

Key-management mutations currently accept either a workspace session or an
API key: `POST`/`PATCH /api/v1/me/api-keys` need the contributor role, and
`/api/v1/api-keys` needs the admin role, and a bound API key inherits its
user's role. That means a leaked bounded key can create a new key with no
allowlists, or PATCH its own allowlists to `[]`, and the restriction is gone
even if the original key is later revoked. This is the existing authentication
model and this change does not alter it. A safer model, recommended as a
follow-up, is to accept only a session identity for key creation and allowlist
changes (and only an admin session for `/api/v1/api-keys`), leaving API-key
authentication for read-only key metadata. Until then, treat a bound key as
being able to manage keys for its user and scope it accordingly.

---

### 4.6 Revoke an API Key

Immediately deactivate an API key. Revoked keys cannot be rotated or used to authenticate.

```http
DELETE /api/v1/api-keys/{id}
Authorization: Bearer <admin_token>
```

#### curl Example
```bash
curl -X DELETE https://api.sorolens.xyz/api/v1/api-keys/a1b2c3d4e5f60718293a4b5c6d7e8f90 \
  -H "Authorization: Bearer sl_admin_secret_token"
```

#### Response (`204 No Content`)
```http
HTTP/1.1 204 No Content
```

---

## 5. Workspace Session Tokens

The web dashboard uses self-service workspace sessions to provide interactive features (managing watchlists, contract portfolios, contract notes, and custom tags) without forcing user login or OAuth setup.

### Minting a Session Token

```http
POST /api/v1/auth/session
```

#### curl Example
```bash
curl -X POST https://api.sorolens.xyz/api/v1/auth/session
```

#### Response (`201 Created`)
```json
{
  "user_id": "sess_89f02c61e4b8593a1234567890abcdef",
  "token": "sls_eyJhbGciOiJIUzI1NiJ9.payload.sig",
  "expires_at": "2026-11-17T12:00:00Z"
}
```

The resulting `token` should be passed as `Authorization: Bearer sls_...` in subsequent requests. It automatically grants the `contributor` role bound to that ephemeral workspace.

---

## 6. Error Responses

When an authentication or authorization check fails, the API responds with a structured JSON error body.

### 401 Unauthorized

#### Scenario A: Missing credential on an admin-gated endpoint
```bash
curl -i https://api.sorolens.xyz/api/v1/api-keys
```
```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "error": "authentication required"
}
```

#### Scenario B: Invalid, revoked, or expired API key
```bash
curl -i -H "Authorization: Bearer sl_revoked_or_invalid_key" \
  https://api.sorolens.xyz/api/v1/contracts
```
```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "error": "invalid API key"
}
```

---

### 403 Forbidden

#### Scenario A: API key missing required scope
Attempting to register a contract using a key that only has `read:contracts`:
```bash
curl -i -X POST https://api.sorolens.xyz/api/v1/contracts \
  -H "Authorization: Bearer sl_read_only_key" \
  -H "Content-Type: application/json" \
  -d '{"id": "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC", "network": "testnet"}'
```
```http
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "error": "missing scope",
  "required": "write:contracts"
}
```

#### Scenario B: Insufficient role
A caller holding the `viewer` role attempting to perform an action requiring `contributor`:
```http
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "error": "insufficient role",
  "required": "contributor",
  "role": "viewer"
}
```

#### Scenario C: Unbound key on a user-gated endpoint
An API key with `write:contracts` that is not bound to a `user_id` attempting to author a personal contract note:
```http
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "error": "insufficient role",
  "required": "contributor"
}
```

---

### 422 Unprocessable Entity

Triggered when request payload validation fails:

```bash
curl -i -X POST https://api.sorolens.xyz/api/v1/api-keys \
  -H "Authorization: Bearer sl_admin_key" \
  -H "Content-Type: application/json" \
  -d '{"name": "test-key", "scopes": ["invalid:scope"]}'
```
```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/json

{
  "error": {
    "code": "INVALID_INPUT",
    "message": "unknown scope invalid:scope; valid scopes are: read:contracts, write:contracts, read:watchdog, admin:*",
    "request_id": "req_5f7c3e1b8a9d0246"
  }
}
```

---

### 404 Not Found

Triggered when referencing a non-existent API key ID:

```bash
curl -i -X GET https://api.sorolens.xyz/api/v1/api-keys/non_existent_id \
  -H "Authorization: Bearer sl_admin_key"
```
```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "error": {
    "code": "NOT_FOUND",
    "message": "API key not found",
    "request_id": "req_8b1a4c9e2d3f5701"
  }
}
```

---

## 7. Summary Checklist for Integration

When integrating client applications or SDKs with Sorolens:
- [ ] Send keys via `Authorization: Bearer sl_...` or query parameter `?api_key=sl_...`.
- [ ] For background workers or monitoring bots, create scoped keys granting only necessary permissions (e.g. `read:contracts`, `read:watchdog`).
- [ ] Store generated plaintext keys securely upon creation; they cannot be retrieved again.
- [ ] Handle `401` (invalid key) and `403` (missing scope or role) gracefully in client retries.
- [ ] For browser sessions, mint workspace session tokens via `POST /api/v1/auth/session`.
