# Sorolens Python SDK

Python client for the Sorolens API, with built-in retry and backoff for transient errors.

## Installation

```bash
pip install sorolens
```

## Quickstart

```python
from sorolens config import ClientConfig, RetryConfig

config = ClientConfig(
    base_url="https://api.sorolens.xyz",
    api_key="...",
)
```

## Retries

The SDK automatically retries failed requests that are likely to succeed on a
second attempt:

- HTTP `429 TooMany Requests`
- Any HTTP `5xx` server error
- Transport-level connection errors

When the server responds with a `Retry-After` header, the SDK honors it
(both the delay-seconds and HTTP-date forms) instead of using its own backoff
calculation. The honored delay is still capped at `max_delay`.

### Defaults

| Option                      | Default                | Description                                           |
| -------------------------- | --------------------- | -------------------------------------------------------- |
| `max_retries`              | `3`                    | Retry attempts after the initial request (`0 disables).      |
| `base_delay`               | `0.5` seconds         | Initial backoff delay.                                      |
| `max_delay`                | `30.0` seconds        | Upper bound on any single delay.                             |
| `backoff_factor`           | `2.0`                 | Multiplier applied after each attempt.                        |
| `jitter`                  | `0.25`                | Fraction of random jitter applied to each delay (0.0 - 1.0).    |
| `retry_statuses`          | `{429}`               | Explicit statuses to retry. 5xx is always retried.            |
| `respect_retry_after`      | `True`                | Honor the `Retry-After` response header.                         |
| `retry_on_connection_errors` | `True`                | Retry on transport-level connection errors.                   |

### Customizing retries

```python
from sorolens.config import ClientConfig, RetryConfig

config = ClientConfig(
    retry=RetryConfig(
        max_retries=5,
        base_delay=0.25,
        max_delay=60.0,
        jitter=0.1,
    ),
)
```

To disable retries entirely:

```python
config = ClientConfig(retry=RetryConfig(max_retries=0))
```

## License

MIT
