package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestStreamEventsSSE_ReceivesPublishedEvent(t *testing.T) {
	st := store.NewMockStore()
	h := &Handler{
		Store:  st,
		Logger: testLogger(),
	}

	ts := httptest.NewServer(http.HandlerFunc(h.StreamEventsSSE))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"?contract_id=CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect to SSE stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream Content-Type, got %q", ct)
	}

	reader := bufio.NewReader(resp.Body)

	// First line should be initial connection event
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read first line: %v", err)
	}
	if !strings.HasPrefix(line, "data: ") {
		t.Fatalf("expected data: prefix, got %q", line)
	}

	// Publish an event for this contract
	targetContract := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	ev := store.Event{
		ID:               "evt-12345",
		ContractID:       targetContract,
		Network:          "testnet",
		Ledger:           100,
		LedgerClosedAt:   time.Now().UTC(),
		TxHash:           "tx-12345",
		Type:             "contract",
		TopicXDR:         []string{"topic1"},
		ValueXDR:         "val1",
		InSuccessfulCall: true,
	}

	// Give subscriber a moment to register then publish
	time.Sleep(50 * time.Millisecond)
	h.Hub().PublishEvent(ev)

	// Read lines until we receive the published event data
	found := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("reading stream: %v", err)
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			var msg StreamMessage
			if err := json.Unmarshal([]byte(payload), &msg); err == nil {
				if msg.Type == "event" && msg.Event != nil && msg.Event.ID == "evt-12345" {
					found = true
					if msg.ContractID != targetContract {
						t.Errorf("contract ID = %q, want %q", msg.ContractID, targetContract)
					}
					break
				}
			}
		}
	}

	if !found {
		t.Fatal("did not receive published event in stream")
	}
}

func TestStreamEventsSSE_FiltersByContractID(t *testing.T) {
	st := store.NewMockStore()
	h := &Handler{
		Store:  st,
		Logger: testLogger(),
	}

	ts := httptest.NewServer(http.HandlerFunc(h.StreamEventsSSE))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Subscribe only to contract A
	contractA := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	contractB := "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"?contract_id="+contractA, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	// Read initial connected line
	_, _ = reader.ReadString('\n')

	time.Sleep(50 * time.Millisecond)

	// Publish event for contract B (should be ignored by subscriber for A)
	h.Hub().PublishEvent(store.Event{
		ID:         "evt-B",
		ContractID: contractB,
	})

	// Publish event for contract A
	h.Hub().PublishEvent(store.Event{
		ID:         "evt-A",
		ContractID: contractA,
	})

	receivedA := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			var msg StreamMessage
			if err := json.Unmarshal([]byte(payload), &msg); err == nil {
				if msg.Type == "event" && msg.Event != nil {
					if msg.Event.ID == "evt-B" {
						t.Fatalf("received event for contract B when filtered to contract A")
					}
					if msg.Event.ID == "evt-A" {
						receivedA = true
						break
					}
				}
			}
		}
	}

	if !receivedA {
		t.Fatal("did not receive event for contract A")
	}
}

func TestStreamEventsSSE_PublishAlert(t *testing.T) {
	st := store.NewMockStore()
	h := &Handler{
		Store:  st,
		Logger: testLogger(),
	}

	ts := httptest.NewServer(http.HandlerFunc(h.StreamEventsSSE))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"?contract_id="+contractID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	_, _ = reader.ReadString('\n')

	time.Sleep(50 * time.Millisecond)

	h.Hub().PublishAlert(contractID, map[string]string{
		"severity": "Warning",
		"message":  "high error rate",
	})

	receivedAlert := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			var msg StreamMessage
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &msg); err == nil {
				if msg.Type == "alert" && msg.ContractID == contractID {
					receivedAlert = true
					break
				}
			}
		}
	}

	if !receivedAlert {
		t.Fatal("did not receive alert in stream")
	}
}
