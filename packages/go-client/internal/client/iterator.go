package client

import (
	"context"
	"errors"
	"iter"
	"net/http"
)

// PageFetcher is a function that retrieves a single page of items given an opaque cursor.
// It returns the slice of items, the cursor for the next page (empty when done), and any error.
type PageFetcher[T any] func(ctx context.Context, cursor string) (items []T, nextCursor string, err error)

// PageIterator provides cursor-based iteration over paginated API resources.
// It fetches pages lazily as items are consumed and handles cursor propagation.
type PageIterator[T any] struct {
	ctx        context.Context
	fetcher    PageFetcher[T]
	cursor     string
	nextCursor string
	items      []T
	index      int
	current    T
	err        error
	hasMore    bool
	exhausted  bool
}

// NewPageIterator constructs a new PageIterator starting from initialCursor.
func NewPageIterator[T any](ctx context.Context, fetcher PageFetcher[T], initialCursor string) *PageIterator[T] {
	if ctx == nil {
		ctx = context.Background()
	}
	return &PageIterator[T]{
		ctx:        ctx,
		fetcher:    fetcher,
		cursor:     initialCursor,
		nextCursor: initialCursor,
		hasMore:    true,
	}
}

// Next advances the iterator to the next item across pages.
// An optional context overrides the iterator's default context for this step.
// It returns true if an item is available via Item(), or false when finished or on error.
func (it *PageIterator[T]) Next(ctxOverride ...context.Context) bool {
	if it.err != nil || it.exhausted {
		return false
	}

	ctx := it.ctx
	if len(ctxOverride) > 0 && ctxOverride[0] != nil {
		ctx = ctxOverride[0]
	}

	// Consume remaining items in the current page buffer
	if it.index < len(it.items) {
		it.current = it.items[it.index]
		it.index++
		return true
	}

	// Buffer is empty. Check if more pages exist.
	if !it.hasMore {
		it.exhausted = true
		return false
	}

	// Check context cancellation before network call
	if err := ctx.Err(); err != nil {
		it.err = err
		return false
	}

	// Fetch next page
	items, nextCursor, err := it.fetcher(ctx, it.nextCursor)
	if err != nil {
		it.err = err
		return false
	}

	it.items = items
	it.index = 0
	it.cursor = it.nextCursor
	it.nextCursor = nextCursor
	it.hasMore = (nextCursor != "")

	if len(items) > 0 {
		it.current = items[0]
		it.index = 1
		return true
	}

	if !it.hasMore {
		it.exhausted = true
		return false
	}

	// If an empty page arrived with a next cursor, continue fetching
	return it.Next(ctx)
}

// Item returns the current item yielded by the most recent Next() call.
func (it *PageIterator[T]) Item() T {
	return it.current
}

// Err returns the error that stopped iteration, if any.
func (it *PageIterator[T]) Err() error {
	return it.err
}

// Cursor returns the cursor of the current page being iterated.
func (it *PageIterator[T]) Cursor() string {
	return it.cursor
}

// NextCursor returns the cursor for the next page, or empty if at the end.
func (it *PageIterator[T]) NextCursor() string {
	return it.nextCursor
}

// HasMore returns true if more items or pages may be available.
func (it *PageIterator[T]) HasMore() bool {
	return !it.exhausted && (it.hasMore || it.index < len(it.items))
}

// NextPage fetches and returns the next complete page of items.
// Useful for batch processing or UI page display.
func (it *PageIterator[T]) NextPage(ctxOverride ...context.Context) ([]T, error) {
	if it.err != nil {
		return nil, it.err
	}
	if !it.HasMore() {
		return nil, nil
	}

	ctx := it.ctx
	if len(ctxOverride) > 0 && ctxOverride[0] != nil {
		ctx = ctxOverride[0]
	}

	if it.index < len(it.items) {
		remaining := it.items[it.index:]
		it.index = len(it.items)
		return remaining, nil
	}

	if err := ctx.Err(); err != nil {
		it.err = err
		return nil, err
	}

	items, nextCursor, err := it.fetcher(ctx, it.nextCursor)
	if err != nil {
		it.err = err
		return nil, err
	}

	it.items = items
	it.index = len(items)
	it.cursor = it.nextCursor
	it.nextCursor = nextCursor
	it.hasMore = (nextCursor != "")
	if !it.hasMore && len(items) == 0 {
		it.exhausted = true
	}
	return items, nil
}

