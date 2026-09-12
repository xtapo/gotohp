package backend

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	PostUploadNone    = "none"
	PostUploadBackup  = "backup"
	PostUploadRecycle = "recycle"
	PostUploadDelete  = "delete"
)

// moveFileWithFallback attempts an atomic os.Rename first; if that fails (e.g. across drives),
// it falls back to copying and removing the source.
func moveFileWithFallback(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fallback to copy + remove
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("copy file data: %w", err)
	}

	sourceFile.Close()
	destFile.Close()

	return os.Remove(src)
}

// resolveSafeBackupPath generates a collision-free target path in targetDir for baseName.
func resolveSafeBackupPath(targetDir, baseName string) string {
	ext := filepath.Ext(baseName)
	stem := strings.TrimSuffix(baseName, ext)
	target := filepath.Join(targetDir, baseName)

	counter := 1
	for {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			return target
		}
		target = filepath.Join(targetDir, fmt.Sprintf("%s_%d%s", stem, counter, ext))
		counter++
	}
}

// moveFileToBackup moves a single file to a backup directory (_BackedUp by default).
func moveFileToBackup(filePath, backupFolder string) (string, error) {
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("source file does not exist: %w", err)
	}

	var targetDir string
	if backupFolder != "" {
		if filepath.IsAbs(backupFolder) {
			targetDir = backupFolder
		} else {
			targetDir = filepath.Join(filepath.Dir(filePath), backupFolder)
		}
	} else {
		targetDir = filepath.Join(filepath.Dir(filePath), "_BackedUp")
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("create backup directory %s: %w", targetDir, err)
	}

	dstPath := resolveSafeBackupPath(targetDir, filepath.Base(filePath))
	if err := moveFileWithFallback(filePath, dstPath); err != nil {
		return "", fmt.Errorf("move file to backup: %w", err)
	}

	return dstPath, nil
}

// ExecutePostUploadAction performs the configured post-upload action on a single file.
func ExecutePostUploadAction(filePath string, opts UploadOptions) error {
	action := strings.ToLower(strings.TrimSpace(opts.PostUploadAction))

	// Backward compatibility: If PostUploadAction is empty but DeleteFromHost is true
	if action == "" || action == PostUploadNone {
		if opts.DeleteFromHost {
			action = PostUploadDelete
		} else {
			return nil
		}
	}

	switch action {
	case PostUploadNone:
		return nil
	case PostUploadDelete:
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to delete file %s: %w", filepath.Base(filePath), err)
		}
		return nil
	case PostUploadRecycle:
		if err := moveToRecycleBin(filePath); err != nil {
			return fmt.Errorf("failed to move file %s to Recycle Bin: %w", filepath.Base(filePath), err)
		}
		return nil
	case PostUploadBackup:
		if _, err := moveFileToBackup(filePath, opts.BackupFolder); err != nil {
			return fmt.Errorf("failed to move file %s to backup: %w", filepath.Base(filePath), err)
		}
		return nil
	default:
		return fmt.Errorf("unknown post upload action: %s", action)
	}
}

// ExecutePostUploadLivePhoto applies the post-upload action to both components of a Live Photo.
func ExecutePostUploadLivePhoto(pair LivePhotoPair, opts UploadOptions) error {
	for _, path := range []string{pair.PhotoPath, pair.VideoPath} {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			if err := ExecutePostUploadAction(path, opts); err != nil {
				return err
			}
		}
	}
	return nil
}

// FreeUpSpaceFiles cleans up a list of files on-demand based on the specified action.
func FreeUpSpaceFiles(filePaths []string, action string, backupFolder string) (int, error) {
	opts := UploadOptions{
		PostUploadAction: action,
		BackupFolder:     backupFolder,
	}

	count := 0
	var lastErr error
	for _, path := range filePaths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			continue // Already deleted or moved
		}
		if err := ExecutePostUploadAction(path, opts); err != nil {
			lastErr = err
		} else {
			count++
		}
	}

	return count, lastErr
}
