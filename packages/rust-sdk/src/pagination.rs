//! Auto-pagination over the cursor-paged event and invocation endpoints.
//!
//! [`Client::contract_events_stream`](crate::Client::contract_events_stream)
//! and [`Client::contract_invocations_stream`](crate::Client::contract_invocations_stream)
//! return an async pager with an `async fn next`;
//! [`BlockingClient::contract_events_iter`](crate::BlockingClient::contract_events_iter)
//! and [`BlockingClient::contract_invocations_iter`](crate::BlockingClient::contract_invocations_iter)
//! return a blocking [`Iterator`]. All four share the rules below:
//!
//! * Pages are fetched lazily: the next request is sent only once every item
//!   of the current page has been yielded.
//! * The walk stops on the first page whose `next_cursor` is empty.
//! * The page size (`limit`) is clamped to [`MAX_PAGE_SIZE`], the server's
//!   limit; a limit of `0` is rejected before any request is sent.
//! * A failed request is yielded as an `Err` *after* every item already
//!   received, and ends the walk.
//! * A cursor the walk already requested ends it with [`Error::Pagination`]
//!   instead of looping forever.

use std::collections::{HashSet, VecDeque};

use crate::error::Error;
use crate::events::ListContractEventsParams;
use crate::invocations::ListContractInvocationsParams;
use crate::types::{Event, EventsPage, Invocation, InvocationsPage};

/// The largest page size the list endpoints accept (`docs/openapi.yaml`).
pub const MAX_PAGE_SIZE: u32 = 200;

/// A page of results plus the cursor for the next one.
pub(crate) trait Page {
    type Item;
    fn into_parts(self) -> (Vec<Self::Item>, String);
}

impl Page for EventsPage {
    type Item = Event;
    fn into_parts(self) -> (Vec<Event>, String) {
        (self.events, self.next_cursor)
    }
}

impl Page for InvocationsPage {
    type Item = Invocation;
    fn into_parts(self) -> (Vec<Invocation>, String) {
        (self.invocations, self.next_cursor)
    }
}

/// Query parameters that carry a cursor and a page size.
pub(crate) trait PagedParams: Clone {
    fn cursor_mut(&mut self) -> &mut Option<String>;
    fn limit_mut(&mut self) -> &mut Option<u32>;
}

impl PagedParams for ListContractEventsParams {
    fn cursor_mut(&mut self) -> &mut Option<String> {
        &mut self.cursor
    }
    fn limit_mut(&mut self) -> &mut Option<u32> {
        &mut self.limit
    }
}

impl PagedParams for ListContractInvocationsParams {
    fn cursor_mut(&mut self) -> &mut Option<String> {
        &mut self.cursor
    }
    fn limit_mut(&mut self) -> &mut Option<u32> {
        &mut self.limit
    }
}

/// The transport-independent state of one walk.
///
/// The async stream and the blocking iterator both drive this: they ask
/// [`Pager::next_request`] what to fetch, hand the outcome to
/// [`Pager::accept`] or [`Pager::fail`], and drain items with
/// [`Pager::pop`].
#[derive(Debug)]
pub(crate) struct Pager<P, T> {
    params: P,
    buffer: VecDeque<T>,
    seen: HashSet<String>,
    pending_error: Option<Error>,
    exhausted: bool,
}

impl<P: PagedParams, T> Pager<P, T> {
    pub(crate) fn new(mut params: P) -> Self {
        let mut pending_error = None;
        match *params.limit_mut() {
            Some(0) => {
                pending_error = Some(Error::InvalidRequest(
                    "page size (limit) must be at least 1".into(),
                ));
            }
            Some(limit) if limit > MAX_PAGE_SIZE => {
                *params.limit_mut() = Some(MAX_PAGE_SIZE);
            }
            _ => {}
        }
        let mut seen = HashSet::new();
        if let Some(cursor) = params.cursor_mut().clone() {
            seen.insert(cursor);
        }
        Self {
            params,
            buffer: VecDeque::new(),
            seen,
            pending_error,
            exhausted: false,
        }
    }

    /// The next item, error, or end of the walk, when it needs no request.
    ///
    /// `None` means the buffer is empty and a page must be fetched with the
    /// parameters from [`Pager::next_request`].
    pub(crate) fn pop(&mut self) -> Option<Option<Result<T, Error>>> {
        if let Some(item) = self.buffer.pop_front() {
            return Some(Some(Ok(item)));
        }
        if let Some(error) = self.pending_error.take() {
            self.exhausted = true;
            return Some(Some(Err(error)));
        }
        if self.exhausted {
            return Some(None);
        }
        None
    }

    /// The parameters for the next page request.
    pub(crate) fn next_request(&self) -> P {
        self.params.clone()
    }

    /// Records a fetched page and the cursor that follows it.
    pub(crate) fn accept<G: Page<Item = T>>(&mut self, page: G) {
        let (items, next_cursor) = page.into_parts();
        self.buffer.extend(items);
        if next_cursor.is_empty() {
            self.exhausted = true;
        } else if !self.seen.insert(next_cursor.clone()) {
            self.pending_error = Some(Error::Pagination(format!(
                "the API returned cursor {next_cursor:?} twice; stopping to avoid a loop"
            )));
        } else {
            *self.params.cursor_mut() = Some(next_cursor);
        }
    }

