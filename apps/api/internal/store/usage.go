package store

import (
	"context"
	"time"
)

// APIKeyUsageRecord represents aggregated request counts for a single key,
// date, and endpoint group (issue #792).
type APIKeyUsageRecord struct {
	KeyID         string    `json:"key_id"`
	Date          time.Time `json:"date"`
	EndpointGroup string    `json:"endpoint_group"`
	RequestCount  int64     `json:"request_count"`
}

// UsageTimelinePoint represents a daily aggregation of API requests.
type UsageTimelinePoint struct {
	Date          string           `json:"date"` // YYYY-MM-DD
	TotalRequests int64            `json:"total_requests"`
	ByGroup       map[string]int64 `json:"by_group"`
}

// KeyUsageSummary provides aggregated request statistics and quota tracking
// for an API key across a given date range.
type KeyUsageSummary struct {
	KeyID          string               `json:"key_id"`
	From           time.Time            `json:"from"`
	To             time.Time            `json:"to"`
	TotalRequests  int64                `json:"total_requests"`
	ByGroup        map[string]int64     `json:"by_group"`
	Timeline       []UsageTimelinePoint `json:"timeline"`
	MonthlyQuota   *int                 `json:"monthly_quota,omitempty"`
	MonthUsed      int64                `json:"month_used"`
	MonthRemaining *int64               `json:"month_remaining,omitempty"`
}

// UsageStore provides data access for API key usage metering and quota tracking.
type UsageStore interface {
	// RecordBatchUsage flushes a batch of usage records to PostgreSQL.
	RecordBatchUsage(ctx context.Context, records []APIKeyUsageRecord) error
	// GetKeyUsage returns usage metrics for the given key between from and to.
	GetKeyUsage(ctx context.Context, keyID string, from, to time.Time) (*KeyUsageSummary, error)
	// GetMonthlyKeyUsage returns total requests recorded for the given key in the specified month.
	GetMonthlyKeyUsage(ctx context.Context, keyID string, year int, month time.Month) (int64, error)
}
