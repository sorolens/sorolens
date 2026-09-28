package handler_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestReportSubscriptionLifecycle(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	// Create (public, no auth needed).
	w := doRequest(srv, http.MethodPost, "/api/v1/reports/subscriptions", "",
		`{"email":"reader@example.com","frequency":"weekly","day_of_week":3}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var created struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Frequency string `json:"frequency"`
		DayOfWeek int    `json:"day_of_week"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Frequency != "weekly" || created.DayOfWeek != 3 {
		t.Fatalf("unexpected subscription: %+v", created)
	}

	// List by email.
	w = doRequest(srv, http.MethodGet, "/api/v1/reports/subscriptions?email=reader@example.com", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", w.Code)
	}
	if !jsonHasField(w.Body.String(), "subscriptions") {
		t.Errorf("list body missing subscriptions: %s", w.Body.String())
	}

	// Unsubscribe via the one-click token (unsigned in tests: bare id).
	w = doRequest(srv, http.MethodGet, "/api/v1/reports/unsubscribe?token="+created.ID, "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("unsubscribe: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	// After unsubscribe the email has no active subscriptions.
	w = doRequest(srv, http.MethodGet, "/api/v1/reports/subscriptions?email=reader@example.com", "", "")
	if w.Code != http.StatusOK || bodyHasNonEmptyList(t, w.Body.Bytes()) {
		t.Fatalf("expected empty list after unsubscribe, got %s", w.Body.String())
	}
}

func TestReportSubscriptionRejectsBadInput(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	cases := []string{
		`{"email":"not-an-email","frequency":"daily"}`,
		`{"email":"ok@example.com","frequency":"hourly"}`,
		`{"email":"ok@example.com","frequency":"weekly","day_of_week":9}`,
	}
	for _, body := range cases {
		w := doRequest(srv, http.MethodPost, "/api/v1/reports/subscriptions", "", body)
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("want 422 for %s, got %d", body, w.Code)
		}
	}
}

func bodyHasNonEmptyList(t *testing.T, body []byte) bool {
	t.Helper()
	var resp struct {
		Subscriptions []json.RawMessage `json:"subscriptions"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	return len(resp.Subscriptions) > 0
}
