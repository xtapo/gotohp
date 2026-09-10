//go:build !cli

package backend

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// OpenDirectoryDialog opens the OS native directory picker dialog and returns the selected folder.
func (g *ConfigManager) OpenDirectoryDialog() (string, error) {
	app := application.Get()
	if app == nil {
		return "", errors.New("application instance is not ready")
	}

	dialog := app.Dialog.OpenFile()
	dialog.CanChooseDirectories(true)
	dialog.CanChooseFiles(false)
	dialog.SetTitle("Select Folder to Auto-Sync")
	return dialog.PromptForSingleSelection()
}
