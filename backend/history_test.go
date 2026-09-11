package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHistoryStore_Lifecycle(t *testing.T) {
	store, err := NewHistoryStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory store: %v", err)
	}
	defer store.Close()

	// 1. Create a session
	sessionID, err := store.CreateSession("manual", "Test Album", 3)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if sessionID <= 0 {
		t.Fatalf("expected positive session ID, got %d", sessionID)
	}

	// 2. Record items: 1 success, 1 failed, 1 skipped
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "file1.jpg")
	file2 := filepath.Join(tempDir, "file2.jpg")
	file3 := filepath.Join(tempDir, "file3.jpg")
	_ = os.WriteFile(file1, []byte("test content 1"), 0o644)
	_ = os.WriteFile(file2, []byte("test content 2"), 0o644)
	_ = os.WriteFile(file3, []byte("test content 3"), 0o644)

	err = store.RecordItem(sessionID, UploadItemRecord{
		FilePath: file1,
		Status:   "success",
		MediaKey: "media-key-1",
	})
	if err != nil {
		t.Fatalf("failed to record success item: %v", err)
	}

	err = store.RecordItem(sessionID, UploadItemRecord{
		FilePath:     file2,
		Status:       "failed",
		ErrorMessage: "401 Unauthorized: token expired",
	})
	if err != nil {
		t.Fatalf("failed to record failed item: %v", err)
	}

	err = store.RecordItem(sessionID, UploadItemRecord{
		FilePath:   file3,
		Status:     "skipped",
		SkipReason: "File already exists in Google Photos",
	})
	if err != nil {
		t.Fatalf("failed to record skipped item: %v", err)
	}

	// 3. Finish session
	err = store.FinishSession(sessionID, "completed", 1, 1, 1)
	if err != nil {
		t.Fatalf("failed to finish session: %v", err)
	}

	// 4. Verify sessions query
	sessions, err := store.GetSessions(10, 0)
	if err != nil {
		t.Fatalf("failed to get sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].SuccessCount != 1 || sessions[0].FailedCount != 1 || sessions[0].SkippedCount != 1 {
		t.Errorf("session stats mismatch: %+v", sessions[0])
	}
	if sessions[0].AlbumName != "Test Album" {
		t.Errorf("expected album name 'Test Album', got %q", sessions[0].AlbumName)
	}

	// 5. Verify session details
	details, err := store.GetSessionDetails(sessionID)
	if err != nil {
		t.Fatalf("failed to get session details: %v", err)
	}
	if len(details.Items) != 3 {
		t.Fatalf("expected 3 items in session, got %d", len(details.Items))
	}

	// 6. Verify failed queue
	failedQueue, err := store.GetFailedQueue()
	if err != nil {
		t.Fatalf("failed to get failed queue: %v", err)
	}
	if len(failedQueue) != 1 {
		t.Fatalf("expected 1 failed item in queue, got %d", len(failedQueue))
	}
	if failedQueue[0].FilePath != file2 {
		t.Errorf("expected failed file %s, got %s", file2, failedQueue[0].FilePath)
	}
	if failedQueue[0].ErrorMessage != "401 Unauthorized: token expired" {
		t.Errorf("expected error message '401 Unauthorized: token expired', got %q", failedQueue[0].ErrorMessage)
	}

	// 7. Test retry resolution: new session uploads file2 successfully
	retrySessionID, err := store.CreateSession("retry", "", 1)
	if err != nil {
		t.Fatalf("failed to create retry session: %v", err)
	}

	err = store.RecordItem(retrySessionID, UploadItemRecord{
		FilePath: file2,
		Status:   "success",
		MediaKey: "media-key-2",
	})
	if err != nil {
		t.Fatalf("failed to record retry success: %v", err)
	}
	_ = store.FinishSession(retrySessionID, "completed", 1, 0, 0)

	// Now failed queue should be empty because file2 was resolved!
	failedQueueAfterRetry, err := store.GetFailedQueue()
	if err != nil {
		t.Fatalf("failed to get failed queue after retry: %v", err)
	}
	if len(failedQueueAfterRetry) != 0 {
		t.Errorf("expected 0 failed items after successful retry, got %d", len(failedQueueAfterRetry))
	}

	// 8. Test ClearHistory
	err = store.ClearHistory()
	if err != nil {
		t.Fatalf("failed to clear history: %v", err)
	}
	remainingSessions, _ := store.GetSessions(10, 0)
	if len(remainingSessions) != 0 {
		t.Errorf("expected 0 sessions after ClearHistory, got %d", len(remainingSessions))
	}
}

func TestHistoryStore_DismissAndClearQueue(t *testing.T) {
	store, err := NewHistoryStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory store: %v", err)
	}
	defer store.Close()

	sessionID, _ := store.CreateSession("manual", "", 2)
	_ = store.RecordItem(sessionID, UploadItemRecord{FilePath: "a.jpg", Status: "failed", ErrorMessage: "err a"})
	_ = store.RecordItem(sessionID, UploadItemRecord{FilePath: "b.jpg", Status: "failed", ErrorMessage: "err b"})

	queue, _ := store.GetFailedQueue()
	if len(queue) != 2 {
		t.Fatalf("expected 2 items in queue, got %d", len(queue))
	}

	// Dismiss item 1
	err = store.DismissFailedItem(queue[0].ID)
	if err != nil {
		t.Fatalf("failed to dismiss item: %v", err)
	}
	queueAfterDismiss, _ := store.GetFailedQueue()
	if len(queueAfterDismiss) != 1 {
		t.Fatalf("expected 1 item after dismiss, got %d", len(queueAfterDismiss))
	}

	// Clear entire queue
	err = store.ClearFailedQueue()
	if err != nil {
		t.Fatalf("failed to clear queue: %v", err)
	}
	queueAfterClear, _ := store.GetFailedQueue()
	if len(queueAfterClear) != 0 {
		t.Fatalf("expected 0 items after clear, got %d", len(queueAfterClear))
	}
}
