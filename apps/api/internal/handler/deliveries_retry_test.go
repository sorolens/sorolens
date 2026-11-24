package handler_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestRetryDeliveryChecksOwnershipBeforeWriting(t *testing.T) {
	ms := seedRBACUsers(t)
	seedMonitored(t, ms)
	ctx := context.Background()
	now := time.Now().UTC()
	for _, id := range []string{"sub_a", "sub_b"} {
		if err := ms.Create(ctx, store.AlertSubscription{ID: id, ContractID: "C1", WebhookURL: "https://example.com/h", ChannelType: "webhook", SeverityFilter: "Critical", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	errMsg := "boom"
	if err := ms.UpsertDelivery(ctx, store.WebhookDelivery{ID: "del_b", SubscriptionID: "sub_b", AttemptNumber: 1, Status: "failed", LastError: &errMsg, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)
	base := "/api/v1/watchdog/subscriptions/"

	cases := []struct{ name, path string }{
		{"delivery of another subscription", base + "sub_a/deliveries/del_b/retry"},
		{"unknown subscription", base + "sub_missing/deliveries/del_b/retry"},
		{"unknown delivery", base + "sub_b/deliveries/del_missing/retry"},
	}
	for _, tc := range cases {
		w := doRequestAsUser(srv, http.MethodPost, tc.path, adminToken, contributorUser, "{}")
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s: want 404, got %d (%s)", tc.name, w.Code, w.Body.String())
		}
	}
	d, err := ms.GetDelivery(ctx, "del_b")
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "failed" || d.AttemptNumber != 1 || d.LastError == nil {
		t.Fatalf("delivery must not be modified by rejected retries: %+v", d)
	}

	// The legitimate retry still works.
	w := doRequestAsUser(srv, http.MethodPost, base+"sub_b/deliveries/del_b/retry", adminToken, contributorUser, "{}")
	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d (%s)", w.Code, w.Body.String())
	}
	d, _ = ms.GetDelivery(ctx, "del_b")
	if d.Status != "pending" || d.AttemptNumber != 2 {
		t.Fatalf("delivery not retried: %+v", d)
	}
}
