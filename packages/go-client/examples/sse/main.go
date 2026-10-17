// Consume the Server-Sent Events endpoint GET /api/v1/stream/events.
//
// Every "data:" line is a JSON event, handled as it arrives off the wire.
//
// Run against a live server:
//
//	SOROLENS_API_URL=https://api.sorolens.xyz go run ./examples/sse
//
// Without SOROLENS_API_URL a canned SSE transcript is replayed offline, which
// is what CI runs.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"
)

const offlineTranscript = "event: contract.event\n" +
	`data: {"type":"contract.event","id":"0","contract_id":"CDEMO"}` + "\n\n" +
	"event: contract.event\n" +
	`data: {"type":"contract.event","id":"1","contract_id":"CDEMO"}` + "\n\n" +
	": keep-alive\n" +
	"event: contract.event\n" +
	`data: {"type":"contract.event","id":"2","contract_id":"CDEMO"}` + "\n\n"

type event struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	ContractID string `json:"contract_id"`
}

func offlineServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/stream/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := w.Write([]byte(offlineTranscript)); err != nil {
			log.Printf("write stream: %v", err)
		}
	})
	return httptest.NewServer(mux)
}

func main() {
	live := os.Getenv("SOROLENS_API_URL")

	baseURL := live
	if baseURL == "" {
		srv := offlineServer()
		defer srv.Close()
		baseURL = srv.URL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/stream/events", nil)
	if err != nil {
		log.Fatalf("build request: %v", err)
	}
	if apiKey := os.Getenv("SOROLENS_API_KEY"); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("open stream: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("stream returned status %d", resp.StatusCode)
	}

	var events []event
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var e event
		if err := json.Unmarshal([]byte(payload), &e); err != nil {
			log.Fatalf("decode event %q: %v", payload, err)
		}
		events = append(events, e)
		fmt.Printf("  %s %s\n", e.Type, e.ID)
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("read stream: %v", err)
	}

	fmt.Printf("received %d events\n", len(events))
	if live == "" {
		if len(events) != 3 {
			log.Fatalf("expected 3 events, got %d", len(events))
		}
		for _, e := range events {
			if e.ContractID != "CDEMO" {
				log.Fatalf("unexpected contract_id %q", e.ContractID)
			}
		}
	}
}
