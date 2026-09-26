"""Exception hierarchy raised by the Sorolens Python client."""

from __future__ import annotations

from typing import Any


class SorolensError(Exception):
    """Base class for every error raised by this SDK."""

    def __init__(self, message: str, *, status_code: int | None = None) -> None:
        super().__init__(message)
        self.message = message
        self.status_code = status_code


class APIError(SorolensError):
    """The API answered with a non-success status code."""


class AuthenticationError(APIError):
    """401: the credential or identity is missing or invalid."""


class ForbiddenError(APIError):
    """403: the credential is valid but lacks the required scope or role."""


class NotFoundError(APIError):
    """404: the requested resource does not exist."""


class ValidationError(APIError):
    """422: the query or request body was rejected."""


class RateLimitError(APIError):
    """429: the per-IP rate limit was exceeded."""


_STATUS_TO_EXCEPTION: dict[int, type[APIError]] = {
    401: AuthenticationError,
    403: ForbiddenError,
    404: NotFoundError,
    422: ValidationError,
    429: RateLimitError,
}


def _error_message(payload: Any, status_code: int) -> str:
    """Best-effort extraction of a human readable message from an error body."""
    if payload is None:
        return f"HTTP {status_code}"
    error = getattr(payload, "error", payload)
    if isinstance(error, str):
        return error
    for attribute in ("message", "required", "role"):
        value = getattr(error, attribute, None)
        if isinstance(value, str) and value:
            return value
    return str(error)


def raise_for_response(status_code: int, parsed: Any, content: bytes = b"") -> None:
    """Raise the mapped :class:`APIError` subclass when ``status_code`` is an error."""
    if status_code < 400:
        return
    exception = _STATUS_TO_EXCEPTION.get(status_code, APIError)
    raise exception(_error_message(parsed, status_code), status_code=status_code)
