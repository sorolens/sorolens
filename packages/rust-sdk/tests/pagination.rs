#![cfg(any(feature = "async", feature = "blocking"))]

//! Auto-pagination over events and invocations (issue #770).
//!
//! Each test serves several cursor pages from `httpmock` and counts the hits
//! on every page, so it checks *when* pages are fetched as well as what the
//! iterator yields.

use httpmock::prelude::*;
use httpmock::Mock;
use serde_json::{json, Value};
use sorolens_sdk::*;

const CONTRACT_ID: &str = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4";
const EVENTS_PATH: &str =
    "/api/v1/contracts/CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4/events";
const INVOCATIONS_PATH: &str =
    "/api/v1/contracts/CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4/invocations";

#[derive(Clone, Copy)]
enum Kind {
    Events,
    Invocations,
}

impl Kind {
    fn key(self) -> &'static str {
        match self {
            Kind::Events => "events",
            Kind::Invocations => "invocations",
        }
    }

    fn path(self) -> &'static str {
        match self {
            Kind::Events => EVENTS_PATH,
            Kind::Invocations => INVOCATIONS_PATH,
        }
    }

    /// One item of page `page`, tagged `<page>-<n>` in its id / tx hash.
    fn item(self, page: usize, n: usize) -> Value {
        let (fixture, field) = match self {
            Kind::Events => (include_str!("fixtures/events_page.json"), "id"),
            Kind::Invocations => (include_str!("fixtures/invocations_page.json"), "tx_hash"),
        };
        let mut item = serde_json::from_str::<Value>(fixture).unwrap()[self.key()][0].clone();
        item[field] = json!(tag(page, n));
        item
    }

    fn page(self, page: usize, next_cursor: &str) -> String {
        json!({
            self.key(): [self.item(page, 0), self.item(page, 1)],
            "next_cursor": next_cursor,
        })
        .to_string()
    }
}

fn tag(page: usize, n: usize) -> String {
    format!("{page}-{n}")
}

fn expected(pages: usize) -> Vec<String> {
    (0..pages)
        .flat_map(|page| (0..2).map(move |n| tag(page, n)))
        .collect()
}

/// Mounts `pages` pages chained by `p1`, `p2`, ... cursors.
///
/// `cursor_after(page)` overrides the cursor that page `page` points at and
/// `fail_on` makes that page answer 500. The returned mocks are in page order.
fn mount<'a>(
    server: &'a MockServer,
    kind: Kind,
    pages: usize,
    fail_on: Option<usize>,
    cursor_after: impl Fn(usize) -> String,
) -> Vec<Mock<'a>> {
    let body = |page: usize| {
        let next = if page + 1 == pages {
            String::new()
        } else {
            cursor_after(page)
        };
        kind.page(page, &next)
    };
    let respond = |page: usize, then: httpmock::Then| {
        if fail_on == Some(page) {
            then.status(500)
                .header("content-type", "application/json")
                .body(r#"{"error":{"code":"INTERNAL","message":"boom"}}"#);
        } else {
            then.status(200)
                .header("content-type", "application/json")
                .body(body(page));
        }
    };
    // Cursor pages are registered before the first page so that a request
    // carrying a cursor never falls through to the cursor-less mock.
    let mut later: Vec<Mock<'a>> = (1..pages)
        .map(|page| {
            server.mock(|when, then| {
                when.method(GET)
                    .path(kind.path())
                    .query_param("cursor", format!("p{page}"));
                respond(page, then);
            })
        })
        .collect();
    let first = server.mock(|when, then| {
        when.method(GET).path(kind.path());
        respond(0, then);
    });
    later.insert(0, first);
    later
}

fn chained(page: usize) -> String {
    format!("p{}", page + 1)
}

