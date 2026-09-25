package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
)

func newRedisCache(t *testing.T) (*RedisCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return &RedisCache{Client: client}, mr
}

// countingHandler returns a JSON body that embeds how many times it ran, so
// a cached response is distinguishable from a fresh one.
func countingHandler(calls *atomic.Int32, status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"call":` + string(rune('0'+n)) + `}`))
	})
}

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	return w
}

func TestCacheMissThenHit(t *testing.T) {
	c, mr := newRedisCache(t)
	var calls atomic.Int32
	h := Cache(c, "test-hit", 30*time.Second, nil)(countingHandler(&calls, http.StatusOK))

	hits := testutil.ToFloat64(cacheRequests.WithLabelValues("test-hit", "hit"))
	misses := testutil.ToFloat64(cacheRequests.WithLabelValues("test-hit", "miss"))

	first := get(h, "/api/v1/contracts?network=testnet&limit=5")
	if first.Header().Get("X-Cache") != "MISS" || first.Body.String() != `{"call":1}` {
		t.Fatalf("first request: X-Cache=%q body=%s", first.Header().Get("X-Cache"), first.Body)
	}
	// Same query in a different order is the same cache entry.
	second := get(h, "/api/v1/contracts?limit=5&network=testnet")
	if second.Header().Get("X-Cache") != "HIT" || second.Body.String() != `{"call":1}` {
		t.Fatalf("second request: X-Cache=%q body=%s", second.Header().Get("X-Cache"), second.Body)
	}
	if second.Header().Get("Content-Type") != "application/json" {
		t.Errorf("hit content type: %q", second.Header().Get("Content-Type"))
	}
	if calls.Load() != 1 {
		t.Fatalf("handler ran %d times, want 1", calls.Load())
	}
	if got := testutil.ToFloat64(cacheRequests.WithLabelValues("test-hit", "hit")) - hits; got != 1 {
		t.Errorf("hit counter: +%v", got)
	}
	if got := testutil.ToFloat64(cacheRequests.WithLabelValues("test-hit", "miss")) - misses; got != 1 {
		t.Errorf("miss counter: +%v", got)
	}

	// Entries expire after the TTL.
	if ttl := mr.TTL(cacheKeyPrefix + "test-hit:GET /api/v1/contracts?limit=5&network=testnet"); ttl != 30*time.Second {
		t.Errorf("ttl: %v", ttl)
	}
	mr.FastForward(31 * time.Second)
	if w := get(h, "/api/v1/contracts?network=testnet&limit=5"); w.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("after ttl: want MISS, got %q", w.Header().Get("X-Cache"))
	}
}

func TestCacheDistinguishesQueries(t *testing.T) {
	c, _ := newRedisCache(t)
	var calls atomic.Int32
	h := Cache(c, "test-query", time.Minute, nil)(countingHandler(&calls, http.StatusOK))

	get(h, "/api/v1/contracts?network=testnet")
	if w := get(h, "/api/v1/contracts?network=mainnet"); w.Header().Get("X-Cache") != "MISS" {
		t.Fatal("different query must not share an entry")
	}
	if calls.Load() != 2 {
		t.Fatalf("handler ran %d times, want 2", calls.Load())
	}
}

func TestCacheSkipsErrors(t *testing.T) {
	c, _ := newRedisCache(t)
	var calls atomic.Int32
	h := Cache(c, "test-err", time.Minute, nil)(countingHandler(&calls, http.StatusNotFound))

	get(h, "/api/v1/contracts/CX")
	if w := get(h, "/api/v1/contracts/CX"); w.Code != http.StatusNotFound || w.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("error responses must not be cached: %d %q", w.Code, w.Header().Get("X-Cache"))
	}
}

func TestInvalidateOnWritePurgesNamespace(t *testing.T) {
	c, mr := newRedisCache(t)
	var calls atomic.Int32
	read := Cache(c, CacheNamespaceContracts, time.Minute, nil)(countingHandler(&calls, http.StatusOK))
	otherNS := Cache(c, CacheNamespaceWatchdog, time.Minute, nil)(countingHandler(&calls, http.StatusOK))

	get(read, "/api/v1/contracts")
	get(read, "/api/v1/contracts/C1")
	get(otherNS, "/api/v1/watchdog/stats")
	if n := len(mr.Keys()); n != 3 {
		t.Fatalf("want 3 cached keys, got %d", n)
	}

	purges := testutil.ToFloat64(cachePurges.WithLabelValues(CacheNamespaceContracts))
	write := InvalidateOnWrite(c, nil, CacheNamespaceContracts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	// A failed write leaves the cache alone.
	failed := InvalidateOnWrite(c, nil, CacheNamespaceContracts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	failed.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/contracts", nil))
	if n := len(mr.Keys()); n != 3 {
		t.Fatalf("failed write purged the cache: %d keys left", n)
	}

	write.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/contracts", nil))
	keys := mr.Keys()
	if len(keys) != 1 || keys[0] != cacheKeyPrefix+"watchdog:GET /api/v1/watchdog/stats?" {
		t.Fatalf("want only the watchdog entry left, got %v", keys)
	}
	if got := testutil.ToFloat64(cachePurges.WithLabelValues(CacheNamespaceContracts)) - purges; got != 1 {
		t.Errorf("purge counter: +%v", got)
	}
	if w := get(read, "/api/v1/contracts"); w.Header().Get("X-Cache") != "MISS" {
		t.Fatal("read after write must miss")
	}
}

func TestRedisCachePurgeManyKeys(t *testing.T) {
	c, mr := newRedisCache(t)
	ctx := context.Background()
	for i := 0; i < 1234; i++ {
		if err := c.Set(ctx, cacheKeyPrefix+"contracts:GET /x?"+string(rune(i)), []byte("{}"), time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Purge(ctx, "contracts"); err != nil {
		t.Fatal(err)
	}
	if n := len(mr.Keys()); n != 0 {
		t.Fatalf("%d keys survived the purge", n)
	}
}

type brokenCache struct{}

func (brokenCache) Get(context.Context, string) ([]byte, bool, error) {
	return nil, false, errors.New("redis down")
}
func (brokenCache) Set(context.Context, string, []byte, time.Duration) error {
	return errors.New("redis down")
}
func (brokenCache) Purge(context.Context, string) error { return errors.New("redis down") }

func TestCacheFailsOpen(t *testing.T) {
	var calls atomic.Int32
	h := Cache(brokenCache{}, "test-broken", time.Minute, nil)(countingHandler(&calls, http.StatusOK))
	for i := 0; i < 2; i++ {
		if w := get(h, "/api/v1/contracts"); w.Code != http.StatusOK {
			t.Fatalf("request failed with a broken cache: %d", w.Code)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("handler ran %d times, want 2", calls.Load())
	}

	write := InvalidateOnWrite(brokenCache{}, nil, "x")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	w := httptest.NewRecorder()
	write.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/contracts", nil))
	if w.Code != http.StatusCreated {
		t.Fatalf("write failed because purge failed: %d", w.Code)
	}
}

func TestCacheDisabled(t *testing.T) {
	var calls atomic.Int32
	next := countingHandler(&calls, http.StatusOK)
	for _, h := range []http.Handler{
		Cache(nil, "x", time.Minute, nil)(next),
		Cache(brokenCache{}, "x", 0, nil)(next),
	} {
		if w := get(h, "/"); w.Header().Get("X-Cache") != "" {
			t.Fatal("disabled cache must not set X-Cache")
		}
	}
}
