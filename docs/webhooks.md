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

## Example implementations

**Python**

```python
import hashlib, hmac, time

def verify(secret: str, signature_header: str, timestamp_header: str, body: bytes) -> bool:
    parts = dict(p.split("=", 1) for p in signature_header.split(","))
    ts = parts["t"]
    if ts != timestamp_header or abs(int(time.time()) - int(ts)) > 300:
        return False
    signed = f"{ts}.".encode() + body
    expected = hmac.new(secret.encode(), signed, hashlib.sha256).hexdigest()
    return any(hmac.compare_digest(sig, expected)
               for key, sig in (p.split("=", 1) for p in signature_header.split(","))
               if key == "v1")
```

**Go**

```go
func verify(secret string, sig, tsHeader string, body []byte) bool {
    ts, sigs, err := webhooksig.ParseSignatureHeader(sig)
    if err != nil || tsHeader != strconv.FormatInt(ts, 10) {
        return false
    }
    if d := time.Since(time.Unix(ts, 0)); d > 5*time.Minute || d < -5*time.Minute {
        return false
    }
    expected := webhooksig.Sign(secret, ts, body)
    for _, s := range sigs {
        if hmac.Equal([]byte(s), []byte(expected)) {
            return true
        }
    }
    return false
}
```

`services/indexer/internal/webhooksig` is the executable reference for this
algorithm (used by both the sender and the tests).

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
- A subscription with no signing secret is never delivered to; unsigned payloads
  are never sent.
