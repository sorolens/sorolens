package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrMaintenanceWindowNotFound = errors.New("store: maintenance window not found")
	ErrInvalidMaintenanceWindow  = errors.New("store: invalid maintenance window")
)

// MaintenanceWindow represents a window during which notifications are suppressed.
type MaintenanceWindow struct {
	ID                    int64     `json:"id"`
	Name                  string    `json:"name"`
	Scope                 string    `json:"scope"` // global | contract | group
	TargetID              string    `json:"target_id,omitempty"`
	StartTime             time.Time `json:"start_time"`
	EndTime               time.Time `json:"end_time"`
	Timezone              string    `json:"timezone"`
	Recurrence            string    `json:"recurrence"` // none | daily | weekly
	SuppressNotifications bool      `json:"suppress_notifications"`
	CreatedBy             string    `json:"created_by"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// IsActive checks whether the given time falls within this maintenance window,
// handling explicit time zones and recurring schedules.
func (w *MaintenanceWindow) IsActive(t time.Time) bool {
	if t.IsZero() {
		t = time.Now().UTC()
	}

	tz := w.Timezone
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}

	tInLoc := t.In(loc)
	startInLoc := w.StartTime.In(loc)
	endInLoc := w.EndTime.In(loc)

	switch strings.ToLower(w.Recurrence) {
	case "daily":
		// Window applies daily between start time-of-day and end time-of-day.
		curMins := tInLoc.Hour()*60 + tInLoc.Minute()
		startMins := startInLoc.Hour()*60 + startInLoc.Minute()
		endMins := endInLoc.Hour()*60 + endInLoc.Minute()

		if startMins <= endMins {
			return curMins >= startMins && curMins <= endMins
		}
		// Overnight window (e.g. 23:00 to 02:00)
		return curMins >= startMins || curMins <= endMins

	case "weekly":
		if tInLoc.Weekday() != startInLoc.Weekday() {
			return false
		}
		curMins := tInLoc.Hour()*60 + tInLoc.Minute()
		startMins := startInLoc.Hour()*60 + startInLoc.Minute()
		endMins := endInLoc.Hour()*60 + endInLoc.Minute()

		if startMins <= endMins {
			return curMins >= startMins && curMins <= endMins
		}
		return curMins >= startMins || curMins <= endMins

	default: // "none" or one-off
		return !t.Before(w.StartTime) && !t.After(w.EndTime)
	}
}

// MaintenanceStore provides CRUD and evaluation for maintenance windows.
type MaintenanceStore interface {
	CreateMaintenanceWindow(ctx context.Context, w MaintenanceWindow) (MaintenanceWindow, error)
	GetMaintenanceWindow(ctx context.Context, id int64) (MaintenanceWindow, error)
	UpdateMaintenanceWindow(ctx context.Context, w MaintenanceWindow) (MaintenanceWindow, error)
	DeleteMaintenanceWindow(ctx context.Context, id int64) error
	ListMaintenanceWindows(ctx context.Context, scope, targetID string) ([]MaintenanceWindow, error)
	IsUnderMaintenance(ctx context.Context, contractID string, groupID string, t time.Time) (*MaintenanceWindow, bool, error)
}

func (s *postgresStore) CreateMaintenanceWindow(ctx context.Context, w MaintenanceWindow) (MaintenanceWindow, error) {
	if w.Name == "" || w.StartTime.IsZero() || w.EndTime.IsZero() || !w.EndTime.After(w.StartTime) {
		return MaintenanceWindow{}, ErrInvalidMaintenanceWindow
	}
	if w.Scope != "global" && w.Scope != "contract" && w.Scope != "group" {
		return MaintenanceWindow{}, ErrInvalidMaintenanceWindow
	}
	if (w.Scope == "contract" || w.Scope == "group") && w.TargetID == "" {
		return MaintenanceWindow{}, ErrInvalidMaintenanceWindow
	}
	if w.Timezone == "" {
		w.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(w.Timezone); err != nil {
		return MaintenanceWindow{}, fmt.Errorf("%w: invalid timezone %s", ErrInvalidMaintenanceWindow, w.Timezone)
	}
	if w.Recurrence == "" {
		w.Recurrence = "none"
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO maintenance_windows (
			name, scope, target_id, start_time, end_time, timezone, recurrence,
			suppress_notifications, created_by, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id, name, scope, COALESCE(target_id, ''), start_time, end_time, timezone,
		          recurrence, suppress_notifications, created_by, created_at, updated_at`,
		w.Name, w.Scope, w.TargetID, w.StartTime, w.EndTime, w.Timezone,
		w.Recurrence, w.SuppressNotifications, w.CreatedBy)

	var out MaintenanceWindow
	err := row.Scan(&out.ID, &out.Name, &out.Scope, &out.TargetID, &out.StartTime,
		&out.EndTime, &out.Timezone, &out.Recurrence, &out.SuppressNotifications,
		&out.CreatedBy, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (s *postgresStore) GetMaintenanceWindow(ctx context.Context, id int64) (MaintenanceWindow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, scope, COALESCE(target_id, ''), start_time, end_time, timezone,
		       recurrence, suppress_notifications, created_by, created_at, updated_at
		FROM maintenance_windows
		WHERE id = $1`, id)

	var out MaintenanceWindow
	err := row.Scan(&out.ID, &out.Name, &out.Scope, &out.TargetID, &out.StartTime,
		&out.EndTime, &out.Timezone, &out.Recurrence, &out.SuppressNotifications,
		&out.CreatedBy, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MaintenanceWindow{}, ErrMaintenanceWindowNotFound
	}
	return out, err
}

func (s *postgresStore) UpdateMaintenanceWindow(ctx context.Context, w MaintenanceWindow) (MaintenanceWindow, error) {
	if w.ID == 0 || w.Name == "" || !w.EndTime.After(w.StartTime) {
		return MaintenanceWindow{}, ErrInvalidMaintenanceWindow
	}
	if w.Timezone == "" {
		w.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(w.Timezone); err != nil {
		return MaintenanceWindow{}, fmt.Errorf("%w: invalid timezone %s", ErrInvalidMaintenanceWindow, w.Timezone)
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE maintenance_windows
		SET name = $2,
		    start_time = $3,
		    end_time = $4,
		    timezone = $5,
		    recurrence = $6,
		    suppress_notifications = $7,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, scope, COALESCE(target_id, ''), start_time, end_time, timezone,
		          recurrence, suppress_notifications, created_by, created_at, updated_at`,
		w.ID, w.Name, w.StartTime, w.EndTime, w.Timezone, w.Recurrence, w.SuppressNotifications)

	var out MaintenanceWindow
	err := row.Scan(&out.ID, &out.Name, &out.Scope, &out.TargetID, &out.StartTime,
		&out.EndTime, &out.Timezone, &out.Recurrence, &out.SuppressNotifications,
		&out.CreatedBy, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MaintenanceWindow{}, ErrMaintenanceWindowNotFound
	}
	return out, err
}

