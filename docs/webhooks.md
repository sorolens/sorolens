# Webhook Signing and Verification

Sorolens signs every watchdog webhook delivery with HMAC-SHA256 so a receiver
can prove a payload came from Sorolens and was not modified in transit. The
scheme follows the same pattern as GitHub and Stripe.

## Headers

Each delivery is an HTTP `POST` with `Content-Type: application/json` and:

| Header | Example | Meaning |
|---|---|---|
| `X-Sorolens-Timestamp` | `1700000000` | Unix time (seconds) the delivery was signed. |
| `X-Sorolens-Signature` | `t=1700000000,v1=5f3a...` | Timestamp `t` and one or more `v1` signatures. |

More than one `v1` signature may be present (during a key rotation a sender can
include both the old and new signatures); accept the delivery if **any** of them
matches.

## Signing algorithm

1. Let `timestamp` be the current Unix time in **seconds**, as a decimal string.
2. Let `body` be the exact raw request body bytes (do not re-serialise the JSON
   before hashing).
3. Build the signed payload by joining the timestamp and the body with a single
   `.`:

   ```
   signed_payload = timestamp + "." + body
   ```

4. Compute the signature over that payload using HMAC-SHA256 with the
   subscription's signing secret as the key. The key is the full `whsec_...`
   string, used as raw UTF-8 bytes - do not strip the `whsec_` prefix or
   base64-decode it.

   ```
   signature = hex(HMAC_SHA256(key = signing_secret, message = signed_payload))
   ```

5. Send `X-Sorolens-Timestamp: <timestamp>` and
   `X-Sorolens-Signature: t=<timestamp>,v1=<signature>`.

## Worked example (test vector)

Pin your implementation against this vector before rolling it out. It is
reproduced by `services/indexer/internal/webhooksig` in
`TestSign_PublishedVector` and by the verification helpers in all three SDKs
(`packages/go-client/webhook`, `packages/python-sdk` and
`packages/rust-sdk/src/webhooks.rs`), so the published algorithm, the sender
and the SDKs cannot drift apart.

| Input | Value |
|---|---|
| signing secret | `whsec_example_0123456789abcdef0123456789abcdef` |
| timestamp | `1700000000` |
| raw body | `{"event":"watchdog.alert","severity":"Critical"}` |
| signed payload | `1700000000.{"event":"watchdog.alert","severity":"Critical"}` |
| expected `v1` signature | `97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615` |

A conforming delivery therefore carries:

```
X-Sorolens-Timestamp: 1700000000
X-Sorolens-Signature: t=1700000000,v1=97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615
```

## Verification (pseudocode)

```
function verify(secret, headers, raw_body, now):
    ts, signatures = parse_signature_header(headers["X-Sorolens-Signature"])
    # parse_signature_header splits on "," then "=" and collects
    # every value whose key is "v1". Order is not significant.

    if headers["X-Sorolens-Timestamp"] != ts:            # header present and agrees
        return REJECT

    if abs(now - ts) > 300 seconds:                      # five-minute replay window
        return REJECT

    signed_payload = ts + "." + raw_body
    expected = hex(HMAC_SHA256(secret, signed_payload))

    for each sig in signatures:
        if constant_time_equal(sig, expected):           # never use ==
            return ACCEPT

    return REJECT
```

Rules a correct receiver must follow:

- Compare signatures in **constant time** (e.g. `hmac.compare_digest`,
  `crypto.timingSafeEqual`). A plain string comparison leaks key material.
- Verify against the **raw** body bytes, before any JSON parsing or
  normalisation.
- Enforce the replay window: reject a timestamp more than **5 minutes** from your
  clock. Store recent delivery ids if you need strict one-time semantics.
- Return `401`/`403` and discard the payload when verification fails.

## Verification helpers

All three official SDKs ship a helper that implements the rules above,
including the five-minute replay window and constant-time comparison. Prefer
them over reimplementing the algorithm.

**Go** — `github.com/sorolens/go-client/webhook`

