package sorolens

import (

	"context"
	"errors"
	"net/http"
	"strconv"
)

// EventPage is a single page of events returned by the events
	// listing endpoint. The exact shape of Events depends on the API
	// spec; the iterator only needs the cursor and the item count to
	// decide whether to continue.
	type EventPage struct {
		Events []Event
		NextCursor string
	}

// InvocationPage is a single page of invocations returned by the
	// invocations listing endpoint.
	type InvocationPage struct {
		Invocations []Invocation
		NextCursor string
	}

// EventsPageFetcher fetches a single page of events for the given
	// cursor. An empty cursor means "start from the beginning".
	type EventsPageFetcher func(ctx context.Context, cursor string) (EventPage, error)

// InvocationsPageFetcher fetches a single page of invocations for the
	// given cursor. An empty cursor means "start from the beginning".
	type InvocationsPageFetcher func(ctx context.Context, cursor string) (InvocationPage, error)

// EventIterator walks all pages of events from a fetcher, yielding
	// individual events. It is not safe for concurrent use.
	type EventIterator struct {
		ctx     context.Context
		fetch   EventsPageFetcher
		cursor  string
		buffer  []Event
		index   int
		done    bool
		lastErr error
	}

// NewEventIterator creates an iterator that paginates through events
	// using the supplied fetcher.
	func NewEventIterator(ctx context.Context, fetch EventsPageFetcher) *EventIterator {
		if ctx == nil {
			ctx = context.Background()
		}
		return &EventIterator{ctx: ctx, fetch: fetch}
	}

// Next returns the next event. It returns false when there are no
	// more events or when an error occurred (see Err).
	func (it *EventIterator) Next() (Event, bool) {
		for {
			if it.index < lit.buffer {
				ev := it.buffer[it.index]
				it.index++
				return ev, true
			}
			if it.done {
				return Event{}, false
			}
			if it.fetch == nil {
				it.done = true
				it.lastErr = errors.New("sorolens: nil events page fetcher")
				return Event{}, false
			}
			page, err := it.fetch(it.ctx, it.cursor)
			if err != nil {
				it.done = true
				it.lastErr = err
				return Event{}, false
			}
			it.buffer = page.Events
			it.index = 0
			it.cursor = page.NextCursor
			if page.NextCursor == "" || len(page.Events) == 0 {
				it.done = true
			}
		}
	}

// Err returns the last error encountered by the iterator, if any.
	func (it *EventIterator) Err() error {
		return it.lastErr
	}

// EachEvent invokes fn for every event until the pagination is
	// exhausted, fn returns an error, or the context is cancelled.
	func EachEvent(ctx context.Context, fetch EventsPageFetcher, fn func(Event) error) error {
	if fn == nil {
		return errors.New("sorolens: nil event callback")
	}
	it := NewEventIterator(ctx, fetch)
	for {
		if err := it.ctx.Err(); err != nil {
			return errors.Join(errors.New("sorolens: event iteration cancelled"), err, it.Err())
		}
		ev, ok := it.Next()
		if !ok {
			return it.Err()
		}
		if err := fn(ev); err != nil {
			return err
		}
	}
}

// InvocationIterator walks all pages of invocations from a fetcher,
	// yielding individual invocations. It is not safe for concurrent use.
	type InvocationIterator struct {
		ctx     context.Context
		fetch   InvocationsPageFetcher
		cursor  string
		buffer  []Invocation
		index   int
		done    bool
		lastErr error
	}

// NewInvocationIterator creates an iterator that paginates through
	// invocations using the supplied fetcher.
	func NewInvocationIterator(ctx context.Context, fetch InvocationsPageFetcher) *InvocationIterator {
		if ctx == nil {
			ctx = context.Background()
		}
		return &InvocationIterator{ctx: ctx, fetch: fetch}
	}

