// Package lock implements a Redis-backed advisory lock for the indexer.
//
// Locks are acquired with SET key value NX PX <ttl> and released with a Lua
// compare-and-delete script, so one process can never delete another process's
// lock. The lock can be renewed while a run is active to keep long passes from
// losing it.
//
// The zero value is not usable; construct one with New.
package lock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// releaseScript deletes key only if its current value equals expectedValue.
const releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
else
	return 0
end
`

// renewScript resets key's TTL to ttl milliseconds only if its current value
// equals expectedValue.
const renewScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
else
	return 0
end
`

// Lock is a Redis-backed advisory lock.
type Lock struct {
	client  *redis.Client
	release *redis.Script
	renew   *redis.Script
}

// New parses url, connects to Redis, and verifies the connection at startup.
// A bad URL or an unreachable server fails fast with a clear error.
func New(url string) (*Lock, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("lock: invalid REDIS_URL %q: %w", url, err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("lock: redis unreachable: %w", err)
	}
	return &Lock{
		client:  client,
		release: redis.NewScript(releaseScript),
		renew:   redis.NewScript(renewScript),
	}, nil
}

// Acquire sets key to value with ttl only if key does not exist (SET NX PX).
// Returns true when the lock was acquired, false when another holder owns it.
func (l *Lock) Acquire(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	val, err := l.client.Do(ctx, "SET", key, value, "NX", "PX", ttl.Milliseconds()).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}
	return val == "OK", nil
}

// SetNX is an alias for Acquire, satisfying the poller.RedisClient interface.
func (l *Lock) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return l.Acquire(ctx, key, value, ttl)
}

// Del deletes key only if its value still equals expectedValue, using a Lua
// compare-and-delete so a stale lock is never removed by the wrong holder.
// It satisfies the poller.RedisClient interface.
func (l *Lock) Del(ctx context.Context, key, expectedValue string) error {
	n, err := l.release.Run(ctx, l.client, []string{key}, expectedValue).Int()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("lock: %s not held by this caller (value mismatch)", key)
	}
	return nil
}

// Release deletes key only if its value still equals expectedValue, using a
// Lua compare-and-delete so a stale lock is never removed by the wrong holder.
func (l *Lock) Release(ctx context.Context, key, expectedValue string) error {
	return l.Del(ctx, key, expectedValue)
}

// Renew resets key's TTL to ttl only if key's current value equals
// expectedValue. Returns true when the renewal succeeded.
func (l *Lock) Renew(ctx context.Context, key, expectedValue string, ttl time.Duration) (bool, error) {
	n, err := l.renew.Run(ctx, l.client, []string{key}, expectedValue, ttl.Milliseconds()).Int()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// Ping checks the Redis connection.
func (l *Lock) Ping(ctx context.Context) error {
	return l.client.Ping(ctx).Err()
}

// Close releases the Redis client.
func (l *Lock) Close() error {
	return l.client.Close()
}
