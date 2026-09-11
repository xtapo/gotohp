package backend

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// UploadSessionSummary represents the high-level summary of an upload session.
type UploadSessionSummary struct {
	ID           int64  `json:"id"`
	Source       string `json:"source"` // "manual", "retry", "autosync"
	AlbumName    string `json:"albumName"`
	TotalFiles   int    `json:"totalFiles"`
	SuccessCount int    `json:"successCount"`
	FailedCount  int    `json:"failedCount"`
	SkippedCount int    `json:"skippedCount"`
	Status       string `json:"status"` // "running", "completed", "cancelled", "failed"
	StartedAt    int64  `json:"startedAt"`
	EndedAt      int64  `json:"endedAt"`
}

// UploadItemRecord represents a single file entry in an upload session.
type UploadItemRecord struct {
	ID           int64  `json:"id"`
	SessionID    int64  `json:"sessionId"`
	FilePath     string `json:"filePath"`
	FileName     string `json:"fileName"`
	FileSize     int64  `json:"fileSize"`
	Status       string `json:"status"` // "success", "failed", "skipped"
	ErrorMessage string `json:"errorMessage"`
	SkipReason   string `json:"skipReason"`
	MediaKey     string `json:"mediaKey"`
	Resolved     bool   `json:"resolved"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

// UploadSessionDetails holds a session summary along with all its item records.
type UploadSessionDetails struct {
	Session UploadSessionSummary `json:"session"`
	Items   []UploadItemRecord   `json:"items"`
}

// FailedItemSummary represents an active failed file in the queue.
type FailedItemSummary struct {
	ID           int64  `json:"id"`
	SessionID    int64  `json:"sessionId"`
	FilePath     string `json:"filePath"`
	FileName     string `json:"fileName"`
	FileSize     int64  `json:"fileSize"`
	ErrorMessage string `json:"errorMessage"`
	CreatedAt    int64  `json:"createdAt"`
}

// HistoryStore coordinates local SQLite database storage for upload history and failed items.
type HistoryStore struct {
	dbPath string
	db     *sql.DB
	mu     sync.RWMutex
}

var (
	historyStoreMu     sync.RWMutex
	globalHistoryStore *HistoryStore
)

// GetHistoryStore returns the singleton HistoryStore, initializing it if needed.
func GetHistoryStore() *HistoryStore {
	historyStoreMu.RLock()
	if globalHistoryStore != nil {
		defer historyStoreMu.RUnlock()
		return globalHistoryStore
	}
	historyStoreMu.RUnlock()

	historyStoreMu.Lock()
	defer historyStoreMu.Unlock()
	if globalHistoryStore != nil {
		return globalHistoryStore
	}

	ensureConfigLoaded()
	dir := filepath.Dir(ConfigPath)
	dbPath := filepath.Join(dir, "history.db")
	store, err := NewHistoryStore(dbPath)
	if err != nil {
		store, _ = NewHistoryStore(":memory:")
	}
	globalHistoryStore = store
	return globalHistoryStore
}

// SetHistoryStore sets the global HistoryStore (useful for testing).
func SetHistoryStore(store *HistoryStore) {
	historyStoreMu.Lock()
	defer historyStoreMu.Unlock()
	globalHistoryStore = store
}

// CloseGlobalHistoryStore closes and resets the global HistoryStore.
func CloseGlobalHistoryStore() {
	historyStoreMu.Lock()
	defer historyStoreMu.Unlock()
	if globalHistoryStore != nil {
		_ = globalHistoryStore.Close()
		globalHistoryStore = nil
	}
}

// NewHistoryStore creates and initializes a new HistoryStore with the given dbPath.
func NewHistoryStore(dbPath string) (*HistoryStore, error) {
	if dbPath == "" {
		return nil, errors.New("dbPath cannot be empty")
	}

	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
	}

	dsn := dbPath
	if dbPath != ":memory:" {
		dsn = fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dbPath)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	store := &HistoryStore{
		dbPath: dbPath,
		db:     db,
	}

	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

func (s *HistoryStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS upload_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT NOT NULL DEFAULT 'manual',
		album_name TEXT,
		total_files INTEGER DEFAULT 0,
		success_count INTEGER DEFAULT 0,
		failed_count INTEGER DEFAULT 0,
		skipped_count INTEGER DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'running',
		started_at INTEGER NOT NULL,
		ended_at INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS upload_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL REFERENCES upload_sessions(id) ON DELETE CASCADE,
		file_path TEXT NOT NULL,
		file_name TEXT NOT NULL,
		file_size INTEGER DEFAULT 0,
		status TEXT NOT NULL,
		error_message TEXT,
		skip_reason TEXT,
		media_key TEXT,
		resolved INTEGER DEFAULT 0,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_started_at ON upload_sessions(started_at DESC);
	CREATE INDEX IF NOT EXISTS idx_items_session_id ON upload_items(session_id);
	CREATE INDEX IF NOT EXISTS idx_items_status_resolved ON upload_items(status, resolved);
	CREATE INDEX IF NOT EXISTS idx_items_file_path ON upload_items(file_path);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Close closes the underlying database.
func (s *HistoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// CreateSession creates a new upload session and returns its ID.
func (s *HistoryStore) CreateSession(source, albumName string, totalFiles int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	if source == "" {
		source = "manual"
	}

	query := `
	INSERT INTO upload_sessions (source, album_name, total_files, status, started_at)
	VALUES (?, ?, ?, 'running', ?)
	`
	res, err := s.db.Exec(query, source, albumName, totalFiles, now)
	if err != nil {
		return 0, fmt.Errorf("failed to create upload session: %w", err)
	}

	return res.LastInsertId()
}

// RecordItem saves an upload item result into the session.
// If status is "success", any previous unresolved failed records for the same file path are marked resolved.
func (s *HistoryStore) RecordItem(sessionID int64, item UploadItemRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if item.FileName == "" && item.FilePath != "" {
		item.FileName = filepath.Base(item.FilePath)
	}

	if item.FileSize == 0 && item.FilePath != "" {
		if fi, err := os.Stat(item.FilePath); err == nil {
			item.FileSize = fi.Size()
		}
	}

	now := time.Now().Unix()
	resolvedInt := 0
	if item.Resolved {
		resolvedInt = 1
	}

	query := `
	INSERT INTO upload_items (
		session_id, file_path, file_name, file_size,
		status, error_message, skip_reason, media_key,
		resolved, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(
		query,
		sessionID, item.FilePath, item.FileName, item.FileSize,
		item.Status, item.ErrorMessage, item.SkipReason, item.MediaKey,
		resolvedInt, now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to record upload item: %w", err)
	}

	// If this file succeeded, mark previous failed attempts for this file path as resolved
	if item.Status == "success" && item.FilePath != "" {
		_, _ = s.db.Exec(`UPDATE upload_items SET resolved = 1, updated_at = ? WHERE file_path = ? AND status = 'failed'`, now, item.FilePath)
	}

	return nil
}

// FinishSession updates the session status, counters, and ended_at timestamp.
func (s *HistoryStore) FinishSession(sessionID int64, status string, successCount, failedCount, skippedCount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	if status == "" {
		if failedCount > 0 && successCount == 0 {
			status = "failed"
		} else {
			status = "completed"
		}
	}

	query := `
	UPDATE upload_sessions
	SET status = ?, success_count = ?, failed_count = ?, skipped_count = ?, ended_at = ?
	WHERE id = ?
	`
	_, err := s.db.Exec(query, status, successCount, failedCount, skippedCount, now, sessionID)
	if err != nil {
		return fmt.Errorf("failed to finish session: %w", err)
	}
	return nil
}

// GetSessions returns a paginated list of upload sessions sorted newest first.
func (s *HistoryStore) GetSessions(limit, offset int) ([]UploadSessionSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `
	SELECT id, source, album_name, total_files, success_count, failed_count, skipped_count, status, started_at, ended_at
	FROM upload_sessions
	ORDER BY started_at DESC, id DESC
	LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query upload sessions: %w", err)
	}
	defer rows.Close()

	var sessions []UploadSessionSummary
	for rows.Next() {
		var s UploadSessionSummary
		var albumName sql.NullString
		if err := rows.Scan(
			&s.ID, &s.Source, &albumName, &s.TotalFiles,
			&s.SuccessCount, &s.FailedCount, &s.SkippedCount,
			&s.Status, &s.StartedAt, &s.EndedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan upload session: %w", err)
		}
		if albumName.Valid {
			s.AlbumName = albumName.String
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if sessions == nil {
		sessions = []UploadSessionSummary{}
	}
	return sessions, nil
}

// GetSessionDetails returns the session summary and all its items.
func (s *HistoryStore) GetSessionDetails(sessionID int64) (*UploadSessionDetails, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionQuery := `
	SELECT id, source, album_name, total_files, success_count, failed_count, skipped_count, status, started_at, ended_at
	FROM upload_sessions
	WHERE id = ?
	`
	var summary UploadSessionSummary
	var albumName sql.NullString
	err := s.db.QueryRow(sessionQuery, sessionID).Scan(
		&summary.ID, &summary.Source, &albumName, &summary.TotalFiles,
		&summary.SuccessCount, &summary.FailedCount, &summary.SkippedCount,
		&summary.Status, &summary.StartedAt, &summary.EndedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}
	if albumName.Valid {
		summary.AlbumName = albumName.String
	}

	itemsQuery := `
	SELECT id, session_id, file_path, file_name, file_size, status, error_message, skip_reason, media_key, resolved, created_at, updated_at
	FROM upload_items
	WHERE session_id = ?
	ORDER BY id ASC
	`
	rows, err := s.db.Query(itemsQuery, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query session items: %w", err)
	}
	defer rows.Close()

	var items []UploadItemRecord
	for rows.Next() {
		var it UploadItemRecord
		var errMsg, skipReason, mediaKey sql.NullString
		var resolvedInt int
		if err := rows.Scan(
			&it.ID, &it.SessionID, &it.FilePath, &it.FileName, &it.FileSize,
			&it.Status, &errMsg, &skipReason, &mediaKey,
			&resolvedInt, &it.CreatedAt, &it.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan item record: %w", err)
		}
		if errMsg.Valid {
			it.ErrorMessage = errMsg.String
		}
		if skipReason.Valid {
			it.SkipReason = skipReason.String
		}
		if mediaKey.Valid {
			it.MediaKey = mediaKey.String
		}
		it.Resolved = resolvedInt != 0
		items = append(items, it)
	}
	if items == nil {
		items = []UploadItemRecord{}
	}

	return &UploadSessionDetails{
		Session: summary,
		Items:   items,
	}, nil
}

// GetFailedQueue returns active, unresolved failed items across all sessions.
func (s *HistoryStore) GetFailedQueue() ([]FailedItemSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, session_id, file_path, file_name, file_size, error_message, created_at
	FROM upload_items
	WHERE status = 'failed' AND resolved = 0
	ORDER BY created_at DESC, id DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query failed queue: %w", err)
	}
	defer rows.Close()

	var items []FailedItemSummary
	seenPaths := make(map[string]bool)

	for rows.Next() {
		var it FailedItemSummary
		var errMsg sql.NullString
		if err := rows.Scan(
			&it.ID, &it.SessionID, &it.FilePath, &it.FileName, &it.FileSize,
			&errMsg, &it.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan failed item: %w", err)
		}
		if errMsg.Valid {
			it.ErrorMessage = errMsg.String
		}
		// Avoid duplicate file paths in the active retry queue (keep most recent)
		if !seenPaths[it.FilePath] {
			seenPaths[it.FilePath] = true
			items = append(items, it)
		}
	}
	if items == nil {
		items = []FailedItemSummary{}
	}
	return items, nil
}

// DismissFailedItem marks a failed item as resolved so it won't appear in the queue.
func (s *HistoryStore) DismissFailedItem(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	_, err := s.db.Exec(`UPDATE upload_items SET resolved = 1, updated_at = ? WHERE id = ?`, now, id)
	return err
}

// ClearFailedQueue marks all current unresolved failed items as resolved.
func (s *HistoryStore) ClearFailedQueue() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	_, err := s.db.Exec(`UPDATE upload_items SET resolved = 1, updated_at = ? WHERE status = 'failed' AND resolved = 0`, now)
	return err
}

// ClearHistory removes all sessions and item records.
func (s *HistoryStore) ClearHistory() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.db.Exec(`DELETE FROM upload_items;`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM upload_sessions;`); err != nil {
		return err
	}
	return nil
}

// GetFailedFilesForRetry returns existing on-disk file paths for failed items in a session.
func (s *HistoryStore) GetFailedFilesForRetry(sessionID int64) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT DISTINCT file_path
	FROM upload_items
	WHERE session_id = ? AND status = 'failed'
	`
	rows, err := s.db.Query(query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var existingPaths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		if _, err := os.Stat(p); err == nil {
			existingPaths = append(existingPaths, p)
		}
	}
	if existingPaths == nil {
		existingPaths = []string{}
	}
	return existingPaths, nil
}

// GetAllFailedFilesForRetry returns distinct on-disk file paths for all unresolved failed items.
func (s *HistoryStore) GetAllFailedFilesForRetry() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT DISTINCT file_path
	FROM upload_items
	WHERE status = 'failed' AND resolved = 0
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var existingPaths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		if _, err := os.Stat(p); err == nil {
			existingPaths = append(existingPaths, p)
		}
	}
	if existingPaths == nil {
		existingPaths = []string{}
	}
	return existingPaths, nil
}
