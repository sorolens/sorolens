package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ObservedShape describes an observed variant of topic arity, types, and value shape.
type ObservedShape struct {
	TopicArity int      `json:"topic_arity"`
	TopicTypes []string `json:"topic_types"`
	ValueType  string   `json:"value_type"`
	Count      int64    `json:"count"`
}

// ContractEventSchema records the inferred event schema for a contract and event name.
type ContractEventSchema struct {
	ContractID      string          `json:"contract_id"`
	EventName       string          `json:"event_name"`
	TopicArity      int             `json:"topic_arity"`
	TopicTypes      []string        `json:"topic_types"`
	ValueShape      string          `json:"value_shape"`
	ObservedShapes  []ObservedShape `json:"observed_shapes"`
	FirstSeenLedger uint32          `json:"first_seen_ledger"`
	LastSeenLedger  uint32          `json:"last_seen_ledger"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// ContractEventSchemaStore is the persistence interface for per-contract event schemas.
type ContractEventSchemaStore interface {
	UpsertContractEventSchema(ctx context.Context, s ContractEventSchema) error
	GetContractEventSchemas(ctx context.Context, contractID string) ([]ContractEventSchema, error)
}

func inferTypeName(v any) string {
	if v == nil {
		return "void"
	}
	switch v.(type) {
	case string:
		return "string"
	case float64, float32, int, int32, int64, uint, uint32, uint64:
		return "number"
	case bool:
		return "bool"
	case []any:
		return "vec"
	case map[string]any:
		return "map"
	default:
		return "scval"
	}
}

// InferEventSchemas groups events by event name (topic[0]) and aggregates observed shapes.
func InferEventSchemas(contractID string, events []Event) []ContractEventSchema {
	grouped := make(map[string]*ContractEventSchema)

	for _, e := range events {
		if len(e.TopicDecoded) == 0 {
			continue
		}
		eventName := fmt.Sprint(e.TopicDecoded[0])
		arity := len(e.TopicDecoded)

		topicTypes := make([]string, arity)
		for i, t := range e.TopicDecoded {
			topicTypes[i] = inferTypeName(t)
		}
		valueType := inferTypeName(e.ValueDecoded)

		schema, exists := grouped[eventName]
		if !exists {
			schema = &ContractEventSchema{
				ContractID:      contractID,
				EventName:       eventName,
				TopicArity:      arity,
				TopicTypes:      topicTypes,
				ValueShape:      valueType,
				FirstSeenLedger: e.Ledger,
				LastSeenLedger:  e.Ledger,
				UpdatedAt:       time.Now(),
			}
			grouped[eventName] = schema
		}

		if e.Ledger < schema.FirstSeenLedger || schema.FirstSeenLedger == 0 {
			schema.FirstSeenLedger = e.Ledger
		}
		if e.Ledger > schema.LastSeenLedger {
			schema.LastSeenLedger = e.Ledger
			schema.TopicArity = arity
			schema.TopicTypes = topicTypes
			schema.ValueShape = valueType
		}

		// Update observed shapes variant counts
		foundShape := false
		for i := range schema.ObservedShapes {
			s := &schema.ObservedShapes[i]
			if s.TopicArity == arity && s.ValueType == valueType && equalTypes(s.TopicTypes, topicTypes) {
				s.Count++
				foundShape = true
				break
			}
		}
		if !foundShape {
			schema.ObservedShapes = append(schema.ObservedShapes, ObservedShape{
				TopicArity: arity,
				TopicTypes: topicTypes,
				ValueType:  valueType,
				Count:      1,
			})
		}
	}

	result := make([]ContractEventSchema, 0, len(grouped))
	for _, s := range grouped {
		result = append(result, *s)
	}
	return result
}

func equalTypes(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *postgresStore) UpsertContractEventSchema(ctx context.Context, schema ContractEventSchema) error {
	topicTypesJSON, _ := json.Marshal(schema.TopicTypes)
	observedJSON, _ := json.Marshal(schema.ObservedShapes)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO contract_event_schemas (
			contract_id, event_name, topic_arity, topic_types,
			value_shape, observed_shapes, first_seen_ledger, last_seen_ledger, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (contract_id, event_name) DO UPDATE SET
			topic_arity = EXCLUDED.topic_arity,
			topic_types = EXCLUDED.topic_types,
			value_shape = EXCLUDED.value_shape,
			observed_shapes = EXCLUDED.observed_shapes,
			first_seen_ledger = LEAST(contract_event_schemas.first_seen_ledger, EXCLUDED.first_seen_ledger),
			last_seen_ledger = GREATEST(contract_event_schemas.last_seen_ledger, EXCLUDED.last_seen_ledger),
			updated_at = NOW()`,
		schema.ContractID, schema.EventName, schema.TopicArity, topicTypesJSON,
		schema.ValueShape, observedJSON, schema.FirstSeenLedger, schema.LastSeenLedger,
	)
	if err != nil {
		return fmt.Errorf("upsert contract event schema: %w", err)
	}
	return nil
}

func (s *postgresStore) GetContractEventSchemas(ctx context.Context, contractID string) ([]ContractEventSchema, error) {
	rows, err := s.readDB(ctx).Query(ctx, `
		SELECT contract_id, event_name, topic_arity, topic_types,
		       value_shape, observed_shapes, first_seen_ledger, last_seen_ledger, updated_at
		FROM contract_event_schemas
		WHERE contract_id = $1
		ORDER BY event_name ASC`, contractID)
	if err != nil {
		return nil, fmt.Errorf("get contract event schemas: %w", err)
	}
	defer rows.Close()

	var out []ContractEventSchema
	for rows.Next() {
		var item ContractEventSchema
		var topicRaw, observedRaw []byte
		if err := rows.Scan(
			&item.ContractID, &item.EventName, &item.TopicArity, &topicRaw,
			&item.ValueShape, &observedRaw, &item.FirstSeenLedger, &item.LastSeenLedger, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(topicRaw, &item.TopicTypes)
		_ = json.Unmarshal(observedRaw, &item.ObservedShapes)
		out = append(out, item)
	}
	return out, rows.Err()
}
