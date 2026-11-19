package reportdigest_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/reportdigest"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// recordingStore instruments every DigestStore call so the tests can pin
// down exactly what RunDigest asked for: the weekday it scheduled for, the
// stats window per frequency, the contract limit after clamping, and any
// injected error.
type recordingStore struct {
	fakeStore

	gotFrequency string
	gotWeekday   int
	gotLimit     int
	gotWindows   []string

	listDueErr  error
	listConnErr error
	statsErr    error
}

func (r *recordingStore) ListDueReportSubscriptions(_ context.Context, freq string, weekday int) ([]store.ReportSubscription, error) {
	r.gotFrequency, r.gotWeekday = freq, weekday
	if r.listDueErr != nil {
		return nil, r.listDueErr
	}
	return r.due, nil
}

func (r *recordingStore) ListContracts(_ context.Context, _ string, limit int, _ store.ContractFilters) ([]store.Contract, string, error) {
	r.gotLimit = limit
	if r.listConnErr != nil {
		return nil, "", r.listConnErr
	}
	return r.contracts, "", nil
}

func (r *recordingStore) GetContractStats(_ context.Context, _, window string) (store.ContractStats, error) {
	r.gotWindows = append(r.gotWindows, window)
	if r.statsErr != nil {
		return store.ContractStats{}, r.statsErr
	}
	return store.ContractStats{}, nil
}

func sub(id string) store.ReportSubscription {
	return store.ReportSubscription{ID: id, Email: id + "@example.com", Frequency: store.ReportDaily}
}

func TestRunDigestRejectsInvalidFrequency(t *testing.T) {
	fs := &recordingStore{}
	mailer := &fakeMailer{}
	if _, err := reportdigest.RunDigest(context.Background(), fs, mailer, reportdigest.Config{
		Frequency: "hourly",
	}, time.Now()); err == nil {
		t.Fatal("an invalid frequency must be rejected before anything is listed")
	}
	if len(mailer.sent) != 0 {
		t.Errorf("no email may be sent for an invalid frequency, sent %d", len(mailer.sent))
	}
}

func TestRunDigestQueriesSchedulingInputs(t *testing.T) {
	fs := &recordingStore{fakeStore: fakeStore{
		due:       []store.ReportSubscription{sub("s1")},
		contracts: []store.Contract{{ID: "CABC"}},
	}}
	now := time.Date(2026, 1, 8, 8, 0, 0, 0, time.UTC) // a Thursday
	if _, err := reportdigest.RunDigest(context.Background(), fs, &fakeMailer{}, reportdigest.Config{
		Frequency: store.ReportWeekly,
		MaxContracts: 7,
	}, now); err != nil {
		t.Fatal(err)
	}
	if fs.gotFrequency != store.ReportWeekly {
		t.Errorf("due subscriptions were listed for frequency %q, want %q", fs.gotFrequency, store.ReportWeekly)
	}
	// Thursday == 4.
	if fs.gotWeekday != 4 {
		t.Errorf("due subscriptions were listed for weekday %d, want 4", fs.gotWeekday)
	}
	if fs.gotLimit != 7 {
		t.Errorf("contract list limit = %d, want 7", fs.gotLimit)
	}
	// The weekly digest must use the weekly stats window.
	if len(fs.gotWindows) != 1 || fs.gotWindows[0] != "7d" {
		t.Errorf("stats windows = %v, want [7d]", fs.gotWindows)
	}
}

