//! Cursor pagination over the list endpoints.
//!
//! Contracts are walked by hand with `ListContractsParams::cursor` and
//! `next_cursor`; events use `Client::contract_events_stream`, which follows
//! the cursors lazily and stops on the last page.
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
    use sorolens_sdk::{Client, ListContractEventsParams, ListContractsParams};

    const CONTRACT_ID: &str = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4";

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

            // Events: the fixture page carries a cursor; the follow-up page ends the walk.
            let events_path = format!("/api/v1/contracts/{CONTRACT_ID}/events");
            let mut last_page: serde_json::Value =
                serde_json::from_str(include_str!("../tests/fixtures/events_page.json"))?;
            last_page["events"][0]["id"] = "evt_last".into();
            last_page["next_cursor"] = "".into();
            let last_page = last_page.to_string();
            server
                .mock_async(|when, then| {
                    when.method(GET)
                        .path(events_path.as_str())
                        .query_param_exists("cursor");
                    then.status(200)
                        .header("content-type", "application/json")
                        .body(last_page.as_str());
                })
                .await;
            server
                .mock_async(|when, then| {
                    when.method(GET).path(events_path.as_str());
                    then.status(200)
                        .header("content-type", "application/json")
                        .body(include_str!("../tests/fixtures/events_page.json"));
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

        // The stream follows `next_cursor` for you, fetching one page at a time.
        // Live runs stream the first contract listed; offline runs use the mocked one.
        let events_contract = match (&live, ids.first()) {
            (Some(_), Some(first)) => first.clone(),
            _ => CONTRACT_ID.to_string(),
        };
        let mut events = client.contract_events_stream(
            &events_contract,
            ListContractEventsParams::default().limit(200),
        );
        let mut event_ids: Vec<String> = Vec::new();
        while let Some(event) = events.next().await {
            event_ids.push(event?.id);
        }
        println!("streamed {} events", event_ids.len());

        if live.is_none() {
            assert_eq!(event_ids.len(), 2, "expected two pages of one event each");
            assert_eq!(event_ids[1], "evt_last");
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
