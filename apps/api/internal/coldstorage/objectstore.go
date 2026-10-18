package coldstorage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrObjectNotFound is returned by an ObjectStore Get when the key does not
// exist. Callers treat it as "nothing archived yet" rather than a failure.
var ErrObjectNotFound = errors.New("coldstorage: object not found")

// ObjectStore is the narrow object-storage surface the cold tier needs. It is
// implemented by S3Store for real deployments and by MemoryStore for tests, so
// the archiver and reader can be exercised without a container.
type ObjectStore interface {
	// Put writes or replaces the object at key.
	Put(ctx context.Context, key string, data []byte) error
	// Get returns the object at key, or ErrObjectNotFound.
	Get(ctx context.Context, key string) ([]byte, error)
	// List returns every key under prefix, in lexicographic order.
	List(ctx context.Context, prefix string) ([]string, error)
	// Delete removes the object at key. Deleting a missing key is not an
	// error, so a cleanup or retention pass is safe to retry.
	Delete(ctx context.Context, key string) error
}

// MemoryStore is an in-process ObjectStore. It backs unit tests and local
// development when no S3-compatible endpoint is configured.
type MemoryStore struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{objects: make(map[string][]byte)}
}

// Put implements ObjectStore.
func (m *MemoryStore) Put(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	m.objects[key] = cp
	return nil
}

// Get implements ObjectStore.
func (m *MemoryStore) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.objects[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrObjectNotFound, key)
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

// List implements ObjectStore.
func (m *MemoryStore) List(_ context.Context, prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var keys []string
	for k := range m.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// Delete implements ObjectStore.
func (m *MemoryStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

// Len reports how many objects are stored. Test helper.
func (m *MemoryStore) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.objects)
}

// ---- key layout -------------------------------------------------------------

// eventsPrefix is the object-storage prefix holding every archived event for
// one contract.
func eventsPrefix(contractID string) string {
	return "events/" + contractID + "/"
}

// objectKey is the Parquet key for the month containing t.
func objectKey(contractID string, t time.Time) string {
	return eventsPrefix(contractID) + t.UTC().Format("2006-01") + ".parquet"
}

// monthKey is the month bucket an event belongs to.
func monthKey(t time.Time) string {
	return t.UTC().Format("2006-01")
}