fn hits(mocks: &[Mock<'_>]) -> Vec<usize> {
    mocks.iter().map(Mock::hits).collect()
}

#[cfg(feature = "async")]
mod asynchronous {

    use super::*;

    /// Drains an async pager into a `Vec`, the way a caller's
    /// `while let Some(item) = pager.next().await` loop would.
    macro_rules! drain {
        ($pager:expr) => {{
            let mut pager = $pager;
            let mut items = Vec::new();
            while let Some(item) = pager.next().await {
                items.push(item);
            }
            items
        }};
    }

    fn client(server: &MockServer) -> Client {
        Client::builder()
            .base_url(server.base_url())
            .build()
            .expect("client builds")
    }

    #[tokio::test]
    async fn events_stream_walks_every_page_and_stops_on_the_last_cursor() {
        let server = MockServer::start_async().await;
        let mocks = mount(&server, Kind::Events, 3, None, chained);
        let client = client(&server);

        let ids: Vec<String> = drain!(client
            .contract_events_stream(CONTRACT_ID, ListContractEventsParams::default().limit(2)))
        .into_iter()
        .map(|event| event.unwrap().id)
        .collect();

        assert_eq!(ids, expected(3));
        assert_eq!(hits(&mocks), vec![1, 1, 1]);
    }

    #[tokio::test]
    async fn invocations_stream_walks_every_page() {
        let server = MockServer::start_async().await;
        let mocks = mount(&server, Kind::Invocations, 3, None, chained);
        let client = client(&server);

        let hashes: Vec<String> = drain!(client
            .contract_invocations_stream(CONTRACT_ID, ListContractInvocationsParams::default()))
        .into_iter()
        .map(|invocation| invocation.unwrap().tx_hash)
        .collect();

        assert_eq!(hashes, expected(3));
        assert_eq!(hits(&mocks), vec![1, 1, 1]);
    }

    #[tokio::test]
    async fn stream_fetches_pages_lazily() {
        let server = MockServer::start_async().await;
        let mocks = mount(&server, Kind::Events, 3, None, chained);
        let client = client(&server);

        let mut events =
            client.contract_events_stream(CONTRACT_ID, ListContractEventsParams::default());
        assert_eq!(hits(&mocks), vec![0, 0, 0], "nothing is fetched up front");

        assert_eq!(events.next().await.unwrap().unwrap().id, "0-0");
        assert_eq!(events.next().await.unwrap().unwrap().id, "0-1");
        assert_eq!(hits(&mocks), vec![1, 0, 0]);

        assert_eq!(events.next().await.unwrap().unwrap().id, "1-0");
        assert_eq!(hits(&mocks), vec![1, 1, 0]);
    }

    #[tokio::test]
    async fn error_mid_stream_comes_after_the_items_already_received() {
        let server = MockServer::start_async().await;
        let mocks = mount(&server, Kind::Invocations, 3, Some(1), chained);
        let client = client(&server);

        let results: Vec<Result<Invocation, Error>> = drain!(client
            .contract_invocations_stream(CONTRACT_ID, ListContractInvocationsParams::default()));

        assert_eq!(results.len(), 3, "two items, then the error, then the end");
        let received: Vec<String> = results[..2]
            .iter()
            .map(|r| r.as_ref().unwrap().tx_hash.clone())
            .collect();
        assert_eq!(received, expected(1));
        assert_eq!(results[2].as_ref().unwrap_err().status(), Some(500));
        assert_eq!(
            hits(&mocks),
            vec![1, 1, 0],
            "nothing is fetched after the error"
        );
    }

    #[tokio::test]
    async fn page_size_is_clamped_to_the_server_maximum() {
        let server = MockServer::start_async().await;
        let clamped = server.mock(|when, then| {
            when.method(GET)
                .path(EVENTS_PATH)
                .query_param("limit", MAX_PAGE_SIZE.to_string());
            then.status(200)
                .header("content-type", "application/json")
                .body(Kind::Events.page(0, ""));
        });
        let client = client(&server);

        let count = drain!(client.contract_events_stream(
            CONTRACT_ID,
            ListContractEventsParams::default().limit(5_000),
        ))
        .len();

        assert_eq!(count, 2);
        clamped.assert();
    }

    #[tokio::test]
    async fn zero_page_size_is_rejected_without_a_request() {
        let server = MockServer::start_async().await;
        let mocks = mount(&server, Kind::Events, 1, None, chained);
        let client = client(&server);

        let results: Vec<Result<Event, Error>> = drain!(client
            .contract_events_stream(CONTRACT_ID, ListContractEventsParams::default().limit(0)));

        assert!(matches!(
            results.as_slice(),
            [Err(Error::InvalidRequest(_))]
        ));
        assert_eq!(hits(&mocks), vec![0]);
    }

    #[tokio::test]
    async fn repeated_cursor_ends_the_stream_with_a_pagination_error() {
        let server = MockServer::start_async().await;
        // Every page points back at "p1", which would loop forever.
        let mocks = mount(&server, Kind::Events, 4, None, |_| "p1".to_string());
        let client = client(&server);

        let results: Vec<Result<Event, Error>> =
            drain!(client.contract_events_stream(CONTRACT_ID, ListContractEventsParams::default()));

        assert_eq!(results.len(), 5, "two pages of items, then the error");
        assert!(results[..4].iter().all(Result::is_ok));
        assert!(matches!(results[4], Err(Error::Pagination(_))));
        assert_eq!(hits(&mocks), vec![1, 1, 0, 0]);
    }
}

#[cfg(feature = "blocking")]
mod blocking {
    use super::*;

    fn client(server: &MockServer) -> BlockingClient {
        BlockingClient::builder()
            .base_url(server.base_url())
            .build()
            .expect("client builds")
    }

    #[test]
    fn events_iter_walks_every_page() {
        let server = MockServer::start();
        let mocks = mount(&server, Kind::Events, 3, None, chained);
        let client = client(&server);

        let ids: Vec<String> = client
            .contract_events_iter(CONTRACT_ID, ListContractEventsParams::default().limit(2))
            .map(|event| event.unwrap().id)
            .collect();

        assert_eq!(ids, expected(3));
        assert_eq!(hits(&mocks), vec![1, 1, 1]);
    }

    #[test]
    fn invocations_iter_fetches_lazily_and_stops_after_an_error() {
        let server = MockServer::start();
        let mocks = mount(&server, Kind::Invocations, 3, Some(1), chained);
        let client = client(&server);

        let mut invocations =
            client.contract_invocations_iter(CONTRACT_ID, ListContractInvocationsParams::default());
        assert_eq!(hits(&mocks), vec![0, 0, 0]);

        assert_eq!(invocations.next().unwrap().unwrap().tx_hash, "0-0");
        assert_eq!(invocations.next().unwrap().unwrap().tx_hash, "0-1");
        assert_eq!(hits(&mocks), vec![1, 0, 0]);

        let error = invocations.next().unwrap().unwrap_err();
        assert_eq!(error.status(), Some(500));
        assert!(invocations.next().is_none());
        assert!(invocations.next().is_none(), "the iterator is fused");
        assert_eq!(hits(&mocks), vec![1, 1, 0]);
    }

    #[test]
    fn repeated_cursor_ends_the_iterator() {
        let server = MockServer::start();
        let mocks = mount(&server, Kind::Invocations, 4, None, |_| "p1".to_string());
        let client = client(&server);

        let results: Vec<_> = client
            .contract_invocations_iter(CONTRACT_ID, ListContractInvocationsParams::default())
            .collect();

        assert_eq!(results.len(), 5);
        assert!(matches!(results[4], Err(Error::Pagination(_))));
        assert_eq!(hits(&mocks), vec![1, 1, 0, 0]);
    }
}
