package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Heartbeat cadence and liveness threshold (issue sorolens#126): the server
// pings every wsHeartbeatInterval and drops clients that fail to answer a
// ping within wsLivenessTimeout. Vars so tests can shorten the schedule.
var (
	wsHeartbeatInterval = 30 * time.Second
	wsLivenessTimeout   = 60 * time.Second
)

// wsSubscribeMessage is a client -> server control frame.
//
//	{"op":"subscribe","id":"s1","filter":{"contract_id":"C1"}}
//	{"op":"unsubscribe","id":"s1"}
type wsSubscribeMessage struct {
	Op     string         `json:"op"`
	ID     string         `json:"id,omitempty"`
	Filter *wsEventFilter `json:"filter,omitempty"`
}

// wsEventFilter selects which published messages a subscription receives.
// A nil/empty ContractID matches every contract (like the SSE hub's ""
// filter).
type wsEventFilter struct {
	ContractID *string `json:"contract_id,omitempty"`
}

// matches reports whether msg satisfies the filter.
func (f wsEventFilter) matches(msg StreamMessage) bool {
	if f.ContractID == nil || *f.ContractID == "" {
		return true
	}
	return msg.ContractID == *f.ContractID
}

// wsServerMessage wraps everything the server pushes: op is set on control
// replies ("subscribed" | "unsubscribed" | "error"), type on data and
// heartbeats ("event" | "alert" | "heartbeat" | "connected").
type wsServerMessage struct {
	Op         string         `json:"op,omitempty"`
	ID         string         `json:"id,omitempty"`
	Type       string         `json:"type"`
	ContractID string         `json:"contract_id,omitempty"`
	Event      *eventResponse `json:"event,omitempty"`
	Alert      any            `json:"alert,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// wsSubscription is one live subscription of one connection.
type wsSubscription struct {
	id     string
	filter wsEventFilter
}

// wsSession carries the per-connection state: the subscription list
// (client-controlled) and the hub receive channel.
type wsSession struct {
	mu    sync.Mutex
	subs  []wsSubscription
	hubCh chan StreamMessage
}

// errStale marks a connection dropped for missing heartbeats.
var errStale = errStaleType{}

type errStaleType struct{}

func (errStaleType) Error() string { return "websocket client missed heartbeats" }

// SubscribeEventsWS handles GET /api/v1/subscribe — the WebSocket
// counterpart of the SSE stream (issue sorolens#126).
//
// Protocol (JSON text frames):
//
//	Client -> Server:
//	  {"op":"subscribe","id":"<client-chosen>","filter":{"contract_id":"C1"}}
//	  {"op":"unsubscribe","id":"<client-chosen>"}
//	  {"op":"ping"}                     (optional application-level ping)
//
//	Server -> Client:
//	  {"type":"connected"}
//	  {"op":"subscribed","id":"<id>"}
//	  {"op":"unsubscribed","id":"<id>"}
//	  {"op":"error","error":"<reason>"} (unknown op / malformed JSON)
//	  {"type":"event","contract_id":"C1","event":{...}}
//	  {"type":"alert","contract_id":"C1","alert":{...}}
//	  {"type":"heartbeat"}
//
// A subscribe with an existing id replaces that subscription's filter, so
// clients can change filters live without reconnecting. The server pings
// every wsHeartbeatInterval and drops the connection when a ping goes
// unanswered for wsLivenessTimeout.
func (h *Handler) SubscribeEventsWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusInternalError, "server closing")

	hub := h.Hub()
	hubCh, unsubscribeHub := hub.Subscribe("")
	defer unsubscribeHub()

	sess := &wsSession{hubCh: hubCh}

	ctx := r.Context()
	conn.SetReadLimit(1 << 20)

	// Writer goroutine: serialises hub fan-out and control replies onto the
	// single connection. Exits when ctx is cancelled or a write fails.
	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.writeLoop(ctx, conn)
	}()

	// First message: announce the connection.
	if err := sess.writeJSON(ctx, conn, wsServerMessage{Type: "connected"}); err != nil {
		<-done
		return
	}

	// Reader loop (runs on the handler goroutine) until the peer closes or
	// a control frame handler triggers a close.
	sess.readLoop(ctx, conn)
	conn.Close(websocket.StatusNormalClosure, "bye")
	<-done
}

// readLoop processes client control frames until the connection closes.
func (s *wsSession) readLoop(ctx context.Context, conn *websocket.Conn) error {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var msg wsSubscribeMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			_ = s.writeJSON(ctx, conn, wsServerMessage{Op: "error", Error: "malformed JSON"})
			continue
		}
		switch msg.Op {
		case "subscribe":
			if msg.ID == "" {
				_ = s.writeJSON(ctx, conn, wsServerMessage{Op: "error", Error: "subscribe requires an id"})
				continue
			}
			filter := wsEventFilter{}
			if msg.Filter != nil {
				filter = *msg.Filter
			}
			s.mu.Lock()
			replaced := false
			for i := range s.subs {
				if s.subs[i].id == msg.ID {
					s.subs[i].filter = filter
					replaced = true
					break
				}
			}
			if !replaced {
				s.subs = append(s.subs, wsSubscription{id: msg.ID, filter: filter})
			}
			s.mu.Unlock()
			_ = s.writeJSON(ctx, conn, wsServerMessage{Op: "subscribed", ID: msg.ID})
		case "unsubscribe":
			s.mu.Lock()
			kept := s.subs[:0]
			for _, sub := range s.subs {
				if sub.id != msg.ID {
					kept = append(kept, sub)
				}
			}
			s.subs = kept
			s.mu.Unlock()
			_ = s.writeJSON(ctx, conn, wsServerMessage{Op: "unsubscribed", ID: msg.ID})
		case "ping":
			_ = s.writeJSON(ctx, conn, wsServerMessage{Type: "heartbeat"})
		default:
			_ = s.writeJSON(ctx, conn, wsServerMessage{Op: "error", Error: "unknown op"})
		}
	}
}

// writeLoop serialises all server -> client frames: heartbeats (whose
// blocking Ping enforces liveness) and hub messages fanned out to every
// matching subscription of this connection.
func (s *wsSession) writeLoop(ctx context.Context, conn *websocket.Conn) error {
	heartbeat := time.NewTicker(wsHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-heartbeat.C:
			// Liveness: Ping sends a ping and waits for the pong. A peer
			// that misses more than wsLivenessTimeout of heartbeats fails
			// here and is dropped (issue sorolens#126, criterion 2).
			pingCtx, cancel := context.WithTimeout(ctx, wsLivenessTimeout)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return errStale
			}
			_ = s.writeJSON(ctx, conn, wsServerMessage{Type: "heartbeat"})
		case msg, ok := <-s.hubCh:
			if !ok {
				return nil
			}
			s.mu.Lock()
			matched := false
			for _, sub := range s.subs {
				if sub.filter.matches(msg) {
					matched = true
					break
				}
			}
			s.mu.Unlock()
			if !matched {
				continue
			}
			out := wsServerMessage{
				Type:       msg.Type,
				ContractID: msg.ContractID,
				Event:      msg.Event,
				Alert:      msg.Alert,
			}
			if err := s.writeJSON(ctx, conn, out); err != nil {
				return err
			}
		}
	}
}

// writeJSON sends one JSON text frame with a bounded write deadline.
func (s *wsSession) writeJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageText, data)
}
