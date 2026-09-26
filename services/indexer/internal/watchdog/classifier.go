// Package watchdog classifies raw Soroban events emitted by the on-chain
// sorolens-watchdog contract and lowers them into the store models used by
// the API and dashboard.
//
// The classifier is intentionally decoupled from the RPC and the store so
// it can be unit-tested with hand-crafted decoded topics and values, and so
// the poller can adopt it without a large refactor.
package watchdog

import (
	"errors"
	"fmt"
	"time"
)

// EventKind is the recognised watchdog event topic.
type EventKind int

const (
	KindUnknown EventKind = iota
	KindContractRegistered
	KindContractDeregistered
	KindHealthCheck
	KindContractAlert
)

// Topic strings emitted by the contract match the type name of the Rust
// `#[contractevent]` struct. Keep these in sync with contracts/watchdog/src/lib.rs.
const (
	TopicContractRegistered   = "ContractRegistered"
	TopicContractDeregistered = "ContractDeregistered"
	TopicHealthCheck          = "HealthCheckEvent"
	TopicContractAlert        = "ContractAlert"
)

// RawEvent is the minimal shape of a Soroban event the classifier needs.
// The poller can provide it without importing the full store package.
type RawEvent struct {
	ContractID       string
	Ledger           int64
	LedgerClosedAt   time.Time
	TxHash           string
	// Decoded topic ScVals as strings; first topic is the event type, second
	// is the monitored contract id, third (when present) is severity.
	Topics           []string
	// Decoded value payload, keyed by field name from the Rust struct.
	Value            map[string]any
}

// ClassifyKind returns the EventKind for a raw event by inspecting its first
// topic. Returns KindUnknown for anything not emitted by the watchdog.
func ClassifyKind(e RawEvent) EventKind {
	if len(e.Topics) == 0 {
		return KindUnknown
	}
	switch e.Topics[0] {
	case TopicContractRegistered:
		return KindContractRegistered
	case TopicContractDeregistered:
		return KindContractDeregistered
	case TopicHealthCheck:
		return KindHealthCheck
	case TopicContractAlert:
		return KindContractAlert
	default:
		return KindUnknown
	}
}

// Registration is the projection of a ContractRegistered event.
type Registration struct {
	ContractID    string
	Name          string
	Owner         string
	CheckInterval int64
	Timestamp     time.Time
}

// Deregistration is the projection of a ContractDeregistered event.
type Deregistration struct {
	ContractID string
	Timestamp  time.Time
}

// Health is the projection of a HealthCheckEvent.
type Health struct {
	ContractID string
	Status     string
	Metadata   string
	Ledger     int64
	TxHash     string
	Timestamp  time.Time
}

// Alert is the projection of a ContractAlert.
type Alert struct {
	ContractID string
	Severity   string
	Message    string
	Ledger     int64
	TxHash     string
	Timestamp  time.Time
}

// ErrWrongKind is returned when an event's topic does not match the
// projection function being called on it.
var ErrWrongKind = errors.New("watchdog: event topic does not match projection")

// ProjectRegistration decodes a ContractRegistered event.
func ProjectRegistration(e RawEvent) (Registration, error) {
	if ClassifyKind(e) != KindContractRegistered {
		return Registration{}, ErrWrongKind
	}
	cid, err := requireString(e.Topics, 1, "contract_id")
	if err != nil {
		return Registration{}, err
	}
	name, _ := stringField(e.Value, "name")
	owner, _ := stringField(e.Value, "owner")
	ts := ledgerOrValueTimestamp(e)
	return Registration{
		ContractID: cid,
		Name:       name,
		Owner:      owner,
		Timestamp:  ts,
	}, nil
}

// ProjectDeregistration decodes a ContractDeregistered event.
func ProjectDeregistration(e RawEvent) (Deregistration, error) {
	if ClassifyKind(e) != KindContractDeregistered {
		return Deregistration{}, ErrWrongKind
	}
	cid, err := requireString(e.Topics, 1, "contract_id")
	if err != nil {
		return Deregistration{}, err
	}
	return Deregistration{
		ContractID: cid,
		Timestamp:  ledgerOrValueTimestamp(e),
	}, nil
}

