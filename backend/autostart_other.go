//go:build !windows

package backend

func setAutostart(enabled bool) error {
	return nil
}

func isAutostartEnabled() bool {
	return false
}
