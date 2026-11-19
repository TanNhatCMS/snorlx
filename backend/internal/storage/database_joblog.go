package storage

import (
	"context"
	"errors"

	"snorlx/backend/internal/models"
)

// ===== Job log archive (database) =====

// SaveJobLog stores or overwrites the archived log of one job. Content is gzip-compressed by
// the archiver before it reaches the storage layer.
func (d *DatabaseStorage) SaveJobLog(ctx context.Context, entry *models.JobLog) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO job_logs (job_id, run_id, content, size_bytes, fetched_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (job_id) DO UPDATE SET
			run_id = EXCLUDED.run_id,
			content = EXCLUDED.content,
			size_bytes = EXCLUDED.size_bytes,
			fetched_at = EXCLUDED.fetched_at
	`, entry.JobID, entry.RunID, entry.Content, entry.SizeBytes, entry.FetchedAt)
	if err != nil {
		return err
	}
	return nil
}

// HasJobLog reports whether an archived log exists for the job.
func (d *DatabaseStorage) HasJobLog(ctx context.Context, jobID int) (bool, error) {
	var exists bool
	err := d.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM job_logs WHERE job_id = $1)`, jobID).Scan(&exists)
	return exists, err
}

// GetJobLog returns the archived log of one job.
func (d *DatabaseStorage) GetJobLog(ctx context.Context, jobID int) (*models.JobLog, error) {
	var entry models.JobLog
	err := d.pool.QueryRow(ctx, `
		SELECT job_id, run_id, content, size_bytes, fetched_at
		FROM job_logs WHERE job_id = $1
	`, jobID).Scan(&entry.JobID, &entry.RunID, &entry.Content, &entry.SizeBytes, &entry.FetchedAt)
	if err != nil {
		return nil, errors.New("job log not found")
	}
	return &entry, nil
}
