package partition

import (
	"context"
	"errors"
	"testing"
	"time"
)

// recorded is one CreateMonthlyPartitionIfNotExists call.
type recorded struct {
	year, month int
}

type mockStore struct {
	created []recorded
	// createErr, when set, is returned by every CreateMonthlyPartitionIfNotExists.
	createErr error

	archiveCalls []int
	archiveNames []string
	archiveErr   error
}

func (m *mockStore) CreateMonthlyPartitionIfNotExists(_ context.Context, year, month int) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, recorded{year, month})
	return nil
}

func (m *mockStore) ArchiveOldPartitions(_ context.Context, retainMonths int) ([]string, error) {
	if m.archiveErr != nil {
		return nil, m.archiveErr
	}
	m.archiveCalls = append(m.archiveCalls, retainMonths)
	return m.archiveNames, nil
}

func TestEnsureCoverageMonths(t *testing.T) {
	// monthsAhead = 3 from 2026-10-16 => Oct, Nov, Dec 2026 and Jan 2027.
	// The year boundary must roll over, not stay in 2026.
	tests := []struct {
		name        string
		now         time.Time
		monthsAhead int
		want        []recorded
	}{
		{
			name:        "mid month rolls into next year",
			now:         time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC),
			monthsAhead: 3,
			want: []recorded{
				{2026, 10}, {2026, 11}, {2026, 12}, {2027, 1},
			},
		},
		{
			// The whole point of anchoring on the first of the month: a naive
			// AddDate(0, i, 0) on Jan 31 jumps to Mar 3 and skips February.
			name:        "jan 31 includes february",
			now:         time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
			monthsAhead: 2,
			want:        []recorded{{2026, 1}, {2026, 2}, {2026, 3}},
		},
		{
			// monthsAhead < 1 is clamped to 1: current month + one ahead.
			name:        "zero clamps to one ahead",
			now:         time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
			monthsAhead: 0,
			want:        []recorded{{2026, 12}, {2027, 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &mockStore{}
			if err := EnsureCoverage(context.Background(), s, tt.now, tt.monthsAhead); err != nil {
				t.Fatalf("EnsureCoverage: %v", err)
			}
			if len(s.created) != len(tt.want) {
				t.Fatalf("created %d partitions, want %d: %+v", len(s.created), len(tt.want), s.created)
			}
			for i, w := range tt.want {
				if s.created[i] != w {
					t.Errorf("created[%d] = %+v, want %+v", i, s.created[i], w)
				}
			}
		})
	}
}

func TestEnsureCoverageWrapsCreateError(t *testing.T) {
	boom := errors.New("boom")
	s := &mockStore{createErr: boom}
	err := EnsureCoverage(context.Background(), s, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), 1)
	if !errors.Is(err, boom) {
		t.Fatalf("EnsureCoverage error = %v, want it to wrap %v", err, boom)
	}
}

func TestRetainDisabled(t *testing.T) {
	s := &mockStore{}
	names, err := Retain(context.Background(), s, 0)
	if err != nil {
		t.Fatalf("Retain: %v", err)
	}
	if names != nil {
		t.Errorf("Retain(0) names = %v, want nil", names)
	}
	if len(s.archiveCalls) != 0 {
		t.Errorf("Retain(0) must not call the store, got %v", s.archiveCalls)
	}
}

func TestRetainPassesWindow(t *testing.T) {
	s := &mockStore{archiveNames: []string{"events_2025_01", "events_2025_02"}}
	names, err := Retain(context.Background(), s, 12)
	if err != nil {
		t.Fatalf("Retain: %v", err)
	}
	if len(s.archiveCalls) != 1 || s.archiveCalls[0] != 12 {
		t.Errorf("archive calls = %v, want [12]", s.archiveCalls)
	}
	if len(names) != 2 {
		t.Errorf("names = %v, want 2 archived partitions", names)
	}
}
