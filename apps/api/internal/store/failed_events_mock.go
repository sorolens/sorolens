package store

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

// In-memory FailedEventStore implementation for MockStore.

func (m *MockStore) ensureFailedEvents() {
	if m.failedEvents == nil {
		m.failedEvents = make(map[int64]FailedEvent)
	}
}

func (m *MockStore) InsertFailedEvent(_ context.Context, fe FailedEvent) error {
	if m.InsertFailedEventErr != nil {
		return m.InsertFailedEventErr
	}
	m.ensureFailedEvents()
	if fe.CreatedAt.IsZero() {
		fe.CreatedAt = time.Now().UTC()
	}
	if fe.Attempts <= 0 {
		fe.Attempts = 3
	}
	// Upsert by event_id.
	for id, existing := range m.failedEvents {
		if existing.EventID == fe.EventID {
			fe.ID = id
			m.failedEvents[id] = fe
			return nil
		}
	}
	m.failedEventSeq++
	fe.ID = m.failedEventSeq
	m.failedEvents[fe.ID] = fe
	return nil
}

func (m *MockStore) ListFailedEvents(_ context.Context, cursor string, limit int) ([]FailedEvent, string, error) {
	if m.ListFailedEventsErr != nil {
		return nil, "", m.ListFailedEventsErr
	}
	m.ensureFailedEvents()
	if limit <= 0 {
		limit = 50
	}
	var cursorID int64
	if cursor != "" {
		cursorID, _ = strconv.ParseInt(cursor, 10, 64)
	}
	// Collect and sort by id DESC.
	ids := make([]int64, 0, len(m.failedEvents))
	for id := range m.failedEvents {
		if cursorID == 0 || id < cursorID {
			ids = append(ids, id)
		}
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] > ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	var out []FailedEvent
	for _, id := range ids {
		out = append(out, m.failedEvents[id])
		if len(out) > limit {
			break
		}
	}
	var next string
	if len(out) > limit {
		out = out[:limit]
		next = strconv.FormatInt(out[len(out)-1].ID, 10)
	}
	return out, next, nil
}

func (m *MockStore) GetFailedEvent(_ context.Context, id int64) (FailedEvent, error) {
	if m.GetFailedEventErr != nil {
		return FailedEvent{}, m.GetFailedEventErr
	}
	m.ensureFailedEvents()
	fe, ok := m.failedEvents[id]
	if !ok {
		return FailedEvent{}, ErrNotFound
	}
	return fe, nil
}

func (m *MockStore) DeleteFailedEvent(_ context.Context, id int64) error {
	if m.DeleteFailedEventErr != nil {
		return m.DeleteFailedEventErr
	}
	m.ensureFailedEvents()
	if _, ok := m.failedEvents[id]; !ok {
		return ErrNotFound
	}
	delete(m.failedEvents, id)
	return nil
}

// SeedFailedEvent is a test helper that inserts a DLQ row and returns it.
func (m *MockStore) SeedFailedEvent(eventID, contractID, errMsg string, payload Event) FailedEvent {
	raw, _ := json.Marshal(payload)
	fe := FailedEvent{
		EventID:      eventID,
		ContractID:   contractID,
		Network:      payload.Network,
		EventPayload: raw,
		ErrorMessage: errMsg,
		Attempts:     3,
		CreatedAt:    time.Now().UTC(),
	}
	_ = m.InsertFailedEvent(context.Background(), fe)
	// Reload with assigned ID.
	for _, v := range m.failedEvents {
		if v.EventID == eventID {
			return v
		}
	}
	return fe
}
