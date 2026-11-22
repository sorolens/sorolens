package store

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockExportJobStore provides an in-memory implementation of ExportJobStore for tests.
type MockExportJobStore struct {
	mu   sync.RWMutex
	jobs map[string]ExportJob
	idGen int
}

// NewMockExportJobStore creates a new MockExportJobStore.
func NewMockExportJobStore() *MockExportJobStore {
	return &MockExportJobStore{
		jobs: make(map[string]ExportJob),
	}
}

func (m *MockExportJobStore) CreateExportJob(_ context.Context, job ExportJob) (ExportJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.idGen++
	if job.ID == "" {
		job.ID = fmt.Sprintf("job-%d", m.idGen)
	}
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now
	m.jobs[job.ID] = job
	return job, nil
}

func (m *MockExportJobStore) GetExportJob(_ context.Context, id string) (ExportJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	j, ok := m.jobs[id]
	if !ok {
		return ExportJob{}, ErrNotFound
	}
	return j, nil
}

func (m *MockExportJobStore) UpdateExportJob(_ context.Context, job ExportJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.jobs[job.ID]
	if !ok {
		return ErrNotFound
	}
	job.CreatedAt = existing.CreatedAt
	job.UpdatedAt = time.Now().UTC()
	m.jobs[job.ID] = job
	return nil
}

func (m *MockExportJobStore) DeleteExportJob(_ context.Context, id string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.jobs[id]
	if !ok || existing.UserID != userID {
		return ErrNotFound
	}
	delete(m.jobs, id)
	return nil
}

func (m *MockExportJobStore) ListActiveExportJobs(_ context.Context, userID string) ([]ExportJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []ExportJob
	for _, j := range m.jobs {
		if j.UserID == userID && (j.Status == ExportJobStatusPending || j.Status == ExportJobStatusProcessing) {
			active = append(active, j)
		}
	}
	return active, nil
}
