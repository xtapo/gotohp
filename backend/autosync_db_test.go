package backend

import (
	"path/filepath"
	"testing"
)

func TestAutoSyncDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "autosync.db")

	db, err := NewAutoSyncDB(dbPath)
	if err != nil {
		t.Fatalf("NewAutoSyncDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	testPath := filepath.Join(tempDir, "test.jpg")
	testSize := int64(1024)
	testModTime := int64(1700000000)
	testSHA1 := "da39a3ee5e6b4b0d3255bfef95601890afd80709"

	// Initially not synced
	synced, err := db.IsFileSynced(testPath, testSize, testModTime)
	if err != nil {
		t.Fatalf("IsFileSynced failed: %v", err)
	}
	if synced {
		t.Fatalf("expected file not synced initially")
	}

	// Record synced file
	if err := db.RecordSyncedFile(testPath, testSize, testModTime, testSHA1); err != nil {
		t.Fatalf("RecordSyncedFile failed: %v", err)
	}

	// Now it should be synced
	synced, err = db.IsFileSynced(testPath, testSize, testModTime)
	if err != nil {
		t.Fatalf("IsFileSynced failed: %v", err)
	}
	if !synced {
		t.Fatalf("expected file to be reported as synced")
	}

	// Modified modTime or size should not be considered synced
	synced, _ = db.IsFileSynced(testPath, testSize+1, testModTime)
	if synced {
		t.Fatalf("expected file with different size to not be synced")
	}

	synced, _ = db.IsFileSynced(testPath, testSize, testModTime+10)
	if synced {
		t.Fatalf("expected file with different modTime to not be synced")
	}

	// Check SHA1 sync
	shaSynced, err := db.IsSHA1Synced(testSHA1)
	if err != nil {
		t.Fatalf("IsSHA1Synced failed: %v", err)
	}
	if !shaSynced {
		t.Fatalf("expected SHA1 to be reported as synced")
	}

	// Count
	count, err := db.GetSyncedCount()
	if err != nil {
		t.Fatalf("GetSyncedCount failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	// Remove
	if err := db.RemoveSyncedFile(testPath); err != nil {
		t.Fatalf("RemoveSyncedFile failed: %v", err)
	}
	synced, _ = db.IsFileSynced(testPath, testSize, testModTime)
	if synced {
		t.Fatalf("expected file to be removed from db")
	}
}
