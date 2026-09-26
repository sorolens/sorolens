package watchdog

import (
	"errors"
	"reflect"
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

// ---- edge case coverage ----------------------------------------------------
//
// The tests below exercise the malformed, incomplete, and replayed event
// shapes the indexer can see from RPC. The classifier must never panic: it
// either returns a usable projection or a typed error the poller can skip.

// TestProjectionsMissingContractIDTopicDoNotPanic covers events whose topic0
// matches a known kind but that carry no (or an empty) contract_id topic. The
// projection helpers must return an error rather than indexing out of range.
func TestProjectionsMissingContractIDTopicDoNotPanic(t *testing.T) {
	if _, err := ProjectRegistration(RawEvent{Topics: []string{TopicContractRegistered}}); err == nil {
		t.Fatal("registration: expected missing contract_id error")
	}
	if _, err := ProjectDeregistration(RawEvent{Topics: []string{TopicContractDeregistered}}); err == nil {
		t.Fatal("deregistration: expected missing contract_id error")
	}
	if _, err := ProjectHealth(RawEvent{Topics: []string{TopicHealthCheck}}); err == nil {
		t.Fatal("health: expected missing contract_id error")
	}
	if _, err := ProjectAlert(RawEvent{Topics: []string{TopicContractAlert}}); err == nil {
		t.Fatal("alert: expected missing contract_id error")
	}

	// An empty-string contract_id topic is treated as missing.
	if _, err := ProjectHealth(RawEvent{Topics: []string{TopicHealthCheck, ""}}); err == nil {
		t.Fatal("health: expected empty contract_id topic to error")
	}

	// No topics at all is a clean skip, not a panic, for every projection.
	empty := RawEvent{}
	if _, err := ProjectRegistration(empty); !errors.Is(err, ErrWrongKind) {
		t.Fatalf("registration: expected ErrWrongKind, got %v", err)
	}
	if _, err := ProjectDeregistration(empty); !errors.Is(err, ErrWrongKind) {
		t.Fatalf("deregistration: expected ErrWrongKind, got %v", err)
	}
	if _, err := ProjectHealth(empty); !errors.Is(err, ErrWrongKind) {
		t.Fatalf("health: expected ErrWrongKind, got %v", err)
	}
	if _, err := ProjectAlert(empty); !errors.Is(err, ErrWrongKind) {
		t.Fatalf("alert: expected ErrWrongKind, got %v", err)
	}
}

// TestProjectHealthNormalizesStatus ensures only values from the on-chain
// HealthStatus enum survive projection. Missing, empty, wrong-typed, and
// out-of-enum values all fall back to the default instead of leaking through.
func TestProjectHealthNormalizesStatus(t *testing.T) {
	cases := []struct {
		name  string
		set   bool
		value any
		want  string
	}{
		{"missing status", false, nil, "Unknown"},
		{"empty status", true, "", "Unknown"},
		{"nil status", true, nil, "Unknown"},
		{"non-string status", true, 42, "Unknown"},
		{"bool status", true, true, "Unknown"},
		{"invalid enum value", true, "Bogus", "Unknown"},
		{"valid Healthy", true, "Healthy", "Healthy"},
		{"valid Degraded", true, "Degraded", "Degraded"},
		{"valid Unresponsive", true, "Unresponsive", "Unresponsive"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			value := map[string]any{}
			if c.set {
				value["status"] = c.value
			}
			e := RawEvent{
				Topics:         []string{TopicHealthCheck, "CABC"},
				LedgerClosedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
				Value:          value,
			}
			h, err := ProjectHealth(e)
			if err != nil {
				t.Fatal(err)
			}
			if h.Status != c.want {
				t.Fatalf("status %#v: got %q want %q", c.value, h.Status, c.want)
			}
		})
	}
}

// TestDuplicateEventProjectionIsIdempotent covers a replayed event carrying the
// same ledger + tx_hash + contract_id. The store dedupes on (tx_hash,
// contract_id) via ux_health_checks_tx_contract, so the classifier must
// project the replay identically and must not mutate the caller's event.
func TestDuplicateEventProjectionIsIdempotent(t *testing.T) {
	event := RawEvent{
		ContractID:     "CABC",
		Ledger:         900,
		LedgerClosedAt: time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC),
		TxHash:         "deadbeef",
		Topics:         []string{TopicHealthCheck, "CABC"},
		Value:          map[string]any{"status": "Degraded", "metadata": "cpu=99"},
	}
	first, err := ProjectHealth(event)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ProjectHealth(event)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("replayed event projected differently:\n first=%+v\nsecond=%+v", first, second)
	}

	// Dedup key components must be carried through unchanged.
	if second.Ledger != event.Ledger || second.TxHash != event.TxHash || second.ContractID != event.ContractID {
		t.Fatalf("dedup key changed on replay: %+v", second)
	}

	// Projection is pure: it must not mutate the caller's slices or maps.
	if !reflect.DeepEqual(event.Topics, []string{TopicHealthCheck, "CABC"}) {
		t.Fatalf("classifier mutated topics: %q", event.Topics)
	}
	if !reflect.DeepEqual(event.Value, map[string]any{"status": "Degraded", "metadata": "cpu=99"}) {
		t.Fatalf("classifier mutated value: %#v", event.Value)
	}
}

