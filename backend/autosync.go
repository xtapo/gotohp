package backend

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// AutoSyncNotifier defines the interface to broadcast events to listeners (e.g. Wails GUI).
type AutoSyncNotifier interface {
	EmitStatus(status AutoSyncStatus)
	EmitFileUploaded(event AutoSyncFileEvent)
}

// AutoSyncFileEvent is emitted when a file has been successfully uploaded via Auto-Sync.
type AutoSyncFileEvent struct {
	Path     string `json:"path"`
	FileName string `json:"fileName"`
	Size     int64  `json:"size"`
}

// NopAutoSyncNotifier is a no-op notifier for headless/testing environments.
type NopAutoSyncNotifier struct{}

func (NopAutoSyncNotifier) EmitStatus(AutoSyncStatus)          {}
func (NopAutoSyncNotifier) EmitFileUploaded(AutoSyncFileEvent) {}

type fileStat struct {
	size    int64
	modTime int64
}

// AutoSyncManager coordinates filesystem monitoring, file stability checking,
// deduplication via Google Photos hash API, and silent background uploads.
type AutoSyncManager struct {
	configMgr *ConfigManager
	notifier  AutoSyncNotifier
	logger    *slog.Logger

	watcher   *fsnotify.Watcher
	mu        sync.RWMutex
	enabled   bool
	isSyncing bool
	started   bool
	stopCh    chan struct{}
	wg        sync.WaitGroup

	watchedDirs map[string]bool // dir -> true

	// Debouncing
	pendingMu    sync.Mutex
	pendingFiles map[string]time.Time // path -> lastEventTime

	// Upload queue
	queueMu     sync.Mutex
	queue       []string // list of stable file paths
	queueNotify chan struct{}

	// Active concurrent upload tracking
	activeMu        sync.Mutex
	activeFiles     map[string]bool // file path -> true
	activeFileNames map[int]string  // workerID -> baseName

	// In-memory session cache to avoid repeatedly hashing unchanged files
	sessionMu     sync.RWMutex
	sessionSynced map[string]fileStat // canonical path -> {size, modTime}

	// Folder-album synchronization mutex
	albumMu sync.Mutex

	// Status & statistics
	syncedCount  int
	lastSyncTime int64
	currentFile  string
}

// NewAutoSyncManager creates a new AutoSyncManager instance without requiring a local database.
func NewAutoSyncManager(configMgr *ConfigManager, notifier AutoSyncNotifier, logger *slog.Logger) (*AutoSyncManager, error) {
	if configMgr == nil {
		configMgr = &ConfigManager{}
	}
	if notifier == nil {
		notifier = NopAutoSyncNotifier{}
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	mgr := &AutoSyncManager{
		configMgr:       configMgr,
		notifier:        notifier,
		logger:          logger,
		watcher:         watcher,
		stopCh:          make(chan struct{}),
		watchedDirs:     make(map[string]bool),
		pendingFiles:    make(map[string]time.Time),
		queueNotify:     make(chan struct{}, 1),
		activeFiles:     make(map[string]bool),
		activeFileNames: make(map[int]string),
		sessionSynced:   make(map[string]fileStat),
	}

	return mgr, nil
}

func (m *AutoSyncManager) isSessionSynced(path string, size, modTime int64) bool {
	canonical := canonicalUploadPath(path)
	m.sessionMu.RLock()
	defer m.sessionMu.RUnlock()
	stat, ok := m.sessionSynced[canonical]
	if !ok {
		return false
	}
	return stat.size == size && stat.modTime == modTime
}

func (m *AutoSyncManager) markSessionSynced(path string, size, modTime int64) {
	canonical := canonicalUploadPath(path)
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()
	m.sessionSynced[canonical] = fileStat{size: size, modTime: modTime}
}

// Start activates the file watcher and worker routines according to configuration.
func (m *AutoSyncManager) Start() {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	cfg := m.configMgr.GetSettings()
	m.enabled = cfg.AutoSyncEnabled
	m.mu.Unlock()

	m.wg.Add(3)
	go m.watchLoop()
	go m.debounceLoop()
	go m.uploadLoop()

	if m.enabled {
		for _, folder := range cfg.SyncFolders {
			_ = m.WatchFolder(folder)
		}

		if cfg.SyncOnStartup {
			go func() {
				time.Sleep(2 * time.Second)
				_ = m.TriggerSyncNow()
			}()
		}
	}

	m.broadcastStatus()
}

// Stop shuts down watcher and background goroutines cleanly.
func (m *AutoSyncManager) Stop() {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return
	}
	m.started = false
	close(m.stopCh)
	m.mu.Unlock()

	_ = m.watcher.Close()
	m.wg.Wait()
}

