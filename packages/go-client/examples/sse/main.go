// Consume the Server-Sent Events endpoint GET /api/v1/stream/events through the
// supported stream package, which reconnects with backoff, resumes with
// Last-Event-ID and de-duplicates replayed events.
//
// Run against a live server:
//
//	SOROLENS_API_URL=https://api.sorolens.xyz go run ./examples/sse
//
// Without SOROLENS_API_URL a canned SSE transcript is replayed offline, which
// is what CI runs.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/sorolens/go-client/stream"
)

const offlineTranscript = "id: 0\nevent: contract.event\n" +
	`data: {"type":"contract.event","id":"0","contract_id":"CDEMO"}` + "\n\n" +
	"id: 1\nevent: contract.event\n" +
	`data: {"type":"contract.event","id":"1","contract_id":"CDEMO"}` + "\n\n" +
	": keep-alive\n" +
	"id: 2\nevent: contract.event\n" +
	`data: {"type":"contract.event","id":"2","contract_id":"CDEMO"}` + "\n\n"

type payload struct {
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

	s, err := stream.StreamEvents(ctx, stream.Options{
		BaseURL:    baseURL,
		APIKey:     os.Getenv("SOROLENS_API_KEY"),
		ContractID: os.Getenv("SOROLENS_CONTRACT_ID"),
	})
	if err != nil {
		log.Fatalf("open stream: %v", err)
	}
	defer s.Close()

	count := 0
	for count < 3 {
		select {
		case ev, ok := <-s.Events:
			if !ok {
				log.Fatalf("stream closed after %d events (last id %q)", count, s.LastEventID())
			}
			var p payload
			if err := json.Unmarshal(ev.Data, &p); err != nil {
				log.Fatalf("decode event %q: %v", ev.Data, err)
			}
			count++
			fmt.Printf("  %s %s (id=%s)\n", p.Type, p.ID, ev.ID)
		case err := <-s.Errors:
			if err != nil {
				log.Fatalf("stream: %v", err)
			}
		case <-ctx.Done():
			log.Fatalf("timed out after %d events", count)
		}
	}

	fmt.Printf("received %d events (last id %q)\n", count, s.LastEventID())
	if live == "" {
		if count != 3 {
			log.Fatalf("expected 3 events, got %d", count)
		}
	}
}
