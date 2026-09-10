package backend

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testAutoSyncNotifier struct {
	statusUpdates []AutoSyncStatus
	uploadedFiles []AutoSyncFileEvent
}

func (n *testAutoSyncNotifier) EmitStatus(status AutoSyncStatus) {
	n.statusUpdates = append(n.statusUpdates, status)
}

func (n *testAutoSyncNotifier) EmitFileUploaded(event AutoSyncFileEvent) {
	n.uploadedFiles = append(n.uploadedFiles, event)
}

func TestAutoSyncManagerWatchAndDebounce(t *testing.T) {
	tempDir := t.TempDir()
	watchDir := filepath.Join(tempDir, "watched")
	if err := os.MkdirAll(watchDir, 0o700); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(tempDir, "gotohp.config")
	if err := LoadConfig(configPath); err != nil {
		t.Fatal(err)
	}

	cfgMgr := &ConfigManager{}
	cfgMgr.SetAutoSyncEnabled(true)
	if err := cfgMgr.AddSyncFolder(watchDir); err != nil {
		t.Fatalf("AddSyncFolder failed: %v", err)
	}

	notifier := &testAutoSyncNotifier{}
	mgr, err := NewAutoSyncManager(cfgMgr, notifier, nil)
	if err != nil {
		t.Fatalf("NewAutoSyncManager failed: %v", err)
	}
	SetActiveAutoSyncManager(mgr)
	mgr.Start()
	defer mgr.Stop()

	// Verify initial status
	status := mgr.GetStatus()
	if !status.Enabled {
		t.Errorf("expected status.Enabled to be true")
	}
	if status.FolderCount != 1 {
		t.Errorf("expected 1 folder, got %d", status.FolderCount)
	}

	// Create a supported file in the watched folder
	testFile := filepath.Join(watchDir, "sample.jpg")
	data := []byte("fake jpeg content data here for testing")
	if err := os.WriteFile(testFile, data, 0o600); err != nil {
		t.Fatal(err)
	}

	// Trigger manual scan to verify queueing
	if err := mgr.TriggerSyncNow(); err != nil {
		t.Fatalf("TriggerSyncNow failed: %v", err)
	}

	// Give a small moment for processing or status reflection
	time.Sleep(100 * time.Millisecond)

	// Verify unwatched
	if err := cfgMgr.RemoveSyncFolder(watchDir); err != nil {
		t.Fatalf("RemoveSyncFolder failed: %v", err)
	}
	folders := cfgMgr.GetSyncFolders()
	if len(folders) != 0 {
		t.Errorf("expected 0 folders after removal, got %d", len(folders))
	}
}
