//go:build cli

package backend

import "errors"

// OpenDirectoryDialog stub for CLI mode.
func (g *ConfigManager) OpenDirectoryDialog() (string, error) {
	return "", errors.New("directory dialog is not available in CLI mode")
}
