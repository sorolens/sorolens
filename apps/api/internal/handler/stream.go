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

// StreamHub manages SSE client subscribers and broadcasts events.
type StreamHub struct {
	mu          sync.RWMutex
	subscribers map[chan StreamMessage]string // client channel -> contract_id filter ("" for all)
}

// NewStreamHub creates an initialized StreamHub.
func NewStreamHub() *StreamHub {
	return &StreamHub{
		subscribers: make(map[chan StreamMessage]string),
	}
}

// Subscribe registers a client channel for events on a specific contract (or all if contractID is "").
// It returns the receive channel and an unsubscribe cleanup function.
func (h *StreamHub) Subscribe(contractID string) (chan StreamMessage, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan StreamMessage, 64)
	h.subscribers[ch] = contractID

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
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch, contractID := range h.subscribers {
		if contractID == "" || contractID == msg.ContractID {
			select {
			case ch <- msg:
			default:
				// Subscriber is lagging or slow, drop message to prevent blocking
			}
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

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, unsubscribe := h.Hub().Subscribe(contractID)
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
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
