package backend

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type testUploadReporter struct {
	NopReporter
	mu      sync.Mutex
	stopped bool
	results []FileUploadResult
}

func (r *testUploadReporter) UploadStop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = true
}

func (r *testUploadReporter) FileResult(res FileUploadResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, res)
}

func (r *testUploadReporter) isStopped() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopped
}

func TestUploadManager_HistoryIntegration(t *testing.T) {
	store, err := NewHistoryStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory store: %v", err)
	}
	defer store.Close()

	reporter := &testUploadReporter{}
	mgr := NewUploadManager(reporter, nil)
	mgr.SetHistoryStore(store)

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "photo.jpg")
	_ = os.WriteFile(testFile, []byte("fake jpeg content"), 0o644)

	// Trigger upload with invalid API credentials so it fails NewApi and records failed item in history
	opts := UploadOptions{
		Threads: 1,
		Api: ApiOptions{
			Account: "nonexistent@test.com",
		},
	}

	mgr.Upload([]string{testFile}, opts)

	// Wait for upload run to complete
	deadline := time.Now().Add(5 * time.Second)
	for !reporter.isStopped() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	if !reporter.isStopped() {
		t.Fatal("timed out waiting for upload manager to stop")
	}

	// Verify session was recorded in HistoryStore
	sessions, err := store.GetSessions(10, 0)
	if err != nil {
		t.Fatalf("failed to get sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session in history, got %d", len(sessions))
	}
	if sessions[0].Status != "failed" {
		t.Errorf("expected session status 'failed', got %q", sessions[0].Status)
	}

	// Verify failed queue has the failed file
	queue, err := store.GetFailedQueue()
	if err != nil {
		t.Fatalf("failed to get failed queue: %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("expected 1 item in failed queue, got %d", len(queue))
	}
	if queue[0].FilePath != testFile {
		t.Errorf("expected failed file %s, got %s", testFile, queue[0].FilePath)
	}
	if queue[0].ErrorMessage == "" {
		t.Errorf("expected error message to be recorded")
	}

	// Verify GetAllFailedFilesForRetry returns this file path
	retryFiles, err := store.GetAllFailedFilesForRetry()
	if err != nil {
		t.Fatalf("failed to get retry files: %v", err)
	}
	if len(retryFiles) != 1 || retryFiles[0] != testFile {
		t.Errorf("expected retry files [%s], got %v", testFile, retryFiles)
	}
}
