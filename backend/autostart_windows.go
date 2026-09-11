//go:build windows

package backend

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runRegistryKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	runRegistryName = "gotohp"
)

func setAutostart(enabled bool) error {
	return setAutostartNamed(runRegistryName, enabled)
}

func setAutostartNamed(name string, enabled bool) error {
	if enabled {
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
		exePath, err = filepath.Abs(exePath)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute executable path: %w", err)
		}

		key, _, err := registry.CreateKey(registry.CURRENT_USER, runRegistryKey, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("failed to open run registry key: %w", err)
		}
		defer key.Close()

		cmd := fmt.Sprintf("\"%s\" --hidden", exePath)
		if err := key.SetStringValue(name, cmd); err != nil {
			return fmt.Errorf("failed to write run registry value: %w", err)
		}
		return nil
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to open run registry key: %w", err)
	}
	defer key.Close()

	if err := key.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("failed to delete run registry value: %w", err)
	}
	return nil
}

func isAutostartEnabled() bool {
	return isAutostartNamedEnabled(runRegistryName)
}

func isAutostartNamedEnabled(name string) bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	val, _, err := key.GetStringValue(name)
	if err != nil {
		return false
	}
	return strings.TrimSpace(val) != ""
}
