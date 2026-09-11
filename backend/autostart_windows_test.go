//go:build windows

package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestAutostartNamedWindows(t *testing.T) {
	testKey := "gotohp_autostart_unit_test"

	// Ensure clean start
	_ = setAutostartNamed(testKey, false)
	t.Cleanup(func() {
		_ = setAutostartNamed(testKey, false)
	})

	if isAutostartNamedEnabled(testKey) {
		t.Fatalf("expected autostart key %q to not exist initially", testKey)
	}

	// Test enabling
	if err := setAutostartNamed(testKey, true); err != nil {
		t.Fatalf("setAutostartNamed(true) failed: %v", err)
	}

	if !isAutostartNamedEnabled(testKey) {
		t.Fatalf("expected autostart key %q to exist after enable", testKey)
	}

	// Verify command formatting
	exePath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exePath, _ = filepath.Abs(exePath)

	key, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("registry.OpenKey failed: %v", err)
	}
	defer key.Close()

	val, _, err := key.GetStringValue(testKey)
	if err != nil {
		t.Fatalf("GetStringValue failed: %v", err)
	}

	if !strings.Contains(val, "--hidden") {
		t.Errorf("registry command %q does not contain --hidden flag", val)
	}
	if !strings.Contains(val, exePath) {
		t.Errorf("registry command %q does not contain exe path %q", val, exePath)
	}

	// Test disabling
	if err := setAutostartNamed(testKey, false); err != nil {
		t.Fatalf("setAutostartNamed(false) failed: %v", err)
	}

	if isAutostartNamedEnabled(testKey) {
		t.Fatalf("expected autostart key %q to be deleted after disable", testKey)
	}
}
