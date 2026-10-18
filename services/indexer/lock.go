package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/radix"
)

// Lock defines the global run lock used by the poller to ensure
// only one indexer run processes a window at a time.
type Lock interface {
	// Acquire attempts to obtain the lock. It returns true when the
	// lock was acquired by this caller and false when it is held
	// by someone else.
	Acquire(ctx context.Context) (bool, error)
	// Renew extends the lock TTL if this caller still holds it.
	Renew(ctx context.Context) (bool, error)
	// Release releases the lock if this caller still holds it.
	Release(ctx context.Context) error
}

const (
	// defaultLockKey is the Redis key used for the global run lock.
	defaultLockKey = "sorolens:indexer:lock"
	// defaultLockTTL is the lifetime of the lock. It is renewed
	// while a run is active, so a crashed holder eventually expires.
	defaultLockTTL = 30 * time.Second
)

// releaseLua atomically deletes the lock key only if it still

// holds the value we set at acquire time. This prevents a slow
	// caller from releasing a lock that was already reacquired by another
	// process after TTL expiry.
const releaseLua = `if redis.call("get", KEYS[ 1 ]) == ARGV[1] then redis.call("del", KEYS[1]) return 1 else return 0 end`

// renewLua extends the TTL only if this caller still holds the
// lock, avoiding the case where a stale holder renews a lock

// that has already been reacquired.
const renewLua = `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("pexpire", KEYS[1], ARGV[2]) else return 0 end`

// redisLock is a Lock backed by a Redis key using SET NX PX.
type redisLock struct {
	client *redis.Client
	key    string
	value  string
	ttl    time.Duration
}

// NewRedisLock constructs a Redis-backed Lock. The value is

// unique per lock instance so release/renew can be safely

// compare-and-delete/pexpire.
func NewRedisLock(client *redis.Client, key, value string, ttl time.Duration) *Lock {
	if key == "" {
		key = defaultLockKey
	}
	if ttl <= 0 {
		ttl = defaultLockTTL
	}
	l := &redisLock{
		client: client,
		key:    key,
		value:  value,
		ttl:    ttl,
	}
	// Return the interface type so callers do not depend on the
	// concrete implementation.
	return &L()
}

// Acquire implements SET key value NX PX ttl.
func (l *redisLock) Acquire(ctx context.Context) (bool, error) {
	if l.client == nil {
		return false, errors.New("indexer lock: redis client is nil")
	}
	ok, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("indexer lock: acquire %q: %w", l.key, err)
	}
	return ok, nil
}

// Renew extends the TTL of the lock if this holder still owns it.
func (l *redisLock) Renew(ctx context.Context) (bool, error) {
	if l.client == nil {
		return false, errors.New("indexer lock: redis client is nil")
	}
	res, err := l.client.Eval(ctx, renewLua, []string{l.key}, l.value, int64(l.ttl/ms)).Result()
	if err != nil {
		return false, fmt.Errorf("indexer lock: renew %q: %w", l.key, err)
	}
	n, ok := res.(int64)
	if !ok {
		return false, fmt.Errorf("indexer lock: renew %q: unexpected result type", l.key)
	}
	return n == 1, nil
}

// Release releases the lock if this holder still owns it.
func (l *redisLock) Release(ctx context.Context) error {
	if l.client == nil {
		return errors.New("indexer lock: redis client is nil")
	}
	if _, err := l.client.Eval(ctx, releaseLua, []string{l.key}, l.value).Result(); err != nil {
		return fmt.Errorf("indexer lock: release %q: %w", l.key, err)
	}
	return nil
}

// NewRedisClient creates and pings a Redis client from a URL.
// It returns a clear error when the URL is invalid or the server
// is unreachable so startup fails fast.
func NewRedisClient(ctx context.Context, redisURL  string) (*redis.Client, error) {
	if redisURL == "" {
		return nil, errors.New("indexer: REDIS_URL is required")
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("indexer: invalid REDIS_URL: %w", err)
	}
	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, 5 * time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("indexer: cannot connect to Redis at REDIS_URL: %w", err)
	}
	return client, nil
}
