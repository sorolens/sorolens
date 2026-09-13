package watchdog

import (
	"errors"
	"testing"
	"time"
)

func TestClassifyKind(t *testing.T) {
	cases := []struct {
		name   string
		topics []string
		want   EventKind
	}{
		{"nil topics", nil, KindUnknown},
		{"empty topics", []string{}, KindUnknown},
		{"unrelated topic", []string{"SomeOther", "x"}, KindUnknown},
		{"registered", []string{TopicContractRegistered, "cid"}, KindContractRegistered},
		{"deregistered", []string{TopicContractDeregistered, "cid"}, KindContractDeregistered},
		{"health", []string{TopicHealthCheck, "cid"}, KindHealthCheck},
		{"alert", []string{TopicContractAlert, "cid", "Critical"}, KindContractAlert},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyKind(RawEvent{Topics: c.topics})
			if got != c.want {
				t.Fatalf("classify %+v: got %v want %v", c.topics, got, c.want)
			}
		})
	}
}

func TestProjectRegistration(t *testing.T) {
	e := RawEvent{
		Topics: []string{TopicContractRegistered, "CABC"},
		Value:  map[string]any{"name": "svc_v1", "owner": "GOWNER"},
	}
	r, err := ProjectRegistration(e)
	if err != nil {
		t.Fatal(err)
	}
	if r.ContractID != "CABC" || r.Name != "svc_v1" || r.Owner != "GOWNER" {
		t.Fatalf("unexpected registration: %+v", r)
	}
}

func TestProjectRegistrationWrongKind(t *testing.T) {
	e := RawEvent{Topics: []string{TopicHealthCheck, "CABC"}}
	if _, err := ProjectRegistration(e); !errors.Is(err, ErrWrongKind) {
		t.Fatalf("expected ErrWrongKind, got %v", err)
	}
}

func TestProjectRegistrationRequiresContractIdTopic(t *testing.T) {
	e := RawEvent{Topics: []string{TopicContractRegistered}}
	if _, err := ProjectRegistration(e); err == nil {
		t.Fatal("expected missing-contract_id error")
	}
}

func TestProjectHealthUsesEventTimestampWhenPresent(t *testing.T) {
	closed := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	pushed := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC).Unix()
	e := RawEvent{
		Topics:         []string{TopicHealthCheck, "CABC"},
		LedgerClosedAt: closed,
		Ledger:         42,
		TxHash:         "tx1",
		Value: map[string]any{
			"status":    "Degraded",
			"metadata":  "cpu=91",
			"timestamp": pushed,
		},
	}
	h, err := ProjectHealth(e)
	if err != nil {
		t.Fatal(err)
	}
	if h.Timestamp.Unix() != pushed {
		t.Fatalf("expected event timestamp %d, got %d", pushed, h.Timestamp.Unix())
	}
	if h.Status != "Degraded" || h.Metadata != "cpu=91" {
		t.Fatalf("wrong body: %+v", h)
	}
	if h.Ledger != 42 || h.TxHash != "tx1" {
		t.Fatalf("wrong provenance: %+v", h)
	}
}

func TestProjectHealthFallsBackToLedgerCloseTime(t *testing.T) {
	closed := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	e := RawEvent{
		Topics:         []string{TopicHealthCheck, "CABC"},
		LedgerClosedAt: closed,
		Value:          map[string]any{"status": "Healthy"},
	}
	h, err := ProjectHealth(e)
	if err != nil {
		t.Fatal(err)
	}
	if !h.Timestamp.Equal(closed) {
		t.Fatalf("expected ledger close time, got %v", h.Timestamp)
	}
}

func TestProjectAlertReadsSeverityFromTopicOrValue(t *testing.T) {
	// Topic form (as emitted by the contract; severity is #[topic]).
	e := RawEvent{
		Topics: []string{TopicContractAlert, "CABC", "Critical"},
		Value:  map[string]any{"message": "queue backing up"},
	}
	a, err := ProjectAlert(e)
	if err != nil {
		t.Fatal(err)
	}
	if a.Severity != "Critical" || a.Message != "queue backing up" {
		t.Fatalf("wrong alert (topic form): %+v", a)
	}

	// Value fallback form.
	e2 := RawEvent{
		Topics: []string{TopicContractAlert, "CABC"},
		Value:  map[string]any{"severity": "Warning", "message": "slow"},
	}
	a2, err := ProjectAlert(e2)
	if err != nil {
		t.Fatal(err)
	}
	if a2.Severity != "Warning" || a2.Message != "slow" {
		t.Fatalf("wrong alert (value form): %+v", a2)
	}
}

func TestProjectDeregistration(t *testing.T) {
	e := RawEvent{
		Topics:         []string{TopicContractDeregistered, "CABC"},
		LedgerClosedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	d, err := ProjectDeregistration(e)
	if err != nil {
		t.Fatal(err)
	}
	if d.ContractID != "CABC" {
		t.Fatalf("wrong contract id: %+v", d)
	}
}