```go
import (
    "io"
    "net/http"
    "time"

    "github.com/sorolens/go-client/webhook"
)

func handler(secret string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Verify the raw bytes exactly as received.
        body, err := io.ReadAll(r.Body)
        if err != nil {
            http.Error(w, "cannot read body", http.StatusBadRequest)
            return
        }

        // A zero tolerance selects the documented five-minute window.
        if err := webhook.Verify(secret, r.Header, body, time.Now(), 0); err != nil {
            http.Error(w, "invalid signature", http.StatusUnauthorized)
            return
        }

        // Safe to process body.
        _ = body
    }
}
```

`webhook.Verify` reads `X-Sorolens-Signature` and `X-Sorolens-Timestamp` from
the header map. Use `webhook.VerifyHeaders` if your framework exposes the two
header values separately, and `webhook.Sign` to produce a delivery in tests.

**Python** — `sorolens.webhooks`

```python
from sorolens.webhooks import WebhookVerificationError, verify_request

SECRET = "whsec_your_signing_secret"

def handler(request):
    try:
        # Pass the exact raw body and the request headers (case-insensitive);
        # the default tolerance is the documented five minutes.
        verify_request(SECRET, request.headers, request.body)
    except WebhookVerificationError:
        return Response(status=401)

    # Safe to process request.body.
```

Use `verify(secret, signature_header, timestamp_header, body)` when you have
the two header values directly, and `sign(secret, timestamp, body)` to produce
a delivery in tests.

**Rust** — `sorolens_sdk::webhooks`

```rust
use sorolens_sdk::webhooks::{verify, SIGNATURE_HEADER, TIMESTAMP_HEADER};

fn handle(secret: &str, headers: &http::HeaderMap, body: &[u8]) -> Result<(), Box<dyn std::error::Error>> {
    let signature = headers[SIGNATURE_HEADER].to_str()?;
    let timestamp = headers[TIMESTAMP_HEADER].to_str()?;
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)?
        .as_secs() as i64;

    // `0` selects the documented five-minute replay window.
    verify(secret, signature, timestamp, body, now, 0)?;

    // Safe to process body.
    Ok(())
}
```

`verify` returns `Err(WebhookError::TimestampOutsideWindow)` for a stale or
future delivery and `Err(WebhookError::SignatureMismatch)` for a forged one.
`sign` and `signature_header` produce a delivery in tests.

`services/indexer/internal/webhooksig` is the executable reference the sender
uses; the SDK helpers implement the same published algorithm and share its
test vector.

## Secret lifecycle

A signing secret is generated when a subscription is created and is stored as a
`whsec_...` value (with its SHA-256 digest) on the subscription row.

| Endpoint | Purpose |
|---|---|
| `POST /api/v1/watchdog/subscriptions` | Creates a subscription. The response includes `signing_secret` **once**. |
| `GET /api/v1/watchdog/subscriptions/{id}/signing-secret` | Returns the secret only within **5 minutes** of creation or of the last rotation; otherwise `403 FORBIDDEN`. |
| `POST /api/v1/watchdog/subscriptions/{id}/rotate` | Generates a new secret, invalidates the old one for future deliveries, and returns the new plaintext once. |

All three require an admin credential. Copy the secret into your receiver's
configuration when it is shown; if you miss the window, rotate to mint a new one.

## Delivery behaviour

- Deliveries are `POST`ed to the subscription's `webhook_url` with a 10-second
  timeout.
- A `5xx` response is retried **once** with the same signed body and timestamp,
  so the retry verifies with the same signature.
- `4xx` responses are not retried.
- Deliveries only go to public hosts (SSRF protection). On every delivery the
  indexer resolves the host, refuses loopback, private, link-local and
  cloud-metadata addresses (including IPv4 carried inside IPv6), and connects
  to the address it checked, so a hostname whose DNS answer changes after the
  subscription was created cannot reach internal services. A host with even one
  forbidden address is refused, and the failure is logged.
- At most 3 redirects are followed, and a redirect to a private address is
  refused like any other delivery to one.
- Set `WEBHOOK_ALLOWED_HOSTS` (comma-separated, `*.example.com` wildcards) on
  the indexer to deliver only to specific hosts. Forbidden address ranges stay
  blocked even for allowlisted hosts.
- A subscription with no signing secret is never delivered to; unsigned payloads
  are never sent.
