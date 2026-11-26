package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Sentinel errors for escalation policy operations.
var (
	ErrEscalationPolicyNotFound = errors.New("store: escalation policy not found")
	ErrInvalidEscalationPolicy  = errors.New("store: invalid escalation policy")
)

// EscalationStep is one ordered step in an escalation policy.
type EscalationStep struct {
	// Order is the zero-based position of this step (0 = first).
	Order int `json:"order"`
	// Channel is the notification channel: email, slack, pagerduty.
	Channel string `json:"channel"`
	// Recipients is a list of addresses or identifiers for the channel.
	Recipients []string `json:"recipients"`
	// DelayMins is how many minutes after the previous step (or alert open time
	// for step 0) before this step fires.
	DelayMins int `json:"delay_mins"`
}

// MarshalSteps serialises steps as JSON for Postgres storage.
func MarshalSteps(steps []EscalationStep) ([]byte, error) {
	return json.Marshal(steps)
}

// UnmarshalSteps deserialises steps from Postgres JSON storage.
func UnmarshalSteps(data []byte) ([]EscalationStep, error) {
	var steps []EscalationStep
	if err := json.Unmarshal(data, &steps); err != nil {
		return nil, err
	}
	return steps, nil
}

// EscalationPolicy defines an ordered sequence of notification steps that fire
// progressively if an alert remains open.
type EscalationPolicy struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Steps     []EscalationStep `json:"steps"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// EscalationPolicyStore is the read/write surface for escalation policies.
type EscalationPolicyStore interface {
	// CreateEscalationPolicy persists a new policy and returns the stored row
	// with server-assigned ID and timestamps.
	CreateEscalationPolicy(ctx context.Context, p EscalationPolicy) (EscalationPolicy, error)
	// GetEscalationPolicy returns the policy with the given ID, or
	// ErrEscalationPolicyNotFound.
	GetEscalationPolicy(ctx context.Context, id string) (EscalationPolicy, error)
	// ListEscalationPolicies returns all policies ordered by name.
	ListEscalationPolicies(ctx context.Context) ([]EscalationPolicy, error)
	// UpdateEscalationPolicy replaces name and steps on the policy identified
	// by p.ID. Returns ErrEscalationPolicyNotFound if absent.
	UpdateEscalationPolicy(ctx context.Context, p EscalationPolicy) (EscalationPolicy, error)
	// DeleteEscalationPolicy removes a policy. Deleting a policy that does not
	// exist is a no-op success.
	DeleteEscalationPolicy(ctx context.Context, id string) error
}
