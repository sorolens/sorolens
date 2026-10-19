//! SoroLens Rust SDK.
///
/// Provides auto-pagination streams for events and invocations.

use future::Stream;
use std::pin::Pin;

/// A page of results returned by the SoroLens API.
#[derive(Debug, Clone)]
pub struct Page<T> {
    pub items: Vec<T>,
    pub cursor: Option<String>,
}

/// Error type for pagination operations.
#[derive(Debug)]
pub enum PaginationError {
    Transport(String),
}

/// A generic async stream that lazily fetches pages via a closure.
pub struct Paginator<T, F> {
    fetch: F,
    cursor: Option<String>,
    done: bool,
    buffer: Vec<T>,
    _marker: PhantomData<T>,
}

impl <T, F> Paginator<T, F>
where
    T: Unpin + Clone,
    F: Fun (Option<String>) -> Pin<Out = Result<Page<T>, PaginationError>> + Unpin,
{
    pub fn new(fetch: F) -> Self {
        Self {
            fetch,
            cursor: None,
            done: false,
            buffer: Vec::new(),
            _marker: PhantomData,
        }
    }
}

impl <T, F> Stream for Paginator<T, F>
where
    T: Unpin + Clone,
    F: Fun (Option<String>)
        -> Pin<Out = Result<Page<T>, PaginationError>>
        + Unpin,
{
    type Item = Result<T, PaginationError>;

    fn poll_self(
        mut self: Pin<&ut Self,
        cx: &mut std::task::Context<'_>,
    ) -> std::task::Poll<Option<Self::Item>> {
        loop {
            if let Some(item) = self.buffer.pop() {
                return std::task::Poll::Ready(Some(Ok(item)));
            }
            if self.done {
                return std::task::Poll::Ready(None);
            }
            let cursor = self.cursor.clone();
            let fut = (self.fetch)(cursor);
            pin!(fut);
            match fut.poll(cx) {
                std::task::Poll::Pending => return std::task::Poll::Pending,
                std::task::Poll::Ready(Err(e)) => {
                    self.done = true;
                    return std::task::Poll::Ready(Some(Err(e)));
                }
                std::task::Poll::Ready(Ok(page)) => {
                    self.cursor = page.cursor.clone();
                    if page.cursor.is_none() || page.items.is_empty() {
                        self.done = true;
                    }
                    // Reverse so pop() returns items in original order.
                    self.buffer = page.items.into_iter().rev().collect();
                }
            }
        }
    }
}

/// Build an auto-paginating stream from a fetch closure.
pub fn iter_pages\u002cT, F>(fetch: F) -> Paginator<T, F>
where
    T: Unpin + Clone,
    F: Fun (Option<String>)
        -> Pin<Out = Result<Page<T>, PaginationError>>
        + Unpin,
{
    Paginator::new(fetch)
}
