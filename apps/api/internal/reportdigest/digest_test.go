package reportdigest_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/reportdigest"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// fakeStore is a hand-rolled DigestStore so the test controls exactly what the
// digest sees.
type fakeStore struct {
	due       []store.ReportSubscription
	contracts []store.Contract
	stats     map[string]store.ContractStats
}

func (f *fakeStore) ListDueReportSubscriptions(_ context.Context, _ string, _ int) ([]store.ReportSubscription, error) {
	return f.due, nil
}
func (f *fakeStore) ListContracts(_ context.Context, _ string, _ int, _ store.ContractFilters) ([]store.Contract, string, error) {
	return f.contracts, "", nil
}
func (f *fakeStore) GetContractStats(_ context.Context, id, _ string) (store.ContractStats, error) {
	return f.stats[id], nil
}

// fakeMailer records every message it is asked to send.
type fakeMailer struct {
	sent []struct{ to, subject, html, text string }
}

func (m *fakeMailer) Send(_ context.Context, to, subject, htmlBody, textBody string) error {
	m.sent = append(m.sent, struct{ to, subject, html, text string }{to, subject, htmlBody, textBody})
	return nil
}

func TestRunDigestSendsAndRendersContent(t *testing.T) {
	fs := &fakeStore{
		due: []store.ReportSubscription{
			{ID: "sub-1", Email: "a@example.com", Frequency: store.ReportDaily},
		},
		contracts: []store.Contract{
			{ID: "CABC", Label: "Router"},
		},
		stats: map[string]store.ContractStats{
			"CABC": {WindowEventCount: 12, WindowInvocationCount: 5, StorageCount: 3},
		},
	}
	mailer := &fakeMailer{}

	sent, err := reportdigest.RunDigest(context.Background(), fs, mailer, reportdigest.Config{
		Frequency:  store.ReportDaily,
		BaseURL:    "https://sorolens.dev",
		SigningKey: []byte("test-key"),
	}, time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if sent != 1 || len(mailer.sent) != 1 {
		t.Fatalf("want 1 email sent, got %d", sent)
	}

	msg := mailer.sent[0]
	if msg.to != "a@example.com" {
		t.Errorf("recipient: got %q", msg.to)
	}
	for _, want := range []string{"Router", "CABC", "12", "5"} {
		if !strings.Contains(msg.text, want) || !strings.Contains(msg.html, want) {
			t.Errorf("digest body missing %q\ntext=%s", want, msg.text)
		}
	}
	// Every email carries a signed one-click unsubscribe link for its own id.
	if !strings.Contains(msg.text, "/api/v1/reports/unsubscribe?token=sub-1.") {
		t.Errorf("missing signed unsubscribe link: %s", msg.text)
	}
}

func TestRunDigestSkippedWhenNoContracts(t *testing.T) {
	fs := &fakeStore{
		due: []store.ReportSubscription{
			{ID: "sub-1", Email: "a@example.com", Frequency: store.ReportDaily},
		},
	}
	mailer := &fakeMailer{}
	sent, err := reportdigest.RunDigest(context.Background(), fs, mailer, reportdigest.Config{
		Frequency: store.ReportDaily,
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if sent != 0 || len(mailer.sent) != 0 {
		t.Fatalf("expected nothing sent when no contracts tracked, got %d", sent)
	}
}

func TestRunDigestNoSubscribers(t *testing.T) {
	fs := &fakeStore{contracts: []store.Contract{{ID: "CABC"}}}
	mailer := &fakeMailer{}
	sent, err := reportdigest.RunDigest(context.Background(), fs, mailer, reportdigest.Config{
		Frequency: store.ReportWeekly,
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if sent != 0 {
		t.Fatalf("no subscribers should send nothing, got %d", sent)
	}
}
