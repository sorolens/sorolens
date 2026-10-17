//! How the SDK surfaces failures: every operation returns `Result<T, Error>`,
//! and a non-2xx response becomes `Error::Api` with the status, machine code,
//! message and server request id.
//!
//! Against a deployment:
//!
//! ```sh
//! SOROLENS_BASE_URL=http://localhost:8080 SOROLENS_API_KEY=sl_dev_... \
//!     cargo run --example errors
//! ```
//!
//! Offline self-check (what CI runs) simulates a 404 and a 429:
//!
//! ```sh
//! cargo run --example errors -- --self-test
//! ```

#[cfg(feature = "async")]
use sorolens_sdk::{Client, Error};

#[cfg(feature = "async")]
const MISSING_ID: &str = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC";

#[cfg(feature = "async")]
#[tokio::main]
async fn main() {
    if let Err(err) = run().await {
        eprintln!("errors: {err}");
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
        server
            .mock_async(|when, then| {
                when.method(httpmock::Method::GET)
                    .path(format!("/api/v1/contracts/{MISSING_ID}"));
                then.status(404)
                    .header("content-type", "application/json")
                    .body(r#"{"error":{"code":"NOT_FOUND","message":"contract not found"}}"#);
            })
            .await;
        server
            .mock_async(|when, then| {
                when.method(httpmock::Method::GET)
                    .path("/api/v1/stats/global");
                then.status(429)
                    .header("content-type", "application/json")
                    .header("retry-after", "1")
                    .body(r#"{"error":{"code":"RATE_LIMITED","message":"too many requests"}}"#);
            })
            .await;

        return classify(&build_client(&server.base_url(), None)?).await;
    }

    let client = build_client(&base_url(), api_key().as_deref())?;
    classify(&client).await
}

#[cfg(feature = "async")]
async fn classify(client: &Client) -> Result<(), Error> {
    match client.get_contract(MISSING_ID).await {
        Ok(contract) => println!("found {}", contract.id),
        Err(err) => print_api_error("get_contract", &err),
    }

    match client.get_global_stats().await {
        Ok(stats) => println!("tracked contracts: {}", stats.tracked_contracts),
        Err(err) => print_api_error("get_global_stats", &err),
    }

    Ok(())
}

#[cfg(feature = "async")]
fn print_api_error(operation: &str, err: &Error) {
    match err {
        Error::Api(api) => {
            let code = api.code.as_deref().unwrap_or("-");
            match &api.request_id {
                Some(id) => println!(
                    "{operation}: HTTP {} {code} — {} (request_id: {id})",
                    api.status, api.message
                ),
                None => println!("{operation}: HTTP {} {code} — {}", api.status, api.message),
            }
        }
        other => println!("{operation}: {other}"),
    }
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
