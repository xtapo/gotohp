package backend

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// AutoSyncDB manages the local SQLite database for tracking synced files.
type AutoSyncDB struct {
	db *sql.DB
	mu sync.RWMutex
}

// SyncedRecord represents a recorded file in the sync database.
type SyncedRecord struct {
	Path       string
	Size       int64
	ModTime    int64
	SHA1       string
	UploadedAt int64
}

// NewAutoSyncDB opens or creates the SQLite database at the specified path.
func NewAutoSyncDB(dbPath string) (*AutoSyncDB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("failed to create directory for autosync db: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open autosync db: %w", err)
	}

	// Optimize SQLite settings for small local embedded db
	_, _ = db.Exec("PRAGMA journal_mode=WAL;")
	_, _ = db.Exec("PRAGMA busy_timeout=5000;")

	createTableQuery := `
	CREATE TABLE IF NOT EXISTS synced_files (
		path TEXT PRIMARY KEY,
		size INTEGER NOT NULL,
		mod_time INTEGER NOT NULL,
		sha1 TEXT NOT NULL,
		uploaded_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_synced_files_sha1 ON synced_files(sha1);
	`
	if _, err := db.Exec(createTableQuery); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return &AutoSyncDB{db: db}, nil
}

// IsFileSynced checks if a file with matching path, size, and modTime has already been synced.
func (s *AutoSyncDB) IsFileSynced(path string, size int64, modTime int64) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	query := `SELECT COUNT(1) FROM synced_files WHERE path = ? AND size = ? AND mod_time = ?`
	err := s.db.QueryRow(query, filepath.Clean(path), size, modTime).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// IsSHA1Synced checks if a file with the given SHA1 hash has already been synced under any path.
func (s *AutoSyncDB) IsSHA1Synced(sha1 string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	query := `SELECT COUNT(1) FROM synced_files WHERE sha1 = ?`
	err := s.db.QueryRow(query, sha1).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RecordSyncedFile records or updates a successfully synced file entry.
func (s *AutoSyncDB) RecordSyncedFile(path string, size int64, modTime int64, sha1 string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO synced_files (path, size, mod_time, sha1, uploaded_at)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(path) DO UPDATE SET
		size = excluded.size,
		mod_time = excluded.mod_time,
		sha1 = excluded.sha1,
		uploaded_at = excluded.uploaded_at;
	`
	_, err := s.db.Exec(query, filepath.Clean(path), size, modTime, sha1, time.Now().Unix())
	return err
}

// RemoveSyncedFile removes a path from the tracking database.
func (s *AutoSyncDB) RemoveSyncedFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM synced_files WHERE path = ?`
	_, err := s.db.Exec(query, filepath.Clean(path))
	return err
}

// GetSyncedCount returns the total number of synced files tracked.
func (s *AutoSyncDB) GetSyncedCount() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM synced_files`).Scan(&count)
	return count, err
}

// Close closes the underlying SQLite database.
func (s *AutoSyncDB) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
