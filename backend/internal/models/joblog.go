package models

import "time"

// JobLog is an archived copy of one workflow job's log. Content is gzip-compressed
// so large job logs stay affordable to keep forever in the database.
type JobLog struct {
	JobID     int       `json:"job_id"`
	RunID     int       `json:"run_id"`
	Content   []byte    `json:"-"`
	SizeBytes int       `json:"size_bytes"`
	FetchedAt time.Time `json:"fetched_at"`
}
