package partition

import (
	"context"
	"errors"
	"testing"
	"time"
)

type partitionCall struct {
	year  int
	month int
}

type mockPartitionStore struct {
	calls      []partitionCall
	year       int
	month      int
	createErr  error
	calledNext bool
	nextErr    error
}

func (m *mockPartitionStore) CreateMonthlyPartitionIfNotExists(_ context.Context, year int, month int) error {
	m.calls = append(m.calls, partitionCall{year: year, month: month})
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
		firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		expectedTarget := firstOfMonth.AddDate(0, 1, 0)
		expectedYear := expectedTarget.Year()
		expectedMonth := int(expectedTarget.Month())

		err := EnsureNextMonthPartition(ctx, mock)
		if err != nil {
			t.Fatalf("EnsureNextMonthPartition error: %v", err)
		}
		if len(mock.calls) < 2 {
			t.Fatalf("expected at least 2 partition calls, got %d", len(mock.calls))
		}
		// mock.calls[1] is the next month partition
		if mock.calls[1].year != expectedYear {
			t.Errorf("year = %d, want %d", mock.calls[1].year, expectedYear)
		}
		if mock.calls[1].month != expectedMonth {
			t.Errorf("month = %d, want %d", mock.calls[1].month, expectedMonth)
		}

		// Ensure 6 future months + current month (total 7 calls)
		if len(mock.calls) != 7 {
			t.Errorf("calls count = %d, want 7", len(mock.calls))
		}
		for i := 0; i < len(mock.calls); i++ {
			expected := firstOfMonth.AddDate(0, i, 0)
			if mock.calls[i].year != expected.Year() || mock.calls[i].month != int(expected.Month()) {
				t.Errorf("calls[%d] = (%d, %d), want (%d, %d)",
					i, mock.calls[i].year, mock.calls[i].month, expected.Year(), expected.Month())
			}
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

func TestMaintainPartitions(t *testing.T) {
	ctx := context.Background()

	t.Run("creates specified months ahead", func(t *testing.T) {
		mock := &mockPartitionStore{}
		err := MaintainPartitions(ctx, mock, 3)
		if err != nil {
			t.Fatalf("MaintainPartitions error: %v", err)
		}
		// current month + 3 future months = 4 calls
		if len(mock.calls) != 4 {
			t.Fatalf("calls count = %d, want 4", len(mock.calls))
		}
	})

	t.Run("defaults to 6 months ahead when monthsAhead < 1", func(t *testing.T) {
		mock := &mockPartitionStore{}
		err := MaintainPartitions(ctx, mock, 0)
		if err != nil {
			t.Fatalf("MaintainPartitions error: %v", err)
		}
		if len(mock.calls) != 7 {
			t.Fatalf("calls count = %d, want 7", len(mock.calls))
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

func TestMonthEndCalculationDoesNotSkipMonth(t *testing.T) {
	// Verify adding months from the 1st of the month does not skip shorter months (e.g., Feb)
	janDate := time.Date(2025, time.January, 31, 12, 0, 0, 0, time.UTC)
	firstOfMonth := time.Date(janDate.Year(), janDate.Month(), 1, 0, 0, 0, 0, janDate.Location())
	nextMonth := firstOfMonth.AddDate(0, 1, 0)
	if nextMonth.Month() != time.February {
		t.Errorf("Month = %v, want February", nextMonth.Month())
	}
}
