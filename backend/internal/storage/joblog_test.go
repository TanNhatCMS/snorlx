package storage

import (
	"context"
	"testing"
	"time"

	"snorlx/backend/internal/models"
)

// ===== Job log archive =====

// seedLogRun creates a repository, workflow and one completed run visible to userID.
func seedLogRun(t *testing.T, s *MemoryStorage, userID int, repoGitHubID, runGitHubID int64) (*models.Repository, *models.WorkflowRun) {
	t.Helper()
	ctx := context.Background()
	repo := seedRepo(t, s, userID, repoGitHubID, "logrepo", true)
	wf, err := s.UpsertWorkflow(ctx, &models.Workflow{
		GitHubID: repoGitHubID*100 + 1,
		RepoID:   repo.ID,
		Name:     "CI",
		Path:     ".github/workflows/ci.yml",
		State:    "active",
	})
	if err != nil {
		t.Fatalf("UpsertWorkflow: %v", err)
	}
	now := time.Now()
	run, err := s.UpsertRun(ctx, &models.WorkflowRun{
		GitHubID:    runGitHubID,
		WorkflowID:  wf.ID,
		RepoID:      repo.ID,
		Status:      "completed",
		Conclusion:  strPtr("success"),
		Event:       "push",
		Branch:      "main",
		StartedAt:   now,
		CompletedAt: &now,
	})
	if err != nil {
		t.Fatalf("UpsertRun: %v", err)
	}
	return repo, run
}

// seedLogJob creates a job attached to runID; completed controls CompletedAt/Status.
func seedLogJob(t *testing.T, s *MemoryStorage, runID int, githubID int64, completed bool) *models.WorkflowJob {
	t.Helper()
	ctx := context.Background()
	job := &models.WorkflowJob{
		GitHubID:  githubID,
		RunID:     runID,
		Name:      "job",
		Status:    "in_progress",
		StartedAt: time.Now(),
	}
	if completed {
		now := time.Now()
		job.Status = "completed"
		job.CompletedAt = &now
	}
	saved, err := s.UpsertJob(ctx, job)
	if err != nil {
		t.Fatalf("UpsertJob(%d): %v", githubID, err)
	}
	return saved
}

func TestJobLog_SaveAndRoundtrip(t *testing.T) {
	s := newTestStorage()
	_, run := seedLogRun(t, s, seedUser(t, s, 1, "alice"), 900, 9100)
	job := seedLogJob(t, s, run.ID, 910, true)
	ctx := context.Background()

	if exists, err := s.HasJobLog(ctx, job.ID); err != nil || exists {
		t.Fatalf("HasJobLog before save = (%v, %v), want (false, nil)", exists, err)
	}

	content := []byte("2026-01-01T00:00:00Z ##[group]Run tests\nall green\n##[endgroup]\n")
	if err := s.SaveJobLog(ctx, &models.JobLog{
		JobID:     job.ID,
		RunID:     run.ID,
		Content:   content,
		SizeBytes: len(content),
		FetchedAt: time.Now(),
	}); err != nil {
		t.Fatalf("SaveJobLog: %v", err)
	}

	if exists, err := s.HasJobLog(ctx, job.ID); err != nil || !exists {
		t.Fatalf("HasJobLog after save = (%v, %v), want (true, nil)", exists, err)
	}

	got, err := s.GetJobLog(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJobLog: %v", err)
	}
	if got == nil || !bytesEqual(got.Content, content) {
		t.Fatalf("GetJobLog content mismatch")
	}
	if got.RunID != run.ID {
		t.Errorf("RunID = %d, want %d", got.RunID, run.ID)
	}
	if got.SizeBytes != len(content) {
		t.Errorf("SizeBytes = %d, want %d", got.SizeBytes, len(content))
	}
}

