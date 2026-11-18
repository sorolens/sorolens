package partition

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockPartitionStore struct {
	year       int
	month      int
	createErr  error
	calledNext bool
	nextErr    error
}

func (m *mockPartitionStore) CreateMonthlyPartitionIfNotExists(_ context.Context, year int, month int) error {
	m.year = year
	m.month = month
	return m.createErr
}

func (m *mockPartitionStore) CreateNextMonthPartition(_ context.Context) error {
	m.calledNext = true
	return m.nextErr
}

func TestEnsureNextMonthPartition(t *testing.T) {
	ctx := context.Background()

	t.Run("creates partition for next month with correct year and month", func(t *testing.T) {
		mock := &mockPartitionStore{}
		now := time.Now()
		expectedTarget := now.AddDate(0, 1, 0)
		expectedYear := expectedTarget.Year()
		expectedMonth := int(expectedTarget.Month())

		err := EnsureNextMonthPartition(ctx, mock)
		if err != nil {
			t.Fatalf("EnsureNextMonthPartition error: %v", err)
		}
		if mock.year != expectedYear {
			t.Errorf("year = %d, want %d", mock.year, expectedYear)
		}
		if mock.month != expectedMonth {
			t.Errorf("month = %d, want %d", mock.month, expectedMonth)
		}
	})

	t.Run("propagates store error", func(t *testing.T) {
		dbErr := errors.New("failed to create partition table")
		mock := &mockPartitionStore{createErr: dbErr}

		err := EnsureNextMonthPartition(ctx, mock)
		if !errors.Is(err, dbErr) {
			t.Fatalf("expected error %v, got %v", dbErr, err)
		}
	})

	t.Run("handles cancelled context", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		mock := &mockPartitionStore{
			createErr: context.Canceled,
		}

		err := EnsureNextMonthPartition(canceledCtx, mock)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}

func TestYearMonthCalculationAcrossYearBoundary(t *testing.T) {
	// Verify date arithmetic rolls December into next January
	decemberDate := time.Date(2025, time.December, 15, 12, 0, 0, 0, time.UTC)
	nextMonth := decemberDate.AddDate(0, 1, 0)
	if nextMonth.Year() != 2026 {
		t.Errorf("Year = %d, want 2026", nextMonth.Year())
	}
	if nextMonth.Month() != time.January {
		t.Errorf("Month = %v, want January", nextMonth.Month())
	}
}
