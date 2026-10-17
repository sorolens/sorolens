//! Walk every page of `GET /api/v1/contracts` with the opaque `next_cursor`,
//! the pattern to follow for any list endpoint.
//!
//! Against a deployment:
//!
//! ```sh
//! SOROLENS_BASE_URL=http://localhost:8080 SOROLENS_API_KEY=sl_dev_... \
//!     cargo run --example pagination
//! ```
//!
//! Offline self-check (what CI runs):
//!
//! ```sh
//! cargo run --example pagination -- --self-test
//! ```

#[cfg(feature = "async")]
use sorolens_sdk::{Client, Error, ListContractsParams};

#[cfg(feature = "async")]
const PAGE_1: &str = r#"{"contracts":[
{"id":"CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAH2","network":"testnet","label":"counter","wasm_hash":"0x1","created_at_ledger":100,"backfill_complete_at":null,"status":"active","added_at":"2026-08-01T00:00:00Z"},
{"id":"CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAC3","network":"mainnet","label":"token","wasm_hash":"0x2","created_at_ledger":200,"backfill_complete_at":null,"status":"backfilling","added_at":"2026-08-02T00:00:00Z"}],
"next_cursor":"cursor-page-2"}"#;

#[cfg(feature = "async")]
const PAGE_2: &str = r#"{"contracts":[
{"id":"CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD4","network":"testnet","label":"vault","wasm_hash":"0x3","created_at_ledger":300,"backfill_complete_at":null,"status":"active","added_at":"2026-08-03T00:00:00Z"},
{"id":"CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAE5","network":"mainnet","label":"oracle","wasm_hash":"0x4","created_at_ledger":400,"backfill_complete_at":null,"status":"paused","added_at":"2026-08-04T00:00:00Z"}],
"next_cursor":""}"#;

#[cfg(feature = "async")]
#[tokio::main]
async fn main() {
    if let Err(err) = run().await {
        eprintln!("pagination: {err}");
        std::process::exit(1);
    }
}

#[cfg(not(feature = "async"))]
fn main() {
    eprintln!("this example requires the default `async` feature");
}

#[cfg(feature = "async")]
async fn run() -> Result<(), Error> {
    if self_test() {
        let server = httpmock::MockServer::start_async().await;
        // Register the cursor mock first: a request carrying a cursor also matches
        // the limit-only mock, and the first registered match wins.
        server
            .mock_async(|when, then| {
                when.method(httpmock::Method::GET)
                    .path("/api/v1/contracts")
                    .query_param("cursor", "cursor-page-2");
                then.status(200)
                    .header("content-type", "application/json")
                    .body(PAGE_2);
            })
            .await;
        server
            .mock_async(|when, then| {
                when.method(httpmock::Method::GET)
                    .path("/api/v1/contracts")
                    .query_param("limit", "2");
                then.status(200)
                    .header("content-type", "application/json")
                    .body(PAGE_1);
            })
            .await;

        let client = build_client(&server.base_url(), None)?;
        let total = walk(&client).await?;
        if total != 4 {
            return Err(Error::Config(format!(
                "self-test expected 4 contracts, got {total}"
            )));
        }
        println!("self-test: paged through {total} contracts");
        return Ok(());
    }

    let client = build_client(&base_url(), api_key().as_deref())?;
    let total = walk(&client).await?;
    println!("fetched {total} contract(s)");
    Ok(())
}

#[cfg(feature = "async")]
async fn walk(client: &Client) -> Result<usize, Error> {
    let mut cursor: Option<String> = None;
    let mut total = 0;
    loop {
        let mut params = ListContractsParams::default().limit(2);
        if let Some(value) = cursor.take() {
            params = params.cursor(value);
        }
        let page = client.list_contracts(&params).await?;
        for contract in &page.contracts {
            println!("{} {} {}", contract.id, contract.network, contract.status);
            total += 1;
        }
        if page.next_cursor.is_empty() {
            break;
        }
        cursor = Some(page.next_cursor);
    }
    Ok(total)
}

#[cfg(feature = "async")]
fn build_client(base_url: &str, api_key: Option<&str>) -> Result<Client, Error> {
    let mut builder = Client::builder().base_url(base_url);
    if let Some(key) = api_key {
        builder = builder.api_key(key);
    }
    builder.build()
}

#[cfg(feature = "async")]
fn base_url() -> String {
    std::env::var("SOROLENS_BASE_URL").unwrap_or_else(|_| "https://api.sorolens.xyz".to_string())
}

#[cfg(feature = "async")]
fn api_key() -> Option<String> {
    std::env::var("SOROLENS_API_KEY").ok()
}

#[cfg(feature = "async")]
fn self_test() -> bool {
    std::env::args().any(|arg| arg == "--self-test")
}
