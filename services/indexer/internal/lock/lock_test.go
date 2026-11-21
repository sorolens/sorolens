package lock

import (
	"context"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestLock_AcquireAndRelease(t *testing.T) {
	t.Parallel()

	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	t.Cleanup(srv.Close)

	l, err := New("redis://" + srv.Addr())
	if err != nil {
		t.Fatalf("new lock: %v", err)
	}
	defer l.Close()

	ctx := context.Background()

	ok, err := l.Acquire(ctx, "key1", "holder-a", 10*time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if !ok {
		t.Fatal("expected acquire to succeed")
	}

	// Release with the wrong value must fail.
	if err := l.Del(ctx, "key1", "holder-b"); err == nil {
		t.Fatal("expected release with wrong value to fail")
	}

	// Release with the right value must succeed.
	if err := l.Del(ctx, "key1", "holder-a"); err != nil {
		t.Fatalf("release: %v", err)
	}

	// After release the key must be gone.
	n, err := l.client.Exists(ctx, "key1").Result()
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if n != 0 {
		t.Fatalf("key should not exist after release, got %d", n)
	}
}

func TestLock_Contention(t *testing.T) {
	t.Parallel()

	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	t.Cleanup(srv.Close)

	l, err := New("redis://" + srv.Addr())
	if err != nil {
		t.Fatalf("new lock: %v", err)
	}
	defer l.Close()

	ctx := context.Background()

	// First holder acquires the lock.
	ok, err := l.Acquire(ctx, "key1", "holder-a", 10*time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if !ok {
		t.Fatal("expected first acquire to succeed")
	}

	// A second holder cannot acquire the same lock.
	ok, err = l.Acquire(ctx, "key1", "holder-b", 10*time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if ok {
		t.Fatal("expected second acquire to fail while lock is held")
	}

	// The first holder releases.
	if err := l.Del(ctx, "key1", "holder-a"); err != nil {
		t.Fatalf("release: %v", err)
	}

	// Now the second holder can acquire.
	ok, err = l.Acquire(ctx, "key1", "holder-b", 10*time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if !ok {
		t.Fatal("expected acquire after release to succeed")
	}

	if err := l.Del(ctx, "key1", "holder-b"); err != nil {
		t.Fatalf("release: %v", err)
	}
}

func TestLock_Expiry(t *testing.T) {
	t.Parallel()

	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	t.Cleanup(srv.Close)

	l, err := New("redis://" + srv.Addr())
	if err != nil {
		t.Fatalf("new lock: %v", err)
	}
	defer l.Close()

	ctx := context.Background()

	ok, err := l.Acquire(ctx, "key1", "holder-a", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if !ok {
		t.Fatal("expected acquire to succeed")
	}

	// While the TTL is still valid, another holder cannot acquire.
	ok, err = l.Acquire(ctx, "key1", "holder-b", 10*time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if ok {
		t.Fatal("expected acquire to fail before expiry")
	}

	// Advance miniredis's simulated clock past the TTL.
	srv.FastForward(200 * time.Millisecond)

	// The key's TTL should have elapsed (ttl <= 0 means expired).
	ttl, err := l.client.PTTL(ctx, "key1").Result()
	if err != nil {
		t.Fatalf("pttl: %v", err)
	}
	if ttl > 0 {
		t.Fatalf("expected expired key, got ttl=%v", ttl)
	}

	// After the TTL has passed, miniredis has cleaned up the key. The lock
	// should be acquirable again by another holder.
	ok, err = l.Acquire(ctx, "key1", "holder-b", 10*time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if !ok {
		t.Fatal("expected acquire after expiry to succeed")
	}

	if err := l.Del(ctx, "key1", "holder-b"); err != nil {
		t.Fatalf("release: %v", err)
	}
}

func TestLock_Renew(t *testing.T) {
	t.Parallel()

	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	t.Cleanup(srv.Close)

	l, err := New("redis://" + srv.Addr())
	if err != nil {
		t.Fatalf("new lock: %v", err)
	}
	defer l.Close()

	ctx := context.Background()

	ok, err := l.Acquire(ctx, "key1", "holder-a", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if !ok {
		t.Fatal("expected acquire to succeed")
	}

	// Renewal with the wrong value must fail.
	ok, err = l.Renew(ctx, "key1", "holder-b", 10*time.Second)
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if ok {
		t.Fatal("expected renewal with wrong value to fail")
	}

	// Renewal with the right value must succeed.
	ok, err = l.Renew(ctx, "key1", "holder-a", 10*time.Second)
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if !ok {
		t.Fatal("expected renewal to succeed")
	}

	if err := l.Del(ctx, "key1", "holder-a"); err != nil {
		t.Fatalf("release: %v", err)
	}
}

func TestLock_ParallelHoldersDifferentKeys(t *testing.T) {
	t.Parallel()

	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	t.Cleanup(srv.Close)

	l, err := New("redis://" + srv.Addr())
	if err != nil {
		t.Fatalf("new lock: %v", err)
	}
	defer l.Close()

	ctx := context.Background()

	var wg sync.WaitGroup
	acquired := make(chan string, 2)

	for i := 0; i < 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := "key-" + string(rune('a'+i))
			ok, err := l.Acquire(ctx, key, "holder", 10*time.Second)
			if err != nil {
				acquired <- "err:" + err.Error()
				return
			}
			if ok {
				acquired <- key
			} else {
				acquired <- "denied"
			}
		}()
	}

	wg.Wait()
	close(acquired)

	var got []string
	for k := range acquired {
		got = append(got, k)
	}

	// Both distinct keys should be acquirable concurrently.
	for _, k := range got {
		if k == "denied" || len(k) > 0 && k[:4] == "err:" {
			t.Fatalf("unexpected result: %s", k)
		}
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 successful acquires, got %v", got)
	}

	// Clean up.
	for i := 0; i < 2; i++ {
		key := "key-" + string(rune('a'+i))
		if err := l.Del(ctx, key, "holder"); err != nil {
			t.Fatalf("release %s: %v", key, err)
		}
	}
}

// TestNewToken_Unique checks that every acquisition gets a distinct token:
// the lock value used to be the constant "1", so two holders were
// indistinguishable. Tokens must also be 128-bit hex strings.
func TestNewToken_Unique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		tok, err := NewToken()
		if err != nil {
			t.Fatalf("new token: %v", err)
		}
		if len(tok) != 32 {
			t.Fatalf("token length: got %d, want 32 (%q)", len(tok), tok)
		}
		if _, err := hex.DecodeString(tok); err != nil {
			t.Fatalf("token %q is not hex: %v", tok, err)
		}
		if _, dup := seen[tok]; dup {
			t.Fatalf("duplicate token generated: %q", tok)
		}
		seen[tok] = struct{}{}
	}
}

// TestLock_CompareAndDeleteSafety checks that a second holder cannot acquire
// a held key and that its release and renewal leave the first holder's lock
// untouched.
func TestLock_CompareAndDeleteSafety(t *testing.T) {
	t.Parallel()

	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	t.Cleanup(srv.Close)

	l, err := New("redis://" + srv.Addr())
	if err != nil {
		t.Fatalf("new lock: %v", err)
	}
	defer l.Close()

	ctx := context.Background()
	tokenA, err := NewToken()
	if err != nil {
		t.Fatalf("token a: %v", err)
	}
	tokenB, err := NewToken()
	if err != nil {
		t.Fatalf("token b: %v", err)
	}

	// A acquires the lock.
	ok, err := l.Acquire(ctx, "key1", tokenA, 10*time.Second)
	if err != nil {
		t.Fatalf("A acquire: %v", err)
	}
	if !ok {
		t.Fatal("expected A to acquire the lock")
	}

	// B cannot acquire the same key while A holds it.
	ok, err = l.Acquire(ctx, "key1", tokenB, 10*time.Second)
	if err != nil {
		t.Fatalf("B acquire: %v", err)
	}
	if ok {
		t.Fatal("expected B's acquire to fail while A holds the lock")
	}

	// B's release must not delete A's lock.
	if err := l.Del(ctx, "key1", tokenB); err == nil {
		t.Fatal("expected B's release to fail with a value mismatch")
	}
	got, err := l.client.Get(ctx, "key1").Result()
	if err != nil {
		t.Fatalf("get lock value: %v", err)
	}
	if got != tokenA {
		t.Fatalf("lock value after B's release: got %q, want A's token %q", got, tokenA)
	}

	// B's renew must not extend or change A's lock.
	renewed, err := l.Renew(ctx, "key1", tokenB, 10*time.Second)
	if err != nil {
		t.Fatalf("B renew: %v", err)
	}
	if renewed {
		t.Fatal("expected B's renew to fail with a value mismatch")
	}
	got, err = l.client.Get(ctx, "key1").Result()
	if err != nil {
		t.Fatalf("get lock value: %v", err)
	}
	if got != tokenA {
		t.Fatalf("lock value after B's renew: got %q, want A's token %q", got, tokenA)
	}

	// A can still renew and release with its own token.
	renewed, err = l.Renew(ctx, "key1", tokenA, 10*time.Second)
	if err != nil {
		t.Fatalf("A renew: %v", err)
	}
	if !renewed {
		t.Fatal("expected A's renew to succeed")
	}
	if err := l.Del(ctx, "key1", tokenA); err != nil {
		t.Fatalf("A release: %v", err)
	}
}

// TestLock_ReleaseAfterRunContextCancelled checks the shutdown path: once the
// run context is cancelled, a release on a fresh context still removes the
// lock so the next process can take over.
func TestLock_ReleaseAfterRunContextCancelled(t *testing.T) {
	t.Parallel()

	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	t.Cleanup(srv.Close)

	l, err := New("redis://" + srv.Addr())
	if err != nil {
		t.Fatalf("new lock: %v", err)
	}
	defer l.Close()

	token, err := NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	ok, err := l.Acquire(runCtx, "key1", token, 10*time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if !ok {
		t.Fatal("expected acquire to succeed")
	}

	// Shutdown cancels the run context; the release must still work because the
	// poller releases on a fresh context.
	cancel()
	releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer releaseCancel()
	if err := l.Del(releaseCtx, "key1", token); err != nil {
		t.Fatalf("release after run context cancelled: %v", err)
	}

	n, err := l.client.Exists(context.Background(), "key1").Result()
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if n != 0 {
		t.Fatalf("key should be gone after release, got %d", n)
	}
}

func TestLock_NewInvalidURL(t *testing.T) {
	t.Parallel()

	if _, err := New("not-a-valid-url"); err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestLock_NewUnreachable(t *testing.T) {
	t.Parallel()

	// Use a port nothing is listening on.
	if _, err := New("redis://127.0.0.1:16379"); err == nil {
		t.Fatal("expected error for unreachable redis")
	}
}
