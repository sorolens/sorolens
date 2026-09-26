package coldstorage

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Reader answers event queries from archived Parquet objects. The API calls it
// only when the hot store has nothing for the requested ledger range, so a
// query that spans the archive boundary still returns data.
type Reader struct {
	bucket ObjectStore
}

// NewReader returns a Reader over the given object store.
func NewReader(bucket ObjectStore) *Reader {
	return &Reader{bucket: bucket}
}

// Events returns the archived events for contractID whose ledger is within
// [from, to] inclusive. A zero bound means unbounded on that side, matching
// the `from`/`to` query parameters on the v1 events endpoint. Results are
// ordered oldest first, the same ordering the Postgres path uses.
//
// limit caps the number of rows returned; 0 means no cap.
func (r *Reader) Events(ctx context.Context, contractID string, from, to uint32, limit int) ([]store.Event, error) {
	keys, err := r.bucket.List(ctx, eventsPrefix(contractID))
	if err != nil {
		return nil, err
	}

	var out []store.Event
	for _, key := range keys {
		data, err := r.bucket.Get(ctx, key)
		if errors.Is(err, ErrObjectNotFound) {
			// Raced with a concurrent delete; nothing to read here.
			continue
		}
		if err != nil {
			return nil, err
		}
		events, err := DecodeEvents(data)
		if err != nil {
			return nil, fmt.Errorf("cold read %s: %w", key, err)
		}
		for _, e := range events {
			if from != 0 && e.Ledger < from {
				continue
			}
			if to != 0 && e.Ledger > to {
				continue
			}
			out = append(out, e)
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Ledger != out[j].Ledger {
			return out[i].Ledger < out[j].Ledger
		}
		return out[i].ID < out[j].ID
	})

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
