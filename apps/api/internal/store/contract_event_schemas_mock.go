package store

import (
	"context"
	"sort"
)

// UpsertContractEventSchema implements ContractEventSchemaStore for the in-memory mock.
func (m *MockStore) UpsertContractEventSchema(_ context.Context, s ContractEventSchema) error {
	if m.contractEventSchemas == nil {
		m.contractEventSchemas = make(map[string]map[string]ContractEventSchema)
	}
	schemas, ok := m.contractEventSchemas[s.ContractID]
	if !ok {
		schemas = make(map[string]ContractEventSchema)
		m.contractEventSchemas[s.ContractID] = schemas
	}
	schemas[s.EventName] = s
	return nil
}

// GetContractEventSchemas implements ContractEventSchemaStore for the in-memory mock.
func (m *MockStore) GetContractEventSchemas(_ context.Context, contractID string) ([]ContractEventSchema, error) {
	if m.contractEventSchemas == nil {
		return []ContractEventSchema{}, nil
	}
	schemas, ok := m.contractEventSchemas[contractID]
	if !ok {
		return []ContractEventSchema{}, nil
	}
	res := make([]ContractEventSchema, 0, len(schemas))
	for _, s := range schemas {
		res = append(res, s)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].EventName < res[j].EventName
	})
	return res, nil
}
