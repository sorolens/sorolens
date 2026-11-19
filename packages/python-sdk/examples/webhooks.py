"""Verify the HMAC-SHA256 signature on incoming Sorolens webhook deliveries.

Wire format (see ``docs/webhooks.md``)::

    X-Sorolens-Timestamp: <unix seconds>
    X-Sorolens-Signature: t=<unix seconds>,v1=<hex HMAC-SHA256>

    signed_payload = "<timestamp>." + raw_body
    signature      = hex(HMAC_SHA256(signing_secret, signed_payload))

The signing secret is the raw ``whsec_...`` string. Timestamps outside a
±5 minute window are rejected to stop replayed deliveries. ``sorolens.webhooks``
implements both the signing and verification sides.

This example signs a delivery and verifies it end to end, so it runs offline::

    python examples/webhooks.py
"""

from __future__ import annotations

import os
import time

from sorolens.webhooks import (
    WebhookVerificationError,
    sign,
    signature_header,
    verify,
)

SECRET = os.environ.get("SOROLENS_WEBHOOK_SECRET", "whsec_example_secret")


def expect_rejection(label: str, *args, **kwargs) -> None:
    try:
        verify(*args, **kwargs)
    except WebhookVerificationError as exc:
        print(f"  rejected ({label}): {exc}")
        return
    raise AssertionError(f"{label}: signature should have been rejected")


def main() -> int:
    body = b'{"event":"contract.alert","contract_id":"CDEMO"}'
    now = int(time.time())
    header = signature_header(now, sign(SECRET, now, body))

    verify(SECRET, header, str(now), body, now=now)
    print(f"  accepted: {header[:24]}...")

    expect_rejection("tampered body", SECRET, header, str(now), b'{"event":"evil"}', now=now)
    expect_rejection("wrong secret", "whsec_other", header, str(now), body, now=now)

    stale = now - 600
    expect_rejection(
        "replayed delivery",
        SECRET,
        signature_header(stale, sign(SECRET, stale, body)),
        str(stale),
        body,
        now=now,
    )

    print("webhook signature verification passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
