package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ExportJobStatus represents the state of an asynchronous export job.
type ExportJobStatus string

const (
	ExportJobStatusPending    ExportJobStatus = "pending"
	ExportJobStatusProcessing ExportJobStatus = "processing"
	ExportJobStatusCompleted  ExportJobStatus = "completed"
	ExportJobStatusFailed     ExportJobStatus = "failed"
)

// ExportJob represents a background export request and its state (issue #785).
type ExportJob struct {
	ID            string          `json:"id"`
	UserID        string          `json:"user_id"`
	Type          string          `json:"type"`   // "events" or "snapshot"
	Format        string          `json:"format"` // "csv" or "ndjson"
	Filters       json.RawMessage `json:"filters"`
	Status        ExportJobStatus `json:"status"`
	Progress      int             `json:"progress"` // 0..100
	ErrorMessage  *string         `json:"error_message,omitempty"`
	ObjectKey     *string         `json:"object_key,omitempty"`
	DownloadURL   *string         `json:"download_url,omitempty"`
	DownloadToken *string         `json:"download_token,omitempty"`
	ExpiresAt     *time.Time      `json:"expires_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// ExportJobStore is the persistence surface for asynchronous export jobs.
type ExportJobStore interface {
	CreateExportJob(ctx context.Context, job ExportJob) (ExportJob, error)
	GetExportJob(ctx context.Context, id string) (ExportJob, error)
	UpdateExportJob(ctx context.Context, job ExportJob) error
	DeleteExportJob(ctx context.Context, id string, userID string) error
	ListActiveExportJobs(ctx context.Context, userID string) ([]ExportJob, error)
}

// scanExportJob scans one export_jobs row into an ExportJob struct.
func scanExportJob(row pgx.Row) (ExportJob, error) {
	var j ExportJob
	var filters []byte
	var statusStr string

	err := row.Scan(
		&j.ID,
		&j.UserID,
		&j.Type,
		&j.Format,
		&filters,
		&statusStr,
		&j.Progress,
		&j.ErrorMessage,
		&j.ObjectKey,
		&j.DownloadURL,
		&j.DownloadToken,
		&j.ExpiresAt,
		&j.CreatedAt,
		&j.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExportJob{}, ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
			return ExportJob{}, ErrNotFound
		}
		return ExportJob{}, err
	}
	j.Status = ExportJobStatus(statusStr)
	j.Filters = filters
	return j, nil
}

// CreateExportJob inserts a new export job into Postgres.
func (s *postgresStore) CreateExportJob(ctx context.Context, job ExportJob) (ExportJob, error) {
	filtersJSON := job.Filters
	if len(filtersJSON) == 0 {
		filtersJSON = []byte("{}")
	}

	query := `
		INSERT INTO export_jobs (
			user_id, type, format, filters, status, progress, error_message,
			object_key, download_url, download_token, expires_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW()
		)
		RETURNING id, user_id, type, format, filters, status, progress, error_message,
		          object_key, download_url, download_token, expires_at, created_at, updated_at
	`

	row := s.pool.QueryRow(ctx, query,
		job.UserID,
		job.Type,
		job.Format,
		filtersJSON,
		string(job.Status),
		job.Progress,
		job.ErrorMessage,
		job.ObjectKey,
		job.DownloadURL,
		job.DownloadToken,
		job.ExpiresAt,
	)

	return scanExportJob(row)
}

// GetExportJob returns the export job by ID.
func (s *postgresStore) GetExportJob(ctx context.Context, id string) (ExportJob, error) {
	query := `
		SELECT id, user_id, type, format, filters, status, progress, error_message,
		       object_key, download_url, download_token, expires_at, created_at, updated_at
		FROM export_jobs
		WHERE id = $1
	`
	return scanExportJob(s.pool.QueryRow(ctx, query, id))
}

// UpdateExportJob updates an existing export job's state, progress, or output.
func (s *postgresStore) UpdateExportJob(ctx context.Context, job ExportJob) error {
	query := `
		UPDATE export_jobs
		SET status = $2,
		    progress = $3,
		    error_message = $4,
		    object_key = $5,
		    download_url = $6,
		    download_token = $7,
		    expires_at = $8,
		    updated_at = NOW()
		WHERE id = $1
	`
	cmd, err := s.pool.Exec(ctx, query,
		job.ID,
		string(job.Status),
		job.Progress,
		job.ErrorMessage,
		job.ObjectKey,
		job.DownloadURL,
		job.DownloadToken,
		job.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("update export job: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteExportJob removes an export job for a given user.
func (s *postgresStore) DeleteExportJob(ctx context.Context, id string, userID string) error {
	query := `DELETE FROM export_jobs WHERE id = $1 AND user_id = $2`
	cmd, err := s.pool.Exec(ctx, query, id, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
			return ErrNotFound
		}
		return fmt.Errorf("delete export job: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListActiveExportJobs returns all pending or processing jobs for a user.
func (s *postgresStore) ListActiveExportJobs(ctx context.Context, userID string) ([]ExportJob, error) {
	query := `
		SELECT id, user_id, type, format, filters, status, progress, error_message,
		       object_key, download_url, download_token, expires_at, created_at, updated_at
		FROM export_jobs
		WHERE user_id = $1 AND status IN ('pending', 'processing')
		ORDER BY created_at ASC
	`
	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list active export jobs: %w", err)
	}
	defer rows.Close()

	var jobs []ExportJob
	for rows.Next() {
		j, err := scanExportJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
