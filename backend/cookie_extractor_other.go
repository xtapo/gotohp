//go:build (!windows) || cli

package backend

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func extractOAuthTokenFromWindow(_ *application.WebviewWindow) (string, error) {
	return "", errors.New("automatic webview cookie extraction is currently only supported on Windows")
}
