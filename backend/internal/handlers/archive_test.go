package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"golang.org/x/oauth2"

	"snorlx/backend/internal/config"
	"snorlx/backend/internal/github"
	"snorlx/backend/internal/models"
)

// ===== Log compression helpers =====

func TestCompressLogRoundtrip(t *testing.T) {
	original := []byte(strings.Repeat("2026-01-01T00:00:00.0000000Z Building project...\n", 500))
	compressed := compressLog(original)
	if len(compressed) >= len(original) {
		t.Fatalf("gzip did not shrink the log: %d -> %d bytes", len(original), len(compressed))
	}
	decompressed, err := decompressLog(compressed)
	if err != nil {
		t.Fatalf("decompressLog: %v", err)
	}
	if string(decompressed) != string(original) {
		t.Fatal("roundtrip changed the log content")
	}
}

// ===== fetchAndStoreJobLog =====

// newArchiveTestServer serves the fake GitHub endpoints the archiver touches:
// the logs endpoint redirects to a signed URL that answers with the log text.
// GetUserClient routes through WithEnterpriseURLs, which prefixes /api/v3, so both
// path shapes are handled.
func newArchiveTestServer(t *testing.T, logText string, logsRequests *atomic.Int64) *httptest.Server {
	t.Helper()
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
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(logText))
			return
		}
		t.Logf("unexpected fake-GitHub request: %s", r.URL.Path)
		http.NotFound(w, r)
	})
	return httptest.NewServer(mux)
}

func TestFetchAndStoreJobLog_StoresGzippedContent(t *testing.T) {
	logText := strings.Repeat("step output\n", 200)
	var logsRequests atomic.Int64
	server := newArchiveTestServer(t, logText, &logsRequests)
	defer server.Close()

	ghClient, err := github.NewClient(&config.Config{GitHubClientID: "id", GitHubClientSecret: "secret", GitHubBaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	var saved *models.JobLog
	store := &mockStorage{
		hasJobLogFunc: func(ctx context.Context, jobID int) (bool, error) { return false, nil },
		saveJobLogFunc: func(ctx context.Context, entry *models.JobLog) error {
			saved = entry
			return nil
		},
	}
	h := newTestHandler(store)
	h.ghClient = ghClient
	h.config.LogArchiveMaxSizeBytes = 1 << 20
	client := ghClient.GetUserClient(context.Background(), &oauth2.Token{AccessToken: "test-token"})
	ctx := context.Background()

	job := models.WorkflowJob{ID: 42, GitHubID: 77, RunID: 5, Status: "completed"}
	if _, err := h.fetchAndStoreJobLog(ctx, client, "acme", "widgets", job); err != nil {
		t.Fatalf("fetchAndStoreJobLog: %v", err)
	}
	if logsRequests.Load() != 1 {
		t.Fatalf("expected 1 log fetch, got %d", logsRequests.Load())
	}
	if saved == nil {
		t.Fatal("log was not stored")
	}
	if saved.JobID != 42 || saved.RunID != 5 {
		t.Errorf("stored ids = (%d, %d), want (42, 5)", saved.JobID, saved.RunID)
	}
	if saved.SizeBytes != len(logText) {
		t.Errorf("SizeBytes = %d, want %d", saved.SizeBytes, len(logText))
	}
	decompressed, err := decompressLog(saved.Content)
	if err != nil {
		t.Fatalf("stored content is not valid gzip: %v", err)
	}
	if string(decompressed) != logText {
		t.Fatal("decompressed content differs from the log text")
	}
}

func TestFetchAndStoreJobLog_SkipsAlreadyArchived(t *testing.T) {
	var logsRequests atomic.Int64
	server := newArchiveTestServer(t, "log", &logsRequests)
	defer server.Close()

	ghClient, err := github.NewClient(&config.Config{GitHubClientID: "id", GitHubClientSecret: "secret", GitHubBaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	store := &mockStorage{
		hasJobLogFunc: func(ctx context.Context, jobID int) (bool, error) { return true, nil },
		saveJobLogFunc: func(ctx context.Context, entry *models.JobLog) error {
			t.Error("SaveJobLog must not be called for an already archived job")
			return nil
		},
	}
	h := newTestHandler(store)
	h.ghClient = ghClient
	h.config.LogArchiveMaxSizeBytes = 1 << 20
	client := ghClient.GetUserClient(context.Background(), &oauth2.Token{AccessToken: "test-token"})
	ctx := context.Background()

	job := models.WorkflowJob{ID: 42, GitHubID: 77, RunID: 5, Status: "completed"}
	if _, err := h.fetchAndStoreJobLog(ctx, client, "acme", "widgets", job); err != nil {
		t.Fatalf("fetchAndStoreJobLog: %v", err)
	}
	if logsRequests.Load() != 0 {
		t.Errorf("already archived job must not be fetched, got %d fetches", logsRequests.Load())
	}
}

func TestFetchAndStoreJobLog_CapsOversizedLog(t *testing.T) {
	var logsRequests atomic.Int64
	server := newArchiveTestServer(t, strings.Repeat("x", 1000), &logsRequests)
	defer server.Close()

	ghClient, err := github.NewClient(&config.Config{GitHubClientID: "id", GitHubClientSecret: "secret", GitHubBaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	store := &mockStorage{
		hasJobLogFunc: func(ctx context.Context, jobID int) (bool, error) { return false, nil },
		saveJobLogFunc: func(ctx context.Context, entry *models.JobLog) error {
			t.Error("oversized log must not be stored")
			return nil
		},
	}
	h := newTestHandler(store)
	h.ghClient = ghClient
	h.config.LogArchiveMaxSizeBytes = 100
	client := ghClient.GetUserClient(context.Background(), &oauth2.Token{AccessToken: "test-token"})
	ctx := context.Background()

	job := models.WorkflowJob{ID: 42, GitHubID: 77, RunID: 5, Status: "completed"}
	if _, err := h.fetchAndStoreJobLog(ctx, client, "acme", "widgets", job); err == nil {
		t.Fatal("expected an error for an oversized log")
	}
}

// ===== archiveRunJobs =====

func TestArchiveRunJobs_ArchivesOnlyCompletedJobsWithoutLogs(t *testing.T) {
	var logsRequests atomic.Int64
	server := newArchiveTestServer(t, "job log", &logsRequests)
	defer server.Close()

	ghClient, err := github.NewClient(&config.Config{GitHubClientID: "id", GitHubClientSecret: "secret", GitHubBaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	archived := make(map[int]bool)
	store := &mockStorage{
		listJobsForRunFunc: func(ctx context.Context, runID int) ([]models.WorkflowJob, error) {
			return []models.WorkflowJob{
				{ID: 1, GitHubID: 101, RunID: runID, Status: "completed"},
				{ID: 2, GitHubID: 102, RunID: runID, Status: "in_progress"},
				{ID: 3, GitHubID: 103, RunID: runID, Status: "completed"},
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
	client := ghClient.GetUserClient(context.Background(), &oauth2.Token{AccessToken: "test-token"})
	ctx := context.Background()

	run := models.WorkflowRun{ID: 5, RepoID: 7}
	if err := h.archiveRunJobs(ctx, client, "acme", "widgets", run); err != nil {
		t.Fatalf("archiveRunJobs: %v", err)
	}
	if !archived[1] || !archived[3] {
		t.Errorf("completed jobs must be archived, got %v", archived)
	}
	if archived[2] {
		t.Error("in-progress job must not be archived")
	}
	if logsRequests.Load() != 2 {
		t.Errorf("expected 2 log fetches, got %d", logsRequests.Load())
	}
}

