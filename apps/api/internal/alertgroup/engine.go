// Package alertgroup implements the alert deduplication and grouping engine
// (issue #269).
//
// The engine is deliberately free of database or HTTP concerns so it can be
// unit-tested with plain Go structs. Persistence is handled by the caller
// via AlertGroupStore.
//
// # Grouping semantics
//
// A group is identified by a GroupKey: the tuple (ContractID, Severity, Rule).
// Rule is an optional tag that the alert producer sets (e.g. "high_error_rate");
// it defaults to "" when the upstream alert carries no rule annotation.
//
// Two alerts belong to the same group when:
//  1. Their GroupKey matches, AND
//  2. The incoming alert arrives within DedupeWindowSecs seconds of the
//     group's last_seen timestamp.
//
// When an alert matches an open group the group's count is incremented and
// last_seen / last_message are updated. When no open group exists, a new one
// is created with count = 1.
//
// This mirrors PagerDuty / Alertmanager grouping semantics as suggested in
// the issue description.
package alertgroup

import (
	"fmt"
	"time"
)

// DefaultDedupeWindowSecs is used when the caller supplies zero.
const DefaultDedupeWindowSecs = 300 // 5 minutes

// Group is the in-memory representation of one alert group. It maps 1-to-1
// with a row in the alert_groups table.
type Group struct {
	// GroupKey is the opaque string key: "contractID|severity|rule".
	GroupKey string

	ContractID       string
	Severity         string
	Rule             string
	Count            int64
	DedupeWindowSecs int64
	FirstSeen        time.Time
	LastSeen         time.Time
	LastMessage      string
	BackfillEligible bool
	State            string     // open | acknowledged | snoozed | resolved
	SnoozedUntil     *time.Time
}

// IncomingAlert is the minimal representation of an alert that the engine
// needs to decide whether to open a new group or merge into an existing one.
type IncomingAlert struct {
	ContractID   string
	Severity     string
	Rule         string // may be empty
	Message      string
	Timestamp    time.Time
	State        string
	SnoozedUntil *time.Time
}

// BuildGroupKey returns the canonical group key for an alert.
func BuildGroupKey(contractID, severity, rule string) string {
	return fmt.Sprintf("%s|%s|%s", contractID, severity, rule)
}

// MergeResult describes what the engine decided for one incoming alert.
type MergeResult int

const (
	// MergeResultNew means no open group matched — a new group was created.
	MergeResultNew MergeResult = iota
	// MergeResultMerged means the alert was merged into an existing group.
	MergeResultMerged
)

// Merge decides whether the incoming alert should open a new group or be
// merged into the provided existing group. It mutates existing in-place when
// merging.
//
// existing may be nil to signal that no group for this key is currently open.
// dedupeWindowSecs overrides the group's own window when > 0.
func Merge(existing *Group, alert IncomingAlert, dedupeWindowSecs int64) (*Group, MergeResult) {
	if dedupeWindowSecs <= 0 {
		dedupeWindowSecs = DefaultDedupeWindowSecs
	}

	if existing != nil {
		window := time.Duration(existing.DedupeWindowSecs) * time.Second
		if !alert.Timestamp.After(existing.LastSeen.Add(window)) {
			// Within the dedupe window — merge.
			existing.Count++
			existing.LastSeen = alert.Timestamp
			existing.LastMessage = alert.Message
			// If the group was snoozed and the snooze window has elapsed, resume (open)
			if existing.State == "snoozed" && existing.SnoozedUntil != nil && !alert.Timestamp.Before(*existing.SnoozedUntil) {
				existing.State = "open"
				existing.SnoozedUntil = nil
			}
			return existing, MergeResultMerged
		}
	}

	state := alert.State
	if state == "" {
		state = "open"
	}

	// Outside window or no existing group — open a new one.
	g := &Group{
		GroupKey:         BuildGroupKey(alert.ContractID, alert.Severity, alert.Rule),
		ContractID:       alert.ContractID,
		Severity:         alert.Severity,
		Rule:             alert.Rule,
		Count:            1,
		DedupeWindowSecs: dedupeWindowSecs,
		FirstSeen:        alert.Timestamp,
		LastSeen:         alert.Timestamp,
		LastMessage:      alert.Message,
		State:            state,
		SnoozedUntil:     alert.SnoozedUntil,
	}
	return g, MergeResultNew
}

// ShouldNotify returns whether notifications should be sent for a given state and snooze expiry at time `now`.
// It returns false for acknowledged alerts, resolved alerts, and snoozed alerts before SnoozedUntil.
// Once SnoozedUntil has passed, notifications resume.
func ShouldNotify(state string, snoozedUntil *time.Time, now time.Time) bool {
	switch state {
	case "acknowledged", "resolved":
		return false
	case "snoozed":
		if snoozedUntil != nil && now.Before(*snoozedUntil) {
			return false
		}
		return true
	default:
		return true
	}
}
