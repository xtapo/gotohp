package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPostUploadBackup(t *testing.T) {
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "photo1.jpg")
	if err := os.WriteFile(sourceFile, []byte("test image data"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := UploadOptions{
		PostUploadAction: PostUploadBackup,
	}

	// Move to default _BackedUp directory
	if err := ExecutePostUploadAction(sourceFile, opts); err != nil {
		t.Fatalf("ExecutePostUploadAction failed: %v", err)
	}

	// Verify original file is gone
	if _, err := os.Stat(sourceFile); !os.IsNotExist(err) {
		t.Errorf("expected source file to be moved, but still exists")
	}

	// Verify backed up file exists in _BackedUp
	backupFile := filepath.Join(tempDir, "_BackedUp", "photo1.jpg")
	if _, err := os.Stat(backupFile); err != nil {
		t.Errorf("expected backup file at %s, got err: %v", backupFile, err)
	}

	// Test collision handling: create another photo1.jpg in tempDir and back it up
	if err := os.WriteFile(sourceFile, []byte("second photo"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ExecutePostUploadAction(sourceFile, opts); err != nil {
		t.Fatalf("second ExecutePostUploadAction failed: %v", err)
	}
	// Verify photo1_1.jpg exists
	collisionFile := filepath.Join(tempDir, "_BackedUp", "photo1_1.jpg")
	if _, err := os.Stat(collisionFile); err != nil {
		t.Errorf("expected collision backup file at %s, got err: %v", collisionFile, err)
	}
}

func TestPostUploadCustomBackupFolder(t *testing.T) {
	tempDir := t.TempDir()
	customBackupDir := filepath.Join(tempDir, "custom_archive")
	sourceFile := filepath.Join(tempDir, "pic.jpg")
	if err := os.WriteFile(sourceFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := UploadOptions{
		PostUploadAction: PostUploadBackup,
		BackupFolder:     customBackupDir,
	}

	if err := ExecutePostUploadAction(sourceFile, opts); err != nil {
		t.Fatalf("ExecutePostUploadAction failed: %v", err)
	}

	target := filepath.Join(customBackupDir, "pic.jpg")
	if _, err := os.Stat(target); err != nil {
		t.Errorf("expected custom backup file at %s, got err: %v", target, err)
	}
}

func TestPostUploadDelete(t *testing.T) {
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "temp.jpg")
	if err := os.WriteFile(sourceFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := UploadOptions{
		PostUploadAction: PostUploadDelete,
	}

	if err := ExecutePostUploadAction(sourceFile, opts); err != nil {
		t.Fatalf("ExecutePostUploadAction failed: %v", err)
	}

	if _, err := os.Stat(sourceFile); !os.IsNotExist(err) {
		t.Errorf("expected file to be deleted")
	}
}

func TestFreeUpSpaceFiles(t *testing.T) {
	tempDir := t.TempDir()
	f1 := filepath.Join(tempDir, "f1.jpg")
	f2 := filepath.Join(tempDir, "f2.jpg")
	_ = os.WriteFile(f1, []byte("1"), 0644)
	_ = os.WriteFile(f2, []byte("2"), 0644)

	count, err := FreeUpSpaceFiles([]string{f1, f2}, PostUploadBackup, "")
	if err != nil {
		t.Fatalf("FreeUpSpaceFiles failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 files freed, got %d", count)
	}
}
