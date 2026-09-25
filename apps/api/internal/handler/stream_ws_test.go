package handler_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// newWSServer starts an httptest server exposing the subscription endpoint
// and returns it along with the Handler so tests can publish hub events.
func newWSServer(t *testing.T) (*httptest.Server, *handler.Handler) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h := &handler.Handler{
		Store:       seededWatchdogStore(t),
		DB:          &store.MockPinger{Healthy: true},
		Redis:       &store.MockPinger{Healthy: true},
		RedisClient: &mockRedisClient{},
		Logger:      logger,
	}
	srv := httptest.NewServer(router.New(h))
	t.Cleanup(srv.Close)
	return srv, h
}

// wsDial connects to the subscription endpoint and performs the handshake.
func wsDial(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/subscribe"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	t.Cleanup(func() { conn.Close(websocket.StatusNormalClosure, "test done") })
	return conn
}

// wsRead reads one server message with a bounded deadline.
func wsRead(t *testing.T, conn *websocket.Conn) wsServerMsg {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("websocket read: %v", err)
	}
	var msg wsServerMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("bad JSON from server: %v (%s)", err, data)
	}
	return msg
}

// wsServerMsg mirrors handler.wsServerMessage for tests.
type wsServerMsg struct {
	Op         string         `json:"op,omitempty"`
	ID         string         `json:"id,omitempty"`
	Type       string         `json:"type"`
	ContractID string         `json:"contract_id,omitempty"`
	Event      map[string]any `json:"event,omitempty"`
	Alert      map[string]any `json:"alert,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// wsSend writes one client message.
func wsSend(t *testing.T, conn *websocket.Conn, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("websocket write: %v", err)
	}
}

func TestWSSubscribeReceiveAndChangeFilter(t *testing.T) {
	srv, h := newWSServer(t)
	conn := wsDial(t, srv)

	// Connected announcement.
	if msg := wsRead(t, conn); msg.Type != "connected" {
		t.Fatalf("first message: want connected, got %+v", msg)
	}

	// Subscribe to CONTRACT_A only.
	wsSend(t, conn, map[string]any{
		"op":     "subscribe",
		"id":     "s1",
		"filter": map[string]any{"contract_id": "CONTRACT_A"},
	})
	if msg := wsRead(t, conn); msg.Op != "subscribed" || msg.ID != "s1" {
		t.Fatalf("want subscribed/s1, got %+v", msg)
	}

	// A matching event is delivered.
	h.Hub().PublishEvent(store.Event{ContractID: "CONTRACT_A", ID: "e1"})
	msg := wsRead(t, conn)
	if msg.Type != "event" || msg.ContractID != "CONTRACT_A" {
		t.Fatalf("want event for CONTRACT_A, got %+v", msg)
	}

	// The filter change takes effect without a reconnect (acceptance
	// criterion 1): publish an event for CONTRACT_B only after the new
	// filter is acknowledged — anything published under the old filter
	// may legitimately be dropped.
	wsSend(t, conn, map[string]any{
		"op":     "subscribe",
		"id":     "s1",
		"filter": map[string]any{"contract_id": "CONTRACT_B"},
	})
	if msg := wsRead(t, conn); msg.Op != "subscribed" {
		t.Fatalf("want subscribed echo, got %+v", msg)
	}
	h.Hub().PublishEvent(store.Event{ContractID: "CONTRACT_B", ID: "e2"})
	msg = wsRead(t, conn)
	if msg.Type != "event" || msg.ContractID != "CONTRACT_B" {
		t.Fatalf("want event for CONTRACT_B after filter change, got %+v", msg)
	}
}

func TestWSUnsubscribeStopsDelivery(t *testing.T) {
	srv, h := newWSServer(t)
	conn := wsDial(t, srv)
	wsRead(t, conn) // connected

	wsSend(t, conn, map[string]any{"op": "subscribe", "id": "s1"})
	if msg := wsRead(t, conn); msg.Op != "subscribed" {
		t.Fatalf("want subscribed, got %+v", msg)
	}

	wsSend(t, conn, map[string]any{"op": "unsubscribe", "id": "s1"})
	if msg := wsRead(t, conn); msg.Op != "unsubscribed" || msg.ID != "s1" {
		t.Fatalf("want unsubscribed/s1, got %+v", msg)
	}

	// Nothing is delivered anymore: the only way to observe "nothing" is a
	// short wait followed by an application ping that must be answered.
	h.Hub().PublishEvent(store.Event{ContractID: "CONTRACT_A", ID: "e9"})
	wsSend(t, conn, map[string]any{"op": "ping"})
	if msg := wsRead(t, conn); msg.Type != "heartbeat" {
		t.Fatalf("want heartbeat after ping, got %+v", msg)
	}
}

func TestWSUnknownOpAndMalformedJSON(t *testing.T) {
	srv, _ := newWSServer(t)
	conn := wsDial(t, srv)
	wsRead(t, conn) // connected

	wsSend(t, conn, map[string]any{"op": "bogus"})
	if msg := wsRead(t, conn); msg.Op != "error" || msg.Error == "" {
		t.Fatalf("want error for unknown op, got %+v", msg)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, []byte("{not json")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if msg := wsRead(t, conn); msg.Op != "error" {
		t.Fatalf("want error for malformed JSON, got %+v", msg)
	}
}

func TestWSSubscribeWithoutIDRejected(t *testing.T) {
	srv, _ := newWSServer(t)
	conn := wsDial(t, srv)
	wsRead(t, conn) // connected

	wsSend(t, conn, map[string]any{"op": "subscribe"})
	if msg := wsRead(t, conn); msg.Op != "error" {
		t.Fatalf("want error for missing id, got %+v", msg)
	}
}
