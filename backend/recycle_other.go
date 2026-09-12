//go:build !windows

package backend

import (
	"os"
)

// moveToRecycleBin on non-Windows platforms falls back to os.Remove.
func moveToRecycleBin(path string) error {
	return os.Remove(path)
}
