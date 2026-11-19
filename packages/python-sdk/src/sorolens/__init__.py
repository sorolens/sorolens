"""Official Python client for the Sorolens API.

The client mirrors the REST surface documented in ``docs/openapi.yaml`` and
ships both a synchronous (:class:`~sorolens.Client`) and an asynchronous
(:class:`~sorolens.AsyncClient`) transport built on ``httpx``.
"""

from __future__ import annotations

from . import exceptions
from ._generated import models
from .client import DEFAULT_BASE_URL, AsyncClient, Client
from .exceptions import (
    APIError,
    AuthenticationError,
    ForbiddenError,
    NotFoundError,
    RateLimitError,
    SorolensError,
    ValidationError,
)
from .retry import RetryPolicy, RetryTransport
from .webhooks import (
    DEFAULT_TOLERANCE,
    SIGNATURE_HEADER,
    TIMESTAMP_HEADER,
    WebhookVerificationError,
    sign,
    signature_header,
    verify,
    verify_request,
)

__version__ = "0.1.0"

__all__ = [
    "APIError",
    "AsyncClient",
    "AuthenticationError",
    "Client",
    "DEFAULT_BASE_URL",
    "DEFAULT_TOLERANCE",
    "ForbiddenError",
    "NotFoundError",
    "RateLimitError",
    "RetryPolicy",
    "RetryTransport",
    "SIGNATURE_HEADER",
    "TIMESTAMP_HEADER",
    "SorolensError",
    "ValidationError",
    "WebhookVerificationError",
    "__version__",
    "exceptions",
    "models",
    "sign",
    "signature_header",
    "verify",
    "verify_request",
]
