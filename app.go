/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct for Wails binding
type App struct {
	ctx         context.Context
	server      *http.Server
	serverMux   sync.Mutex
	isRunning   bool
	port        string
	llmEndpoint string
	enableTTS   bool
	authMgr     *AuthManager
	assets      embed.FS
}

// AppConfig holds the persistent application configuration
type AppConfig struct {
	Port            string          `json:"port"`
	LLMEndpoint     string          `json:"llmEndpoint"`
	EnableTTS       bool            `json:"enableTTS"`
	TTS             ServerTTSConfig `json:"tts"`
	StartOnBoot     bool            `json:"startOnBoot"`
	MinimizeToTray  bool            `json:"minimizeToTray"`
	AutoStartServer bool            `json:"autoStartServer"`
}

var configFile = "config.json"

// GetAppDataDir returns the application data directory
// Windows: Executable directory
// Others: ~/Documents/DKST-LLM-Chat
func GetAppDataDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	exeDir := filepath.Dir(exePath)

	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		return exeDir
	}

	// Mac -> ~/Documents/DKST-LLM-Chat
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return exeDir // Fallback
	}

	docDir := filepath.Join(homeDir, "Documents", "DKST LLM Chat")
	if err := os.MkdirAll(docDir, 0755); err != nil {
		return exeDir // Fallback
	}
	return docDir
}

// GetResourcePath returns the absolute path for a resource
// It handles running from source (cwd) and running from a bundle (executable dir)
func GetResourcePath(relativePath string) string {
	if filepath.IsAbs(relativePath) {
		return relativePath
	}

	// Check AppDataDir first (deployment/production priority)
	appDataDir := GetAppDataDir()
	fullPath := filepath.Join(appDataDir, relativePath)
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath
	}

	// Then check relative to executable (bootstrap/bundle source)
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		prodPath := filepath.Join(exeDir, relativePath)
		if _, err := os.Stat(prodPath); err == nil {
			return prodPath
		}
	}

	// Finally check current working directory (dev mode)
	if _, err := os.Stat(relativePath); err == nil {
		return relativePath
	}

	// Default to AppDataDir path even if missing (for creation)
	return fullPath
}

// CheckAndSetupPaths ensures required files/folders exist in AppDataDir
func (a *App) CheckAndSetupPaths() {
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		return // Portable mode, expect files next to exe
	}

	appDataDir := GetAppDataDir()
	exePath, _ := os.Executable()
	bundleDir := filepath.Dir(exePath)

	// List of things to copy from bundle to AppDataDir if missing
	items := []string{"onnxruntime", "users.json", "config.json"}

	for _, item := range items {
		destPath := filepath.Join(appDataDir, item)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			srcPath := filepath.Join(bundleDir, item)
			// Try finding in CWD if not in bundle (dev mode fallback)
			if _, err := os.Stat(srcPath); os.IsNotExist(err) {
				if _, err := os.Stat(item); err == nil {
					srcPath = item
				} else {
					continue // Source missing, skip
				}
			}

			fmt.Printf("Setup: Copying %s to %s\n", srcPath, destPath)
			if err := copyRecursive(srcPath, destPath); err != nil {
				fmt.Printf("Failed to copy %s: %v\n", item, err)
			}
		}
	}
}

func copyRecursive(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if err := os.MkdirAll(dst, 0755); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyRecursive(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
				return err
			}
		}
	} else {
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer out.Close()

		if _, err := io.Copy(out, in); err != nil {
			return err
		}
	}
	return nil
}

// Helper to avoid import mess in this tool call, I'll add the function fully in next steps or assuming imports.
// To do this cleanly, I'll read app.go imports first.

// NewApp creates a new App instance
func NewApp(assets embed.FS) *App {
	a := &App{
		authMgr: NewAuthManager(GetResourcePath("users.json")),
		assets:  assets,
	}
	a.loadConfig()
	return a
}