func (s *postgresStore) DeleteMaintenanceWindow(ctx context.Context, id int64) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM maintenance_windows WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrMaintenanceWindowNotFound
	}
	return nil
}

func (s *postgresStore) ListMaintenanceWindows(ctx context.Context, scope, targetID string) ([]MaintenanceWindow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, scope, COALESCE(target_id, ''), start_time, end_time, timezone,
		       recurrence, suppress_notifications, created_by, created_at, updated_at
		FROM maintenance_windows
		WHERE ($1 = '' OR scope = $1)
		  AND ($2 = '' OR target_id = $2)
		ORDER BY start_time DESC`, scope, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MaintenanceWindow
	for rows.Next() {
		var w MaintenanceWindow
		if err := rows.Scan(&w.ID, &w.Name, &w.Scope, &w.TargetID, &w.StartTime,
			&w.EndTime, &w.Timezone, &w.Recurrence, &w.SuppressNotifications,
			&w.CreatedBy, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *postgresStore) IsUnderMaintenance(ctx context.Context, contractID string, groupID string, t time.Time) (*MaintenanceWindow, bool, error) {
	if t.IsZero() {
		t = time.Now().UTC()
	}

	windows, err := s.ListMaintenanceWindows(ctx, "", "")
	if err != nil {
		return nil, false, err
	}

	for _, w := range windows {
		// Scope matching
		matchesScope := false
		switch w.Scope {
		case "global":
			matchesScope = true
		case "contract":
			if contractID != "" && w.TargetID == contractID {
				matchesScope = true
			}
		case "group":
			if groupID != "" && w.TargetID == groupID {
				matchesScope = true
			}
		}

		if matchesScope && w.IsActive(t) {
			win := w
			return &win, win.SuppressNotifications, nil
		}
	}

	return nil, false, nil
}