func TestJobLog_OverwriteKeepsLatest(t *testing.T) {
	s := newTestStorage()
	_, run := seedLogRun(t, s, seedUser(t, s, 1, "alice"), 900, 9100)
	job := seedLogJob(t, s, run.ID, 910, true)
	ctx := context.Background()

	first := []byte("first\n")
	if err := s.SaveJobLog(ctx, &models.JobLog{JobID: job.ID, RunID: run.ID, Content: first, SizeBytes: len(first), FetchedAt: time.Now()}); err != nil {
		t.Fatalf("SaveJobLog first: %v", err)
	}
	second := []byte("second, longer\n")
	if err := s.SaveJobLog(ctx, &models.JobLog{JobID: job.ID, RunID: run.ID, Content: second, SizeBytes: len(second), FetchedAt: time.Now()}); err != nil {
		t.Fatalf("SaveJobLog second: %v", err)
	}

	got, err := s.GetJobLog(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJobLog: %v", err)
	}
	if !bytesEqual(got.Content, second) {
		t.Fatalf("GetJobLog returned stale content after overwrite")
	}
	if got.SizeBytes != len(second) {
		t.Errorf("SizeBytes = %d, want %d", got.SizeBytes, len(second))
	}
}

func TestJobLog_GetMissingReturnsError(t *testing.T) {
	s := newTestStorage()
	got, err := s.GetJobLog(context.Background(), 424242)
	if err == nil || got != nil {
		t.Fatalf("GetJobLog(missing) = (%v, %v), want (nil, error)", got, err)
	}
}

func TestJobLog_ListJobsMissingLogs(t *testing.T) {
	s := newTestStorage()
	repo, run := seedLogRun(t, s, seedUser(t, s, 1, "alice"), 900, 9100)
	done1 := seedLogJob(t, s, run.ID, 911, true)
	done2 := seedLogJob(t, s, run.ID, 912, true)
	done3 := seedLogJob(t, s, run.ID, 913, true)
	running := seedLogJob(t, s, run.ID, 914, false)
	ctx := context.Background()

	// Only done1 has its log stored.
	content := []byte("done1\n")
	if err := s.SaveJobLog(ctx, &models.JobLog{JobID: done1.ID, RunID: run.ID, Content: content, SizeBytes: len(content), FetchedAt: time.Now()}); err != nil {
		t.Fatalf("SaveJobLog: %v", err)
	}

	missing, err := s.ListJobsMissingLogs(ctx, repo.ID, 100)
	if err != nil {
		t.Fatalf("ListJobsMissingLogs: %v", err)
	}

	got := make(map[int]bool, len(missing))
	for _, j := range missing {
		got[j.ID] = true
	}
	if got[done2.ID] == false || got[done3.ID] == false {
		t.Errorf("missing logs must include completed jobs %d and %d, got %v", done2.ID, done3.ID, missing)
	}
	if got[done1.ID] {
		t.Errorf("job %d already has a log and must not be listed", done1.ID)
	}
	if got[running.ID] {
		t.Errorf("in-progress job %d must not be listed", running.ID)
	}
}

func TestJobLog_ListJobsMissingLogs_RespectsLimit(t *testing.T) {
	s := newTestStorage()
	repo, run := seedLogRun(t, s, seedUser(t, s, 1, "alice"), 900, 9100)
	seedLogJob(t, s, run.ID, 915, true)
	seedLogJob(t, s, run.ID, 916, true)
	seedLogJob(t, s, run.ID, 917, true)

	missing, err := s.ListJobsMissingLogs(context.Background(), repo.ID, 2)
	if err != nil {
		t.Fatalf("ListJobsMissingLogs: %v", err)
	}
	if len(missing) != 2 {
		t.Errorf("len(missing) = %d, want 2 (limit respected)", len(missing))
	}
}

// bytesEqual compares two byte slices without importing bytes in every test body.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// strPtr is a helper for nullable string fields used by the seed helpers above.
func strPtr(s string) *string { return &s }
