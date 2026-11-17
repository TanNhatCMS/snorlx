package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"snorlx/backend/internal/models"

	gh "github.com/google/go-github/v93/github"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

// The log archiver downloads each completed job's log once and stores it in the storage
// backend, so dashboard history outlives GitHub's own 90-day log retention. A run triggers
// archiving from three places: a workflow_run completed webhook, a workflow_job completed
// webhook, and the live poller (or discovery pass) when a run turns completed. Content is
// stored gzip-compressed; job logs are repetitive text, so the ratio is around 10:1.

const (
	// archiveTimeout bounds one asynchronous archive pass for a run.
	archiveTimeout = 10 * time.Minute
	// logDownloadTimeout bounds the download of one job's log body.
	logDownloadTimeout = 2 * time.Minute
)

// logDownloadClient fetches signed log URLs; no GitHub authentication is needed for them.
var logDownloadClient = &http.Client{Timeout: logDownloadTimeout}

// compressLog gzips b.
func compressLog(b []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(b)
	_ = w.Close()
	return buf.Bytes()
}

// decompressLog reverses compressLog.
func decompressLog(b []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// downloadLog fetches a signed log URL and enforces the configured size cap.
func (h *Handler) downloadLog(ctx context.Context, url string, maxSize int) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := logDownloadClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("log download returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxSize)+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxSize {
		return nil, fmt.Errorf("job log exceeds LOG_ARCHIVE_MAX_SIZE_BYTES (%d > %d)", len(body), maxSize)
	}
	return body, nil
}

// fetchAndStoreJobLog archives one job's log unless it is already stored. It reports whether
// the log was actually stored (false for already-archived or not-yet-completed jobs).
func (h *Handler) fetchAndStoreJobLog(ctx context.Context, client *gh.Client, owner, repoName string, job models.WorkflowJob) (bool, error) {
	if exists, err := h.storage.HasJobLog(ctx, job.ID); err != nil {
		return false, err
	} else if exists {
		return false, nil
	}
	if job.Status != "completed" {
		return false, nil
	}
	logsURL, err := h.ghClient.GetWorkflowJobLogs(ctx, client, owner, repoName, job.GitHubID)
	if err != nil {
		return false, fmt.Errorf("fetch log URL for job %d: %w", job.GitHubID, err)
	}
	body, err := h.downloadLog(ctx, logsURL, h.config.LogArchiveMaxSizeBytes)
	if err != nil {
		return false, fmt.Errorf("download log for job %d: %w", job.GitHubID, err)
	}
	if err := h.storage.SaveJobLog(ctx, &models.JobLog{
		JobID:     job.ID,
		RunID:     job.RunID,
		Content:   compressLog(body),
		SizeBytes: len(body),
		FetchedAt: time.Now(),
	}); err != nil {
		return false, err
	}
	return true, nil
}

// archiveRunJobs archives the logs of every completed job of the run that is not stored yet.
// Per-job failures are logged and skipped so one broken job cannot block the others.
func (h *Handler) archiveRunJobs(ctx context.Context, client *gh.Client, owner, repoName string, run models.WorkflowRun) error {
	jobs, err := h.storage.ListJobsForRun(ctx, run.ID)
	if err != nil {
		return fmt.Errorf("list jobs for run %d: %w", run.GitHubID, err)
	}
	archived := 0
	for _, job := range jobs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		stored, err := h.fetchAndStoreJobLog(ctx, client, owner, repoName, job)
		if err != nil {
			log.Warn().Err(err).Int64("run_github_id", run.GitHubID).Int64("job_github_id", job.GitHubID).
				Msg("Log archive: failed to archive job log")
			continue
		}
		if stored {
			archived++
		}
	}
	if archived > 0 {
		log.Info().Int64("run_github_id", run.GitHubID).Int("archived", archived).
			Msg("Log archive: stored job logs")
	}
	return nil
}

// archiveClientForRepo builds a GitHub client from the token of a user who can see the repo,
// the same trust model the live poller uses.
func (h *Handler) archiveClientForRepo(ctx context.Context, repoID int) (*gh.Client, bool) {
	userIDs, err := h.storage.ListUsersWithRepositoryAccess(ctx, repoID)
	if err != nil || len(userIDs) == 0 {
		return nil, false
	}
	for _, id := range userIDs {
		user, err := h.storage.GetUserByID(ctx, id)
		if err != nil || user == nil || user.AccessToken == "" {
			continue
		}
		client := h.ghClient.GetUserClient(ctx, &oauth2.Token{AccessToken: user.AccessToken})
		if client != nil {
			return client, true
		}
	}
	return nil, false
}

// archiveRunJobsAsync archives a run's job logs in the background. It runs off the request
// context: webhook deliveries return immediately and poller passes must not block.
func (h *Handler) archiveRunJobsAsync(repoFullName string, run models.WorkflowRun) {
	if !h.config.LogArchiveEnabled {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), archiveTimeout)
		defer cancel()
		owner, repoName, ok := splitFullName(repoFullName)
		if !ok {
			return
		}
		client, ok := h.archiveClientForRepo(ctx, run.RepoID)
		if !ok {
			log.Debug().Int64("run_github_id", run.GitHubID).
				Msg("Log archive: no usable token for repository, skipping")
			return
		}
		if err := h.archiveRunJobs(ctx, client, owner, repoName, run); err != nil {
			log.Warn().Err(err).Int64("run_github_id", run.GitHubID).
				Msg("Log archive: run pass failed")
		}
	}()
}
