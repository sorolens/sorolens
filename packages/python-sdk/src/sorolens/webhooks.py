"""Verify incoming Sorolens webhook deliveries.

Every delivery is an HTTP ``POST`` carrying:

.. code-block:: text

    X-Sorolens-Timestamp: <unix seconds>
    X-Sorolens-Signature: t=<unix seconds>,v1=<hex HMAC-SHA256>

The signed payload is the timestamp, a ``.`` separator and the exact request
body::

    signed_payload = "<timestamp>." + raw_body
    signature      = hex(HMAC_SHA256(signing_secret, signed_payload))

The signing secret is the full ``whsec_...`` string, used as raw UTF-8 bytes.
Timestamps outside the documented five-minute window are rejected so a captured
delivery cannot be replayed. See ``docs/webhooks.md`` for the protocol and the
shared test vector.

Framework handlers can verify headers directly::

    from sorolens.webhooks import WebhookVerificationError, verify_request

    try:
        verify_request(SECRET, request.headers, request.body)
    except WebhookVerificationError:
        return Response(status=401)
"""

from __future__ import annotations

import hashlib
import hmac
import time
from collections.abc import Mapping

SIGNATURE_HEADER = "X-Sorolens-Signature"
TIMESTAMP_HEADER = "X-Sorolens-Timestamp"
SIGNATURE_SCHEME = "v1"

#: Replay window applied when the caller does not supply one, in seconds.
DEFAULT_TOLERANCE = 300


class WebhookVerificationError(ValueError):
    """A delivery failed signature or replay verification.

    Treat it as a ``401``/``403``: discard the payload and do not process it.
    """


def sign(secret: str, timestamp: int, body: bytes) -> str:
    """Return the hex ``v1`` signature for ``body`` at ``timestamp``."""
    signed_payload = f"{timestamp}.".encode() + body
    return hmac.new(secret.encode(), signed_payload, hashlib.sha256).hexdigest()


def signature_header(timestamp: int, signature: str) -> str:
    """Render the value of ``X-Sorolens-Signature``."""
    return f"t={timestamp},{SIGNATURE_SCHEME}={signature}"


def verify(
    secret: str,
    signature_header: str,
    timestamp_header: str,
    body: bytes,
    *,
    now: int | None = None,
    tolerance: int = DEFAULT_TOLERANCE,
) -> None:
    """Verify a delivery, raising :class:`WebhookVerificationError` on failure.

    ``body`` must be the exact raw request body bytes, before any JSON parsing
    or re-serialisation. ``now`` defaults to the current Unix time and exists so
    callers and tests can pin the clock; ``tolerance`` is the replay window in
    seconds (``0`` or less selects :data:`DEFAULT_TOLERANCE`).
    """
    if tolerance <= 0:
        tolerance = DEFAULT_TOLERANCE

    timestamp, signatures = _parse_signature_header(signature_header)
    if not signatures:
        raise WebhookVerificationError("signature header carries no v1 signature")

    try:
        header_timestamp = int(timestamp_header.strip())
    except (AttributeError, ValueError) as exc:
        raise WebhookVerificationError("missing or non-numeric X-Sorolens-Timestamp") from exc
    if header_timestamp != timestamp:
        raise WebhookVerificationError("X-Sorolens-Timestamp does not match the signed timestamp")

    current = int(time.time()) if now is None else now
    if abs(current - timestamp) > tolerance:
        raise WebhookVerificationError("signature timestamp outside the replay window")

    expected = sign(secret, timestamp, body)
    for candidate in signatures:
        if hmac.compare_digest(candidate, expected):
            return
    raise WebhookVerificationError("signature mismatch")


def verify_request(
    secret: str,
    headers: Mapping[str, str],
    body: bytes,
    *,
    now: int | None = None,
    tolerance: int = DEFAULT_TOLERANCE,
) -> None:
    """Verify a delivery from a header mapping (case-insensitive keys)."""
    verify(
        secret,
        _header(headers, SIGNATURE_HEADER),
        _header(headers, TIMESTAMP_HEADER),
        body,
        now=now,
        tolerance=tolerance,
    )


def _header(headers: Mapping[str, str], name: str) -> str:
    wanted = name.lower()
    for key, value in headers.items():
        if key.lower() == wanted:
            return value
    return ""


def _parse_signature_header(header: str) -> tuple[int, list[str]]:
    if header is None or not header.strip():
        raise WebhookVerificationError("missing X-Sorolens-Signature header")

    timestamp: int | None = None
    signatures: list[str] = []
    for piece in header.split(","):
        key, separator, value = piece.strip().partition("=")
        if not separator:
            raise WebhookVerificationError("malformed X-Sorolens-Signature header")
        key = key.strip()
        value = value.strip()
        if key == "t":
            try:
                timestamp = int(value)
            except ValueError as exc:
                raise WebhookVerificationError(
                    "malformed timestamp in X-Sorolens-Signature"
                ) from exc
        elif key == SIGNATURE_SCHEME and value:
            signatures.append(value)

    if timestamp is None:
        raise WebhookVerificationError("X-Sorolens-Signature is missing the timestamp")
    return timestamp, signatures
