"""Verify a Sorolens webhook delivery (HMAC-SHA256, see docs/webhooks.md).

The receiver must verify the **raw request body** before parsing any JSON, compare
signatures in constant time, and reject timestamps outside a five-minute window.
This example needs no network or SDK calls and runs entirely offline::

    python examples/webhooks.py

In your own handler, pass the raw bytes of the request body and the two headers::

    verify(secret, headers["X-Sorolens-Signature"], headers["X-Sorolens-Timestamp"], raw_body)

and answer 401 when it raises :class:`InvalidSignature`.
"""

from __future__ import annotations

import argparse
import hashlib
import hmac
import time

TOLERANCE_SECONDS = 300

# Published test vector from docs/webhooks.md.
SECRET = "whsec_example_0123456789abcdef0123456789abcdef"
TIMESTAMP = "1700000000"
BODY = b'{"event":"watchdog.alert","severity":"Critical"}'
SIGNATURE = "97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615"


class InvalidSignature(ValueError):
    """The delivery could not be verified and must be rejected."""


def parse_signature_header(header: str) -> tuple[int, list[str]]:
    """Read ``t=<unix>[,v1=<hex>...]`` in any order; return the timestamp and v1 values."""
    timestamp: int | None = None
    signatures: list[str] = []
    for part in header.split(","):
        key, separator, value = part.strip().partition("=")
        if not separator:
            raise InvalidSignature("malformed X-Sorolens-Signature header")
        if key == "t":
            try:
                timestamp = int(value)
            except ValueError:
                raise InvalidSignature("malformed X-Sorolens-Signature header") from None
        elif key == "v1" and value:
            signatures.append(value)
    if timestamp is None:
        raise InvalidSignature("malformed X-Sorolens-Signature header")
    return timestamp, signatures


def sign(secret: str, timestamp: int, body: bytes) -> str:
    """Return hex(HMAC-SHA256(secret, "<timestamp>.<body>"))."""
    signed_payload = f"{timestamp}.".encode() + body
    return hmac.new(secret.encode(), signed_payload, hashlib.sha256).hexdigest()


def verify(
    secret: str,
    signature_header: str,
    timestamp_header: str,
    body: bytes,
    *,
    now: float | None = None,
) -> None:
    """Return normally when the delivery is authentic; raise :class:`InvalidSignature` if not."""
    timestamp, signatures = parse_signature_header(signature_header)
    if not signatures:
        raise InvalidSignature("no v1 signature present")
    if timestamp_header.strip() != str(timestamp):
        raise InvalidSignature("timestamp header does not match the signature header")
    if abs((time.time() if now is None else now) - timestamp) > TOLERANCE_SECONDS:
        raise InvalidSignature("timestamp outside the replay window")

    expected = sign(secret, timestamp, body)
    # During a key rotation the sender may include several v1 values: any match is enough.
    if not any(hmac.compare_digest(candidate, expected) for candidate in signatures):
        raise InvalidSignature("signature mismatch")


def _expect_rejected(reason: str, *args: object, **kwargs: object) -> None:
    try:
        verify(*args, **kwargs)  # type: ignore[arg-type]
    except InvalidSignature as exc:
        if reason not in str(exc):
            raise AssertionError(f"rejected for the wrong reason: {exc}") from exc
        return
    raise AssertionError(f"delivery was accepted but should fail with: {reason}")


def run() -> None:
    signed_at = float(TIMESTAMP)
    header = f"t={TIMESTAMP},v1={SIGNATURE}"

    verify(SECRET, header, TIMESTAMP, BODY, now=signed_at)
    print("pinned test vector verified")

    tampered = b'{"event":"watchdog.alert","severity":"Critical","extra":true}'
    _expect_rejected("signature mismatch", SECRET, header, TIMESTAMP, tampered, now=signed_at)
    print("tampered body rejected")

    _expect_rejected(
        "replay window", SECRET, header, TIMESTAMP, BODY, now=signed_at + TOLERANCE_SECONDS + 60
    )
    print("stale delivery rejected")

    rotated = f"t={TIMESTAMP},v1={'0' * 64},v1={SIGNATURE}"
    verify(SECRET, rotated, TIMESTAMP, BODY, now=signed_at)
    print("rotated signature accepted")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--self-test", action="store_true", help="accepted for consistency")
    parser.parse_args(argv)
    try:
        run()
    except AssertionError as exc:
        print(f"webhooks: {exc}")
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
