package simulator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultCacheTTL is how long identical simulations are served from cache.
const DefaultCacheTTL = 30 * time.Second

// cacheMaxEntries bounds the cache so a burst of unique transactions cannot
// grow it without limit.
const cacheMaxEntries = 512

type cacheEntry struct {
	result    Result
	expiresAt time.Time
}

// Cache is a small in-memory TTL cache for simulation results. Identical
// simulations are cheap to replay but not free, so results are reused for a
// short window.
type Cache struct {
	mu    sync.Mutex
	ttl   time.Duration
	now   func() time.Time
	items map[string]cacheEntry
}

// NewCache returns a cache with the given TTL. A non-positive TTL falls back
// to DefaultCacheTTL.
func NewCache(ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	return &Cache{
		ttl:   ttl,
		now:   time.Now,
		items: make(map[string]cacheEntry),
	}
}

// Key derives a stable cache key from the given parts.
func (c *Cache) Key(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

// Get returns the cached result for key when it is still fresh.
func (c *Cache) Get(key string) (Result, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok {
		return Result{}, false
	}
	if c.now().After(e.expiresAt) {
		delete(c.items, key)
		return Result{}, false
	}
	return e.result, true
}

// Set stores a result under key.
func (c *Cache) Set(key string, res Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) >= cacheMaxEntries {
		c.evictExpiredLocked()
	}
	// If the cache is still full of fresh entries, drop one arbitrary entry
	// rather than growing past the bound.
	if len(c.items) >= cacheMaxEntries {
		for k := range c.items {
			delete(c.items, k)
			break
		}
	}
	c.items[key] = cacheEntry{result: res, expiresAt: c.now().Add(c.ttl)}
}

func (c *Cache) evictExpiredLocked() {
	now := c.now()
	for k, e := range c.items {
		if now.After(e.expiresAt) {
			delete(c.items, k)
		}
	}
}

// Service pairs an Engine with a result cache, so callers get the caching
// behavior without reimplementing it.
type Service struct {
	engine Engine
	cache  *Cache
}

// NewService returns a Service. A nil engine selects the default
// IndexedEngine; a non-positive TTL selects DefaultCacheTTL.
func NewService(engine Engine, ttl time.Duration) *Service {
	if engine == nil {
		engine = NewIndexedEngine()
	}
	return &Service{engine: engine, cache: NewCache(ttl)}
}

// Key derives a cache key from the given parts (see Cache.Key).
func (s *Service) Key(parts ...string) string {
	return s.cache.Key(parts...)
}

// Simulate returns a cached result when one is fresh, otherwise it runs the
// engine and caches the outcome. The returned Result carries Cached=true when
// it was served from cache.
func (s *Service) Simulate(ctx context.Context, in Input) (Result, error) {
	key := in.Key
	if key == "" {
		key = s.cache.Key("ledger", strconv.FormatUint(uint64(in.Ledger), 10))
	}
	if res, ok := s.cache.Get(key); ok {
		res.Cached = true
		return res, nil
	}
	res, err := s.engine.Simulate(ctx, in)
	if err != nil {
		return res, err
	}
	s.cache.Set(key, res)
	return res, nil
}
