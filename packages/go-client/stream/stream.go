// Package stream is the supported client for the Sorolens Server-Sent Events
// endpoint (GET /api/v1/stream/events).
//
// It turns the raw SSE wire format into a typed event channel and owns the
// hard parts of a long-lived stream:
//
//   - reconnects with exponential backoff and jitter,
//   - resumes from the last delivered event with the standard Last-Event-ID
//     request header, so a dropped connection does not replay everything,
//   - detects stale connections through a heartbeat read deadline,
//   - distinguishes permanent failures (401, 403, 404 and other 4xx) from
//     transient ones and stops instead of retrying forever,
//   - closes promptly when the context is cancelled.
//
// # Delivery guarantees
//
// Delivery is at-least-once: the server keeps a bounded replay buffer and a
// reconnect may resend the last event of the previous connection. The client
// de-duplicates against the ids it has already delivered, so in practice
// callers observe each event at most once per Stream as long as the event id is
// present in the stream (which the Sorolens server always sets). Ordering is
// the server's emission order and is preserved across reconnects for the
// events still in the server replay buffer; events published while the client
// was disconnected are replayed before new ones.
package stream

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// DefaultHeartbeatTimeout is how long the client waits for any bytes
	// (including SSE comment heartbeats) before treating the connection as
	// stale and reconnecting.
	DefaultHeartbeatTimeout = 45 * time.Second
	DefaultInitialBackoff   = 500 * time.Millisecond
	DefaultMaxBackoff       = 30 * time.Second

	// eventBuffer is the number of events buffered between the reader and the
	// consumer. A full buffer applies backpressure to the connection instead
	// of dropping events.
	eventBuffer = 32

	// seenHistory is how many recently delivered event ids are remembered for
	// de-duplication across reconnects.
	seenHistory = 256
)

// Options configures StreamEvents.
type Options struct {
	// BaseURL is the Sorolens API root, e.g. "https://api.sorolens.xyz".
	BaseURL string
	// APIKey is sent as a bearer token when non-empty.
	APIKey string
	// ContractID limits the stream to one contract when non-empty.
	ContractID string
	// LastEventID resumes from an event already seen by a previous process.
	LastEventID string
	// HTTPClient overrides the transport. The stream never mutates it.
	HTTPClient *http.Client

	// InitialBackoff and MaxBackoff bound the reconnect delay. Zero values
	// use DefaultInitialBackoff and DefaultMaxBackoff.
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	// HeartbeatTimeout is the read deadline. Zero uses
	// DefaultHeartbeatTimeout; a negative value disables stale detection.
	HeartbeatTimeout time.Duration
	// MaxAttempts caps consecutive failed reconnects before the stream gives
	// up. Zero means unlimited; the context still bounds the lifetime.
	MaxAttempts int
	// OnReconnect is called before a reconnect attempt with the attempt number
	// (starting at 1) and the error or EOF that triggered it.
	OnReconnect func(attempt int, cause error)
}

// Event is one decoded SSE event.
type Event struct {
	ID   string
	Type string
	Data json.RawMessage
}

// PermanentError reports a failure that retrying cannot fix.
type PermanentError struct {
	StatusCode int
	Body       string
}