// ProjectHealth decodes a HealthCheckEvent.
func ProjectHealth(e RawEvent) (Health, error) {
	if ClassifyKind(e) != KindHealthCheck {
		return Health{}, ErrWrongKind
	}
	cid, err := requireString(e.Topics, 1, "contract_id")
	if err != nil {
		return Health{}, err
	}
	status, _ := stringField(e.Value, "status")
	status = normalizeHealthStatus(status)
	metadata, _ := stringField(e.Value, "metadata")
	return Health{
		ContractID: cid,
		Status:     status,
		Metadata:   metadata,
		Ledger:     e.Ledger,
		TxHash:     e.TxHash,
		Timestamp:  ledgerOrValueTimestamp(e),
	}, nil
}

// ProjectAlert decodes a ContractAlert.
func ProjectAlert(e RawEvent) (Alert, error) {
	if ClassifyKind(e) != KindContractAlert {
		return Alert{}, ErrWrongKind
	}
	cid, err := requireString(e.Topics, 1, "contract_id")
	if err != nil {
		return Alert{}, err
	}
	severity := ""
	if len(e.Topics) >= 3 {
		severity = e.Topics[2]
	}
	if severity == "" {
		if s, ok := stringField(e.Value, "severity"); ok {
			severity = s
		}
	}
	if severity == "" {
		severity = "Info"
	}
	message, _ := stringField(e.Value, "message")
	return Alert{
		ContractID: cid,
		Severity:   severity,
		Message:    message,
		Ledger:     e.Ledger,
		TxHash:     e.TxHash,
		Timestamp:  ledgerOrValueTimestamp(e),
	}, nil
}

// ---- helpers ---------------------------------------------------------------

// knownHealthStatuses enumerates the values the on-chain `HealthStatus` enum
// (contracts/watchdog/src/lib.rs) can emit on a HealthCheckEvent. RPC payloads
// are untrusted, so anything outside this set is treated as malformed.
var knownHealthStatuses = map[string]struct{}{
	"Healthy":      {},
	"Degraded":     {},
	"Unresponsive": {},
}

// defaultHealthStatus is returned when a HealthCheckEvent carries a missing,
// empty, or out-of-enum status value.
const defaultHealthStatus = "Unknown"

// normalizeHealthStatus validates a decoded status string against the on-chain
// HealthStatus enum and falls back to defaultHealthStatus for malformed input,
// so a bad payload cannot leak an arbitrary status into the store.
func normalizeHealthStatus(status string) string {
	if _, ok := knownHealthStatuses[status]; ok {
		return status
	}
	return defaultHealthStatus
}

func requireString(topics []string, idx int, name string) (string, error) {
	if idx >= len(topics) || topics[idx] == "" {
		return "", fmt.Errorf("watchdog: missing %s topic", name)
	}
	return topics[idx], nil
}

func stringField(m map[string]any, name string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[name]
	if !ok {
		return "", false
	}
	if s, ok := v.(string); ok {
		return s, true
	}
	return fmt.Sprintf("%v", v), true
}

func ledgerOrValueTimestamp(e RawEvent) time.Time {
	// The contract puts a Unix timestamp in the event body; prefer it when
	// present, otherwise fall back to the ledger close time supplied by RPC.
	if e.Value != nil {
		if v, ok := e.Value["timestamp"]; ok {
			switch n := v.(type) {
			case int64:
				if n > 0 {
					return time.Unix(n, 0).UTC()
				}
			case uint64:
				if n > 0 {
					return time.Unix(int64(n), 0).UTC()
				}
			case float64:
				if n > 0 {
					return time.Unix(int64(n), 0).UTC()
				}
			}
		}
	}
	if !e.LedgerClosedAt.IsZero() {
		return e.LedgerClosedAt.UTC()
	}
	return time.Now().UTC()
}
