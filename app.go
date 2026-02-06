/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"bytes"
	"context"
	"dinkisstyle-chat/mcp"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct for Wails binding
type App struct {
	ctx          context.Context
	server       *http.Server
	serverMux    sync.Mutex
	isRunning    bool
	port         string
	llmEndpoint  string
	llmApiToken  string
	llmMode      string // "standard" or "stateful"
	enableTTS    bool
	enableMCP    bool
	enableMemory bool
	authMgr      *AuthManager
	assets       embed.FS
	isQuitting   bool

	// Server-side Model Cache
	modelCache     []byte
	modelCacheMux  sync.RWMutex
	modelCacheTime time.Time
}

// AppConfig holds the persistent application configuration
type AppConfig struct {
	Port            string          `json:"port"`
	LLMEndpoint     string          `json:"llmEndpoint"`
	LLMApiToken     string          `json:"llmApiToken"`
	LLMMode         string          `json:"llmMode"`
	EnableTTS       bool            `json:"enableTTS"`
	EnableMCP       bool            `json:"enableMCP"`
	EnableMemory    bool            `json:"enableMemory"`
	TTS             ServerTTSConfig `json:"tts"`
	StartOnBoot     bool            `json:"startOnBoot"`
	MinimizeToTray  bool            `json:"minimizeToTray"`
	AutoStartServer bool            `json:"autoStartServer"`
}

// HealthCheckResult holds the result of system health checks
type HealthCheckResult struct {
	LLMStatus   string `json:"llmStatus"`   // "ok", "error"
	LLMMessage  string `json:"llmMessage"`  // details
	TTSStatus   string `json:"ttsStatus"`   // "ok", "disabled", "error"
	TTSMessage  string `json:"ttsMessage"`  // details
	ServerModel string `json:"serverModel"` // Loaded model name if available
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
	items := []string{"onnxruntime", "users.json", "config.json", "Dictionary_editor.py", "system_prompts.json"}

	// Copy specific items
	for _, item := range items {
		destPath := filepath.Join(appDataDir, item)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			srcPath := filepath.Join(bundleDir, item)
			if _, err := os.Stat(srcPath); os.IsNotExist(err) {
				if _, err := os.Stat(item); err == nil {
					srcPath = item
				} else {
					continue
				}
			}
			copyRecursive(srcPath, destPath)
		}
	}

	// Copy all dictionary files (dictionary_*.txt)
	// Check bundle first, then CWD
	dictPattern := "dictionary_*.txt"
	matches, _ := filepath.Glob(filepath.Join(bundleDir, dictPattern))
	if len(matches) == 0 {
		matches, _ = filepath.Glob(dictPattern) // CWD check
	}

	for _, srcPath := range matches {
		filename := filepath.Base(srcPath)
		destPath := filepath.Join(appDataDir, filename)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			fmt.Printf("Setup: Copying dictionary %s to %s\n", filename, destPath)
			copyRecursive(srcPath, destPath)
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
	fmt.Printf("Loading config from: %s\n", cfgPath)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Printf("Config file not found, using defaults\n")
		return // Use defaults
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Printf("Failed to parse config: %v\n", err)
		return
	}

	if cfg.Port != "" {
		a.port = cfg.Port
	}
	if cfg.LLMEndpoint != "" {
		a.llmEndpoint = cfg.LLMEndpoint
	}
	// Default to standard if empty
	a.llmMode = "standard"
	if cfg.LLMMode != "" {
		a.llmMode = cfg.LLMMode
	}
	a.llmApiToken = cfg.LLMApiToken
	a.enableTTS = cfg.EnableTTS
	a.enableMCP = cfg.EnableMCP
	a.enableMemory = cfg.EnableMemory

	fmt.Printf("[loadConfig] Loaded Config from %s\n", cfgPath)
	fmt.Printf("   -> Port: %s, Endpoint: %s, Mode: %s, MCP: %v, Memory: %v\n", a.port, a.llmEndpoint, a.llmMode, a.enableMCP, a.enableMemory)

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

	// Proactively sync to MCP context
	mcp.SetContext("default", a.enableMemory)
}