func (e *PermanentError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("go-client/stream: permanent HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("go-client/stream: permanent HTTP %d: %s", e.StatusCode, e.Body)
}

// Stream is a running subscription. Read from Events until it is closed; a
// terminal failure is delivered on Errors. Close (or cancelling the context
// passed to StreamEvents) stops the stream and closes both channels.
type Stream struct {
	Events <-chan Event
	Errors <-chan error

	events chan Event
	errors chan error
	done   chan struct{}

	cancel context.CancelFunc
	ctx    context.Context
	opts   Options
	client *http.Client

	mu     sync.Mutex
	lastID string
	seen   map[string]struct{}
	order  []string
}

// StreamEvents connects to the Sorolens event stream and returns immediately.
// The context bounds the whole stream: when it is cancelled the goroutines
// stop and both channels are closed.
func StreamEvents(ctx context.Context, opts Options) (*Stream, error) {
	if strings.TrimSpace(opts.BaseURL) == "" {
		return nil, errors.New("go-client/stream: BaseURL is required")
	}
	if _, err := url.ParseRequestURI(opts.BaseURL); err != nil {
		return nil, fmt.Errorf("go-client/stream: invalid BaseURL: %w", err)
	}
	if opts.InitialBackoff <= 0 {
		opts.InitialBackoff = DefaultInitialBackoff
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = DefaultMaxBackoff
	}
	if opts.MaxBackoff < opts.InitialBackoff {
		opts.MaxBackoff = opts.InitialBackoff
	}
	if opts.HeartbeatTimeout == 0 {
		opts.HeartbeatTimeout = DefaultHeartbeatTimeout
	}

	runCtx, cancel := context.WithCancel(ctx)
	s := &Stream{
		events: make(chan Event, eventBuffer),
		errors: make(chan error, 1),
		done:   make(chan struct{}),
		cancel: cancel,
		ctx:    runCtx,
		opts:   opts,
		client: opts.HTTPClient,
		lastID: opts.LastEventID,
		seen:   make(map[string]struct{}, seenHistory),
		order:  make([]string, 0, seenHistory),
	}
	if s.client == nil {
		s.client = &http.Client{}
	}
	s.Events = s.events
	s.Errors = s.errors

	go s.run(runCtx)
	return s, nil
}

// LastEventID is the id of the most recently delivered event, suitable for
// passing as Options.LastEventID when resuming from a new process.
func (s *Stream) LastEventID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastID
}

// Close stops the stream and waits for its goroutine to exit.
func (s *Stream) Close() {
	s.cancel()
	<-s.done
}

// Done is closed once the stream has stopped.
func (s *Stream) Done() <-chan struct{} { return s.done }

