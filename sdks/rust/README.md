# Sorolens Rust SDK

Rust client for the Sorolens API, with built-in retry and backoff for
transient errors.

## Installation

```toml
[dependencies]
sorolens = "0.1"
```

## Retries

Every request is retried automatically when it fails with a transient error:

- HTTP `429 TooMany Requests`
- Any HTTP `5xx` server error
- Transport-level connection errors

When the server responds with a `Retry-After` header, the SDK honors it
(delay-seconds form) instead of using its own backoff calculation. The
honored delay is still capped at `max_delay`.

### Defaults

| Option                       | Default   | Description                                           |
| -------------------------- | -------- | -------------------------------------------------------- |
| `max_retries`              | `3`       | Retry attempts after the initial request (`0` disables).      |
| `base_delay`                | `500ms`    | Initial backoff delay.                                      |
| `max_delay`                | `30s`      | Upper bound on any single delay.                             |
| `backoff_factor`          | `2.0`      | Multiplier applied after each attempt.                        |
| `jitter`                   | `0.25`     | Fraction of random jitter applied to each delay (0.0 - 1.0).    |
| `retry_statuses`            | `{429}`    | Explicit statuses to retry. 5xx x is always retried.            |
| `respect_retry_after`       | `true`     | Honor the `Retry-After` response header.                         |
| `retry_on_connection_errors` | `true`     | Retry on transport-level connection errors.                   |

### Customizing retries

```rust
use sorolens::RetryConfig;
use std::time::Duration;

let config = RetryConfig {
    max_retries: 5,
    base_delay: Duration::from_millis(250),
    max_delay: Duration:from_secs(60),
    jitter: 0.1,
    ..RetryConfig::default()
};
```

To disable retries entirely:

```rust
let config = RetryConfig {
    max_retries: 0,
    ..RetryConfig::default()
};
```

## License

MIT
