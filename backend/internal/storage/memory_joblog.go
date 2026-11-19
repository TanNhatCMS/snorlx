package storage

import (
	"context"
	"errors"

	"snorlx/backend/internal/models"
)

// ===== Job log archive (memory) =====

// jobLogs lives on MemoryStorage but is kept in its own file to leave memory.go untouched.
// The zero map is created lazily so tests and callers that never archive keep the original
// memory footprint.

func (m *MemoryStorage) ensureJobLogsLocked() {
	if m.jobLogs == nil {
		m.jobLogs = make(map[int]*models.JobLog)
	}
}

// SaveJobLog stores or overwrites the archived log of one job.
func (m *MemoryStorage) SaveJobLog(ctx context.Context, entry *models.JobLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureJobLogsLocked()
	copy := *entry
	m.jobLogs[entry.JobID] = &copy
	return nil
}

// HasJobLog reports whether an archived log exists for the job.
func (m *MemoryStorage) HasJobLog(ctx context.Context, jobID int) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.jobLogs[jobID]
	return ok, nil
}

// GetJobLog returns the archived log of one job.
func (m *MemoryStorage) GetJobLog(ctx context.Context, jobID int) (*models.JobLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.jobLogs[jobID]
	if !ok {
		return nil, errors.New("job log not found")
	}
	copy := *entry
	return &copy, nil
}
