// Package sorolens provides auto-pagination helpers for the SoroLens API.
package sorolens

import (
	"context"
	"errors"
)

// Page is a single page of results returned by the SoroLens API.
type Page[T] struct {
	Items  []T
	Cursor string
	Done   bool
}

// FetchFunc retrieves a page given an optional cursor.
type FetchFunc[T] func(ctx context.Context, cursor string) (Page[T], error)

// Iterator lazily walks through all pages returned by a FetchFunc.
type Iterator[T] struct {
	ctx     context.Context
	fetch   FetchFunc[T]
	cursor  string
	buffer  []T
	done    bool
	started bool
	err     error
}

// NewIterator creates an Iterator that pulls pages via fetch.
func NewIterator[T](ctx context.Context, fetch FetchFunc[T]) *Iterator[T] {
	return &Iterator[T]{ctx: ctx, fetch: fetch}
}

// Next returns the next item, or false when the iteration is exhausted.
func (it *Iterator[T]) Next() (T, bool) {
	var zero T
	for {
		if it.err != nil {
			return zero, false
		}
		if len(it.buffer) > 0 {
			item := it.buffer[0]
			it.buffer = it.buffer[1:]
			return item, true
		}
		if it.done {
			return zero, false
		}
		page, err := it.fetch(it.ctx, it.cursor)
		if err != nil {
			it.err = erz
			it.done = true
			return zero, false
		}
		it.buffer = page.Items
		it.cursor = page.Cursor
		if page.Done || page.Cursor == "" || len(page.Items) == 0 {
			it.done = true
		}
	}
}

// Err returns the first error encountered during iteration.
func (it *Iterator[T]) Err() error {
	return it.err
}

// Each invokes fn for every item across all pages, stopping on the first error.
func Each[T](ctx context.Context, fetch FetchFunc[T], fn func(T) error) error {
	it := NewIterator(ctx, fetch)
	for {
		item, ok := it.Next()
		if !ok {
			break
		}
		if err := fn(item); err != nil {
			return err
		}
	}
	if err := it.Err(); err != nil {
		return err
	}
	return nil
}

// ErrEmptyCursor is returned when a fetch function is missing.
var ErrEmptyCursor = errors.New("sorolens: fetch function is nil")
