# sorolens-sdk

Typed Rust client for the [Sorolens](../../) API — Soroban contract events,
invocations, storage, contract analytics, and on-chain watchdog monitoring.

The crate is hand-written from [`docs/openapi.yaml`](../../docs/openapi.yaml)
and mirrors the surface of the generated Go client at
[`packages/go-client`](../go-client). It is meant for Soroban contract test
harnesses, off-chain workers, and CLIs written in Rust.

## Features

| Feature | Default | Description |
| --- | --- | --- |
| `async` | yes | Asynchronous `Client` built on `reqwest`. |
| `blocking` | no | Blocking `BlockingClient` for CLIs and harnesses without an async runtime. |

Both transports share the same request builders and response models, so a call
made by one is byte-for-byte the same request as the other.

## Installation

```toml
[dependencies]
sorolens-sdk = "0.1"
```

With the blocking transport as well:

```toml
[dependencies]
sorolens-sdk = { version = "0.1", features = ["blocking"] }
```

## Quickstart

```rust
use sorolens_sdk::{
    Client, ClientConfig, Error, ListContractsParams, ListWatchdogAlertsParams,
};

#[tokio::main]
async fn main() -> Result<(), Error> {
    let client = Client::new(
        ClientConfig::builder()
            .base_url("https://api.sorolens.xyz")
            .api_key("sl_your_api_key")
            .timeout(std::time::Duration::from_secs(10))
            .build()?,
    )?;

    let page = client.list_contracts(&ListContractsParams::default()).await?;
    for contract in &page.contracts {
        println!("{} is {}", contract.id, contract.status);
    }

    let alerts = client
        .list_watchdog_alerts(&ListWatchdogAlertsParams::default())
        .await?;
    println!("{} alerts", alerts.alerts.len());

    Ok(())
}
```

### Blocking

```rust
use sorolens_sdk::{BlockingClient, Error, GlobalStats};

fn main() -> Result<(), Error> {
    let client = BlockingClient::builder()
        .base_url("https://api.sorolens.xyz")
        .api_key("sl_your_api_key")
        .build()?;

    let GlobalStats { tracked_contracts, .. } = client.get_global_stats()?;
    println!("{tracked_contracts} contracts tracked");

    Ok(())
}
```

## Authentication

Most read endpoints accept an optional API key, sent as
`Authorization: Bearer <token>`. Configure it with `ClientConfigBuilder::api_key`
(or `Client::builder().api_key(..)`). Anonymous reads are allowed on the public
endpoints, and the header is simply omitted when no key is set.

Watchlist endpoints use an identity header instead; set it with
`ClientConfigBuilder::user_id` once those routes are covered.

## Endpoint coverage

| Module | Endpoints |
| --- | --- |
| `contracts` | list, get, register |
| `events` | list, stream |
| `invocations` | list |
| `storage` | list, snapshot |
| `stats` | global stats, contract stats, forecast |
| `watchdog` | stats, alerts, monitored contracts, health checks |
| `health` | `/health`, `/readyz` |

Admin API-key management and the per-user watchlist routes are not covered yet.

Every path is taken verbatim from `docs/openapi.yaml`; nothing is invented.

## Errors

Every operation returns `Result<T, Error>`. Non-2xx responses become
`Error::Api(ApiError)`, carrying:

* `status` — the HTTP status code,
* `code` — the machine-readable code (`INVALID_INPUT`, `RATE_LIMITED`, ...),
* `message` — the message from the API, or a synthesised fallback,
* `request_id` — the server-side request id when the API supplies one.

Local validation failures (an empty contract ID, an out-of-range `horizon` or
`limit`) are rejected with `Error::InvalidRequest` **before** any network call.

## Testing

The suite runs entirely offline: recorded responses shaped like the schemas in
`docs/openapi.yaml` live in `tests/fixtures/` and are served by
[`httpmock`](https://crates.io/crates/httpmock).

```sh
cargo test --all-features   # unit tests, integration tests, and doctests
cargo test --doc            # doctests only
```

## Publishing

The crate carries the metadata needed to publish (`description`, `license`,
`repository`, `keywords`, `categories`). Releases are cut from a
`rust-sdk-v<version>` tag by `.github/workflows/rust-sdk.yml`, which runs
`cargo publish` against the `CARGO_REGISTRY_TOKEN` repository secret.