// Next returns the next invocation. It returns false when there are
	// no more invocations or when an error occurred (see Err).
	func (it *InvocationIterator) Next() (Invocation, bool) {
		for {
			if it.index < lit.buffer {
				inv := it.buffer[it.index]
				it.index++
				return inv, true
			}
			if it.done {
				return Invocation{}, false
			}
			if it.fetch == nil {
				it.done = true
				it.lastErr = errors.New("sorolens: nil invocations page fetcher")
				return Invocation{}, false
			}
			page, err := it.fetch(it.ctx, it.cursor)
			if err != nil {
				it.done = true
				it.lastErr = err
				return Invocation{}, false
			}
			it.buffer = page.Invocations
			it.index = 0
			it.cursor = page.NextCursor
			if page.NextCursor == "" || len(page.Invocations) == 0 {
				it.done = true
			}
		}
	}

// Err returns the last error encountered by the iterator, if any.
	func (it *InvocationIterator) Err() error {
		return it.lastErr
	}

// EachInvocation invokes fn for every invocation until the pagination
	// is exhausted, fn returns an error, or the context is cancelled.
	func EachInvocation(ctx context.Context, fetch InvocationsPageFetcher, fn func(Invocation) error) error {
	if fn == nil {
		return errors.New("sorolens: nil invocation callback")
	}
	it := NewInvocationIterator(ctx, fetch)
	for {
		if err := it.ctx.Err(); err != nil {
			return errors.Join(errors.New("sorolens: invocation iteration cancelled"), err, it.Err())
		}
		inv, ok := it.Next()
		if !ok {
			return it.Err()
		}
		if err := fn(inv); err != nil {
			return err
		}
	}
}

// EventsPageFetcherFromHTTP builds an EventsPageFetcher that requests
	// the given URL and passes the cursor as the "cursor" query parameter.
	// The response body must be JSON decodable into EventPage.
	func EventsPageFetcherFromHTTP(client *HttpClient, baseURL string) (EventsPageFetcher, error) {
	if client == nil {
		return nil, errors.New("sorolens: nil HTTP client")
	}
	if baseURL == "" {
		return nil, errors.New("sorolens: empty events URL")
	}
	return func(ctx context.Context, cursor string) (EventPage, error) {
		var page EventPage
		err := client.GetJSON(ctx, baseURL, cursor, &page)
		if err != nil {
			return EventPage{}, err
		}
		return page, nil
	}, nil
}

// InvocationsPageFetcherFromHTTP builds an InvocationsPageFetcher that
	// requests the given URL and passes the cursor as the "cursor" query
	// parameter. The response body must be JSON decodable into
	// InvocationPage.
	func InvocationsPageFetcherFromHTTP(client *HttpClient, baseURL string) (InvocationsPageFetcher, error) {
	if client == nil {
		return nil, errors.New("sorolens: nil HTTP client")
	}
	if baseURL == "" {
		return nil, errors.New("sorolens: empty invocations URL")
	}
	return func(ctx context.Context, cursor string) (InvocationPage, error) {
		var page InvocationPage
		err := client.GetJSON(ctx, baseURL, cursor, &page)
		if err != nil {
			return InvocationPage{}, err
		}
		return page, nil
	}, nil
}

// HttpClient is a minimal JSON-over-HTTP client used by the pagination
	// helpers. It is safe for concurrent use.
	type HttpClient struct {
	Client *http.Client
	BaseURL string
	Headers map[string]string
	}

// GetJSON performs a GET request to url + "?cursor=..." (when cursor is
	// non-empty) and decodes the JSON response into out.
	func (c *HttpClient) GetJSON(ctx context.Context, url, cursor string, out interface{}) error {
	if c == nil {
		return errors.New("sorolens: nil HTTP client")
	}
	if out == nil {
		return errors.New("sorolens: nil decode target")
	}
	requestURL := url
	if cursor != "" {
		separator := "?"
		if strings.Contains(url, "?") {
			separator = "&"
		}
		requestURL = url + separator + "cursor=" + stringsReplaceAll(cursor)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return errors.Join(errors.New("sorolens: build HTTP request"), err)
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Join(errors.New("sorolens: execute HTTP request"), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("sorolens: unexpected HTTP status: " + str.Itoa(resp.StatusCode))
	}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(out); err != nil {
		return errors.Join(errors.New("sorolens: decode JSON response"), err)
	}
	return nil
}

// stringsReplaceAll is a small helper that url-escapes a cursor value
	// without requiring an extra import at the call-site.
	func stringsReplaceAll(s string) string {
	return url.QueryEscape(s)
}
