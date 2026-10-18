//! Cursor pagination over `GET /api/v1/contracts`.
//!
//! Run against a live server:
//!
//! ```text
//! SOROLENS_API_URL=https://api.sorolens.xyz cargo run --example pagination
//! ```
//!
//! Without `SOROLENS_API_URL` the example serves canned pages through a local
//! `httpmock` server and asserts the full walk offline (that is what CI runs).

#[cfg(feature = "async")]
mod example {
    use httpmock::prelude::*;
    use httpmock::MockServer;
    use sorolens_sdk::{Client, ListContractsParams};

    const PAGE_2: &str = r#"{
      "contracts": [
        {
          "id": "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC",
          "network": "futurenet",
          "label": "vault",
          "wasm_hash": "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
          "created_at_ledger": 77,
          "backfill_complete_at": null,
          "status": "active",
          "added_at": "2026-03-04T09:00:00Z"
        }
      ],
      "next_cursor": ""
    }"#;

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
            // Registered first: only matches follow-up requests that carry the cursor.
            server
                .mock_async(|when, then| {
                    when.method(GET)
                        .path("/api/v1/contracts")
                        .query_param_exists("cursor");
                    then.status(200)
                        .header("content-type", "application/json")
                        .body(PAGE_2);
                })
                .await;
            server
                .mock_async(|when, then| {
                    when.method(GET).path("/api/v1/contracts");
                    then.status(200)
                        .header("content-type", "application/json")
                        .body(include_str!("../tests/fixtures/contracts_page.json"));
                })
                .await;
        }

        let client = Client::builder().base_url(base_url).build()?;

        let mut ids: Vec<String> = Vec::new();
        let mut cursor: Option<String> = None;
        loop {
            let params = match &cursor {
                Some(cursor) => ListContractsParams::default()
                    .limit(2)
                    .cursor(cursor.clone()),
                None => ListContractsParams::default().limit(2),
            };
            let page = client.list_contracts(&params).await?;
            for contract in &page.contracts {
                println!("{}\t{}\t{}", contract.id, contract.network, contract.label);
                ids.push(contract.id.clone());
            }
            if page.next_cursor.is_empty() {
                break;
            }
            cursor = Some(page.next_cursor);
        }

        println!("paginated through {} contracts", ids.len());
        if live.is_none() {
            assert_eq!(ids.len(), 3, "expected two pages totalling three contracts");
            let mut unique = ids.clone();
            unique.sort();
            unique.dedup();
            assert_eq!(unique.len(), ids.len(), "cursor pages repeated items");
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
