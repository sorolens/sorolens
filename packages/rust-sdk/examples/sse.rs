//! Consume the Server-Sent Events endpoint `GET /api/v1/stream/events`.
//!
//! The SDK's `stream_contract_events` short-polls
//! `/api/v1/contracts/{id}/stream`; this example reads the real SSE endpoint
//! with the same `reqwest` transport, so every `data:` line is handled as it
//! arrives.
//!
//! Run against a live server:
//!
//! ```text
//! SOROLENS_API_URL=https://api.sorolens.xyz cargo run --example sse
//! ```
//!
//! Without `SOROLENS_API_URL` a canned SSE transcript is replayed offline,
//! which is what CI runs.

#[cfg(feature = "async")]
mod example {
    use httpmock::prelude::*;
    use httpmock::MockServer;
    use sorolens_sdk::ClientConfig;

    const OFFLINE_TRANSCRIPT: &str = "event: contract.event\n\
        data: {\"type\":\"contract.event\",\"id\":\"0\",\"contract_id\":\"CDEMO\"}\n\
        \n\
        event: contract.event\n\
        data: {\"type\":\"contract.event\",\"id\":\"1\",\"contract_id\":\"CDEMO\"}\n\
        \n\
        : keep-alive\n\
        event: contract.event\n\
        data: {\"type\":\"contract.event\",\"id\":\"2\",\"contract_id\":\"CDEMO\"}\n\
        \n";

    pub async fn run() -> Result<(), Box<dyn std::error::Error>> {
        let live = std::env::var("SOROLENS_API_URL").ok();
        let server = match &live {
            Some(_) => None,
            None => Some(MockServer::start_async().await),
        };

        let base_url = match (&live, &server) {
            (Some(url), _) => url.clone(),
            (None, Some(server)) => server.base_url(),
            (None, None) => unreachable!("offline mode always starts a mock server"),
        };

        if let Some(server) = &server {
            server
                .mock_async(|when, then| {
                    when.method(GET).path("/api/v1/stream/events");
                    then.status(200)
                        .header("content-type", "text/event-stream")
                        .body(OFFLINE_TRANSCRIPT);
                })
                .await;
        }

        let api_key = std::env::var("SOROLENS_API_KEY")
            .ok()
            .filter(|key| !key.is_empty());
        let config = ClientConfig::builder().base_url(base_url).build()?;

        let http = reqwest::Client::new();
        let mut request = http.get(format!("{}/api/v1/stream/events", config.base_url()));
        if let Some(api_key) = &api_key {
            request = request.header("Authorization", format!("Bearer {api_key}"));
        }

        let mut response = request.send().await?;
        if !response.status().is_success() {
            return Err(format!("stream returned status {}", response.status()).into());
        }

        let mut pending = String::new();
        let mut ids: Vec<String> = Vec::new();

        while let Some(chunk) = response.chunk().await? {
            pending.push_str(&String::from_utf8_lossy(&chunk));
            while let Some(newline) = pending.find('\n') {
                let line = pending[..newline].trim_end_matches('\r').to_string();
                pending.drain(..=newline);

                let Some(payload) = line.strip_prefix("data:") else {
                    continue;
                };
                let payload = payload.trim();
                if payload.is_empty() {
                    continue;
                }
                let event: serde_json::Value = serde_json::from_str(payload)?;
                let event_type = event["type"].as_str().unwrap_or("unknown");
                let id = event["id"].as_str().unwrap_or_default().to_string();
                println!("  {event_type} {id}");
                ids.push(id);
            }
        }

        println!("received {} events", ids.len());
        if live.is_none() {
            assert_eq!(ids, vec!["0".to_string(), "1".to_string(), "2".to_string()]);
        }
        Ok(())
    }
}

#[cfg(feature = "async")]
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    example::run().await
}

#[cfg(not(feature = "async"))]
fn main() {
    eprintln!("this example requires the \"async\" feature (enabled by default)");
}
