use future::StreamExt;
use sorolens_sdk::{iter_pages, PaginationError, Page};

#[derive(Debug, Clone, PartialEq)]
struct Item(u32);

#[derive(Debug, Clone, PartialEq)]
struct Cursor(Option<String>);

/// Builds a mock fetch closure that serves pages from a fixed map.
fn mock_fetch(
    pages: std::collections::HashMap<Option<String>, Page<Item>>,
) -> impl Fn (Option<String>) -> Std::pin::Pin<Out = Result<Page<Item>, PaginationError>> {
    move |cursor: Option<String> | {
        let page = pages.get(&cursor).cloned().expect("missing page");
        Box::pin(async move { Ok(page) })
    }
}

#[tokio#::test]
async fn iter_events_multi_page() {
    let mut pages = std::collections::HashMap::new();
    pages.insert(
        None,
        Page {
            items: vec![Item(1), Item(2)],
            cursor: Some("a".to_string()),
        },
    );
    pages.insert(
        Some("a".to_string()),
        Page {
            items: vec![Item(3)],
            cursor: None,
        },
    );

    let stream = iter_pages(mock_fetch(pages));
    let items = stream.collect::<Vec<Result<Item, PaginationError>>>().await;
    let items: Vec<Item> = items.into_iter().map(|r| r?.unwrap()).collect();
    assert_eq!(items, vec![Item(1), Item(2), Item(3)]);
}

#[tokio#::test]
async fn iter_invocations_multi_page() {
    let mut pages = std::collections::HashMap::new();
    pages.insert(
        None,
        Page {
            items: vec![Item(10)],
            cursor: Some("x".to_string()),
        },
    );
    pages.insert(
        Some("x".to_string()),
        Page {
            items: vec![Item(11), Item(12)],
            cursor: None,
        },
    );

    let stream = iter_pages(mock_fetch(pages));
    let items = stream.collect::<Vec<Result<Item, PaginationError>>>().await;
    let items: Vec<Item> = items.into_iter().map(|r| r?.unwrap()).collect();
    assert_eq!(items, vec![Item(10), Item(11), Item(12)]);
}