func (a *App) loadConfig() {
	// Set defaults
	a.port = "8080"
	a.llmEndpoint = "http://127.0.0.1:1234"
	a.enableTTS = false
	a.enableTTS = false
	ttsConfig = ServerTTSConfig{VoiceStyle: "M1.json", Speed: 1.0, Threads: 4}

	cfgPath := GetResourcePath(configFile)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return // Use defaults
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}

	if cfg.Port != "" {
		a.port = cfg.Port
	}
	if cfg.LLMEndpoint != "" {
		a.llmEndpoint = cfg.LLMEndpoint
	}
	a.enableTTS = cfg.EnableTTS

	// Update global TTS config if loaded values are valid
	if cfg.TTS.VoiceStyle != "" {
		ttsConfig.VoiceStyle = cfg.TTS.VoiceStyle
	}
	if cfg.TTS.Speed > 0 {
		ttsConfig.Speed = cfg.TTS.Speed
	}
	if cfg.TTS.Threads > 0 {
		ttsConfig.Threads = cfg.TTS.Threads
	}
}

func (a *App) saveConfig() {
	cfgPath := GetResourcePath(configFile)

	// Read existing config to preserve other fields
	var cfg AppConfig
	data, err := os.ReadFile(cfgPath)
	if err == nil {
		json.Unmarshal(data, &cfg)
	}

	// Update fields managed by this function
	cfg.Port = a.port
	cfg.LLMEndpoint = a.llmEndpoint
	cfg.EnableTTS = a.enableTTS
	cfg.TTS = ttsConfig

	data, err = json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal config: %v\n", err)
		return
	}

	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		fmt.Printf("Failed to save config: %v\n", err)
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Setup paths for non-Windows
	a.CheckAndSetupPaths()

	// Reload config now that paths are set up and files potentially copied
	a.loadConfig()

	// Check for Auto Start Server
	if a.GetAutoStartServer() {
		fmt.Println("Auto-starting server based on configuration...")
		go a.StartServerWithCurrentConfig()
	}

	// Initialize TTS if enabled
	if a.enableTTS {
		if !a.CheckAssets() {
			selection, err := wruntime.MessageDialog(ctx, wruntime.MessageDialogOptions{
				Type:          wruntime.QuestionDialog,
				Title:         "TTS Assets Missing",
				Message:       "Required TTS models are missing. Do you want to download them now? (approx. 300MB)\nThe application might pause while downloading.",
				Buttons:       []string{"Yes", "No"},
				DefaultButton: "Yes",
				CancelButton:  "No",
			})

			if err == nil && selection == "Yes" {
				// Show info dialog that download is starting
				wruntime.MessageDialog(ctx, wruntime.MessageDialogOptions{
					Type:    wruntime.InfoDialog,
					Title:   "Downloading Assets",
					Message: "Download starting. Please check the terminal for progress if attached.\nThe window might be unresponsive until download completes.",
				})

				if err := a.DownloadAssets(); err != nil {
					wruntime.MessageDialog(ctx, wruntime.MessageDialogOptions{
						Type:    wruntime.ErrorDialog,
						Title:   "Download Failed",
						Message: fmt.Sprintf("Failed to download assets: %v", err),
					})
					return
				}

				wruntime.MessageDialog(ctx, wruntime.MessageDialogOptions{
					Type:    wruntime.InfoDialog,
					Title:   "Download Complete",
					Message: "TTS assets downloaded successfully.",
				})

				// Notify frontend
				wruntime.EventsEmit(ctx, "assets-ready")
			} else {
				fmt.Println("TTS assets download skipped by user. TTS disabled.")
				return
			}
		}

		if err := InitTTS(GetResourcePath("assets"), ttsConfig.Threads); err != nil {
			fmt.Printf("Initial TTS Init failed: %v\n", err)
		}
	}
}

// GetServerStatus returns the current server status
func (a *App) GetServerStatus() map[string]interface{} {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	return map[string]interface{}{
		"running":     a.isRunning,
		"port":        a.port,
		"llmEndpoint": a.llmEndpoint,
		"enableTTS":   a.enableTTS,
	}
}

// SetLLMEndpoint sets the LLM API endpoint
func (a *App) SetLLMEndpoint(url string) {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	a.llmEndpoint = url
	a.saveConfig()
}

