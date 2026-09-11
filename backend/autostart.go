package backend

// SetAutostart enables or disables launching gotohp when the user logs into the system.
func SetAutostart(enabled bool) error {
	return setAutostart(enabled)
}

// IsAutostartEnabled reports whether autostart is currently registered in the OS.
func IsAutostartEnabled() bool {
	return isAutostartEnabled()
}

// SyncAutostart verifies and refreshes autostart registration based on the saved preference.
func SyncAutostart() {
	ensureConfigLoaded()
	configMu.RLock()
	enabled := AppConfig.Preferences.StartWithWindows
	configMu.RUnlock()

	if enabled {
		_ = SetAutostart(true)
	}
}
