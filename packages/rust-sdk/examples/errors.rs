//! Error handling: non-2xx responses become `Error::Api`, local validation
//! failures become `Error::InvalidRequest`.
//!
//! Run against a live server (expect a 404 for the missing contract):
//!
//! ```text
//! SOROLENS_API_URL=https://api.sorolens.xyz cargo run --example errors
//! ```
//!
//! Without `SOROLENS_API_URL` the example replays canned failures through a
//! local `httpmock` server and asserts the mapping offline.

#[cfg(feature = "async")]
mod example {
    use httpmock::prelude::*;
    use httpmock::MockServer;
    use sorolens_sdk::{Client, Error};

    const MISSING_ID: &str = "CMISSINGCONTRACTIDENTIFIER00000000000000000000000000000000";

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
                    when.method(GET).path(format!("/api/v1/contracts/{MISSING_ID}"));
                    then.status(404)
                        .header("content-type", "application/json")
                        .body(
                            r#"{"error":{"code":"NOT_FOUND","message":"contract not found","request_id":"0f8b0f0e-6a1f-4f0f-9f4e-2b1c9f0e6a1f"}}"#,
                        );
                })
                .await;
            server
                .mock_async(|when, then| {
                    when.method(GET).path("/api/v1/stats/global");
                    then.status(500)
                        .header("content-type", "text/plain")
                        .body("boom");
                })
                .await;
        }

        let client = Client::builder().base_url(base_url).build()?;

        println!("missing contract:");
        match client.get_contract(MISSING_ID).await {
            Err(Error::Api(err)) => {
                println!("  {err}");
                if live.is_none() {
                    assert_eq!(err.status, 404);
                    assert_eq!(err.code(), Some("NOT_FOUND"));
                    assert_eq!(err.message, "contract not found");
                    assert!(err.request_id().is_some());
                }
            }
            Err(other) => return Err(format!("expected a 404 ApiError, got {other}").into()),
            Ok(contract) => println!("  contract found: {}", contract.id),
        }

        println!("unparsable failure body:");
        match client.get_global_stats().await {
            Err(Error::Api(err)) => println!("  {err}"),
            Err(other) => return Err(format!("expected an ApiError, got {other}").into()),
            Ok(_) => println!("  request succeeded"),
        }

        println!("local validation:");
        match client.get_contract("   ").await {
            Err(Error::InvalidRequest(message)) => {
                println!("  {message}");
                if live.is_none() {
                    assert!(message.contains("contract id"));
                }
            }
            other => return Err(format!("expected InvalidRequest, got {other:?}").into()),
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
