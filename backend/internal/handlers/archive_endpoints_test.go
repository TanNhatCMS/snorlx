package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"snorlx/backend/internal/config"
	"snorlx/backend/internal/github"
	"snorlx/backend/internal/models"
)

// ===== GetJobLogs local-first + content endpoint =====

func TestGetJobLogs_ReturnsLocalContentURL(t *testing.T) {
	store := &mockStorage{
		getJobFunc: func(ctx context.Context, id int) (*models.WorkflowJob, error) {
			return &models.WorkflowJob{ID: id, GitHubID: 77, RunID: 5}, nil
		},
		hasJobLogFunc: func(ctx context.Context, jobID int) (bool, error) { return true, nil },
		getRunFunc: func(ctx context.Context, id int) (*models.WorkflowRun, error) {
			return &models.WorkflowRun{ID: 5, RepoID: 7}, nil
		},
		hasRepoAccessFunc: func(ctx context.Context, userID, repoID int) (bool, error) { return true, nil },
	}
	h := newTestHandler(store)

	req := withURLParam(withUser(httptest.NewRequest(http.MethodGet, "/api/jobs/42/logs", nil), &models.User{ID: 2, AccessToken: "t"}), "id", "42")
	rec := httptest.NewRecorder()
	h.GetJobLogs(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var out map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if out["url"] != "/api/jobs/42/logs/content" {
		t.Fatalf("url = %q, want /api/jobs/42/logs/content", out["url"])
	}
}

func TestGetJobLogs_FallsBackToGitHubWhenNotArchived(t *testing.T) {
	var logsRequests atomic.Int64
	server := newArchiveTestServer(t, "github log", &logsRequests)
	defer server.Close()

	ghClient, err := github.NewClient(&config.Config{GitHubClientID: "id", GitHubClientSecret: "secret", GitHubBaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	store := &mockStorage{
		getJobFunc: func(ctx context.Context, id int) (*models.WorkflowJob, error) {
			return &models.WorkflowJob{ID: id, GitHubID: 77, RunID: 5}, nil
		},
		hasJobLogFunc: func(ctx context.Context, jobID int) (bool, error) { return false, nil },
		getRunFunc: func(ctx context.Context, id int) (*models.WorkflowRun, error) {
			return &models.WorkflowRun{ID: 5, RepoID: 7}, nil
		},
		hasRepoAccessFunc: func(ctx context.Context, userID, repoID int) (bool, error) { return true, nil },
		getRepositoryFunc: func(ctx context.Context, id int) (*models.Repository, error) {
			return &models.Repository{ID: id, FullName: "acme/widgets"}, nil
		},
	}
	h := newTestHandler(store)
	h.ghClient = ghClient

	req := withURLParam(withUser(httptest.NewRequest(http.MethodGet, "/api/jobs/42/logs", nil), &models.User{ID: 2, AccessToken: "t"}), "id", "42")
	rec := httptest.NewRecorder()
	h.GetJobLogs(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if !strings.HasPrefix(out["url"], "http") || !strings.Contains(out["url"], "/signed/77") {
		t.Fatalf("expected GitHub signed URL fallback, got %q", out["url"])
	}
}

func TestGetJobLogContent_ServesDecompressedLog(t *testing.T) {
	content := []byte("line one\nline two\n")
	store := &mockStorage{
		getJobFunc: func(ctx context.Context, id int) (*models.WorkflowJob, error) {
			return &models.WorkflowJob{ID: id, GitHubID: 77, RunID: 5}, nil
		},
		getJobLogFunc: func(ctx context.Context, jobID int) (*models.JobLog, error) {
			return &models.JobLog{JobID: jobID, RunID: 5, Content: compressLog(content), SizeBytes: len(content), FetchedAt: time.Now()}, nil
		},
		getRunFunc: func(ctx context.Context, id int) (*models.WorkflowRun, error) {
			return &models.WorkflowRun{ID: 5, RepoID: 7}, nil
		},
		hasRepoAccessFunc: func(ctx context.Context, userID, repoID int) (bool, error) { return true, nil },
	}
	h := newTestHandler(store)

	req := withURLParam(withUser(httptest.NewRequest(http.MethodGet, "/api/jobs/42/logs/content", nil), &models.User{ID: 2, AccessToken: "t"}), "id", "42")
	rec := httptest.NewRecorder()
	h.GetJobLogContent(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	if rec.Body.String() != string(content) {
		t.Fatalf("body = %q, want %q", rec.Body.String(), string(content))
	}
}

// ===== Backfill =====

func TestBackfillRepositoryLogs_ArchivesMissingLogs(t *testing.T) {
	var logsRequests atomic.Int64
	server := newArchiveTestServer(t, "backfill log", &logsRequests)
	defer server.Close()

	ghClient, err := github.NewClient(&config.Config{GitHubClientID: "id", GitHubClientSecret: "secret", GitHubBaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	archived := make(map[int]bool)
	store := &mockStorage{
		getRepositoryFunc: func(ctx context.Context, id int) (*models.Repository, error) {
			return &models.Repository{ID: id, FullName: "acme/widgets"}, nil
		},
		hasRepoAccessFunc: func(ctx context.Context, userID, repoID int) (bool, error) { return true, nil },
		listRunsFunc: func(ctx context.Context, userID int, filters *models.RunFilters, page, pageSize int) ([]models.WorkflowRun, int, error) {
			if filters == nil || filters.RepoID != 7 {
				t.Errorf("backfill must filter by repo, got %+v", filters)
			}
			return []models.WorkflowRun{{ID: 5, GitHubID: 555, RepoID: 7, Status: "completed"}}, 1, nil
		},
		listJobsForRunFunc: func(ctx context.Context, runID int) ([]models.WorkflowJob, error) {
			return []models.WorkflowJob{
				{ID: 1, GitHubID: 101, RunID: runID, Status: "completed"},
				{ID: 2, GitHubID: 102, RunID: runID, Status: "completed"},
				{ID: 3, GitHubID: 103, RunID: runID, Status: "queued"},
			}, nil
		},
		hasJobLogFunc: func(ctx context.Context, jobID int) (bool, error) { return archived[jobID], nil },
		saveJobLogFunc: func(ctx context.Context, entry *models.JobLog) error {
			archived[entry.JobID] = true
			return nil
		},
	}
	h := newTestHandler(store)
	h.ghClient = ghClient
	h.config.LogArchiveMaxSizeBytes = 1 << 20

	req := withURLParam(withUser(httptest.NewRequest(http.MethodPost, "/api/repositories/7/backfill-logs", nil), &models.User{ID: 2, AccessToken: "t"}), "id", "7")
	rec := httptest.NewRecorder()
	h.BackfillRepositoryLogs(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Archived int `json:"archived"`
		Failed   int `json:"failed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if out.Archived != 2 || out.Failed != 0 {
		t.Fatalf("archived=%d failed=%d, want 2/0", out.Archived, out.Failed)
	}
	if !archived[1] || !archived[2] || archived[3] {
		t.Errorf("completed jobs must be archived, queued job must not: %v", archived)
	}
}

// One backfill call must not spend the calling user's whole hourly GitHub core quota: total
// log-fetch attempts are capped, and a rate-limit response stops the pass at once.
func TestBackfillRepositoryLogs_CapsLogFetches(t *testing.T) {
	var logsRequests atomic.Int64
	server := newArchiveTestServer(t, "log", &logsRequests)
	defer server.Close()

	ghClient, err := github.NewClient(&config.Config{GitHubClientID: "id", GitHubClientSecret: "secret", GitHubBaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	archived := make(map[int]bool)
	store := &mockStorage{
		getRepositoryFunc: func(ctx context.Context, id int) (*models.Repository, error) {
			return &models.Repository{ID: id, FullName: "acme/widgets"}, nil
		},
		hasRepoAccessFunc: func(ctx context.Context, userID, repoID int) (bool, error) { return true, nil },
		listRunsFunc: func(ctx context.Context, userID int, filters *models.RunFilters, page, pageSize int) ([]models.WorkflowRun, int, error) {
			return []models.WorkflowRun{{ID: 5, GitHubID: 555, RepoID: 7, Status: "completed"}}, 1, nil
		},
		listJobsForRunFunc: func(ctx context.Context, runID int) ([]models.WorkflowJob, error) {
			return []models.WorkflowJob{
				{ID: 1, GitHubID: 101, RunID: runID, Status: "completed"},
				{ID: 2, GitHubID: 102, RunID: runID, Status: "completed"},
				{ID: 3, GitHubID: 103, RunID: runID, Status: "completed"},
			}, nil
		},
		hasJobLogFunc:  func(ctx context.Context, jobID int) (bool, error) { return archived[jobID], nil },
		saveJobLogFunc: func(ctx context.Context, entry *models.JobLog) error { archived[entry.JobID] = true; return nil },
	}
	h := newTestHandler(store)
	h.backfillMaxLogFetches = 2
	h.ghClient = ghClient
	h.config.LogArchiveMaxSizeBytes = 1 << 20

	req := withURLParam(withUser(httptest.NewRequest(http.MethodPost, "/api/repositories/7/backfill-logs", nil), &models.User{ID: 2, AccessToken: "t"}), "id", "7")
	rec := httptest.NewRecorder()
	h.BackfillRepositoryLogs(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := logsRequests.Load(); got != 2 {
		t.Fatalf("log fetches = %d, want 2 (cap)", got)
	}
	if !archived[1] || !archived[2] || archived[3] {
		t.Fatalf("cap must stop after the second job: %v", archived)
	}
}

func TestBackfillRepositoryLogs_StopsOnRateLimit(t *testing.T) {
	var logsRequests atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/api/v3")
		if strings.HasPrefix(p, "/repos/acme/widgets/actions/jobs/") && strings.HasSuffix(p, "/logs") {
			if logsRequests.Add(1) >= 2 {
				// Second attempt: GitHub says the core quota is exhausted.
				w.Header().Set("X-RateLimit-Limit", "5000")
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", "1893456000")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
				return
			}
			jobID := strings.TrimSuffix(strings.TrimPrefix(p, "/repos/acme/widgets/actions/jobs/"), "/logs")
			http.Redirect(w, r, "http://"+r.Host+"/signed/"+jobID, http.StatusFound)
			return
		}
		if strings.HasPrefix(p, "/signed/") {
			_, _ = w.Write([]byte("rate limit log"))
			return
		}
		http.NotFound(w, r)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	ghClient, err := github.NewClient(&config.Config{GitHubClientID: "id", GitHubClientSecret: "secret", GitHubBaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	archived := make(map[int]bool)
	store := &mockStorage{
		getRepositoryFunc: func(ctx context.Context, id int) (*models.Repository, error) {
			return &models.Repository{ID: id, FullName: "acme/widgets"}, nil
		},
		hasRepoAccessFunc: func(ctx context.Context, userID, repoID int) (bool, error) { return true, nil },
		listRunsFunc: func(ctx context.Context, userID int, filters *models.RunFilters, page, pageSize int) ([]models.WorkflowRun, int, error) {
			return []models.WorkflowRun{{ID: 5, GitHubID: 555, RepoID: 7, Status: "completed"}}, 1, nil
		},
		listJobsForRunFunc: func(ctx context.Context, runID int) ([]models.WorkflowJob, error) {
			return []models.WorkflowJob{
				{ID: 1, GitHubID: 101, RunID: runID, Status: "completed"},
				{ID: 2, GitHubID: 102, RunID: runID, Status: "completed"},
				{ID: 3, GitHubID: 103, RunID: runID, Status: "completed"},
			}, nil
		},
		hasJobLogFunc:  func(ctx context.Context, jobID int) (bool, error) { return archived[jobID], nil },
		saveJobLogFunc: func(ctx context.Context, entry *models.JobLog) error { archived[entry.JobID] = true; return nil },
	}
	h := newTestHandler(store)
	h.ghClient = ghClient
	h.config.LogArchiveMaxSizeBytes = 1 << 20

	req := withURLParam(withUser(httptest.NewRequest(http.MethodPost, "/api/repositories/7/backfill-logs", nil), &models.User{ID: 2, AccessToken: "t"}), "id", "7")
	rec := httptest.NewRecorder()
	h.BackfillRepositoryLogs(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := logsRequests.Load(); got != 2 {
		t.Fatalf("a rate-limit response must stop the pass after the next attempt, got %d fetches", got)
	}
	if archived[3] {
		t.Fatal("job after rate-limit response must not be attempted")
	}
}
