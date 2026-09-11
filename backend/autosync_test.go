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
	defer func() {
		mgr.Stop()
		CloseGlobalHistoryStore()
	}()

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

func TestAutoSyncSessionCache(t *testing.T) {
	mgr, err := NewAutoSyncManager(nil, nil, nil)
	if err != nil {
		t.Fatalf("NewAutoSyncManager failed: %v", err)
	}

	testPath := filepath.Clean("/test/photos/image.jpg")
	size := int64(1024)
	modTime := int64(1700000000)

	// Initially not synced
	if mgr.isSessionSynced(testPath, size, modTime) {
		t.Errorf("expected file not to be synced initially")
	}

	// Mark as synced
	mgr.markSessionSynced(testPath, size, modTime)
	if !mgr.isSessionSynced(testPath, size, modTime) {
		t.Errorf("expected file to be marked synced")
	}

	// Modified size -> not synced
	if mgr.isSessionSynced(testPath, size+1, modTime) {
		t.Errorf("expected modified size to not be synced")
	}

	// Modified modTime -> not synced
	if mgr.isSessionSynced(testPath, size, modTime+1) {
		t.Errorf("expected modified modTime to not be synced")
	}
}

func TestAutoSyncFolderAlbumConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "gotohp.config")
	if err := LoadConfig(configPath); err != nil {
		t.Fatal(err)
	}

	cfgMgr := &ConfigManager{}
	cfgMgr.SetAutoAlbumEnabled(true)
	settings := cfgMgr.GetSettings()
	if !settings.AutoAlbumEnabled {
		t.Errorf("expected AutoAlbumEnabled to be true")
	}

	folder := filepath.Clean("/media/vacation")
	albumKey := "AF1QipM9abcdefghij"

	cfgMgr.SetFolderAlbumKey(folder, albumKey)
	retrieved := cfgMgr.GetFolderAlbumKey(folder)
	if retrieved != albumKey {
		t.Errorf("expected album key %s, got %s", albumKey, retrieved)
	}

	// Verify persistence by reloading from disk
	if err := LoadConfig(configPath); err != nil {
		t.Fatal(err)
	}
	reloaded := cfgMgr.GetFolderAlbumKey(folder)
	if reloaded != albumKey {
		t.Errorf("expected reloaded album key %s, got %s", albumKey, reloaded)
	}
}

func TestAutoSyncPersistenceAcrossRestarts(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "history.db")
	store, err := NewHistoryStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create history store: %v", err)
	}
	defer store.Close()
	SetHistoryStore(store)
	defer SetHistoryStore(nil)

	mgr1, err := NewAutoSyncManager(nil, nil, nil)
	if err != nil {
		t.Fatalf("NewAutoSyncManager failed: %v", err)
	}

	testPath := filepath.Clean("/media/photos/sample.jpg")
	size := int64(2048)
	modTime := int64(1750000000)

	// In session 1, file is not synced initially
	if mgr1.isSessionSynced(testPath, size, modTime) {
		t.Errorf("expected file not to be synced in mgr1 initially")
	}

	// Record in SQLite
	err = store.RecordAutoSyncFile(testPath, size, modTime, "dummy-sha1", "AF1QipMediaKey")
	if err != nil {
		t.Fatalf("RecordAutoSyncFile failed: %v", err)
	}

	// mgr1 should now detect it as synced (via SQLite fallback and populate RAM)
	if !mgr1.isSessionSynced(testPath, size, modTime) {
		t.Errorf("expected mgr1 to detect file as synced via SQLite")
	}

	// Simulate app restart: create a completely fresh manager with empty RAM cache
	mgr2, err := NewAutoSyncManager(nil, nil, nil)
	if err != nil {
		t.Fatalf("NewAutoSyncManager (restart) failed: %v", err)
	}

	// Verify mgr2 loaded initial synced count from SQLite
	status := mgr2.GetStatus()
	if status.SyncedCount != 1 {
		t.Errorf("expected SyncedCount to be 1 from SQLite on startup, got %d", status.SyncedCount)
	}

	// Verify mgr2 recognises the file as already synced WITHOUT re-running or touching remote
	if !mgr2.isSessionSynced(testPath, size, modTime) {
		t.Errorf("expected mgr2 to recognize file as synced across app restart")
	}

	// If file was modified on disk (size or modTime change), it must NOT be considered synced
	if mgr2.isSessionSynced(testPath, size+100, modTime) {
		t.Errorf("expected modified file size to NOT be considered synced")
	}
	if mgr2.isSessionSynced(testPath, size, modTime+60) {
		t.Errorf("expected modified file timestamp to NOT be considered synced")
	}
}
