//! Consume the `/api/v1/stream/events` Server-Sent Events stream. The SDK's
//! `stream_contract_events` is a short-poll convenience endpoint; the live,
//! long-lived stream is raw SSE, read here with `reqwest`.
//!
//! Against a deployment:
//!
//! ```sh
//! SOROLENS_BASE_URL=http://localhost:8080 SOROLENS_API_KEY=sl_dev_... \
//!     cargo run --example sse
//! ```
//!
//! Offline self-check (what CI runs):
//!
//! ```sh
//! cargo run --example sse -- --self-test
//! ```

#[cfg(feature = "async")]
const EVENTS: &str = concat!(
    "data: {\"id\":\"evt_1\",\"contract_id\":\"CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAH2\",\"type\":\"transfer\",\"ledger\":101}\n\n",
    "data: {\"id\":\"evt_2\",\"contract_id\":\"CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAH2\",\"type\":\"mint\",\"ledger\":102}\n\n",
);

#[cfg(feature = "async")]
#[tokio::main]
async fn main() {
    if let Err(err) = run().await {
        eprintln!("sse: {err}");
        std::process::exit(1);
    }
}

#[cfg(not(feature = "async"))]
fn main() {
    eprintln!("this example requires the default `async` feature");
}

#[cfg(feature = "async")]
async fn run() -> Result<(), Box<dyn std::error::Error>> {
    if self_test() {
        let server = httpmock::MockServer::start_async().await;
        server
            .mock_async(|when, then| {
                when.method(httpmock::Method::GET)
                    .path("/api/v1/stream/events");
                then.status(200)
                    .header("content-type", "text/event-stream")
                    .body(EVENTS);
            })
            .await;

        let seen = consume(&server.base_url(), None).await?;
        if seen != 2 {
            return Err(format!("self-test expected 2 events, got {seen}").into());
        }
        println!("self-test: read {seen} events");
        return Ok(());
    }

    consume(&base_url(), api_key()).await?;
    Ok(())
}

#[cfg(feature = "async")]
async fn consume(
    base_url: &str,
    api_key: Option<String>,
) -> Result<usize, Box<dyn std::error::Error>> {
    let url = format!("{}/api/v1/stream/events", base_url.trim_end_matches('/'));
    let mut request = reqwest::Client::new().get(url);
    if let Some(key) = api_key {
        request = request.bearer_auth(key);
    }

    let mut response = request.send().await?;
    if !response.status().is_success() {
        return Err(format!("unexpected status {}", response.status()).into());
    }

    let mut buffer = String::new();
    let mut seen = 0;
    while let Some(chunk) = response.chunk().await? {
        buffer.push_str(&String::from_utf8_lossy(&chunk));
        while let Some(newline) = buffer.find('\n') {
            let line = buffer[..newline].trim().to_string();
            buffer.drain(..=newline);

            let Some(data) = line.strip_prefix("data:") else {
                continue;
            };
            let event: serde_json::Value = serde_json::from_str(data.trim())?;
            println!(
                "{} {} {}",
                event["id"].as_str().unwrap_or(""),
                event["contract_id"].as_str().unwrap_or(""),
                event["type"].as_str().unwrap_or(""),
            );
            seen += 1;
        }
    }
    Ok(seen)
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