// SetEnabled enables or disables auto-sync monitoring.
func (m *AutoSyncManager) SetEnabled(enabled bool) {
	m.mu.Lock()
	prev := m.enabled
	m.enabled = enabled
	m.mu.Unlock()

	if prev == enabled {
		return
	}

	if enabled {
		cfg := m.configMgr.GetSettings()
		for _, folder := range cfg.SyncFolders {
			_ = m.WatchFolder(folder)
		}
		if cfg.SyncOnStartup {
			go func() {
				_ = m.TriggerSyncNow()
			}()
		}
	} else {
		m.removeAllWatched()
	}

	m.broadcastStatus()
}

// WatchFolder registers a folder and its subfolders with the filesystem watcher.
func (m *AutoSyncManager) WatchFolder(folder string) error {
	cleanFolder := filepath.Clean(folder)
	info, err := os.Stat(cleanFolder)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", cleanFolder)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		return nil
	}

	opts := m.configMgr.SessionUploadOptions()
	return filepath.WalkDir(cleanFolder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") || (opts.ExcludePattern != "" && name == opts.ExcludePattern) {
			return filepath.SkipDir
		}
		if !m.watchedDirs[path] {
			if err := m.watcher.Add(path); err == nil {
				m.watchedDirs[path] = true
				m.logger.Debug("AutoSync: watching directory", "path", path)
			}
		}
		return nil
	})
}

// UnwatchFolder removes a folder and all its subfolders from the watcher.
func (m *AutoSyncManager) UnwatchFolder(folder string) error {
	cleanFolder := filepath.Clean(folder)

	m.mu.Lock()
	defer m.mu.Unlock()

	for watched := range m.watchedDirs {
		if watched == cleanFolder || strings.HasPrefix(watched, cleanFolder+string(filepath.Separator)) {
			_ = m.watcher.Remove(watched)
			delete(m.watchedDirs, watched)
			m.logger.Debug("AutoSync: unwatched directory", "path", watched)
		}
	}

	// Also purge any pending files from this folder
	m.pendingMu.Lock()
	for p := range m.pendingFiles {
		if p == cleanFolder || strings.HasPrefix(p, cleanFolder+string(filepath.Separator)) {
			delete(m.pendingFiles, p)
		}
	}
	m.pendingMu.Unlock()

	return nil
}

func (m *AutoSyncManager) removeAllWatched() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for watched := range m.watchedDirs {
		_ = m.watcher.Remove(watched)
		delete(m.watchedDirs, watched)
	}
}

// watchLoop handles raw filesystem events from fsnotify.
func (m *AutoSyncManager) watchLoop() {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopCh:
			return
		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			m.logger.Warn("AutoSync watcher error", "error", err)
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}

			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) == 0 {
				continue
			}

			path := filepath.Clean(event.Name)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}

			// If a new directory was created, watch it too
			if info.IsDir() {
				if event.Op&fsnotify.Create != 0 {
					_ = m.WatchFolder(path)
				}
				continue
			}

			// Verify if the file is supported
			if !isSupportedByGooglePhotos(path) {
				continue
			}

			// Ignore temporary or partial files
			baseName := filepath.Base(path)
			if strings.HasPrefix(baseName, ".") ||
				strings.HasSuffix(baseName, ".tmp") ||
				strings.HasSuffix(baseName, ".crdownload") ||
				strings.HasSuffix(baseName, ".part") {
				continue
			}

			m.pendingMu.Lock()
			m.pendingFiles[path] = time.Now()
			m.pendingMu.Unlock()
		}
	}
}

