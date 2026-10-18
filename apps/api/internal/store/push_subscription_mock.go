package store

import (
	"context"
	"time"
)

// ---- store.PushSubscriptionStore ----------------------------------------
//
// Mock implementation kept in its own file so the shared mock.go struct
// (which neighbouring features keep growing) only carries the field.

// AddPushSubscription is a test helper that seeds a subscription directly.
func (m *MockStore) AddPushSubscription(s PushSubscription) {
	m.pushSubs = append(m.pushSubs, s)
}

func (m *MockStore) UpsertPushSubscription(_ context.Context, s PushSubscription) (PushSubscription, bool, error) {
	if m.PushStoreErr != nil {
		return PushSubscription{}, false, m.PushStoreErr
	}
	now := time.Now().UTC()
	for i := range m.pushSubs {
		if m.pushSubs[i].Endpoint == s.Endpoint {
			// Update in place, preserving id/created_at like the Postgres
			// ON CONFLICT DO UPDATE does.
			m.pushSubs[i].P256dhKey = s.P256dhKey
			m.pushSubs[i].AuthKey = s.AuthKey
			m.pushSubs[i].UserID = s.UserID
			m.pushSubs[i].Filters = s.Filters
			m.pushSubs[i].UpdatedAt = now
			return m.pushSubs[i], false, nil
		}
	}
	if len(s.Filters) == 0 {
		s.Filters = []byte("{}")
	}
	s.CreatedAt = now
	s.UpdatedAt = now
	m.pushSubs = append(m.pushSubs, s)
	return s, true, nil
}

func (m *MockStore) GetPushSubscription(_ context.Context, id string) (PushSubscription, error) {
	for _, s := range m.pushSubs {
		if s.ID == id {
			return s, nil
		}
	}
	return PushSubscription{}, ErrNotFound
}

func (m *MockStore) ListPushSubscriptions(_ context.Context, userID string) ([]PushSubscription, error) {
	var out []PushSubscription
	for _, s := range m.pushSubs {
		if userID == "" || s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *MockStore) DeletePushSubscription(_ context.Context, id string) error {
	for i := range m.pushSubs {
		if m.pushSubs[i].ID == id {
			m.pushSubs = append(m.pushSubs[:i], m.pushSubs[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (m *MockStore) DeletePushSubscriptionByEndpoint(_ context.Context, endpoint string) error {
	for i := range m.pushSubs {
		if m.pushSubs[i].Endpoint == endpoint {
			m.pushSubs = append(m.pushSubs[:i], m.pushSubs[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}