// SetEnableTTS enables or disables TTS
func (a *App) SetEnableTTS(enabled bool) {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	a.enableTTS = enabled
	if enabled && globalTTS == nil {
		go func() {
			if err := InitTTS(GetResourcePath("assets"), ttsConfig.Threads); err != nil {
				fmt.Printf("Dynamic TTS Init failed: %v\n", err)
			}
		}()
	}
	a.saveConfig()
}

// Startup Settings - exposed to Wails frontend

// SetStartOnBoot enables/disables start on boot
func (a *App) SetStartOnBoot(enabled bool) {
	if enabled {
		if err := RegisterStartup(); err != nil {
			fmt.Printf("Failed to register startup: %v\n", err)
		}
	} else {
		if err := UnregisterStartup(); err != nil {
			fmt.Printf("Failed to unregister startup: %v\n", err)
		}
	}
	a.saveStartupSetting("startOnBoot", enabled)
}

// GetStartOnBoot returns the start on boot setting
func (a *App) GetStartOnBoot() bool {
	return a.loadStartupSetting("startOnBoot")
}

// SetMinimizeToTray enables/disables minimize to tray
func (a *App) SetMinimizeToTray(enabled bool) {
	a.saveStartupSetting("minimizeToTray", enabled)
}

// GetMinimizeToTray returns the minimize to tray setting
func (a *App) GetMinimizeToTray() bool {
	// Default to true - loadStartupSetting returns false if not set
	// so we need to check if the key exists in config
	cfgPath := GetResourcePath(configFile)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return true // Default to true
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return true // Default to true
	}
	return cfg.MinimizeToTray
}

// SetAutoStartServer enables/disables auto start server
func (a *App) SetAutoStartServer(enabled bool) {
	a.saveStartupSetting("autoStartServer", enabled)
}

// GetAutoStartServer returns the auto start server setting
func (a *App) GetAutoStartServer() bool {
	return a.loadStartupSetting("autoStartServer")
}

// Helper methods for startup settings persistence
func (a *App) saveStartupSetting(key string, value bool) {
	cfgPath := GetResourcePath(configFile)
	data, err := os.ReadFile(cfgPath)

	var cfg AppConfig
	if err == nil {
		json.Unmarshal(data, &cfg)
	}

	switch key {
	case "startOnBoot":
		cfg.StartOnBoot = value
	case "minimizeToTray":
		cfg.MinimizeToTray = value
	case "autoStartServer":
		cfg.AutoStartServer = value
	}

	// Preserve existing values
	if cfg.Port == "" {
		cfg.Port = a.port
	}
	if cfg.LLMEndpoint == "" {
		cfg.LLMEndpoint = a.llmEndpoint
	}
	cfg.EnableTTS = a.enableTTS
	cfg.TTS = ttsConfig

	newData, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, newData, 0644)
}

func (a *App) loadStartupSetting(key string) bool {
	cfgPath := GetResourcePath(configFile)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return false
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false
	}

	switch key {
	case "startOnBoot":
		return cfg.StartOnBoot
	case "minimizeToTray":
		return cfg.MinimizeToTray
	case "autoStartServer":
		return cfg.AutoStartServer
	}
	return false
}

// StartServer starts the HTTP server on the specified port
func (a *App) StartServer(port string) error {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()

	if a.isRunning {
		return fmt.Errorf("server is already running on port %s", a.port)
	}

	a.port = port
	mux := createServerMux(a, a.authMgr)
	a.server = &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	a.isRunning = true
	fmt.Printf("Server started on http://localhost:%s\n", port)
	a.saveConfig()
	UpdateTrayServerState()
	return nil
}

// StartServerWithCurrentConfig starts the server using the current port configuration
func (a *App) StartServerWithCurrentConfig() error {
	port := a.port
	if port == "" {
		port = "7860"
	}
	return a.StartServer(port)
}

// SetPort sets the server port
func (a *App) SetPort(port string) {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	a.port = port
	a.saveConfig()
}