// TestZeroAndNegativeTimestampFallBack covers events with a zero or negative
// body timestamp. They must not yield the Unix epoch or a panic; the ledger
// close time is used when present, otherwise a sane wall-clock default.
func TestZeroAndNegativeTimestampFallBack(t *testing.T) {
	closed := time.Date(2025, 7, 8, 9, 10, 11, 0, time.UTC)
	for _, rawTS := range []any{int64(0), int64(-5), uint64(0), float64(0)} {
		e := RawEvent{
			Topics:         []string{TopicHealthCheck, "CABC"},
			LedgerClosedAt: closed,
			Value:          map[string]any{"status": "Healthy", "timestamp": rawTS},
		}
		h, err := ProjectHealth(e)
		if err != nil {
			t.Fatal(err)
		}
		if !h.Timestamp.Equal(closed) {
			t.Fatalf("timestamp %#v: expected ledger close time %v, got %v", rawTS, closed, h.Timestamp)
		}
	}

	// No body timestamp and no ledger close time: fall back to the wall clock
	// rather than time.Time{}.
	before := time.Now().UTC()
	h, err := ProjectHealth(RawEvent{
		Topics: []string{TopicHealthCheck, "CABC"},
		Value:  map[string]any{"status": "Healthy"},
	})
	after := time.Now().UTC()
	if err != nil {
		t.Fatal(err)
	}
	if h.Timestamp.IsZero() || h.Timestamp.Before(before) || h.Timestamp.After(after) {
		t.Fatalf("expected wall-clock fallback in [%v, %v], got %v", before, after, h.Timestamp)
	}
}

// TestEmptyPayloadIsSkippedGracefully covers a completely empty decoded event,
// which is what the poller sees when XDR decoding yields nothing useful.
func TestEmptyPayloadIsSkippedGracefully(t *testing.T) {
	e := RawEvent{}
	if got := ClassifyKind(e); got != KindUnknown {
		t.Fatalf("empty event classified as %v, want KindUnknown", got)
	}
	if _, err := ProjectRegistration(e); !errors.Is(err, ErrWrongKind) {
		t.Fatalf("empty registration: expected ErrWrongKind, got %v", err)
	}
	if _, err := ProjectDeregistration(e); !errors.Is(err, ErrWrongKind) {
		t.Fatalf("empty deregistration: expected ErrWrongKind, got %v", err)
	}
	if _, err := ProjectHealth(e); !errors.Is(err, ErrWrongKind) {
		t.Fatalf("empty health: expected ErrWrongKind, got %v", err)
	}
	if _, err := ProjectAlert(e); !errors.Is(err, ErrWrongKind) {
		t.Fatalf("empty alert: expected ErrWrongKind, got %v", err)
	}
}

// TestMalformedValuePayloadDoesNotPanic hammers every projection with the
// wrong-typed and nil payloads an untrusted RPC response can produce.
func TestMalformedValuePayloadDoesNotPanic(t *testing.T) {
	cases := []RawEvent{
		{Topics: []string{TopicHealthCheck, "CABC"}},
		{Topics: []string{TopicHealthCheck, "CABC"}, Value: map[string]any{"status": nil}},
		{Topics: []string{TopicHealthCheck, "CABC"}, Value: map[string]any{"status": true, "metadata": 7}},
		{Topics: []string{TopicContractAlert, "CABC"}, Value: map[string]any{"severity": 7, "message": nil}},
		{Topics: []string{TopicContractAlert, "CABC", ""}, Value: map[string]any{}},
		{Topics: []string{TopicContractRegistered, "CABC"}, Value: map[string]any{"name": 1, "owner": []string{"x"}}},
		{Topics: []string{TopicContractDeregistered, "CABC"}, Value: map[string]any{"timestamp": "not-a-number"}},
	}
	for i, e := range cases {
		func(i int, e RawEvent) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("case %d panicked on %+v: %v", i, e, r)
				}
			}()
			_, _ = ProjectRegistration(e)
			_, _ = ProjectDeregistration(e)
			_, _ = ProjectHealth(e)
			_, _ = ProjectAlert(e)
		}(i, e)
	}
}

// TestProjectAlertSeverityFallbacks covers the severity resolution order:
// topic, then value, then the Info default.
func TestProjectAlertSeverityFallbacks(t *testing.T) {
	cases := []struct {
		name   string
		topics []string
		value  map[string]any
		want   string
	}{
		{"topic beats value", []string{TopicContractAlert, "CABC", "Critical"}, map[string]any{"severity": "Warning"}, "Critical"},
		{"value used when no topic", []string{TopicContractAlert, "CABC"}, map[string]any{"severity": "Warning"}, "Warning"},
		{"empty topic falls back to value", []string{TopicContractAlert, "CABC", ""}, map[string]any{"severity": "Warning"}, "Warning"},
		{"missing severity defaults to Info", []string{TopicContractAlert, "CABC"}, map[string]any{"message": "slow"}, "Info"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, err := ProjectAlert(RawEvent{Topics: c.topics, Value: c.value})
			if err != nil {
				t.Fatal(err)
			}
			if a.Severity != c.want {
				t.Fatalf("severity: got %q want %q", a.Severity, c.want)
			}
		})
	}
}
