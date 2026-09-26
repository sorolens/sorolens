package store

import (
	"context"
	"time"
)

// PerformanceBaseline represents a daily snapshot of resource usage.
type PerformanceBaseline struct {
	ContractID   string
	FunctionName string
	AvgCPU       int64
	AvgMem       int64
	AvgFee       int64
	SnapshotDate time.Time
}

// 7-day average metrics
type PerformanceMetrics struct {
	ContractID   string
	FunctionName string
	AvgCPU       int64
	AvgMem       int64
	AvgFee       int64
}

// CheckRegressionsResult holds the result of a regression check.
type CheckRegressionsResult struct {
	AlertsEmitted int
}

// PerformanceStore defines methods for computing baselines and detecting regressions.
type PerformanceStore interface {
	ComputeAndStoreBaselines(ctx context.Context, snapshotDate time.Time) error
	CheckAndEmitRegressions(ctx context.Context, snapshotDate time.Time) (int, error)
}