func (s *Stream) run(ctx context.Context) {
	defer close(s.done)
	defer close(s.events)
	defer close(s.errors)

	attempt := 0
	for {
		if ctx.Err() != nil {
			return
		}
		err := s.connect(ctx)
		if ctx.Err() != nil {
			return
		}

		var permanent *PermanentError
		if errors.As(err, &permanent) {
			s.deliverError(err)
			return
		}

		// EOF after a healthy connection is a normal drop: reconnect without
		// counting it as a failed attempt.
		if errors.Is(err, io.EOF) || errors.Is(err, errStreamEnded) {
			attempt = 0
		} else {
			attempt++
		}
		if s.opts.MaxAttempts > 0 && attempt > s.opts.MaxAttempts {
			s.deliverError(fmt.Errorf("go-client/stream: giving up after %d attempts: %w", attempt, err))
			return
		}
		if err != nil && s.opts.OnReconnect != nil {
			s.opts.OnReconnect(attempt, err)
		}

		delay := s.backoff(attempt)
		if errors.Is(err, errStale) || errors.Is(err, io.EOF) {
			// A stale or cleanly closed connection still backs off, but from
			// the base delay: the server is reachable.
			delay = s.backoff(0)
		}
		var rateLimited *retryAfterError
		if errors.As(err, &rateLimited) && rateLimited.after > 0 {
			// Honour Retry-After, but never sleep longer than MaxBackoff.
			delay = rateLimited.after
			if delay > s.opts.MaxBackoff {
				delay = s.opts.MaxBackoff
			}
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Stream) deliverError(err error) {
	select {
	case s.errors <- err:
	default:
	}
}

func (s *Stream) backoff(attempt int) time.Duration {
	base := float64(s.opts.InitialBackoff)
	max := float64(s.opts.MaxBackoff)
	delay := base
	for i := 0; i < attempt && delay < max; i++ {
		delay *= 2
	}
	if delay > max {
		delay = max
	}
	// Full jitter: random point in [delay/2, delay], which spreads reconnect
	// storms without ever exceeding the configured cap.
	jittered := delay/2 + rand.Float64()*(delay/2)
	return time.Duration(jittered)
}

var (
	errStreamEnded = errors.New("go-client/stream: stream ended")
	errStale       = errors.New("go-client/stream: heartbeat deadline exceeded")
)

func (s *Stream) connect(ctx context.Context) error {
	endpoint, err := s.endpoint()
	if err != nil {
		return &PermanentError{StatusCode: 0, Body: err.Error()}
	}

	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return &PermanentError{Body: err.Error()}
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if s.opts.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.opts.APIKey)
	}
	if id := s.LastEventID(); id != "" {
		req.Header.Set("Last-Event-ID", id)
	}

	var stale atomic.Bool
	var timer *time.Timer
	if s.opts.HeartbeatTimeout > 0 {
		timer = time.AfterFunc(s.opts.HeartbeatTimeout, func() {
			stale.Store(true)
			cancel()
		})
		defer timer.Stop()
	}
	touch := func() {
		if timer != nil {
			timer.Reset(s.opts.HeartbeatTimeout)
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if stale.Load() {
			return errStale
		}
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &retryAfterError{after: parseRetryAfter(resp.Header.Get("Retry-After")), body: string(body)}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		permanent := resp.StatusCode >= 400 && resp.StatusCode < 500
		if permanent {
			return &PermanentError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
		}
		return fmt.Errorf("go-client/stream: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	touch()

	reader := bufio.NewReaderSize(resp.Body, 32*1024)
	var (
		id, name string
		data     strings.Builder
	)
	for {
		line, readErr := reader.ReadString('\n')
		if line != "" {
			touch()
			line = strings.TrimRight(line, "\r\n")
			switch {
			case line == "":
				if err := s.dispatch(&name, &id, &data); err != nil {
					return err
				}
			case strings.HasPrefix(line, ":"):
				// Comment / heartbeat.
			default:
				field, value, _ := strings.Cut(line, ":")
				value = strings.TrimPrefix(value, " ")
				switch field {
				case "id":
					id = value
				case "event":
					name = value
				case "data":
					if data.Len() > 0 {
						data.WriteByte('\n')
					}
					data.WriteString(value)
				}
			}
		}
		if readErr != nil {
			// Flush a trailing event that was not terminated by a blank line.
			if data.Len() > 0 {
				if err := s.dispatch(&name, &id, &data); err != nil {
					return err
				}
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if stale.Load() {
				return errStale
			}
			if errors.Is(readErr, io.EOF) {
				return errStreamEnded
			}
			return readErr
		}
	}
}

func (s *Stream) dispatch(name, id *string, data *strings.Builder) error {
	if data.Len() == 0 && *name == "" {
		*id = ""
		return nil
	}
	payload := data.String()
	data.Reset()
	ev := Event{ID: *id, Type: *name, Data: json.RawMessage(payload)}
	*id = ""
	*name = ""
	if ev.Type == "" {
		ev.Type = "message"
	}
	if payload == "" {
		return nil
	}
	if ev.ID != "" && s.alreadySeen(ev.ID) {
		return nil
	}
	select {
	case <-s.ctx.Done():
		return context.Canceled
	case s.events <- ev:
	}
	if ev.ID != "" {
		s.remember(ev.ID)
	}
	return nil
}

func (s *Stream) alreadySeen(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.seen[id]
	return ok
}

func (s *Stream) remember(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastID = id
	if _, ok := s.seen[id]; ok {
		return
	}
	s.seen[id] = struct{}{}
	s.order = append(s.order, id)
	if len(s.order) > seenHistory {
		oldest := s.order[0]
		s.order = s.order[1:]
		delete(s.seen, oldest)
	}
}

func (s *Stream) endpoint() (string, error) {
	base := strings.TrimRight(s.opts.BaseURL, "/")
	u, err := url.Parse(base + "/api/v1/stream/events")
	if err != nil {
		return "", err
	}
	if s.opts.ContractID != "" {
		q := u.Query()
		q.Set("contract_id", s.opts.ContractID)
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

type retryAfterError struct {
	after time.Duration
	body  string
}

func (e *retryAfterError) Error() string {
	return fmt.Sprintf("go-client/stream: HTTP 429: %s", strings.TrimSpace(e.body))
}

// parseRetryAfter understands both delta-seconds and HTTP-date values.
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return 0
}