func (a *App) saveConfig() {
	cfgPath := GetResourcePath(configFile)
	fmt.Printf("[saveConfig] Saving config to: %s\n", cfgPath)

	// Read existing config to preserve other fields
	var cfg AppConfig
	data, err := os.ReadFile(cfgPath)
	if err == nil {
		json.Unmarshal(data, &cfg)
	}

	// Update fields managed by this function
	cfg.Port = a.port
	cfg.LLMEndpoint = a.llmEndpoint
	cfg.LLMMode = a.llmMode
	cfg.LLMApiToken = a.llmApiToken
	cfg.EnableTTS = a.enableTTS
	cfg.EnableMCP = a.enableMCP
	cfg.EnableMemory = a.enableMemory
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

	// Initialize Model Cache
	a.loadModelCacheFromDisk()
	// Background fetch to update cache
	go func() {
		fmt.Println("[startup] Starting background model fetch...")
		if _, err := a.FetchAndCacheModels(); err != nil {
			fmt.Printf("[startup] Background model fetch failed: %v\n", err)
		} else {
			fmt.Println("[startup] Background model fetch success")
		}
	}()

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

// shutdown is called when the app terminates
func (a *App) shutdown(ctx context.Context) {
	fmt.Println("Shutting down application...")
	a.StopServer()
	QuitSystemTray()
}

// Quit initiates the application shutdown
func (a *App) Quit() {
	a.isQuitting = true
	wruntime.Quit(a.ctx)
}

// GetServerStatus returns the current server status
func (a *App) GetServerStatus() map[string]interface{} {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	return map[string]interface{}{
		"running":      a.isRunning,
		"port":         a.port,
		"llmEndpoint":  a.llmEndpoint,
		"llmMode":      a.llmMode,
		"hasApiToken":  a.llmApiToken != "",
		"enableTTS":    a.enableTTS,
		"enableMCP":    a.enableMCP,
		"enableMemory": a.enableMemory,
	}
}

// SetLLMEndpoint sets the LLM API endpoint
func (a *App) SetLLMEndpoint(url string) {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	fmt.Printf("[Config] Changing LLM Endpoint from '%s' to '%s'\n", a.llmEndpoint, url)
	a.llmEndpoint = url
	a.saveConfig()
}

// GetLLMApiToken returns the LLM API Token (for UI display)
func (a *App) GetLLMApiToken() string {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	return a.llmApiToken
}

// SetLLMApiToken sets the LLM API Token
func (a *App) SetLLMApiToken(token string) {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()

	masked := ""
	if len(token) > 8 {
		masked = token[:4] + "..." + token[len(token)-4:]
	} else if len(token) > 0 {
		masked = "***"
	}
	fmt.Printf("[Config] Setting API Token: %s (Len: %d)\n", masked, len(token))

	a.llmApiToken = token
	a.saveConfig()
}

// SetLLMMode sets the LLM Mode (standard/stateful)
func (a *App) SetLLMMode(mode string) {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	a.llmMode = mode
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

// SetEnableMCP sets the MCP enabled state
func (a *App) SetEnableMCP(enabled bool) {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	a.enableMCP = enabled
	a.saveConfig()
}

// SetEnableMemory sets the Personal Memory enabled state
func (a *App) SetEnableMemory(enabled bool) {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	a.enableMemory = enabled
	a.saveConfig()

	// Proactively sync to MCP context
	mcp.SetContext("default", enabled)
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
	if cfg.LLMMode == "" {
		cfg.LLMMode = a.llmMode
	}
	// Note: We don't necessarily force Token persist here if it's missing in loaded config
	// but for consistency let's ensure current state is preserved
	cfg.LLMApiToken = a.llmApiToken
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

	// Wrap mux with logging middleware
	loggingMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[HTTP] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		mux.ServeHTTP(w, r)
	})

	a.server = &http.Server{
		Addr:    ":" + port,
		Handler: loggingMux,
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.server.Shutdown(ctx); err != nil {
			a.server.Close() // Force close
			return fmt.Errorf("failed to stop server gracefully: %v", err)
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

// GetUserDetail returns detailed info for a specific user (exposed to Wails)
func (a *App) GetUserDetail(id string) (map[string]interface{}, error) {
	return a.authMgr.GetUserDetail(id)
}

// UpdateUserPassword changes a user's password (exposed to Wails)
func (a *App) UpdateUserPassword(id, newPassword string) error {
	return a.authMgr.UpdatePassword(id, newPassword)
}

// UpdateUserRole changes a user's role (exposed to Wails)
func (a *App) UpdateUserRole(id, role string) error {
	return a.authMgr.UpdateUserRole(id, role)
}

// SetUserApiToken sets API token for a specific user (exposed to Wails)
func (a *App) SetUserApiToken(id, token string) error {
	return a.authMgr.SetUserApiToken(id, token)
}

// GetUserApiToken returns API token for a specific user (exposed to Wails)
func (a *App) GetUserApiToken(id string) (string, error) {
	return a.authMgr.GetUserApiToken(id)
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

// CheckHealth performs a system health check (exposed to Wails)
func (a *App) CheckHealth() HealthCheckResult {
	// ... existing implementation ...
	result := HealthCheckResult{
		LLMStatus:  "ok",
		TTSStatus:  "ok",
		TTSMessage: "Ready",
	}

	// 1. Check LLM Connectivity
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("GET", a.llmEndpoint+"/v1/models", nil)
	if err != nil {
		result.LLMStatus = "error"
		result.LLMMessage = fmt.Sprintf("Request Error: %v", err)
	} else {
		// Add API Token if present
		if a.llmApiToken != "" {
			req.Header.Set("Authorization", "Bearer "+a.llmApiToken)
		}

		resp, err := client.Do(req)
		if err != nil {
			result.LLMStatus = "error"
			result.LLMMessage = fmt.Sprintf("Unreachable: %v", err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				result.LLMStatus = "error"
				result.LLMMessage = fmt.Sprintf("Error: HTTP %d", resp.StatusCode)
			} else {
				// Try to parse model name
				var modelResp struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&modelResp); err == nil && len(modelResp.Data) > 0 {
					// Only report ServerModel if there is exactly one (typical for single-model loaders like LM Studio)
					// If multiple, we can't guess which one is active/intended without more info.
					if len(modelResp.Data) == 1 {
						result.ServerModel = modelResp.Data[0].ID
					} else {
						// Multiple models available, don't confuse user by picking first one
						result.ServerModel = ""
					}
					result.LLMMessage = "Connected"
				} else {
					result.LLMMessage = "Connected (No models)"
				}
			}
		}
	}

	// 2. Check TTS Status
	if !a.enableTTS {
		result.TTSStatus = "disabled"
		result.TTSMessage = "Disabled in settings"
	} else {
		globalTTSMutex.RLock()
		isInit := globalTTS != nil
		globalTTSMutex.RUnlock()

		if !isInit {
			result.TTSStatus = "error"
			result.TTSMessage = "Not initialized (Check assets)"
		}
	}

	return result
}

// FetchAndCacheModels fetches models from the LLM server and caches them
func (a *App) FetchAndCacheModels() ([]byte, error) {
	a.serverMux.Lock()
	endpoint := strings.TrimSuffix(a.llmEndpoint, "/")
	endpoint = strings.TrimSuffix(endpoint, "/v1")
	mode := a.llmMode
	token := strings.TrimSpace(a.llmApiToken)
	a.serverMux.Unlock()

	// Sanitize token
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}

	fmt.Printf("[FetchAndCacheModels] Using LLM Endpoint: '%s' (Original: '%s') Mode: %s\n", endpoint, a.llmEndpoint, mode)

	modelsURL := endpoint + "/v1/models"
	if mode == "stateful" {
		modelsURL = endpoint + "/api/v1/models"
	}

	fmt.Printf("[App] Fetching models from: %s (Mode: %s)\n", modelsURL, mode)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		req.Header.Set("Authorization", "Bearer lm-studio")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Success - Update Cache
	a.modelCacheMux.Lock()
	a.modelCache = bodyBytes
	a.modelCacheTime = time.Now()
	a.modelCacheMux.Unlock()

	// Persist to disk
	cachePath := GetResourcePath("models_cache.json")
	if err := os.WriteFile(cachePath, bodyBytes, 0644); err != nil {
		fmt.Printf("[FetchAndCacheModels] Failed to write cache to disk: %v\n", err)
	} else {
		fmt.Printf("[FetchAndCacheModels] Models cached to %s\n", cachePath)
	}

	return bodyBytes, nil
}

// LoadModel sends a request to load a specific model
func (a *App) LoadModel(modelID string) error {
	a.serverMux.Lock()
	endpoint := strings.TrimSuffix(a.llmEndpoint, "/")
	endpoint = strings.TrimSuffix(endpoint, "/v1")
	mode := a.llmMode
	token := strings.TrimSpace(a.llmApiToken)
	a.serverMux.Unlock()

	// Sanitize token
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}

	loadURL := endpoint + "/v1/models/load"
	// Some servers might use different endpoints.
	if mode == "stateful" {
		// LM Studio defines /v1/models/load for both?
		// Let's assume standard extension unless API varies.
		// Stateful API usually implies chat endpoint behavior, but loading is often global.
		// We'll stick to /v1/models/load as it's the common extension.
	}

	fmt.Printf("[LoadModel] Requesting load for model: %s to %s\n", modelID, loadURL)

	payload := map[string]interface{}{
		"model": modelID,
		// "context_length": ... optional
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 30 * time.Second} // Loading takes time
	req, err := http.NewRequest("POST", loadURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	isMasked := strings.HasPrefix(token, "***") || strings.HasSuffix(token, "...")
	if token != "" && !isMasked {
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		req.Header.Set("Authorization", "Bearer lm-studio")
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	fmt.Printf("[LoadModel] Successfully loaded model: %s\n", modelID)
	return nil
}

// loadModelCacheFromDisk loads the model cache from the file
func (a *App) loadModelCacheFromDisk() {
	cachePath := GetResourcePath("models_cache.json")
	data, err := os.ReadFile(cachePath)
	if err == nil && len(data) > 0 {
		a.modelCacheMux.Lock()
		a.modelCache = data
		a.modelCacheTime = time.Now() // Effectively "old" but loaded
		a.modelCacheMux.Unlock()
		fmt.Printf("[loadModelCacheFromDisk] Loaded %d bytes from %s\n", len(data), cachePath)
	}
}

// GetCachedModels returns the cached models or nil if empty
func (a *App) GetCachedModels() []byte {
	a.modelCacheMux.RLock()
	defer a.modelCacheMux.RUnlock()
	if len(a.modelCache) == 0 {
		return nil
	}
	// Return a copy to be safe
	dst := make([]byte, len(a.modelCache))
	copy(dst, a.modelCache)
	return dst
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

// GetTTSDictionary returns the dictionary for the specified language
func (a *App) GetTTSDictionary(lang string) map[string]string {
	if lang == "" {
		lang = "ko"
	}
	filename := fmt.Sprintf("dictionary_%s.txt", lang)
	dictFile := filepath.Join(GetAppDataDir(), filename)

	// Create default if missing (only for ko/en as examples, or empty for others)
	if _, err := os.Stat(dictFile); os.IsNotExist(err) {
		// Provide basic defaults for ko/en if creating from scratch
		var defaultContent string
		if lang == "ko" {
			defaultContent = "macOS, Mac OS\ndinki, 딩키\n"
		} else if lang == "en" {
			defaultContent = "macOS, Mac O S\nGUI, G U I\n"
		} else {
			defaultContent = "" // Empty for others by default
		}
		if defaultContent != "" {
			os.WriteFile(dictFile, []byte(defaultContent), 0644)
		}
	}

	result := make(map[string]string)
	content, err := os.ReadFile(dictFile)
	if err != nil {
		// fmt.Printf("Failed to read dictionary %s: %v\n", filename, err)
		return result
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if key != "" {
				result[key] = val
			} else {
				fmt.Printf("[Dictionary] Warning: Empty key in %s line %d\n", filename, i+1)
			}
		} else {
			fmt.Printf("[Dictionary] Warning: Malformed line in %s line %d (missing comma)\n", filename, i+1)
		}
	}
	return result
}

// ShowAbout triggers the about modal in the frontend
func (a *App) ShowAbout() {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "show-about")
	}
}

// SystemPrompt represents a single system prompt preset
type SystemPrompt struct {
	Title  string `json:"title"`
	Prompt string `json:"prompt"`
}

// GetSystemPrompts returns the list of system prompt presets from system_prompts.json
func (a *App) GetSystemPrompts() []SystemPrompt {
	promptsFile := filepath.Join(GetAppDataDir(), "system_prompts.json")

	// Create default if missing
	if _, err := os.Stat(promptsFile); os.IsNotExist(err) {
		defaultPrompts := []SystemPrompt{
			{Title: "Default", Prompt: "You are a helpful AI assistant."},
		}
		if data, err := json.MarshalIndent(defaultPrompts, "", "  "); err == nil {
			os.WriteFile(promptsFile, data, 0644)
		}
	}

	content, err := os.ReadFile(promptsFile)
	if err != nil {
		fmt.Printf("[Prompts] Failed to read system_prompts.json: %v\n", err)
		return []SystemPrompt{{Title: "Default", Prompt: "You are a helpful AI assistant."}}
	}

	var prompts []SystemPrompt
	if err := json.Unmarshal(content, &prompts); err != nil {
		fmt.Printf("[Prompts] Failed to parse system_prompts.json: %v\n", err)
		return []SystemPrompt{{Title: "Default", Prompt: "You are a helpful AI assistant."}}
	}

	return prompts
}

// Show makes the window visible
func (a *App) Show() {
	if a.ctx != nil {
		wruntime.WindowShow(a.ctx)
	}
}

// OpenMemoryFolder opens the folder containing the user's memory files
func (a *App) OpenMemoryFolder(userID string) string {
	dir, err := mcp.GetUserMemoryDir(userID)
	if err != nil {
		return fmt.Sprintf("Error determining path: %v", err)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Sprintf("Error creating directory: %v", err)
	}

	// Open the directory
	if runtime.GOOS == "darwin" {
		wruntime.BrowserOpenURL(a.ctx, "file://"+dir)
	} else if runtime.GOOS == "windows" {
		wruntime.BrowserOpenURL(a.ctx, dir) // Windows usually works with path
	} else {
		wruntime.BrowserOpenURL(a.ctx, "file://"+dir)
	}
	return ""
}

// ResetMemory clears all memory files in the user's memory folder
func (a *App) ResetMemory(userID string) string {
	dir, err := mcp.GetUserMemoryDir(userID)
	if err != nil {
		return fmt.Sprintf("Error determining path: %v", err)
	}

	// Reset all .md files in the directory
	files := []string{"personal.md", "work.md", "log.md", "index.md", "index.json"}
	for _, f := range files {
		filePath := filepath.Join(dir, f)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			log.Printf("[Memory] Failed to remove %s: %v", f, err)
		}
	}

	return "Memory reset successfully."
}