// debounceLoop inspects pending files and queues them once they are stable.
func (m *AutoSyncManager) debounceLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.processPendingFiles()
		}
	}
}

func (m *AutoSyncManager) processPendingFiles() {
	m.pendingMu.Lock()
	now := time.Now()
	var readyPaths []string

	for path, lastSeen := range m.pendingFiles {
		// Wait at least 2.5 seconds after the last filesystem event
		if now.Sub(lastSeen) < 2500*time.Millisecond {
			continue
		}

		info, err := os.Stat(path)
		if err != nil {
			// File was deleted or moved
			delete(m.pendingFiles, path)
			continue
		}

		if info.Size() == 0 {
			// Zero byte file, still creating or empty
			continue
		}

		// Attempt to open the file to verify it is not locked by another process
		file, err := os.Open(path)
		if err != nil {
			// File is currently locked (e.g. copying/downloading in progress)
			continue
		}
		_ = file.Close()

		delete(m.pendingFiles, path)
		readyPaths = append(readyPaths, path)
	}
	m.pendingMu.Unlock()

	if len(readyPaths) == 0 {
		return
	}

	for _, path := range readyPaths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		if m.isSessionSynced(path, info.Size(), info.ModTime().Unix()) {
			continue
		}

		m.enqueue(path)
	}
}

func (m *AutoSyncManager) enqueue(path string) {
	m.queueMu.Lock()
	for _, p := range m.queue {
		if p == path {
			m.queueMu.Unlock()
			return
		}
	}
	m.queue = append(m.queue, path)
	m.queueMu.Unlock()

	select {
	case m.queueNotify <- struct{}{}:
	default:
	}

	m.broadcastStatus()
}

// uploadLoop consumes items from the queue and uploads them silently in the background.
func (m *AutoSyncManager) uploadLoop() {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopCh:
			return
		case <-m.queueNotify:
			m.processQueue()
		}
	}
}

func (m *AutoSyncManager) processQueue() {
	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		m.queueMu.Lock()
		if len(m.queue) == 0 {
			m.queueMu.Unlock()
			m.mu.Lock()
			m.isSyncing = false
			m.currentFile = ""
			m.mu.Unlock()
			m.broadcastStatus()
			return
		}
		paths := m.queue
		m.queue = nil
		m.queueMu.Unlock()

		opts := m.configMgr.SessionUploadOptions()
		opts = opts.normalized()

		// Filter out files that are already actively uploading or already synced in this session
		var freshPaths []string
		m.activeMu.Lock()
		for _, p := range paths {
			if m.activeFiles[p] {
				continue
			}
			if info, err := os.Stat(p); err == nil {
				if m.isSessionSynced(p, info.Size(), info.ModTime().Unix()) {
					continue
				}
			}
			freshPaths = append(freshPaths, p)
		}
		m.activeMu.Unlock()

		if len(freshPaths) == 0 {
			continue
		}

		// Classify work items (handles Apple Live Photo pairing if enabled)
		workItems, _ := ClassifyUploadWork(freshPaths, LivePhotoClassificationOptions{
			Enabled:             opts.PairLivePhotos,
			SkipIncomplete:      opts.SkipIncompleteLivePhotos,
			IgnoreAppleMetadata: opts.IgnoreAppleMetadata,
			Cancelled: func() bool {
				select {
				case <-m.stopCh:
					return true
				default:
					return false
				}
			},
		}, nil)

		if len(workItems) == 0 {
			continue
		}

		// Determine number of concurrent workers based on user preferences
		numWorkers := opts.Threads
		if numWorkers < 1 {
			numWorkers = 1
		}
		if numWorkers > len(workItems) {
			numWorkers = len(workItems)
		}

		m.mu.Lock()
		m.isSyncing = true
		m.mu.Unlock()
		m.broadcastStatus()

		m.runConcurrentUploads(workItems, opts, numWorkers)
	}
}

