package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"dinkisstyle-chat/internal/mcp"
)

const chromeForTestingManifestURL = "https://googlechromelabs.github.io/chrome-for-testing/last-known-good-versions-with-downloads.json"

type BrowserAssetStatus struct {
	Supported      bool   `json:"supported"`
	Installed      bool   `json:"installed"`
	Downloading    bool   `json:"downloading"`
	Version        string `json:"version"`
	ExecutablePath string `json:"executablePath"`
	InstallDir     string `json:"installDir"`
	Message        string `json:"message"`
}

type chromeForTestingMetadata struct {
	Version     string `json:"version"`
	DownloadURL string `json:"downloadUrl"`
	Platform    string `json:"platform"`
	InstalledAt string `json:"installedAt"`
}

var browserAssetState struct {
	sync.Mutex
	downloading bool
}

func chromeForTestingExecutablePath() string {
	return filepath.Join(getChromeForTestingInstallDir(), "chrome-headless-shell")
}

func findChromeForTestingExecutable() string {
	path := chromeForTestingExecutablePath()
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Mode()&0111 == 0 {
		return ""
	}
	return path
}

func chromeForTestingPlatform() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("Chrome for Testing management is currently available only on macOS")
	}
	switch runtime.GOARCH {
	case "arm64":
		return "mac-arm64", nil
	case "amd64":
		return "mac-x64", nil
	default:
		return "", fmt.Errorf("unsupported macOS architecture: %s", runtime.GOARCH)
	}
}

func readChromeForTestingMetadata() chromeForTestingMetadata {
	var metadata chromeForTestingMetadata
	data, err := os.ReadFile(filepath.Join(getChromeForTestingInstallDir(), "metadata.json"))
	if err == nil {
		_ = json.Unmarshal(data, &metadata)
	}
	return metadata
}

func (a *App) GetBrowserAssetStatus() BrowserAssetStatus {
	_, platformErr := chromeForTestingPlatform()
	executable := findChromeForTestingExecutable()
	metadata := readChromeForTestingMetadata()

	browserAssetState.Lock()
	downloading := browserAssetState.downloading
	browserAssetState.Unlock()

	status := BrowserAssetStatus{
		Supported:      platformErr == nil,
		Installed:      executable != "",
		Downloading:    downloading,
		Version:        metadata.Version,
		ExecutablePath: executable,
		InstallDir:     getChromeForTestingInstallDir(),
		Message:        "Dedicated Chromium is not installed.",
	}
	if platformErr != nil {
		status.Message = platformErr.Error()
	} else if downloading {
		status.Message = "Downloading and installing Chrome for Testing..."
	} else if status.Installed {
		status.Message = "Dedicated Chromium is ready for MCP web access."
	}
	return status
}

func fetchChromeForTestingDownload() (chromeForTestingMetadata, error) {
	platform, err := chromeForTestingPlatform()
	if err != nil {
		return chromeForTestingMetadata{}, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(chromeForTestingManifestURL)
	if err != nil {
		return chromeForTestingMetadata{}, fmt.Errorf("fetch Chrome for Testing manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return chromeForTestingMetadata{}, fmt.Errorf("fetch Chrome for Testing manifest: HTTP %d", resp.StatusCode)
	}

	var manifest struct {
		Channels map[string]struct {
			Version   string `json:"version"`
			Downloads struct {
				ChromeHeadlessShell []struct {
					Platform string `json:"platform"`
					URL      string `json:"url"`
				} `json:"chrome-headless-shell"`
			} `json:"downloads"`
		} `json:"channels"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&manifest); err != nil {
		return chromeForTestingMetadata{}, fmt.Errorf("decode Chrome for Testing manifest: %w", err)
	}
	stable, ok := manifest.Channels["Stable"]
	if !ok {
		return chromeForTestingMetadata{}, fmt.Errorf("Chrome for Testing manifest has no Stable channel")
	}
	for _, download := range stable.Downloads.ChromeHeadlessShell {
		if download.Platform == platform && strings.HasPrefix(download.URL, "https://") {
			return chromeForTestingMetadata{
				Version:     stable.Version,
				DownloadURL: download.URL,
				Platform:    platform,
			}, nil
		}
	}
	return chromeForTestingMetadata{}, fmt.Errorf("Chrome for Testing has no download for %s", platform)
}

func downloadChromeArchive(url, destination string) error {
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download Chrome for Testing: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > 0 && resp.ContentLength > 500<<20 {
		return fmt.Errorf("Chrome for Testing archive is unexpectedly large")
	}

	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(out, io.LimitReader(resp.Body, (500<<20)+1))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > 500<<20 {
		return fmt.Errorf("Chrome for Testing archive exceeded the size limit")
	}
	return nil
}

func installChromeForTesting(metadata chromeForTestingMetadata) error {
	installDir := getChromeForTestingInstallDir()
	parentDir := filepath.Dir(installDir)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return err
	}
	workDir, err := os.MkdirTemp(parentDir, ".chrome-for-testing-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	archivePath := filepath.Join(workDir, "chrome.zip")
	if err := downloadChromeArchive(metadata.DownloadURL, archivePath); err != nil {
		return err
	}
	extractDir := filepath.Join(workDir, "extract")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return err
	}
	if output, err := exec.Command("/usr/bin/ditto", "-x", "-k", archivePath, extractDir).CombinedOutput(); err != nil {
		return fmt.Errorf("extract Chrome for Testing: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	extractedRuntime := filepath.Join(extractDir, "chrome-headless-shell-"+metadata.Platform)
	extractedExecutable := filepath.Join(extractedRuntime, "chrome-headless-shell")
	if info, err := os.Stat(extractedExecutable); err != nil || info.IsDir() || info.Mode()&0111 == 0 {
		return fmt.Errorf("downloaded archive does not contain a valid Chrome for Testing executable")
	}

	newInstallDir := filepath.Join(workDir, "install")
	if err := os.MkdirAll(newInstallDir, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(extractedRuntime)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.Rename(filepath.Join(extractedRuntime, entry.Name()), filepath.Join(newInstallDir, entry.Name())); err != nil {
			return err
		}
	}
	metadata.InstalledAt = time.Now().UTC().Format(time.RFC3339)
	metadataData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(newInstallDir, "metadata.json"), metadataData, 0644); err != nil {
		return err
	}

	backupDir := installDir + ".previous"
	_ = os.RemoveAll(backupDir)
	if _, err := os.Stat(installDir); err == nil {
		if err := os.Rename(installDir, backupDir); err != nil {
			return err
		}
	}
	if err := os.Rename(newInstallDir, installDir); err != nil {
		_ = os.Rename(backupDir, installDir)
		return err
	}
	_ = os.RemoveAll(backupDir)
	return nil
}

func (a *App) DownloadBrowserAsset() error {
	browserAssetState.Lock()
	if browserAssetState.downloading {
		browserAssetState.Unlock()
		return fmt.Errorf("Chrome for Testing is already being downloaded")
	}
	browserAssetState.downloading = true
	browserAssetState.Unlock()
	defer func() {
		browserAssetState.Lock()
		browserAssetState.downloading = false
		browserAssetState.Unlock()
	}()

	metadata, err := fetchChromeForTestingDownload()
	if err != nil {
		return err
	}
	if err := installChromeForTesting(metadata); err != nil {
		return err
	}
	executable := findChromeForTestingExecutable()
	if executable == "" {
		return fmt.Errorf("Chrome for Testing installation completed without a usable executable")
	}
	mcp.SetBrowserExecutablePath(executable)
	return nil
}
