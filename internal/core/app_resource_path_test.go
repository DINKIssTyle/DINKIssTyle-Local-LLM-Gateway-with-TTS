package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveResourcePathPrefersAppDataOverBundleOnMac(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	appDataDir := filepath.Join(tmpDir, "Documents", macAppDataDirName)
	bundleDir := filepath.Join(tmpDir, "DKST LLM Chat Server.app", "Contents", "Resources")
	exePath := filepath.Join(tmpDir, "DKST LLM Chat Server.app", "Contents", "MacOS", "DKST LLM Chat Server")

	appConfig := filepath.Join(appDataDir, "config.json")
	bundleConfig := filepath.Join(bundleDir, "config.json")

	mustWriteFile(t, appConfig, []byte(`{"port":"9443"}`))
	mustWriteFile(t, bundleConfig, []byte(`{"port":"8443"}`))

	resolved := resolveResourcePath("config.json", appDataDir, exePath, "darwin")
	if resolved != appConfig {
		t.Fatalf("expected app data config to win, got %s", resolved)
	}
}

func TestResolveResourcePathFallsBackToBundleOnMac(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	appDataDir := filepath.Join(tmpDir, "Documents", macAppDataDirName)
	bundleConfig := filepath.Join(tmpDir, "DKST LLM Chat Server.app", "Contents", "Resources", "config.json")
	exePath := filepath.Join(tmpDir, "DKST LLM Chat Server.app", "Contents", "MacOS", "DKST LLM Chat Server")

	mustWriteFile(t, bundleConfig, []byte(`{"port":"8443"}`))

	resolved := resolveResourcePath("config.json", appDataDir, exePath, "darwin")
	if resolved != bundleConfig {
		t.Fatalf("expected bundled config fallback, got %s", resolved)
	}
}

func TestResolveResourcePathUsesExecutableDirOnWindows(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "DKST LLM Chat Server.exe")
	exeDir := filepath.Dir(exePath)
	configPath := filepath.Join(exeDir, "config.json")

	mustWriteFile(t, configPath, []byte(`{"port":"8443"}`))

	resolved := resolveResourcePath("config.json", exeDir, exePath, "windows")
	if resolved != configPath {
		t.Fatalf("expected executable-dir config on windows, got %s", resolved)
	}
}

func TestResolveResourcePathUsesExecutableDirOnLinux(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "DKST LLM Chat Server")
	exeDir := filepath.Dir(exePath)
	configPath := filepath.Join(exeDir, "config.json")

	mustWriteFile(t, configPath, []byte(`{"port":"8443"}`))

	resolved := resolveResourcePath("config.json", exeDir, exePath, "linux")
	if resolved != configPath {
		t.Fatalf("expected executable-dir config on linux, got %s", resolved)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
