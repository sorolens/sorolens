package stream

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func sseEvent(id, payload string) string {
	if id == "" {
		return "data: " + payload + "\n\n"
	}
	return "id: " + id + "\ndata: " + payload + "\n\n"
}

func collect(t *testing.T, s *Stream, want int, timeout time.Duration) []Event {
	t.Helper()
	deadline := time.After(timeout)
	var got []Event
	for len(got) < want {
		select {
		case ev, ok := <-s.Events:
			if !ok {
				t.Fatalf("events channel closed after %d/%d events", len(got), want)
			}
			got = append(got, ev)
		case err, ok := <-s.Errors:
			if ok && err != nil {
				t.Fatalf("stream error after %d/%d events: %v", len(got), want, err)
			}
		case <-deadline:
			t.Fatalf("timed out after %d/%d events", len(got), want)
		}
	}
	return got
}

func TestResumesWithLastEventID(t *testing.T) {
	var attempt atomic.Int32
	lastHeader := make(chan string, 4)
	release := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/stream/events" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		lastHeader <- r.Header.Get("Last-Event-ID")
		switch attempt.Add(1) {
		case 1:
			fmt.Fprint(w, sseEvent("1", `{"id":"1"}`))
			fmt.Fprint(w, sseEvent("2", `{"id":"2"}`))
			w.(http.Flusher).Flush()
			// Returning ends the response: the client sees a clean EOF.
			return
		default:
			fmt.Fprint(w, sseEvent("3", `{"id":"3"}`))
			fmt.Fprint(w, sseEvent("4", `{"id":"4"}`))
			w.(http.Flusher).Flush()
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s, err := StreamEvents(ctx, Options{
		BaseURL:        srv.URL,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("StreamEvents: %v", err)
	}
	defer s.Close()

	events := collect(t, s, 4, 5*time.Second)
	for i, ev := range events {
		if want := fmt.Sprint(i + 1); ev.ID != want {
			t.Fatalf("event %d id = %q, want %q", i, ev.ID, want)
		}
		if ev.Type != "message" {
			t.Fatalf("event %d type = %q, want message", i, ev.Type)
		}
	}

	if got := <-lastHeader; got != "" {
		t.Fatalf("first connection sent Last-Event-ID %q, want empty", got)
	}
	if got := <-lastHeader; got != "2" {
		t.Fatalf("reconnect sent Last-Event-ID %q, want 2", got)
	}
	if got := s.LastEventID(); got != "4" {
		t.Fatalf("LastEventID() = %q, want 4", got)
	}
}

func TestDeduplicatesEventsReplayedOnReconnect(t *testing.T) {
	var attempt atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Both connections replay event 2, which the server is allowed to do
		// because delivery is at-least-once.
		fmt.Fprint(w, sseEvent("1", `{"n":1}`))
		fmt.Fprint(w, sseEvent("2", `{"n":2}`))
		if attempt.Add(1) == 1 {
			return
		}
		fmt.Fprint(w, sseEvent("3", `{"n":3}`))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := StreamEvents(ctx, Options{
		BaseURL:        srv.URL,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("StreamEvents: %v", err)
	}
	defer s.Close()

	got := collect(t, s, 3, 5*time.Second)
	for i, ev := range got {
		if want := fmt.Sprint(i + 1); ev.ID != want {
			t.Fatalf("delivered id %q at position %d, want %q (duplicate or gap)", ev.ID, i, want)
		}
	}
}

func TestPermanentErrorStopsRetries(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "invalid api key", http.StatusUnauthorized)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := StreamEvents(ctx, Options{
		BaseURL:        srv.URL,
		APIKey:         "wrong",
		InitialBackoff: time.Millisecond,
		MaxBackoff:     time.Millisecond,
	})
	if err != nil {
		t.Fatalf("StreamEvents: %v", err)
	}
	defer s.Close()

	select {
	case err := <-s.Errors:
		var permanent *PermanentError
		if !errors.As(err, &permanent) {
			t.Fatalf("error = %v, want *PermanentError", err)
		}
		if permanent.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", permanent.StatusCode)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for permanent error")
	}

	select {
	case <-s.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not stop after a permanent error")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("made %d requests, want exactly 1 (no retries on 401)", got)
	}
}

func TestHeartbeatTimeoutReconnects(t *testing.T) {
	var attempt atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if attempt.Add(1) == 1 {
			fmt.Fprint(w, sseEvent("1", `{"n":1}`))
			w.(http.Flusher).Flush()
			// Go silent: no heartbeat, no data. The client must notice.
			select {
			case <-release:
			case <-r.Context().Done():
			}
			return
		}
		fmt.Fprint(w, sseEvent("2", `{"n":2}`))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := StreamEvents(ctx, Options{
		BaseURL:          srv.URL,
		HeartbeatTimeout: 150 * time.Millisecond,
		InitialBackoff:   5 * time.Millisecond,
		MaxBackoff:       10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("StreamEvents: %v", err)
	}
	defer s.Close()

	got := collect(t, s, 2, 5*time.Second)
	if got[0].ID != "1" || got[1].ID != "2" {
		t.Fatalf("ids = %q,%q, want 1,2", got[0].ID, got[1].ID)
	}
	if got := attempt.Load(); got < 2 {
		t.Fatalf("server saw %d connections, want at least 2", got)
	}
}

func TestCancellationStopsGoroutines(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseEvent("1", `{"n":1}`))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	before := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	s, err := StreamEvents(ctx, Options{
		BaseURL:          srv.URL,
		HeartbeatTimeout: -1,
		InitialBackoff:   10 * time.Millisecond,
		MaxBackoff:       10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("StreamEvents: %v", err)
	}
	collect(t, s, 1, 5*time.Second)
	cancel()

	select {
	case <-s.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("stream goroutine did not stop within 2s of cancellation")
	}

	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before+1 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before+1 {
		t.Fatalf("goroutines: before=%d after=%d (leak)", before, after)
	}
}

func TestOptionsValidation(t *testing.T) {
	if _, err := StreamEvents(context.Background(), Options{}); err == nil {
		t.Fatal("expected an error for a missing BaseURL")
	}
}

func TestParseRetryAfter(t *testing.T) {
	if got := parseRetryAfter("12"); got != 12*time.Second {
		t.Fatalf("delta seconds = %v, want 12s", got)
	}
	future := time.Now().Add(3 * time.Second).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(future); got <= 0 || got > 4*time.Second {
		t.Fatalf("http-date = %v, want ~3s", got)
	}
	if got := parseRetryAfter("nonsense"); got != 0 {
		t.Fatalf("garbage = %v, want 0", got)
	}
}
