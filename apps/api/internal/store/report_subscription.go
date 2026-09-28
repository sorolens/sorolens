package store

import (
	"context"
	"time"
)

// Report digest frequencies.
const (
	ReportDaily  = "daily"
	ReportWeekly = "weekly"
)

// ValidReportFrequencies is the set a subscription may be created with.
var ValidReportFrequencies = map[string]bool{
	ReportDaily:  true,
	ReportWeekly: true,
}

// ReportSubscription is an email digest subscription (issue #330).
type ReportSubscription struct {
	ID             string
	Email          string
	Frequency      string
	DayOfWeek      int
	CreatedAt      time.Time
	UnsubscribedAt *time.Time
}

// Active reports whether the subscription has not been unsubscribed.
func (s ReportSubscription) Active() bool { return s.UnsubscribedAt == nil }

// ReportSubscriptionStore is the data-access surface for email digest
// subscriptions.
type ReportSubscriptionStore interface {
	// CreateReportSubscription persists a new subscription.
	CreateReportSubscription(ctx context.Context, s ReportSubscription) error
	// ListReportSubscriptions returns the active subscriptions for an email.
	ListReportSubscriptions(ctx context.Context, email string) ([]ReportSubscription, error)
	// GetReportSubscription returns one subscription by id, or ErrNotFound.
	GetReportSubscription(ctx context.Context, id string) (ReportSubscription, error)
	// DeleteReportSubscription marks a subscription unsubscribed. Returns
	// ErrNotFound if it does not exist or is already unsubscribed.
	DeleteReportSubscription(ctx context.Context, id string) error
	// ListDueReportSubscriptions returns active subscriptions that should
	// receive a digest now: every daily subscription, and weekly ones whose
	// day_of_week matches weekday (0=Sunday..6=Saturday).
	ListDueReportSubscriptions(ctx context.Context, frequency string, weekday int) ([]ReportSubscription, error)
}
