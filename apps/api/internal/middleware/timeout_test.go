package middleware

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestTimeout_SlowHandlerGets503 is the issue #154 acceptance test: a
// handler that outlives the budget yields a 503 in the JSON error envelope,
// promptly, rather than whenever the handler gets around to finishing.
func TestTimeout_SlowHandlerGets503(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release // ignores its context, like a stuck query without ctx
		w.WriteHeader(http.StatusOK)
	})

	h := RequestID(Timeout(20 * time.Millisecond)(slow))
	rr := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("503 took %v; the timeout did not fire promptly", elapsed)
	}
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rr.Body.String(), err)
	}
	if body.Error.Code != "TIMEOUT" {
		t.Errorf("error.code = %q, want TIMEOUT", body.Error.Code)
	}
	if body.Error.RequestID == "" || body.Error.RequestID != rr.Header().Get("X-Request-ID") {
		t.Errorf("error.request_id = %q, want the X-Request-ID %q", body.Error.RequestID, rr.Header().Get("X-Request-ID"))
	}
}

// TestTimeout_CancelsHandlerContext proves the deadline propagates to the
// handler so store calls using r.Context() abort instead of holding a
// connection (the "why" of issue #154).
func TestTimeout_CancelsHandlerContext(t *testing.T) {
	got := make(chan error, 1)
	h := Timeout(20 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		got <- r.Context().Err()
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

	select {
	case err := <-got:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("handler ctx err = %v, want DeadlineExceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handler context was never cancelled")
	}
}

// TestTimeout_FastHandlerPassesThrough checks status, headers and body of a
// handler that finishes in time are delivered unchanged.
func TestTimeout_FastHandlerPassesThrough(t *testing.T) {
	h := Timeout(time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("X-Custom", "yes")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "a,b\n")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "text/csv" || rr.Header().Get("X-Custom") != "yes" {
		t.Errorf("headers not passed through: %v", rr.Header())
	}
	if rr.Body.String() != "a,b\n" {
		t.Errorf("body = %q", rr.Body.String())
	}
}

// TestTimeout_HandlerOwn503KeepsItsContentType guards timeoutJSONWriter: it
// must only label the timeout response, not a handler's own 503.
func TestTimeout_HandlerOwn503KeepsItsContentType(t *testing.T) {
	h := Timeout(time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if ct := rr.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("Content-Type = %q, want the handler's text/plain", ct)
	}
}

// TestTimeout_LateWritesAreDiscarded is the regression test for the old
// hand-rolled writer, which let the handler goroutine keep writing to the
// connection after the 503 was sent. Run with -race. The late write itself
// may return nil (it can land in the buffer just before TimeoutHandler takes
// its lock); what matters is that it never reaches the client.
func TestTimeout_LateWritesAreDiscarded(t *testing.T) {
	finished := make(chan struct{})
	h := Timeout(10 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		<-r.Context().Done()
		w.Header().Set("X-Late", "1")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "late body")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	<-finished

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "late body") || rr.Header().Get("X-Late") != "" {
		t.Errorf("late handler output leaked into the response: %q %v", rr.Body.String(), rr.Header())
	}
}

// TestTimeout_PanicReachesRecoverer keeps Recoverer working behind Timeout:
// the panic happens on the handler goroutine and must be re-raised.
func TestTimeout_PanicReachesRecoverer(t *testing.T) {
	h := Recoverer(discardLogger())(Timeout(time.Second)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 from Recoverer", rr.Code)
	}
}

// TestStreamTimeout_KeepsFlusherAndCancelsAtDeadline checks the stream
// wrapper hands the handler a flushable writer and a context with the
// configured deadline.
func TestStreamTimeout_KeepsFlusherAndCancelsAtDeadline(t *testing.T) {
	var (
		flushable bool
		deadline  time.Time
		ctxErr    error
	)
	h := StreamTimeout(30 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, flushable = w.(http.Flusher)
		deadline, _ = r.Context().Deadline()
		<-r.Context().Done()
		ctxErr = r.Context().Err()
	}))
	start := time.Now()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

	if !flushable {
		t.Error("stream handler did not receive an http.Flusher")
	}
	if d := deadline.Sub(start); d <= 0 || d > time.Second {
		t.Errorf("context deadline %v after start, want about 30ms", d)
	}
	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Errorf("ctx err = %v, want DeadlineExceeded", ctxErr)
	}
}

// TestStreamTimeout_OutlivesServerWriteTimeout runs a real server whose
// WriteTimeout is shorter than the stream budget, through Logger (the
// production chain). Without the write-deadline extension, or without
// statusRecorder forwarding Flush/Unwrap, the second frame never arrives.
func TestStreamTimeout_OutlivesServerWriteTimeout(t *testing.T) {
	stream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: first\n\n")
		f.Flush()
		select {
		case <-time.After(150 * time.Millisecond): // past the 50ms WriteTimeout
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, "data: second\n\n")
		f.Flush()
		<-r.Context().Done()
	})

	srv := httptest.NewUnstartedServer(Logger(discardLogger())(StreamTimeout(400 * time.Millisecond)(stream)))
	srv.Config.WriteTimeout = 50 * time.Millisecond
	srv.Start()
	defer srv.Close()

	start := time.Now()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var frames []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, "data: ") {
			frames = append(frames, strings.TrimPrefix(line, "data: "))
		}
	}
	elapsed := time.Since(start)

	if len(frames) != 2 || frames[1] != "second" {
		t.Fatalf("frames = %v, want [first second]; the stream was cut early", frames)
	}
	if elapsed > 2*time.Second {
		t.Errorf("stream stayed open %v; StreamTimeout did not end it", elapsed)
	}
}