// Collect reads all remaining items across all pages into a slice.
func (it *PageIterator[T]) Collect(ctxOverride ...context.Context) ([]T, error) {
	var results []T
	for it.Next(ctxOverride...) {
		results = append(results, it.Item())
	}
	if err := it.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// All returns a standard Go 1.23+ sequence iterator (iter.Seq2[T, error]).
// This enables natural for-range loops: `for item, err := range it.All() { ... }`.
func (it *PageIterator[T]) All(ctxOverride ...context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for it.Next(ctxOverride...) {
			if !yield(it.Item(), nil) {
				return
			}
		}
		if err := it.Err(); err != nil {
			var zero T
			yield(zero, err)
		}
	}
}

// ---- public iterator methods on ApiClient ----------------------------------

// IterateContracts returns a PageIterator over contracts.
func (c *ApiClient) IterateContracts(ctx context.Context, params *ListContractsParams) *PageIterator[Contract] {
	var initialCursor string
	p := ListContractsParams{}
	if params != nil {
		p = *params
		if p.Cursor != nil {
			initialCursor = *p.Cursor
		}
	}
	return NewPageIterator(ctx, func(reqCtx context.Context, cursor string) ([]Contract, string, error) {
		pCopy := p
		if cursor != "" {
			pCopy.Cursor = &cursor
		}
		resp, err := c.ListContracts(reqCtx, &pCopy)
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, "", decodeError(resp.Status(), resp.Body)
		}
		if resp.JSON200 == nil {
			return nil, "", errors.New("go-client: unexpected nil JSON200 in ListContracts response")
		}
		return resp.JSON200.Contracts, resp.JSON200.NextCursor, nil
	}, initialCursor)
}

// IterateContractEvents returns a PageIterator over events for a specific contract.
func (c *ApiClient) IterateContractEvents(ctx context.Context, contractID string, params *ListContractEventsParams) *PageIterator[Event] {
	var initialCursor string
	p := ListContractEventsParams{}
	if params != nil {
		p = *params
		if p.Cursor != nil {
			initialCursor = *p.Cursor
		}
	}
	return NewPageIterator(ctx, func(reqCtx context.Context, cursor string) ([]Event, string, error) {
		pCopy := p
		if cursor != "" {
			pCopy.Cursor = &cursor
		}
		resp, err := c.ListContractEvents(reqCtx, contractID, &pCopy)
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, "", decodeError(resp.Status(), resp.Body)
		}
		if resp.JSON200 == nil {
			return nil, "", errors.New("go-client: unexpected nil JSON200 in ListContractEvents response")
		}
		return resp.JSON200.Events, resp.JSON200.NextCursor, nil
	}, initialCursor)
}

// IterateAllEvents returns a PageIterator over events across all contracts.
func (c *ApiClient) IterateAllEvents(ctx context.Context, params *ListAllEventsParams) *PageIterator[Event] {
	var initialCursor string
	p := ListAllEventsParams{}
	if params != nil {
		p = *params
		if p.Cursor != nil {
			initialCursor = *p.Cursor
		}
	}
	return NewPageIterator(ctx, func(reqCtx context.Context, cursor string) ([]Event, string, error) {
		pCopy := p
		if cursor != "" {
			pCopy.Cursor = &cursor
		}
		resp, err := c.ListAllEvents(reqCtx, &pCopy)
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, "", decodeError(resp.Status(), resp.Body)
		}
		if resp.JSON200 == nil {
			return nil, "", errors.New("go-client: unexpected nil JSON200 in ListAllEvents response")
		}
		return resp.JSON200.Events, resp.JSON200.NextCursor, nil
	}, initialCursor)
}

// IterateContractInvocations returns a PageIterator over invocations for a contract.
func (c *ApiClient) IterateContractInvocations(ctx context.Context, contractID string, params *ListContractInvocationsParams) *PageIterator[Invocation] {
	var initialCursor string
	p := ListContractInvocationsParams{}
	if params != nil {
		p = *params
		if p.Cursor != nil {
			initialCursor = *p.Cursor
		}
	}
	return NewPageIterator(ctx, func(reqCtx context.Context, cursor string) ([]Invocation, string, error) {
		pCopy := p
		if cursor != "" {
			pCopy.Cursor = &cursor
		}
		resp, err := c.ListContractInvocations(reqCtx, contractID, &pCopy)
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, "", decodeError(resp.Status(), resp.Body)
		}
		if resp.JSON200 == nil {
			return nil, "", errors.New("go-client: unexpected nil JSON200 in ListContractInvocations response")
		}
		return resp.JSON200.Invocations, resp.JSON200.NextCursor, nil
	}, initialCursor)
}

