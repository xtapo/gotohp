//go:build !cli

package backend

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// WailsAutoSyncNotifier delivers auto-sync events to the Wails event bus.
type WailsAutoSyncNotifier struct {
	app *application.App
}

// NewWailsAutoSyncNotifier returns a new WailsAutoSyncNotifier.
func NewWailsAutoSyncNotifier(app *application.App) *WailsAutoSyncNotifier {
	return &WailsAutoSyncNotifier{app: app}
}

func (w *WailsAutoSyncNotifier) EmitStatus(status AutoSyncStatus) {
	if w.app != nil {
		w.app.Event.Emit("autosync:status", status)
	}
}

func (w *WailsAutoSyncNotifier) EmitFileUploaded(event AutoSyncFileEvent) {
	if w.app != nil {
		w.app.Event.Emit("autosync:file-uploaded", event)
	}
}
