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

// ListJobsMissingLogs returns completed jobs of repoID whose log has not been archived yet,
// newest completion first.
func (d *DatabaseStorage) ListJobsMissingLogs(ctx context.Context, repoID int, limit int) ([]models.WorkflowJob, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT j.id, j.github_id, j.run_id, j.name, j.status, j.conclusion, j.started_at, j.completed_at, j.duration_seconds
		FROM workflow_jobs j
		JOIN workflow_runs r ON r.id = j.run_id
		LEFT JOIN job_logs l ON l.job_id = j.id
		WHERE r.repo_id = $1 AND j.status = 'completed' AND j.completed_at IS NOT NULL AND l.job_id IS NULL
		ORDER BY j.completed_at DESC
		LIMIT $2
	`, repoID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]models.WorkflowJob, 0, 16)
	for rows.Next() {
		var job models.WorkflowJob
		if err := rows.Scan(&job.ID, &job.GitHubID, &job.RunID, &job.Name, &job.Status,
			&job.Conclusion, &job.StartedAt, &job.CompletedAt, &job.DurationSeconds); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}