// IterateContractStorage returns a PageIterator over storage entries for a contract.
func (c *ApiClient) IterateContractStorage(ctx context.Context, contractID string, params *ListContractStorageParams) *PageIterator[StorageEntry] {
	var initialCursor string
	p := ListContractStorageParams{}
	if params != nil {
		p = *params
		if p.Cursor != nil {
			initialCursor = *p.Cursor
		}
	}
	return NewPageIterator(ctx, func(reqCtx context.Context, cursor string) ([]StorageEntry, string, error) {
		pCopy := p
		if cursor != "" {
			pCopy.Cursor = &cursor
		}
		resp, err := c.ListContractStorage(reqCtx, contractID, &pCopy)
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, "", decodeError(resp.Status(), resp.Body)
		}
		if resp.JSON200 == nil {
			return nil, "", errors.New("go-client: unexpected nil JSON200 in ListContractStorage response")
		}
		return resp.JSON200.Storage, resp.JSON200.NextCursor, nil
	}, initialCursor)
}

// IterateWatchdogAlerts returns a PageIterator over watchdog alerts.
func (c *ApiClient) IterateWatchdogAlerts(ctx context.Context, params *ListWatchdogAlertsParams) *PageIterator[ContractAlert] {
	var initialCursor string
	p := ListWatchdogAlertsParams{}
	if params != nil {
		p = *params
		if p.Cursor != nil {
			initialCursor = *p.Cursor
		}
	}
	return NewPageIterator(ctx, func(reqCtx context.Context, cursor string) ([]ContractAlert, string, error) {
		pCopy := p
		if cursor != "" {
			pCopy.Cursor = &cursor
		}
		resp, err := c.ListWatchdogAlerts(reqCtx, &pCopy)
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, "", decodeError(resp.Status(), resp.Body)
		}
		if resp.JSON200 == nil {
			return nil, "", errors.New("go-client: unexpected nil JSON200 in ListWatchdogAlerts response")
		}
		return resp.JSON200.Alerts, resp.JSON200.NextCursor, nil
	}, initialCursor)
}

// IterateMonitoredContracts returns a PageIterator over watchdog-monitored contracts.
func (c *ApiClient) IterateMonitoredContracts(ctx context.Context, params *ListMonitoredContractsParams) *PageIterator[MonitoredContract] {
	var initialCursor string
	p := ListMonitoredContractsParams{}
	if params != nil {
		p = *params
		if p.Cursor != nil {
			initialCursor = *p.Cursor
		}
	}
	return NewPageIterator(ctx, func(reqCtx context.Context, cursor string) ([]MonitoredContract, string, error) {
		pCopy := p
		if cursor != "" {
			pCopy.Cursor = &cursor
		}
		resp, err := c.ListMonitoredContracts(reqCtx, &pCopy)
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, "", decodeError(resp.Status(), resp.Body)
		}
		if resp.JSON200 == nil {
			return nil, "", errors.New("go-client: unexpected nil JSON200 in ListMonitoredContracts response")
		}
		return resp.JSON200.Contracts, resp.JSON200.NextCursor, nil
	}, initialCursor)
}

// IterateContractAlerts returns a PageIterator over watchdog alerts for a specific contract.
func (c *ApiClient) IterateContractAlerts(ctx context.Context, contractID string, params *ListContractAlertsParams) *PageIterator[ContractAlert] {
	var initialCursor string
	p := ListContractAlertsParams{}
	if params != nil {
		p = *params
		if p.Cursor != nil {
			initialCursor = *p.Cursor
		}
	}
	return NewPageIterator(ctx, func(reqCtx context.Context, cursor string) ([]ContractAlert, string, error) {
		pCopy := p
		if cursor != "" {
			pCopy.Cursor = &cursor
		}
		resp, err := c.ListContractAlerts(reqCtx, contractID, &pCopy)
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, "", decodeError(resp.Status(), resp.Body)
		}
		if resp.JSON200 == nil {
			return nil, "", errors.New("go-client: unexpected nil JSON200 in ListContractAlerts response")
		}
		return resp.JSON200.Alerts, resp.JSON200.NextCursor, nil
	}, initialCursor)
}