func (m *AutoSyncManager) runConcurrentUploads(workItems []UploadWorkItem, opts UploadOptions, numWorkers int) {
	itemChan := make(chan UploadWorkItem, len(workItems))
	for _, item := range workItems {
		itemChan <- item
	}
	close(itemChan)

	var workerWg sync.WaitGroup

	for workerID := 0; workerID < numWorkers; workerID++ {
		workerWg.Add(1)
		go func(id int) {
			defer workerWg.Done()

			api, err := NewApi(opts.Api)
			if err != nil {
				m.logger.Error("AutoSync: failed to initialize API client for worker", "workerID", id, "error", err)
				return
			}

			for item := range itemChan {
				select {
				case <-m.stopCh:
					return
				default:
				}

				m.uploadSingleWorkItem(item, api, opts, id)
			}
		}(workerID)
	}

	workerWg.Wait()
}

func (m *AutoSyncManager) uploadSingleWorkItem(item UploadWorkItem, api *Api, opts UploadOptions, workerID int) {
	primaryPath := uploadWorkPrimaryPath(item)
	allPaths := uploadWorkPaths(item)
	baseName := filepath.Base(primaryPath)

	m.activeMu.Lock()
	for _, p := range allPaths {
		m.activeFiles[p] = true
	}
	m.activeFileNames[workerID] = baseName
	m.activeMu.Unlock()

	m.updateCurrentFileStatus()

	defer func() {
		m.activeMu.Lock()
		for _, p := range allPaths {
			delete(m.activeFiles, p)
		}
		delete(m.activeFileNames, workerID)
		m.activeMu.Unlock()

		m.updateCurrentFileStatus()
	}()

	// Double check if already synced in session
	if info, err := os.Stat(primaryPath); err == nil {
		if m.isSessionSynced(primaryPath, info.Size(), info.ModTime().Unix()) {
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	mediaKey, skipped, err := uploadWorkItem(ctx, api, item, opts, workerID, NopReporter{})
	cancel()

	if err != nil {
		m.logger.Error("AutoSync: upload failed", "path", primaryPath, "workerID", workerID, "error", err)
		return
	}

	// Mark all paths in this work item as session synced
	for _, p := range allPaths {
		if fi, err := os.Stat(p); err == nil {
			m.markSessionSynced(p, fi.Size(), fi.ModTime().Unix())
		}
	}

	m.mu.Lock()
	m.syncedCount++
	m.lastSyncTime = time.Now().Unix()
	m.mu.Unlock()

	m.logger.Info("AutoSync: processed item", "path", primaryPath, "workerID", workerID, "mediaKey", mediaKey, "skipped", skipped)

	// Add to folder album if auto-album is enabled
	if mediaKey != "" {
		m.assignMediaToFolderAlbum(api, primaryPath, mediaKey)
	}

	if !skipped {
		var fileSize int64
		if fi, err := os.Stat(primaryPath); err == nil {
			fileSize = fi.Size()
		}
		m.notifier.EmitFileUploaded(AutoSyncFileEvent{
			Path:     primaryPath,
			FileName: baseName,
			Size:     fileSize,
		})
	}
}

func (m *AutoSyncManager) assignMediaToFolderAlbum(api *Api, primaryPath, mediaKey string) {
	if mediaKey == "" {
		return
	}
	cfg := m.configMgr.GetSettings()
	if !cfg.AutoAlbumEnabled {
		return
	}

	parentDir := filepath.Dir(primaryPath)
	albumTitle := filepath.Base(parentDir)
	if albumTitle == "" || albumTitle == "." || albumTitle == string(filepath.Separator) {
		return
	}

	m.albumMu.Lock()
	defer m.albumMu.Unlock()

	albumKey := m.configMgr.GetFolderAlbumKey(parentDir)
	albumTarget := albumKey
	if albumTarget == "" {
		albumTarget = albumTitle
	}

	albumMgr := NewAlbumManager(api, NopReporter{}, m.logger, m.stopCh)
	albumKeys, err := albumMgr.AddToAlbum([]string{mediaKey}, albumTarget)
	if err != nil && albumTarget != albumTitle {
		// If adding to existing album failed (e.g. deleted remotely), fall back to creating a new album
		albumKeys, err = albumMgr.AddToAlbum([]string{mediaKey}, albumTitle)
	}

	if err != nil {
		m.logger.Warn("AutoSync: failed to add to album", "folder", parentDir, "album", albumTitle, "error", err)
		return
	}

	if len(albumKeys) > 0 && albumKeys[0] != "" {
		m.configMgr.SetFolderAlbumKey(parentDir, albumKeys[0])
	}
}

func (m *AutoSyncManager) updateCurrentFileStatus() {
	m.activeMu.Lock()
	count := len(m.activeFileNames)
	var first string
	for _, name := range m.activeFileNames {
		first = name
		break
	}
	m.activeMu.Unlock()

	m.mu.Lock()
	if count == 0 {
		m.currentFile = ""
	} else if count == 1 {
		m.currentFile = first
	} else {
		m.currentFile = fmt.Sprintf("%s (+%d khác)", first, count-1)
	}
	m.mu.Unlock()

	m.broadcastStatus()
}

// TriggerSyncNow initiates an on-demand catch-up scan across all configured sync folders.
func (m *AutoSyncManager) TriggerSyncNow() error {
	cfg := m.configMgr.GetSettings()
	if len(cfg.SyncFolders) == 0 {
		return nil
	}

	opts := cfg.UploadOptions()
	var discoveredFiles []string

	for _, folder := range cfg.SyncFolders {
		files, err := FilterGooglePhotosFiles([]string{folder}, opts)
		if err != nil {
			continue
		}
		discoveredFiles = append(discoveredFiles, files...)
	}

	for _, file := range discoveredFiles {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if m.isSessionSynced(file, info.Size(), info.ModTime().Unix()) {
			continue
		}
		m.enqueue(file)
	}

	return nil
}

// GetStatus returns the current state of the AutoSyncManager.
func (m *AutoSyncManager) GetStatus() AutoSyncStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.queueMu.Lock()
	qCount := len(m.queue)
	m.queueMu.Unlock()

	cfg := m.configMgr.GetSettings()

	msg := "Idle"
	if !m.enabled {
		msg = "Disabled"
	} else if m.isSyncing {
		m.activeMu.Lock()
		actCount := len(m.activeFileNames)
		m.activeMu.Unlock()
		if actCount > 1 {
			msg = fmt.Sprintf("Đang đồng bộ %d luồng song song (%d trong hàng đợi)", actCount, qCount)
		} else if m.currentFile != "" {
			msg = fmt.Sprintf("Đang đồng bộ: %s", m.currentFile)
		} else {
			msg = "Đang đồng bộ..."
		}
	} else if qCount > 0 {
		msg = fmt.Sprintf("%d items queued", qCount)
	}

	return AutoSyncStatus{
		Enabled:        m.enabled,
		IsSyncing:      m.isSyncing,
		FolderCount:    len(cfg.SyncFolders),
		WatchedFolders: cfg.SyncFolders,
		QueueCount:     qCount,
		SyncedCount:    m.syncedCount,
		LastSyncTime:   m.lastSyncTime,
		CurrentFile:    m.currentFile,
		StatusMessage:  msg,
	}
}

func (m *AutoSyncManager) broadcastStatus() {
	status := m.GetStatus()
	m.notifier.EmitStatus(status)
}
