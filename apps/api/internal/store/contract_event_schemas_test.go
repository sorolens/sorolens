package store

import (
	"context"
	"testing"
	"time"
)

func TestInferEventSchemas_DriftAndVariants(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAH2"

	// Event stream v1: transfer with [transfer, from, to] and amount in value
	eventsV1 := []Event{
		{
			ContractID:   contractID,
			Ledger:       100,
			TopicDecoded: []any{"transfer", "GAAA", "GBBB"},
			ValueDecoded: 50.0,
		},
		{
			ContractID:   contractID,
			Ledger:       105,
			TopicDecoded: []any{"transfer", "GCCC", "GDDD"},
			ValueDecoded: 100.0,
		},
	}

	schemas := InferEventSchemas(contractID, eventsV1)
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
	s := schemas[0]
	if s.EventName != "transfer" {
		t.Errorf("expected event name transfer, got %q", s.EventName)
	}
	if s.TopicArity != 3 {
		t.Errorf("expected arity 3, got %d", s.TopicArity)
	}
	if len(s.ObservedShapes) != 1 || s.ObservedShapes[0].Count != 2 {
		t.Errorf("expected 1 shape with count 2, got %v", s.ObservedShapes)
	}

	// Upgrade v2 introduces schema drift: additional memo topic or map value
	eventsV2 := append(eventsV1, Event{
		ContractID:   contractID,
		Ledger:       200,
		TopicDecoded: []any{"transfer", "GEEE", "GFFF", "memo_text"},
		ValueDecoded: map[string]any{"amount": 250.0, "fee": 1.0},
	})

	updatedSchemas := InferEventSchemas(contractID, eventsV2)
	if len(updatedSchemas) != 1 {
		t.Fatalf("expected 1 schema after drift, got %d", len(updatedSchemas))
	}
	us := updatedSchemas[0]
	if us.FirstSeenLedger != 100 || us.LastSeenLedger != 200 {
		t.Errorf("expected ledgers 100..200, got %d..%d", us.FirstSeenLedger, us.LastSeenLedger)
	}
	if len(us.ObservedShapes) != 2 {
		t.Errorf("expected 2 observed shapes after drift, got %d", len(us.ObservedShapes))
	}
}

func TestMockStore_ContractEventSchemas(t *testing.T) {
	m := NewMockStore()
	schema := ContractEventSchema{
		ContractID:      "CAAA",
		EventName:       "mint",
		TopicArity:      2,
		TopicTypes:      []string{"string", "string"},
		ValueShape:      "number",
		FirstSeenLedger: 50,
		LastSeenLedger:  60,
		UpdatedAt:       time.Now(),
	}

	if err := m.UpsertContractEventSchema(context.Background(), schema); err != nil {
		t.Fatalf("upsert schema failed: %v", err)
	}

	list, err := m.GetContractEventSchemas(context.Background(), "CAAA")
	if err != nil {
		t.Fatalf("get schemas failed: %v", err)
	}
	if len(list) != 1 || list[0].EventName != "mint" {
		t.Fatalf("expected 1 schema 'mint', got %v", list)
	}
}
