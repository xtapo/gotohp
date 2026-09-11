//go:build !cli

package backend

import "github.com/wailsapp/wails/v3/pkg/application"

func init() {
	application.RegisterEvent[AlbumStatus]("albumProgress")
	application.RegisterEvent[AlbumStatus]("albumComplete")
	application.RegisterEvent[AlbumError]("albumError")
	application.RegisterEvent[UploadBatchStart]("uploadStart")
	application.RegisterEvent[application.Void]("uploadStop")
	application.RegisterEvent[FileUploadResult]("FileStatus")
	application.RegisterEvent[ThreadStatus]("ThreadStatus")
	application.RegisterEvent[PreflightWarning]("uploadWarning")
	application.RegisterEvent[application.Void]("uploadCancel")
	application.RegisterEvent[application.Void]("uploadPause")
	application.RegisterEvent[application.Void]("uploadResume")
	application.RegisterEvent[application.Void]("uploadPaused")
	application.RegisterEvent[application.Void]("uploadResumed")
	application.RegisterEvent[int64]("uploadSetBandwidthLimit")
	application.RegisterEvent[int64]("uploadTotalBytes")
	application.RegisterEvent[int64]("uploadTotalBytesDelta")
	application.RegisterEvent[FilesDroppedEvent]("files-dropped")
	application.RegisterEvent[StartUploadEvent]("startUpload")
	application.RegisterEvent[AutoSyncStatus]("autosync:status")
	application.RegisterEvent[AutoSyncFileEvent]("autosync:file-uploaded")
}
