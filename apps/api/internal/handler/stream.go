package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// StreamMessage represents a server-sent event payload.
type StreamMessage struct {
	Type       string         `json:"type"` // "event" | "alert" | "heartbeat" | "connected"
	ContractID string         `json:"contract_id,omitempty"`
	Event      *eventResponse `json:"event,omitempty"`
	Alert      any            `json:"alert,omitempty"`
}

// streamReplayLimit bounds how many recent events the hub keeps so a client
// that reconnects with Last-Event-ID can catch up. It also sizes subscriber
// channels so replay can never block a publish.
const streamReplayLimit = 256

// StreamHub manages SSE client subscribers and broadcasts events.
type StreamHub struct {
	mu          sync.RWMutex
	subscribers map[chan StreamMessage]string // client channel -> contract_id filter ("" for all)
	recent      []StreamMessage               // bounded replay buffer of recent events
}

// NewStreamHub creates an initialized StreamHub.
func NewStreamHub() *StreamHub {
	return &StreamHub{
		subscribers: make(map[chan StreamMessage]string),
		recent:      make([]StreamMessage, 0, streamReplayLimit),
	}
}

// Subscribe registers a client channel for events on a specific contract (or all if contractID is "").
// It returns the receive channel and an unsubscribe cleanup function.
func (h *StreamHub) Subscribe(contractID string) (chan StreamMessage, func()) {
	return h.SubscribeAfter(contractID, "")
}

// SubscribeAfter is Subscribe plus SSE resume support: when lastEventID names
// an event still in the hub's bounded replay buffer, every event published
// after it (for the same contract filter) is queued on the returned channel
// before new live events. The subscription is registered before the replay is
// read, so no event can slip between catch-up and live delivery. If the id has
// already fallen out of the buffer the whole buffer is replayed; clients
// de-duplicate by id, and at-least-once delivery is the documented guarantee.
func (h *StreamHub) SubscribeAfter(contractID, lastEventID string) (chan StreamMessage, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan StreamMessage, streamReplayLimit)
	h.subscribers[ch] = contractID

	if lastEventID != "" {
		start := 0
		for i, msg := range h.recent {
			if msg.Event != nil && msg.Event.ID == lastEventID {
				start = i + 1
				break
			}
		}
		for _, msg := range h.recent[start:] {
			if contractID == "" || contractID == msg.ContractID {
				ch <- msg
			}
		}
	}

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subscribers[ch]; ok {
			delete(h.subscribers, ch)
			close(ch)
		}
	}

	return ch, unsubscribe
}

// Publish broadcasts a message to all matching subscribers.
func (h *StreamHub) Publish(msg StreamMessage) {
	h.mu.Lock()
	if msg.Type == "event" && msg.Event != nil && msg.Event.ID != "" {
		h.recent = append(h.recent, msg)
		if len(h.recent) > streamReplayLimit {
			h.recent = h.recent[len(h.recent)-streamReplayLimit:]
		}
	}
	channels := make([]chan StreamMessage, 0, len(h.subscribers))
	for ch, contractID := range h.subscribers {
		if contractID == "" || contractID == msg.ContractID {
			channels = append(channels, ch)
		}
	}
	h.mu.Unlock()

	for _, ch := range channels {
		select {
		case ch <- msg:
		default:
			// Subscriber is lagging or slow, drop message to prevent blocking
		}
	}
}

// PublishEvent broadcasts a contract event to subscribers.
func (h *StreamHub) PublishEvent(e store.Event) {
	resp := eventFromStore(e)
	h.Publish(StreamMessage{
		Type:       "event",
		ContractID: e.ContractID,
		Event:      &resp,
	})
}

// PublishAlert broadcasts an alert to subscribers.
func (h *StreamHub) PublishAlert(contractID string, alert any) {
	h.Publish(StreamMessage{
		Type:       "alert",
		ContractID: contractID,
		Alert:      alert,
	})
}

// Hub returns the handler's StreamHub, lazily initializing if needed.
func (h *Handler) Hub() *StreamHub {
	if h.StreamHub == nil {
		h.StreamHub = NewStreamHub()
	}
	return h.StreamHub
}

// StreamEventsSSE handles GET /api/v1/stream/events?contract_id=<optional>.
// It establishes a Server-Sent Events stream using http.Flusher and pushes
// real-time events and alerts as they are published.
func (h *Handler) StreamEventsSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "streaming unsupported")
		return
	}

	contractID := r.URL.Query().Get("contract_id")
	lastEventID := r.Header.Get("Last-Event-ID")
	if lastEventID == "" {
		// The header is the standard mechanism; the query parameter is a
		// convenience for clients that cannot set headers on an EventSource.
		lastEventID = r.URL.Query().Get("last_event_id")
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, unsubscribe := h.Hub().SubscribeAfter(contractID, lastEventID)
	defer unsubscribe()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// Initial ping / connected message
	initialMsg, _ := json.Marshal(map[string]string{
		"type":    "connected",
		"message": "event stream connected",
	})
	fmt.Fprintf(w, "data: %s\n\n", initialMsg)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return

		case <-heartbeatTicker.C:
			// Heartbeat keeps connection alive through proxies and firewalls
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()

		case msg, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			// The id field is what EventSource clients echo back in
			// Last-Event-ID after a drop, so it must be emitted for events.
			if msg.Event != nil && msg.Event.ID != "" {
				fmt.Fprintf(w, "id: %s\n", msg.Event.ID)
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
