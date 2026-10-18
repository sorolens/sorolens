"""Verify the HMAC-SHA256 signature on incoming Sorolens webhook deliveries.

Wire format (see ``docs/webhooks.md``)::

    X-Sorolens-Timestamp: <unix seconds>
    X-Sorolens-Signature: t=<unix seconds>,v1=<hex HMAC-SHA256>

    signed_payload = "<timestamp>." + raw_body
    signature      = hex(HMAC_SHA256(signing_secret, signed_payload))

The signing secret is the raw ``whsec_...`` string. Timestamps outside a
±5 minute window are rejected to stop replayed deliveries.

This example signs a delivery and verifies it end to end, so it runs offline::

    python examples/webhooks.py
"""

from __future__ import annotations

import hashlib
import hmac
import os
import time

SECRET = os.environ.get("SOROLENS_WEBHOOK_SECRET", "whsec_example_secret")
TOLERANCE_SECONDS = 300


def sign(secret: str, raw_body: bytes, timestamp: int) -> str:
    """Produce the value of ``X-Sorolens-Signature`` for a delivery."""
    signed_payload = f"{timestamp}.".encode() + raw_body
    digest = hmac.new(secret.encode(), signed_payload, hashlib.sha256)
    return f"t={timestamp},v1={digest.hexdigest()}"


def verify(
    secret: str,
    raw_body: bytes,
    timestamp: str,
    signature_header: str,
    *,
    now: int | None = None,
) -> None:
    """Raise ``ValueError`` unless the delivery is authentic and fresh."""
    try:
        ts = int(timestamp)
    except ValueError as exc:
        raise ValueError("invalid X-Sorolens-Timestamp") from exc

    current = int(time.time()) if now is None else now
    if abs(current - ts) > TOLERANCE_SECONDS:
        raise ValueError("signature timestamp outside the tolerance window")

    parts = dict(
        piece.split("=", 1) for piece in signature_header.split(",") if "=" in piece
    )
    received = parts.get("v1")
    if not received:
        raise ValueError("signature header is missing the v1 value")

    expected = sign(secret, raw_body, ts).split("v1=", 1)[1]
    if not hmac.compare_digest(expected, received.lower()):
        raise ValueError("signature mismatch")


def expect_rejection(label: str, *args, **kwargs) -> None:
    try:
        verify(*args, **kwargs)
    except ValueError as exc:
        print(f"  rejected ({label}): {exc}")
        return
    raise AssertionError(f"{label}: signature should have been rejected")


def main() -> int:
    body = b'{"event":"contract.alert","contract_id":"CDEMO"}'
    now = int(time.time())
    header = sign(SECRET, body, now)

    verify(SECRET, body, str(now), header, now=now)
    print(f"  accepted: {header[:24]}...")

    expect_rejection("tampered body", SECRET, b'{"event":"evil"}', str(now), header, now=now)
    expect_rejection("wrong secret", "whsec_other", body, str(now), header, now=now)
    expect_rejection(
        "replayed delivery",
        SECRET,
        body,
        str(now - 600),
        sign(SECRET, body, now - 600),
        now=now,
    )

    print("webhook signature verification passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
