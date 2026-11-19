package lock

import (
	"context"
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
