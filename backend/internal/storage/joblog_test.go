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

// UpsertRun must store a copy, not the caller's struct: memory storage is the only mode that
// mutates runs in place, and callers (the log archiver) snapshot runs outside the storage
// lock, so a shared pointer is a data race under concurrent webhook deliveries.
func TestUpsertRun_ReturnsCopyNotSharedPointer(t *testing.T) {
	s := newTestStorage()
	input := &models.WorkflowRun{GitHubID: 77, WorkflowID: 1, RepoID: 1, Status: "in_progress"}
	saved, err := s.UpsertRun(context.Background(), input)
	if err != nil {
		t.Fatalf("UpsertRun: %v", err)
	}

	// Mutating the caller's struct must not change the stored run.
	input.Status = "queued"

	got, err := s.GetRunByGitHubID(context.Background(), 77)
	if err != nil || got == nil {
		t.Fatalf("GetRunByGitHubID: (%v, %v)", got, err)
	}
	if got.Status != "in_progress" {
		t.Fatalf("stored run leaked caller mutations: status = %q, want in_progress", got.Status)
	}

	// A second upsert must replace the stored object, not mutate it, so pointers handed out
	// earlier stay stable.
	saved2, err := s.UpsertRun(context.Background(), &models.WorkflowRun{GitHubID: 77, WorkflowID: 1, RepoID: 1, Status: "completed"})
	if err != nil {
		t.Fatalf("second UpsertRun: %v", err)
	}
	if saved2.Status != "completed" {
		t.Fatalf("second upsert status = %q, want completed", saved2.Status)
	}
	if got.Status != "in_progress" {
		t.Fatalf("earlier snapshot mutated by second upsert: %q", got.Status)
	}
	if saved2 == saved {
		t.Fatal("UpsertRun must return a new object for an existing run (copy-on-write)")
	}
}