    /// Records a failed page request; it is yielded after buffered items.
    pub(crate) fn fail(&mut self, error: Error) {
        self.pending_error = Some(error);
    }
}

macro_rules! async_pager {
    (
        $(#[$meta:meta])*
        $name:ident, $item:ty, $params:ty, $list:ident, $method:ident, $doc:literal
    ) => {
        $(#[$meta])*
        #[cfg(feature = "async")]
        #[derive(Debug)]
        pub struct $name<'a> {
            client: &'a crate::Client,
            id: &'a str,
            pager: Pager<$params, $item>,
        }

        #[cfg(feature = "async")]
        impl $name<'_> {
            /// Returns the next item, fetching the next page only when the
            /// current one is used up. Returns `None` once the walk is over;
            /// an `Err` comes after every item already received and ends it.
            pub async fn next(&mut self) -> Option<Result<$item, Error>> {
                loop {
                    if let Some(next) = self.pager.pop() {
                        return next;
                    }
                    match self.client.$list(self.id, &self.pager.next_request()).await {
                        Ok(page) => self.pager.accept(page),
                        Err(error) => self.pager.fail(error),
                    }
                }
            }
        }

        #[cfg(feature = "async")]
        impl crate::Client {
            #[doc = $doc]
            ///
            /// Each page is requested only once the previous one has been
            /// consumed, and the walk ends after the page whose
            /// `next_cursor` is empty. A failed request is yielded as an
            /// `Err` after the items already received, then the walk ends.
            /// `params.limit` sets the page size, clamped to
            /// [`MAX_PAGE_SIZE`]; `params.cursor` starts the walk part-way.
            ///
            /// ```no_run
            /// use sorolens_sdk::{Client, Error, ListContractEventsParams};
            ///
            /// # async fn run(client: Client) -> Result<(), Error> {
            /// let mut events = client.contract_events_stream(
            ///     "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
            ///     ListContractEventsParams::default().limit(200),
            /// );
            /// while let Some(event) = events.next().await {
            ///     println!("{}", event?.id);
            /// }
            /// # Ok(())
            /// # }
            /// ```
            pub fn $method<'a>(&'a self, id: &'a str, params: $params) -> $name<'a> {
                $name {
                    client: self,
                    id,
                    pager: Pager::new(params),
                }
            }
        }
    };
}

async_pager! {
    /// Async pager over every event of a contract.
    ///
    /// Returned by `Client::contract_events_stream`; call
    /// [`next`](Self::next) in a `while let` loop.
    ContractEventsStream,
    Event,
    ListContractEventsParams,
    list_contract_events,
    contract_events_stream,
    "Streams every event of a contract, following cursors lazily."
}

async_pager! {
    /// Async pager over every invocation of a contract.
    ///
    /// Returned by `Client::contract_invocations_stream`; call
    /// [`next`](Self::next) in a `while let` loop.
    ContractInvocationsStream,
    Invocation,
    ListContractInvocationsParams,
    list_contract_invocations,
    contract_invocations_stream,
    "Streams every invocation of a contract, following cursors lazily."
}

macro_rules! blocking_iter {
    (
        $(#[$meta:meta])*
        $name:ident, $item:ty, $params:ty, $list:ident, $method:ident, $doc:literal
    ) => {
        $(#[$meta])*
        #[cfg(feature = "blocking")]
        #[derive(Debug)]
        pub struct $name<'a> {
            client: &'a crate::BlockingClient,
            id: &'a str,
            pager: Pager<$params, $item>,
        }

        #[cfg(feature = "blocking")]
        impl Iterator for $name<'_> {
            type Item = Result<$item, Error>;

            fn next(&mut self) -> Option<Self::Item> {
                loop {
                    if let Some(next) = self.pager.pop() {
                        return next;
                    }
                    match self.client.$list(self.id, &self.pager.next_request()) {
                        Ok(page) => self.pager.accept(page),
                        Err(error) => self.pager.fail(error),
                    }
                }
            }
        }

        #[cfg(feature = "blocking")]
        impl std::iter::FusedIterator for $name<'_> {}

        #[cfg(feature = "blocking")]
        impl crate::BlockingClient {
            #[doc = $doc]
            ///
            /// Pages are fetched lazily on the calling thread; the iterator
            /// ends after the page whose `next_cursor` is empty, or right
            /// after yielding the first `Err` (which comes after every item
            /// already received). `params.limit` sets the page size, clamped
            /// to [`MAX_PAGE_SIZE`].
            pub fn $method<'a>(&'a self, id: &'a str, params: $params) -> $name<'a> {
                $name {
                    client: self,
                    id,
                    pager: Pager::new(params),
                }
            }
        }
    };
}

blocking_iter! {
    /// Blocking iterator over every event of a contract.
    ///
    /// Returned by `BlockingClient::contract_events_iter`.
    ContractEventsIter,
    Event,
    ListContractEventsParams,
    list_contract_events,
    contract_events_iter,
    "Iterates over every event of a contract, following cursors lazily."
}

blocking_iter! {
    /// Blocking iterator over every invocation of a contract.
    ///
    /// Returned by `BlockingClient::contract_invocations_iter`.
    ContractInvocationsIter,
    Invocation,
    ListContractInvocationsParams,
    list_contract_invocations,
    contract_invocations_iter,
    "Iterates over every invocation of a contract, following cursors lazily."
}