// StopServer stops the HTTP server
func (a *App) StopServer() error {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()

	if !a.isRunning {
		return fmt.Errorf("server is not running")
	}

	if a.server != nil {
		if err := a.server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("failed to stop server: %v", err)
		}
	}

	a.isRunning = false
	fmt.Println("Server stopped")
	UpdateTrayServerState()
	return nil
}

// GetUsers returns list of users (exposed to Wails)
func (a *App) GetUsers() []map[string]string {
	return a.authMgr.GetUsers()
}

// AddUser adds a new user (exposed to Wails)
func (a *App) AddUser(id, password, role string) error {
	return a.authMgr.AddUser(id, password, role)
}

// DeleteUser removes a user (exposed to Wails)
func (a *App) DeleteUser(id string) error {
	return a.authMgr.DeleteUser(id)
}

// GetVoiceStyles returns a list of available voice style files (JSON)
func (a *App) GetVoiceStyles() []string {
	var styles []string
	folder := GetResourcePath(filepath.Join("assets", "voice_styles"))
	entries, err := os.ReadDir(folder)
	if err != nil {
		return styles
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			styles = append(styles, entry.Name())
		}
	}
	return styles
}

// GetTTSConfig returns current TTS configuration
func (a *App) GetTTSConfig() ServerTTSConfig {
	return ttsConfig
}

// SetTTSConfig updates the global TTS configuration
func (a *App) SetTTSConfig(style string, speed float32) {
	ttsConfig.VoiceStyle = style
	ttsConfig.Speed = speed
	a.saveConfig()
}

// SetTTSThreads updates TTS thread count and reloads model
func (a *App) SetTTSThreads(threads int) {
	if threads <= 0 {
		threads = 4
	}
	ttsConfig.Threads = threads
	a.saveConfig()

	if a.enableTTS {
		fmt.Printf("Reloading TTS with %d threads...\n", threads)
		go func() {
			if err := InitTTS(GetResourcePath("assets"), threads); err != nil {
				fmt.Printf("Failed to reload TTS: %v\n", err)
			}
		}()
	}
}

// CheckAssets checks if required assets exist
func (a *App) CheckAssets() bool {
	assetsDir := GetResourcePath("assets")
	requiredFiles := []string{
		"onnx/duration_predictor.onnx",
		"onnx/text_encoder.onnx",
		"onnx/vector_estimator.onnx",
		"onnx/vocoder.onnx",
		"onnx/unicode_indexer.json",
		"LICENSE",
		"voice_styles/M1.json",
		"voice_styles/F1.json",
	}

	for _, file := range requiredFiles {
		if _, err := os.Stat(filepath.Join(assetsDir, file)); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// DownloadAssets downloads missing assets
func (a *App) DownloadAssets() error {
	downloader := NewDownloader()
	assetsDir := filepath.Join(GetAppDataDir(), "assets")
	if err := downloader.DownloadAssets(assetsDir); err != nil {
		return err
	}

	// Initialize TTS after download
	if err := InitTTS(assetsDir, 4); err != nil {
		return fmt.Errorf("download succeeded but TTS init failed: %w", err)
	}

	// Notify frontend
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "assets-ready")
	}

	return nil
}

// GetLicenseText returns the content of LICENSE files
func (a *App) GetLicenseText() string {
	var builder strings.Builder

	// App/Assets License
	assetsLicensePath := GetResourcePath(filepath.Join("assets", "LICENSE"))
	if content, err := os.ReadFile(assetsLicensePath); err == nil {
		builder.WriteString("=== Assets / Model License ===\n")
		builder.WriteString(string(content))
		builder.WriteString("\n\n")
	}

	// ONNX Runtime License
	onnxLicensePath := GetResourcePath(filepath.Join("onnxruntime", "LICENSE.txt"))
	if content, err := os.ReadFile(onnxLicensePath); err == nil {
		builder.WriteString("=== ONNX Runtime License ===\n")
		builder.WriteString(string(content))
		builder.WriteString("\n\n")
	}

	return builder.String()
}

// ShowAbout triggers the about modal in the frontend
func (a *App) ShowAbout() {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "show-about")
	}
}
