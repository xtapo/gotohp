//go:build cli

package backend

import "errors"

// StartInAppGoogleLogin stub for CLI mode.
func (g *ConfigManager) StartInAppGoogleLogin() (string, error) {
	return "", errors.New("in-app login is not supported in CLI mode; use 'gotohp-cli creds add' instead")
}

// CancelInAppGoogleLogin stub for CLI mode.
func (g *ConfigManager) CancelInAppGoogleLogin() error {
	return nil
}

// OpenInAppGooglePhotos stub for CLI mode.
func (g *ConfigManager) OpenInAppGooglePhotos(rawURL string) error {
	return errors.New("in-app browser is not supported in CLI mode")
}
