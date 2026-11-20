"""Tests for the shared Sorolens webhook verification helper."""

from __future__ import annotations

import pytest

from sorolens.webhooks import (
    DEFAULT_TOLERANCE,
    SIGNATURE_HEADER,
    TIMESTAMP_HEADER,
    WebhookVerificationError,
    sign,
    signature_header,
    verify,
    verify_request,
)

# Shared vector from docs/webhooks.md, reproduced by the Go and Rust SDKs.
SECRET = "whsec_example_0123456789abcdef0123456789abcdef"
TIMESTAMP = 1700000000
BODY = b'{"event":"watchdog.alert","severity":"Critical"}'
SIGNATURE = "97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615"


def test_sign_matches_published_vector() -> None:
    assert sign(SECRET, TIMESTAMP, BODY) == SIGNATURE


def test_signature_header_matches_published_vector() -> None:
    assert (
        signature_header(TIMESTAMP, SIGNATURE)
        == f"t=1700000000,v1={SIGNATURE}"
    )


def test_verify_accepts_published_vector() -> None:
    verify(
        SECRET,
        signature_header(TIMESTAMP, SIGNATURE),
        str(TIMESTAMP),
        BODY,
        now=TIMESTAMP,
    )


def test_verify_rejects_tampered_body() -> None:
    with pytest.raises(WebhookVerificationError):
        verify(
            SECRET,
            signature_header(TIMESTAMP, SIGNATURE),
            str(TIMESTAMP),
            b'{"event":"watchdog.alert","severity":"Info"}',
            now=TIMESTAMP,
        )


def test_verify_rejects_wrong_secret() -> None:
    with pytest.raises(WebhookVerificationError):
        verify(
            "whsec_wrong",
            signature_header(TIMESTAMP, SIGNATURE),
            str(TIMESTAMP),
            BODY,
            now=TIMESTAMP,
        )


@pytest.mark.parametrize(
    "now",
    [TIMESTAMP + DEFAULT_TOLERANCE + 1, TIMESTAMP - DEFAULT_TOLERANCE - 1],
)
def test_verify_rejects_stale_or_future_timestamp(now: int) -> None:
    with pytest.raises(WebhookVerificationError, match="replay window"):
        verify(
            SECRET,
            signature_header(TIMESTAMP, SIGNATURE),
            str(TIMESTAMP),
            BODY,
            now=now,
        )


def test_verify_accepts_timestamp_at_the_boundary() -> None:
    verify(
        SECRET,
        signature_header(TIMESTAMP, SIGNATURE),
        str(TIMESTAMP),
        BODY,
        now=TIMESTAMP + DEFAULT_TOLERANCE,
    )


def test_verify_accepts_any_matching_v1_signature() -> None:
    header = (
        f"t={TIMESTAMP},v1={sign('whsec_old', TIMESTAMP, BODY)},"
        f"v1={sign('whsec_new', TIMESTAMP, BODY)}"
    )
    verify("whsec_new", header, str(TIMESTAMP), BODY, now=TIMESTAMP)


def test_verify_rejects_mismatched_headers() -> None:
    with pytest.raises(WebhookVerificationError):
        verify(
            SECRET,
            signature_header(TIMESTAMP, SIGNATURE),
            str(TIMESTAMP + 1),
            BODY,
            now=TIMESTAMP,
        )


def test_verify_request_reads_headers_case_insensitively() -> None:
    headers = {
        SIGNATURE_HEADER.lower(): signature_header(TIMESTAMP, SIGNATURE),
        TIMESTAMP_HEADER.lower(): str(TIMESTAMP),
    }
    verify_request(SECRET, headers, BODY, now=TIMESTAMP)


def test_verify_request_rejects_missing_headers() -> None:
    with pytest.raises(WebhookVerificationError):
        verify_request(SECRET, {}, BODY, now=TIMESTAMP)
