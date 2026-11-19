package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"snorlx/backend/internal/github"
	"snorlx/backend/internal/models"

	"github.com/go-chi/chi/v5"
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
		// Bọc status để caller (backfill) nhận ra 403/429 rate limit và dừng cả pass,
		// giống hệt lỗi 403/429 ở bước lấy URL log.
		return nil, &github.StatusCodeError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("log download returned status %d", resp.StatusCode),
		}
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

// ===== HTTP endpoints =====

const (
	// backfillRetentionDays matches GitHub's longest log retention window: logs of runs older
	// than this are already gone from GitHub and cannot be recovered.
	backfillRetentionDays = 90
	// backfillMaxRuns bounds one backfill call so a huge repository cannot hammer the GitHub
	// API within a single request; call the endpoint again for the next slice.
	backfillMaxRuns = 500
	// backfillPageSize is how many runs one ListRuns page holds.
	backfillPageSize = 50
)

// defaultBackfillMaxLogFetches caps the log-fetch attempts one backfill call spends, so a
// first backfill cannot exhaust the calling user's hourly GitHub core quota (which also
// feeds the live poller). Tests override it through the Handler field.
const defaultBackfillMaxLogFetches = 500

// GetJobLogContent serves an archived job log as plain text. Auth and access checks mirror
// GetJobLogs so archived and live logs are exposed identically.
func (h *Handler) GetJobLogContent(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	user := h.getUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	job, err := h.storage.GetJob(r.Context(), id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if _, ok := h.loadAccessibleRun(w, r, user, job.RunID); !ok {
		return
	}
	entry, err := h.storage.GetJobLog(r.Context(), job.ID)
	if err != nil {
		http.Error(w, "Log not archived", http.StatusNotFound)
		return
	}
	body, err := decompressLog(entry.Content)
	if err != nil {
		log.Error().Err(err).Int("job_id", job.ID).Msg("Log archive: corrupt stored log")
		http.Error(w, "Corrupt archived log", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(body)
}

// BackfillRepositoryLogs archives the job logs of the repository's completed runs within
// GitHub's retention window. It is safe to call repeatedly: already archived jobs are skipped,
// and one call processes at most backfillMaxRuns runs.
func (h *Handler) BackfillRepositoryLogs(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	user := h.getUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	repo, err := h.storage.GetRepository(r.Context(), id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if ok, err := h.storage.HasRepositoryAccess(r.Context(), user.ID, id); err != nil || !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	owner, repoName, ok := splitFullName(repo.FullName)
	if !ok {
		http.Error(w, "Invalid repository name", http.StatusInternalServerError)
		return
	}
	client := h.ghClient.GetUserClient(r.Context(), &oauth2.Token{AccessToken: user.AccessToken})
	if client == nil {
		http.Error(w, "Failed to build GitHub client", http.StatusInternalServerError)
		return
	}

	maxFetches := h.backfillMaxLogFetches
	if maxFetches <= 0 {
		maxFetches = defaultBackfillMaxLogFetches
	}
	archived, failed, scanned, fetches := 0, 0, 0, 0
	var rateLimited bool
	filters := &models.RunFilters{RepoID: id, StartDate: time.Now().AddDate(0, 0, -backfillRetentionDays)}
backfill:
	for page := 1; scanned < backfillMaxRuns; page++ {
		runs, total, err := h.storage.ListRuns(r.Context(), user.ID, filters, page, backfillPageSize)
		if err != nil {
			http.Error(w, "Failed to list runs", http.StatusInternalServerError)
			return
		}
		if len(runs) == 0 {
			break
		}
		for i := range runs {
			run := runs[i]
			if run.Status != "completed" {
				continue
			}
			scanned++
			jobs, err := h.storage.ListJobsForRun(r.Context(), run.ID)
			if err != nil {
				failed++
				log.Warn().Err(err).Int64("run_github_id", run.GitHubID).
					Msg("Log archive: backfill could not list jobs")
				continue
			}
			for _, job := range jobs {
				if fetches >= maxFetches {
					break backfill
				}
				fetches++
				stored, err := h.fetchAndStoreJobLog(r.Context(), client, owner, repoName, job)
				if err != nil {
					// A rate-limit response means every further attempt would fail too and
					// starve the live poller's token; stop the whole pass. Log-fetch errors are
					// plain wrapper errors, so the status comes via StatusCodeError.
					if github.IsRateLimit(err) {
						rateLimited = true
						failed++
						log.Warn().Int64("job_github_id", job.GitHubID).
							Msg("Log archive: backfill stopped, GitHub rate limit reached")
						break backfill
					}
					failed++
					log.Warn().Err(err).Int64("job_github_id", job.GitHubID).
						Msg("Log archive: backfill failed to archive job log")
					continue
				}
				if stored {
					archived++
				}
			}
			if scanned >= backfillMaxRuns || rateLimited {
				break
			}
		}
		if page*backfillPageSize >= total {
			break
		}
	}
	log.Info().Int("repo_id", id).Int("archived", archived).Int("failed", failed).
		Int("scanned", scanned).Bool("rate_limited", rateLimited).Msg("Log archive: backfill pass finished")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"archived":     archived,
		"failed":       failed,
		"scanned":      scanned,
		"rate_limited": rateLimited,
	})
}
