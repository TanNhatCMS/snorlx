package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"snorlx/backend/internal/config"
	"snorlx/backend/internal/github"
	"snorlx/backend/internal/models"
)

// Một 403/429 trên NỘI DUNG log (signed URL) cũng phải dừng cả pass backfill,
// không chỉ 403 ở bước lấy URL — nếu không pass sẽ tiếp tục tốn request vô ích.
func TestBackfillRepositoryLogs_StopsOnRateLimitedDownload(t *testing.T) {
	var logsRequests atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/api/v3")
		if strings.HasPrefix(p, "/repos/acme/widgets/actions/jobs/") && strings.HasSuffix(p, "/logs") {
			logsRequests.Add(1)
			jobID := strings.TrimSuffix(strings.TrimPrefix(p, "/repos/acme/widgets/actions/jobs/"), "/logs")
			http.Redirect(w, r, "http://"+r.Host+"/signed/"+jobID, http.StatusFound)
			return
		}
		if strings.HasPrefix(p, "/signed/") {
			// GitHub trả 403 rate limit ngay ở bước tải nội dung.
			w.Header().Set("X-RateLimit-Limit", "5000")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", "1893456000")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
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

	var out struct {
		Failed      int  `json:"failed"`
		RateLimited bool `json:"rate_limited"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if got := logsRequests.Load(); got != 1 {
		t.Fatalf("rate-limited download must stop the pass after the first job, got %d job fetches", got)
	}
	if out.Failed != 1 {
		t.Fatalf("failed = %d, want 1 (the rate-limited job)", out.Failed)
	}
	if !out.RateLimited {
		t.Fatal("response must flag rate_limited so the caller knows to wait before re-calling")
	}
}
