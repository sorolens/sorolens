package store

import (
	"context"
	"time"
)

// In-memory PushSubscriptionStore for handler tests (issue #592). The MockStore
// field is initialised lazily so tests that never touch push do not pay for it.

func (m *MockStore) pushStore() *[]PushSubscription {
	if m.pushSubscriptions == nil {
		m.pushSubscriptions = make([]PushSubscription, 0)
	}
	return &m.pushSubscriptions
}

func (m *MockStore) UpsertPushSubscription(_ context.Context, sub PushSubscription) (bool, error) {
	subs := m.pushStore()
	for i := range *subs {
		if (*subs)[i].Endpoint == sub.Endpoint {
			if (*subs)[i].UserID != sub.UserID {
				return false, nil
			}
			if sub.CreatedAt.IsZero() {
				sub.CreatedAt = (*subs)[i].CreatedAt
			}
			sub.UpdatedAt = time.Now().UTC()
			(*subs)[i] = sub
			return true, nil
		}
	}
	if sub.CreatedAt.IsZero() {
		sub.CreatedAt = time.Now().UTC()
	}
	sub.UpdatedAt = sub.CreatedAt
	*subs = append(*subs, sub)
	return true, nil
}

func (m *MockStore) ListPushSubscriptions(_ context.Context, userID string) ([]PushSubscription, error) {
	out := make([]PushSubscription, 0)
	for _, sub := range *m.pushStore() {
		if userID == "" || sub.UserID == userID {
			out = append(out, sub)
		}
	}
	return out, nil
}

func (m *MockStore) DeletePushSubscription(_ context.Context, userID, endpoint string) (bool, error) {
	subs := m.pushStore()
	for i, sub := range *subs {
		if sub.Endpoint == endpoint && (userID == "" || sub.UserID == userID) {
			*subs = append((*subs)[:i], (*subs)[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// AddPushSubscription seeds a subscription in tests.
func (m *MockStore) AddPushSubscription(sub PushSubscription) {
	_, _ = m.UpsertPushSubscription(context.Background(), sub)
}