func TestRunDigestDailyUses24hWindowAndClampsLimit(t *testing.T) {
	fs := &recordingStore{fakeStore: fakeStore{
		due:       []store.ReportSubscription{sub("s1")},
		contracts: []store.Contract{{ID: "CABC"}},
	}}
	if _, err := reportdigest.RunDigest(context.Background(), fs, &fakeMailer{}, reportdigest.Config{
		Frequency: store.ReportDaily,
		// Non-positive limits fall back to the default of 50.
	}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(fs.gotWindows) != 1 || fs.gotWindows[0] != "24h" {
		t.Errorf("stats windows = %v, want [24h]", fs.gotWindows)
	}
	if fs.gotLimit != 50 {
		t.Errorf("contract list limit = %d, want default 50", fs.gotLimit)
	}

	fs2 := &recordingStore{fakeStore: fakeStore{
		due:       []store.ReportSubscription{sub("s1")},
		contracts: []store.Contract{{ID: "CABC"}},
	}}
	if _, err := reportdigest.RunDigest(context.Background(), fs2, &fakeMailer{}, reportdigest.Config{
		Frequency:   store.ReportDaily,
		MaxContracts: 500, // capped at 100
	}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if fs2.gotLimit != 50 {
		t.Errorf("contract list limit = %d, want clamp 50 (500 -> 50)", fs2.gotLimit)
	}
}

func TestRunDigestStopsAtFirstSendError(t *testing.T) {
	fs := &fakeStore{
		due:       []store.ReportSubscription{sub("s1"), sub("s2")},
		contracts: []store.Contract{{ID: "CABC"}},
	}
	failing := &failingMailer{failFrom: 1}
	sent, err := reportdigest.RunDigest(context.Background(), fs, failing, reportdigest.Config{
		Frequency: store.ReportDaily,
	}, time.Now())
	if err == nil {
		t.Fatal("a send error must abort the run with an error")
	}
	if sent != 1 {
		t.Errorf("sent = %d, want 1 (the message before the failure)", sent)
	}
}

type failingMailer struct {
	sent     int
	failFrom int
}

func (m *failingMailer) Send(_ context.Context, _, _, _, _ string) error {
	if m.sent >= m.failFrom {
		return errors.New("smtp down")
	}
	m.sent++
	return nil
}

func TestRunDigestPropagatesStoreErrors(t *testing.T) {
	for name, inject := range map[string]func(*recordingStore){
		"list due":   func(r *recordingStore) { r.listDueErr = errors.New("boom") },
		"list conn":  func(r *recordingStore) { r.listConnErr = errors.New("boom") },
		"get stats":  func(r *recordingStore) { r.statsErr = errors.New("boom") },
	} {
		r := &recordingStore{fakeStore: fakeStore{
			due:       []store.ReportSubscription{sub("s1")},
			contracts: []store.Contract{{ID: "CABC"}},
		}}
		inject(r)
		if _, err := reportdigest.RunDigest(context.Background(), r, &fakeMailer{}, reportdigest.Config{
			Frequency: store.ReportDaily,
		}, time.Now()); err == nil {
			t.Errorf("%s: expected an error, got none", name)
		}
	}
}

// The unsubscribe token is what authorizes the one-click unsubscribe, so it
// must be exactly id + "." + HMAC-SHA256(signing key, id) — not the bare id,
// and not a signature over anything else.
func TestRunDigestUnsubscribeTokenIsExactHMAC(t *testing.T) {
	fs := &fakeStore{
		due:       []store.ReportSubscription{sub("sub-9")},
		contracts: []store.Contract{{ID: "CABC"}},
	}
	mailer := &fakeMailer{}
	if _, err := reportdigest.RunDigest(context.Background(), fs, mailer, reportdigest.Config{
		Frequency:  store.ReportDaily,
		BaseURL:    "https://sorolens.dev/",
		SigningKey: []byte("secret"),
	}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("want 1 email, got %d", len(mailer.sent))
	}
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte("sub-9"))
	want := "sub-9." + hex.EncodeToString(mac.Sum(nil))
	if !strings.Contains(mailer.sent[0].text, "token="+want) {
		t.Errorf("unsubscribe token is not the exact HMAC of the subscription id:\n%s", mailer.sent[0].text)
	}
	// The signed link must not be the bare id (that would let anyone
	// unsubscribe anyone).
	if strings.Contains(mailer.sent[0].text, "token=sub-9\n") {
		t.Errorf("unsubscribe link carries the unsigned subscription id")
	}
}

// Without a signing key the digest runs in unsigned mode and the token is
// the bare subscription id, matching the handler's unsigned mode.
func TestRunDigestUnsignedTokenWithoutKey(t *testing.T) {
	fs := &fakeStore{
		due:       []store.ReportSubscription{sub("sub-1")},
		contracts: []store.Contract{{ID: "CABC"}},
	}
	mailer := &fakeMailer{}
	if _, err := reportdigest.RunDigest(context.Background(), fs, mailer, reportdigest.Config{
		Frequency: store.ReportDaily,
		BaseURL:   "https://sorolens.dev",
	}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mailer.sent[0].text, "token=sub-1\n") {
		t.Errorf("unsigned mode must use the bare subscription id:\n%s", mailer.sent[0].text)
	}
	if strings.Contains(mailer.sent[0].text, "token=sub-1.") {
		t.Errorf("no key configured, but the token looks signed:\n%s", mailer.sent[0].text)
	}
}

// A trailing slash on the base URL must not produce a double slash: the
// handler routes the path literally.
func TestRunDigestTrimsBaseURLSlash(t *testing.T) {
	fs := &fakeStore{
		due:       []store.ReportSubscription{sub("s1")},
		contracts: []store.Contract{{ID: "CABC"}},
	}
	mailer := &fakeMailer{}
	if _, err := reportdigest.RunDigest(context.Background(), fs, mailer, reportdigest.Config{
		Frequency: store.ReportDaily,
		BaseURL:   "https://sorolens.dev///",
	}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(mailer.sent[0].text, "sorolens.dev//") {
		t.Errorf("base URL slash not trimmed:\n%s", mailer.sent[0].text)
	}
	if !strings.Contains(mailer.sent[0].text, "https://sorolens.dev/api/v1/reports/unsubscribe?token=") {
		t.Errorf("unsubscribe URL malformed:\n%s", mailer.sent[0].text)
	}
}

func TestRunDigestSubjectAndPeriodPerFrequency(t *testing.T) {
	fs := &fakeStore{
		due:       []store.ReportSubscription{sub("s1")},
		contracts: []store.Contract{{ID: "CABC"}},
	}
	w := &fakeMailer{}
	if _, err := reportdigest.RunDigest(context.Background(), fs, w, reportdigest.Config{
		Frequency: store.ReportWeekly,
	}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if w.sent[0].subject != "Your weekly Sorolens digest" {
		t.Errorf("weekly subject = %q", w.sent[0].subject)
	}
	if !strings.Contains(w.sent[0].text, "7 days") {
		t.Errorf("weekly body must say 7 days:\n%s", w.sent[0].text)
	}

	d := &fakeMailer{}
	if _, err := reportdigest.RunDigest(context.Background(), fs, d, reportdigest.Config{
		Frequency: store.ReportDaily,
	}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if d.sent[0].subject != "Your daily Sorolens digest" {
		t.Errorf("daily subject = %q", d.sent[0].subject)
	}
	if !strings.Contains(d.sent[0].text, "24 hours") {
		t.Errorf("daily body must say 24 hours:\n%s", d.sent[0].text)
	}
}
