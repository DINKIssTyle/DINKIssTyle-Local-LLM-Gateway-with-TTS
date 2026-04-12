/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package core

import (
	"bytes"
	"context"
	"dinkisstyle-chat/internal/config"
	"dinkisstyle-chat/internal/mcp"
	"dinkisstyle-chat/internal/promptkit"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct for Wails binding
type App struct {
	ctx               context.Context
	server            *http.Server // HTTPS Server
	httpServer        *http.Server // HTTP Compatibility Server
	serverMux         sync.Mutex
	isRunning         bool
	port              string
	llmEndpoint       string
	llmApiToken       string
	llmMode           string // "standard" or "stateful"
	enableTTS         bool
	enableMCP         bool
	enableDebugTrace  bool
	certDomain        string
	authMgr           *AuthManager
	assets            embed.FS
	IsQuitting        bool
	welcomeDismissed  bool
	alwaysShowWelcome bool
	serverUILanguage  string

	// Server-side Model Cache
	modelCache     []byte
	modelCacheMux  sync.RWMutex
	modelCacheTime time.Time

	baseListener    net.Listener // Primary TCP listener for hybrid HTTP/HTTPS
	secondaryServer *http.Server // Secondary HTTP server for backward compatibility
}

// AppConfig holds the persistent application configuration
type AppConfig struct {
	Port              string               `json:"port"`
	LLMEndpoint       string               `json:"llmEndpoint"`
	LLMApiToken       string               `json:"llmApiToken"`
	LLMMode           string               `json:"llmMode"`
	EnableTTS         bool                 `json:"enableTTS"`
	TTS               ServerTTSConfig      `json:"tts"`
	Embedding         EmbeddingModelConfig `json:"embedding"`
	StartOnBoot       bool                 `json:"startOnBoot"`
	MinimizeToTray    bool                 `json:"minimizeToTray"`
	AutoStartServer   bool                 `json:"autoStartServer"`
	CertDomain        string               `json:"certDomain"`
	DebugTraceEnabled bool                 `json:"debugTraceEnabled"`
	WelcomeDismissed  bool                 `json:"welcomeDismissed"`
	AlwaysShowWelcome bool                 `json:"alwaysShowWelcome"`
	ServerUILanguage  string               `json:"serverUILanguage"`
}

type WelcomeState struct {
	ShowModal           bool   `json:"showModal"`
	RequiresMigration   bool   `json:"requiresMigration"`
	MigrationMessage    string `json:"migrationMessage"`
	RequiresTTSDownload bool   `json:"requiresTtsDownload"`
	TTSDownloadMessage  string `json:"ttsDownloadMessage"`
	CanSkipTTSDownload  bool   `json:"canSkipTtsDownload"`
	RestartRecommended  bool   `json:"restartRecommended"`
	PrimaryMessage      string `json:"primaryMessage"`
	SecondaryMessage    string `json:"secondaryMessage"`
}

const (
	legacyMacAppDataDirName = "DKST LLM Chat"
	macAppDataDirName       = "DKST LLM Chat Server"
)

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
// Others: ~/Documents/DKST LLM Chat Server
func GetAppDataDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	exeDir := filepath.Dir(exePath)

	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		return exeDir
	}

	// Mac -> ~/Documents/DKST LLM Chat Server
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return exeDir // Fallback
	}

	docDir := filepath.Join(homeDir, "Documents", macAppDataDirName)
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

		// 1. MacOS Bundle: Check Contents/Resources (Preferred for data)
		if runtime.GOOS == "darwin" {
			resPath := filepath.Join(filepath.Dir(exeDir), "Resources", relativePath)
			if _, err := os.Stat(resPath); err == nil {
				return resPath
			}
		}

		// 2. Check next to executable (Legacy/Standard/onnxruntime)
		prodPath := filepath.Join(exeDir, relativePath)
		if _, err := os.Stat(prodPath); err == nil {
			return prodPath
		}
	}

	// Finally check current working directory (dev mode)
	if _, err := os.Stat(relativePath); err == nil {
		return relativePath
	}

	// Also check in bundle/ for dev mode
	bundlePath := filepath.Join("bundle", relativePath)
	if _, err := os.Stat(bundlePath); err == nil {
		return bundlePath
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
	resourceDir := bundleDir
	if runtime.GOOS == "darwin" {
		resourceDir = filepath.Join(filepath.Dir(bundleDir), "Resources")
	}

	copyIfMissing := func(destRelative string, sourceCandidates ...string) {
		destPath := filepath.Join(appDataDir, destRelative)
		if pathExists(destPath) {
			return
		}
		for _, candidate := range sourceCandidates {
			if !pathExists(candidate) {
				continue
			}
			if err := copyRecursive(candidate, destPath); err != nil {
				fmt.Printf("Setup: failed to copy %s -> %s: %v\n", candidate, destPath, err)
			}
			return
		}
	}

	copyIfMissing("users.json",
		filepath.Join(resourceDir, "users.json"),
		filepath.Join(bundleDir, "users.json"),
		filepath.Join("bundle", "users.json"),
	)
	copyIfMissing("config.json",
		filepath.Join(resourceDir, "config.json"),
		filepath.Join(bundleDir, "config.json"),
		filepath.Join("bundle", "config.json"),
	)
	copyIfMissing("system_prompts.json",
		filepath.Join(resourceDir, "system_prompts.json"),
		filepath.Join(bundleDir, "system_prompts.json"),
		filepath.Join("bundle", "system_prompts.json"),
	)
	copyIfMissing("ThirdPartyNotices.md",
		filepath.Join(resourceDir, "ThirdPartyNotices.md"),
		filepath.Join(bundleDir, "ThirdPartyNotices.md"),
		filepath.Join("bundle", "ThirdPartyNotices.md"),
	)
	copyIfMissing(filepath.Join(assetsDirName, runtimeDirName, onnxRuntimeDirName),
		filepath.Join(resourceDir, assetsDirName, runtimeDirName, onnxRuntimeDirName),
		filepath.Join(bundleDir, assetsDirName, runtimeDirName, onnxRuntimeDirName),
		filepath.Join("bundle", assetsDirName, runtimeDirName, onnxRuntimeDirName),
		filepath.Join(assetsDirName, runtimeDirName, onnxRuntimeDirName),
		filepath.Join("onnxruntime"),
	)
	copyIfMissing(filepath.Join(dictionaryDirName, "Dictionary_editor.py"),
		filepath.Join(resourceDir, dictionaryDirName, "Dictionary_editor.py"),
		filepath.Join(bundleDir, dictionaryDirName, "Dictionary_editor.py"),
		filepath.Join("bundle", dictionaryDirName, "Dictionary_editor.py"),
		filepath.Join(dictionaryDirName, "Dictionary_editor.py"),
		filepath.Join("bundle", "Dictionary_editor.py"),
	)

	dictPatterns := []string{
		filepath.Join(resourceDir, dictionaryDirName, "dictionary_*.txt"),
		filepath.Join(bundleDir, dictionaryDirName, "dictionary_*.txt"),
		filepath.Join("bundle", dictionaryDirName, "dictionary_*.txt"),
		filepath.Join(dictionaryDirName, "dictionary_*.txt"),
		filepath.Join("bundle", "dictionary_*.txt"),
	}
	for _, pattern := range dictPatterns {
		matches, _ := filepath.Glob(pattern)
		for _, srcPath := range matches {
			filename := filepath.Base(srcPath)
			destPath := filepath.Join(appDataDir, dictionaryDirName, filename)
			if pathExists(destPath) {
				continue
			}
			fmt.Printf("Setup: Copying dictionary %s to %s\n", filename, destPath)
			if err := copyRecursive(srcPath, destPath); err != nil {
				fmt.Printf("Setup: failed to copy dictionary %s: %v\n", filename, err)
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
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
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

// NewApp creates a new App instance
func NewApp(assets embed.FS) *App {
	a := &App{
		authMgr: NewAuthManager(GetResourcePath("users.json")),
		assets:  assets,
	}
	globalApp = a
	mcp.SetHelpDocsFS(assets, "frontend/public/help")
	mcp.SetUserMemoryRetentionProvider(func(userID string) (mcp.MemoryRetentionConfig, bool) {
		if strings.TrimSpace(userID) == "" {
			return mcp.DefaultMemoryRetentionConfig(), false
		}
		return a.authMgr.ResolveUserMemoryRetentionConfig(userID), true
	})
	a.loadConfig()
	setDebugTraceCollectorEnabled(a.enableDebugTrace)
	mcp.SetTraceHook(func(ev mcp.TraceEvent) {
		AddDebugTrace(ev.Source, ev.Stage, ev.Message, ev.Details)
	})
	return a
}

func (a *App) reloadUsersFromCurrentStorage() error {
	if a.authMgr == nil {
		return nil
	}
	a.authMgr.usersFile = GetResourcePath("users.json")
	return a.authMgr.LoadUsers()
}

func (a *App) loadConfig() {
	// Set defaults
	a.port = "8080"
	a.llmEndpoint = "http://127.0.0.1:1234"
	a.enableTTS = true
	a.enableMCP = true
	a.llmMode = "stateful"
	a.certDomain = "localhost"
	ttsConfig = ServerTTSConfig{
		Engine:     "supertonic",
		VoiceStyle: "F1.json",
		Speed:      1.0,
		Threads:    1,
		OSRate:     1.0,
		OSPitch:    1.0,
	}
	embeddingConfig = defaultEmbeddingConfig()

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
	// Default to LM Studio if empty
	a.llmMode = "stateful"
	if cfg.LLMMode != "" {
		a.llmMode = cfg.LLMMode
	}
	a.llmApiToken = cfg.LLMApiToken
	a.enableTTS = cfg.EnableTTS
	a.enableDebugTrace = cfg.DebugTraceEnabled
	a.welcomeDismissed = cfg.WelcomeDismissed
	a.alwaysShowWelcome = cfg.AlwaysShowWelcome
	a.serverUILanguage = "ko"
	if strings.EqualFold(cfg.ServerUILanguage, "en") {
		a.serverUILanguage = "en"
	}
	setDebugTraceCollectorEnabled(a.enableDebugTrace)
	if cfg.CertDomain != "" {
		a.certDomain = cfg.CertDomain
	}

	fmt.Printf("[loadConfig] Loaded Config from %s\n", cfgPath)
	fmt.Printf("   -> Port: %s, Endpoint: %s, Mode: %s\n", a.port, a.llmEndpoint, a.llmMode)

	// Update global TTS config if loaded values are valid
	if cfg.TTS.Engine != "" {
		ttsConfig.Engine = cfg.TTS.Engine
	}
	if cfg.TTS.VoiceStyle != "" {
		ttsConfig.VoiceStyle = cfg.TTS.VoiceStyle
	}
	if cfg.TTS.Speed > 0 {
		ttsConfig.Speed = cfg.TTS.Speed
	}
	if cfg.TTS.Threads > 0 {
		ttsConfig.Threads = cfg.TTS.Threads
	}
	ttsConfig.OSVoiceURI = cfg.TTS.OSVoiceURI
	ttsConfig.OSVoiceName = cfg.TTS.OSVoiceName
	ttsConfig.OSVoiceLang = cfg.TTS.OSVoiceLang
	if cfg.TTS.OSRate > 0 {
		ttsConfig.OSRate = cfg.TTS.OSRate
	}
	if cfg.TTS.OSPitch > 0 {
		ttsConfig.OSPitch = cfg.TTS.OSPitch
	}
	embeddingConfig = normalizeEmbeddingConfig(cfg.Embedding)
	applyEmbeddingRuntimeConfig()
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
	cfg.DebugTraceEnabled = a.enableDebugTrace
	cfg.CertDomain = a.certDomain
	cfg.WelcomeDismissed = a.welcomeDismissed
	cfg.AlwaysShowWelcome = a.alwaysShowWelcome
	cfg.ServerUILanguage = a.GetServerUILanguage()
	cfg.TTS = ttsConfig
	cfg.Embedding = currentEmbeddingModelConfig()

	data, err = json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal config: %v\n", err)
		return
	}

	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		fmt.Printf("Failed to save config: %v\n", err)
	}
}

// Startup is called when the app starts
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	globalApp = a
	a.startRetentionMaintenanceLoop()

	// Setup paths for non-Windows
	a.CheckAndSetupPaths()
	if err := a.reloadUsersFromCurrentStorage(); err != nil {
		fmt.Printf("Failed to reload users after path setup: %v\n", err)
	}

	// Reload config now that paths are set up and files potentially copied
	a.loadConfig()
	if a.enableDebugTrace {
		wruntime.WindowSetMinSize(ctx, config.DebugWindowWidth, config.NormalWindowHeight)
		wruntime.WindowSetSize(ctx, config.DebugWindowWidth, config.NormalWindowHeight)
	} else {
		wruntime.WindowSetMinSize(ctx, config.NormalWindowWidth, config.NormalWindowHeight)
		wruntime.WindowSetSize(ctx, config.NormalWindowWidth, config.NormalWindowHeight)
	}

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

	// Initialize TTS only when assets are already present.
	if a.enableTTS && ttsConfig.Engine == "supertonic" && a.CheckAssets() {
		if err := InitTTS(getTTSAssetsDir(), ttsConfig.Threads); err != nil {
			fmt.Printf("Initial TTS Init failed: %v\n", err)
		}
	}
}

func (a *App) startRetentionMaintenanceLoop() {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				if err := mcp.RunRetentionMaintenance(); err != nil {
					log.Printf("[Retention] scheduled maintenance warning: %v", err)
				} else {
					log.Printf("[Retention] scheduled maintenance completed")
				}
			}
		}
	}()
}

// Shutdown is called when the app terminates
func (a *App) Shutdown(ctx context.Context) {
	fmt.Println("Shutting down application...")
	a.StopServer()
	QuitSystemTray()
}

// Quit initiates the application shutdown
func (a *App) Quit() {
	a.IsQuitting = true
	wruntime.Quit(a.ctx)
}

// GetServerStatus returns the current server status
func (a *App) GetServerStatus() map[string]interface{} {
	a.serverMux.Lock()
	running := a.isRunning
	port := a.port
	llmEndpoint := a.llmEndpoint
	llmMode := a.llmMode
	hasAPIToken := a.llmApiToken != ""
	enableTTS := a.enableTTS
	enableMCP := a.enableMCP
	enableDebugTrace := a.enableDebugTrace
	a.serverMux.Unlock()

	if !running && port != "" && isLocalServerHealthy(port) {
		running = true
	}

	llmBusy := currentLLMActivityBusy()

	return map[string]interface{}{
		"running":          running,
		"llmBusy":          llmBusy,
		"port":             port,
		"llmEndpoint":      llmEndpoint,
		"llmMode":          llmMode,
		"hasApiToken":      hasAPIToken,
		"enableTTS":        enableTTS,
		"enableMCP":        enableMCP,
		"enableDebugTrace": enableDebugTrace,
	}
}

func isLocalServerHealthy(port string) bool {
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:" + port + "/api/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
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
	if enabled {
		applyEmbeddingRuntimeConfig()
		if ttsConfig.Engine == "supertonic" && globalTTS == nil {
			go func() {
				if err := InitTTS(getTTSAssetsDir(), ttsConfig.Threads); err != nil {
					fmt.Printf("Dynamic TTS Init failed: %v\n", err)
				}
			}()
		}
	} else {
		destroyTTSRuntime()
		applyEmbeddingRuntimeConfig()
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

// GetDebugTraceEnabled returns whether structured debug tracing is enabled.
func (a *App) GetDebugTraceEnabled() bool {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	return a.enableDebugTrace
}

// SetDebugTraceEnabled enables or disables structured debug tracing.
func (a *App) SetDebugTraceEnabled(enabled bool) {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	a.enableDebugTrace = enabled
	setDebugTraceCollectorEnabled(enabled)
	a.saveConfig()
}

// GetDebugTraceEntries returns the buffered debug trace entries.
func (a *App) GetDebugTraceEntries() []DebugTraceEntry {
	return getDebugTraceEntriesSnapshot()
}

// ClearDebugTrace clears the buffered debug trace entries.
func (a *App) ClearDebugTrace() {
	clearDebugTraceEntries()
}

// GetCertDomain returns the certificate domain
func (a *App) GetCertDomain() string {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	return a.certDomain
}

// SetCertDomain sets the certificate domain
func (a *App) SetCertDomain(domain string) {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	a.certDomain = domain
	a.saveConfig()
}

// GenerateCertificate forces regeneration of a self-signed certificate for the given domain.
func (a *App) GenerateCertificate(domain string) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	appDataDir := GetAppDataDir()
	certDir := getWritableCertDir()
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return err
	}
	certPath := filepath.Join(certDir, domain+".crt")
	keyPath := filepath.Join(certDir, domain+".key")
	derPath := filepath.Join(certDir, domain+".der.crt")

	// Remove existing files to force regeneration
	os.Remove(certPath)
	os.Remove(keyPath)
	os.Remove(derPath)
	os.Remove(filepath.Join(appDataDir, domain+".crt"))
	os.Remove(filepath.Join(appDataDir, domain+".key"))
	os.Remove(filepath.Join(appDataDir, domain+".der.crt"))

	_, _, err := ensureSelfSignedCert(appDataDir, domain)
	if err != nil {
		fmt.Printf("[GenerateCertificate] Error: %v\n", err)
		return err
	}

	fmt.Printf("[GenerateCertificate] Successfully generated certs for %s\n", domain)
	return nil
}

// OpenCertFolder opens the application's data directory in the system file explorer.
func (a *App) OpenCertFolder() error {
	certDir := getWritableCertDir()
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", certDir)
	case "darwin":
		cmd = exec.Command("open", certDir)
	case "linux":
		cmd = exec.Command("xdg-open", certDir)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

func CreateAppMenu(app *App) *menu.Menu {
	men := menu.NewMenu()

	if runtime.GOOS == "darwin" {
		// macOS: AppMenu handles all standard roles (About, Hide, Services, Quit)
		men.Append(menu.AppMenu())
	} else {
		appMenu := men.AddSubmenu("App")
		appMenu.AddText("About DKST LLM Chat Server", keys.CmdOrCtrl("i"), func(_ *menu.CallbackData) {
			app.ShowAbout()
		})
		appMenu.AddSeparator()
		appMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
			app.Quit()
		})
	}

	men.Append(menu.EditMenu())

	windowMenu := men.AddSubmenu("Window")
	windowMenu.AddText("Minimize", keys.CmdOrCtrl("m"), func(_ *menu.CallbackData) {
		if app.ctx != nil {
			wruntime.WindowMinimise(app.ctx)
		}
	})
	windowMenu.AddText("Zoom", nil, func(_ *menu.CallbackData) {
		if app.ctx != nil {
			wruntime.WindowMaximise(app.ctx)
		}
	})

	return men
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
	cfg.Embedding = currentEmbeddingModelConfig()

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

// ---------------------------------------------------------------------------
// Hybrid HTTP/HTTPS Listener logic (Same port protocol detection)
// ---------------------------------------------------------------------------

func (a *App) sniffProtocol(conn net.Conn, tlsChan, httpChan chan net.Conn) {
	// Read the first byte to detect TLS vs HTTP
	// TLS handshake starts with 0x16 (record type: handshake)
	peek := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	n, err := conn.Read(peek)

	// Create a new connection that "peeks" the first byte
	pConn := &peekingConn{
		Conn:     conn,
		peeked:   peek,
		peekLen:  n,
		peekRead: false,
	}

	// Reset deadline for the actual server processing
	conn.SetReadDeadline(time.Time{})

	if err == nil && n > 0 && peek[0] == 0x16 {
		// It's TLS
		select {
		case tlsChan <- pConn:
		case <-time.After(1 * time.Second):
			conn.Close()
		}
	} else {
		// It's likely HTTP (or error/timeout)
		select {
		case httpChan <- pConn:
		case <-time.After(1 * time.Second):
			conn.Close()
		}
	}
}

// peekingConn is a net.Conn that allows re-reading the first peeked byte
type peekingConn struct {
	net.Conn
	peeked   []byte
	peekLen  int
	peekRead bool
}

func (c *peekingConn) Read(b []byte) (int, error) {
	if !c.peekRead && c.peekLen > 0 {
		c.peekRead = true
		n := copy(b, c.peeked[:c.peekLen])
		return n, nil
	}
	return c.Conn.Read(b)
}

// chanListener is a net.Listener that gets connections from a channel
type chanListener struct {
	addr  net.Addr
	conns chan net.Conn
	done  chan struct{}
}

func (l *chanListener) Accept() (net.Conn, error) {
	select {
	case conn, ok := <-l.conns:
		if !ok {
			return nil, fmt.Errorf("listener closed")
		}
		return conn, nil
	case <-l.done:
		return nil, fmt.Errorf("listener shutting down")
	}
}

func (l *chanListener) Close() error {
	// We don't close the chan here to avoid panic on other streamers,
	// but we signal Accept to stop.
	select {
	case <-l.done:
		// already closed
	default:
		close(l.done)
	}
	return nil
}

func (l *chanListener) Addr() net.Addr {
	return l.addr
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
		path := r.URL.Path
		isChatSessionPoll := path == "/api/chat-session/current" || path == "/api/chat-session/events"
		if !isChatSessionPoll {
			log.Printf("[HTTP] %s %s from %s", r.Method, path, r.RemoteAddr)
		}
		mux.ServeHTTP(w, r)
	})

	a.server = &http.Server{
		Addr:    ":" + port,
		Handler: loggingMux,
	}

	// Calculate secondary port for HTTP compatibility (legacy, but we keep it for backward compatibility if needed)
	portInt, _ := strconv.Atoi(port)
	httpPort := strconv.Itoa(portInt + 1)

	// Redirect Handler: Redirects HTTP to HTTPS on the SAME port
	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow MCP and API endpoints to work on HTTP to avoid breaking local clients
		if strings.HasPrefix(r.URL.Path, "/mcp/") || strings.HasPrefix(r.URL.Path, "/api/") {
			loggingMux.ServeHTTP(w, r)
			return
		}

		host := r.Host
		if h, _, err := net.SplitHostPort(r.Host); err == nil {
			host = h
		}
		// We redirect to the SAME port but with https://
		target := fmt.Sprintf("https://%s:%s%s", host, port, r.URL.Path)
		if len(r.URL.RawQuery) > 0 {
			target += "?" + r.URL.RawQuery
		}
		log.Printf("[REDIRECT] HTTP -> HTTPS redirect for %s", r.URL.Path)
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})

	a.httpServer = &http.Server{
		Addr:    ":" + port, // Same port as HTTPS
		Handler: redirectHandler,
	}

	// Cert paths
	certFile, keyFile, err := ensureSelfSignedCert(GetAppDataDir(), a.certDomain)
	if err != nil {
		return fmt.Errorf("failed to ensure self-signed cert: %v", err)
	}

	// Hybrid Protocol Detection Listener
	baseListener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("failed to start listener on port %s: %v", port, err)
	}
	a.baseListener = baseListener

	tlsChan := make(chan net.Conn)
	httpChan := make(chan net.Conn)

	// Accept loop with protocol sniffing
	go func() {
		for {
			conn, err := baseListener.Accept()
			if err != nil {
				if a.isRunning {
					log.Printf("[HYBRID] Accept error: %v", err)
				}
				return
			}
			go a.sniffProtocol(conn, tlsChan, httpChan)
		}
	}()

	// Start HTTPS Server with TLS Listener
	go func() {
		log.Printf("[SERVER] Starting HTTPS Server on :%s (Hybrid)", port)
		tlsListener := &chanListener{
			addr:  baseListener.Addr(),
			conns: tlsChan,
			done:  make(chan struct{}),
		}
		if err := a.server.ServeTLS(tlsListener, certFile, keyFile); err != nil && err != http.ErrServerClosed {
			log.Printf("[SERVER] HTTPS Server error: %v", err)
		}
	}()

	// Start HTTP Redirection Server with HTTP Listener
	go func() {
		log.Printf("[SERVER] Starting HTTP Redirector on :%s (Hybrid)", port)
		httpListener := &chanListener{
			addr:  baseListener.Addr(),
			conns: httpChan,
			done:  make(chan struct{}),
		}
		if err := a.httpServer.Serve(httpListener); err != nil && err != http.ErrServerClosed {
			log.Printf("[SERVER] HTTP Server error: %v", err)
		}
	}()

	// Also start a secondary HTTP port for absolute compatibility if requested (legacy behavior)
	go func() {
		log.Printf("[SERVER] Starting Secondary HTTP Server on :%s", httpPort)
		// We use a separate server instance for this to avoid port collisions
		a.secondaryServer = &http.Server{
			Addr:    ":" + httpPort,
			Handler: redirectHandler,
		}
		if err := a.secondaryServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[SERVER] Secondary HTTP error: %v", err)
		}
	}()

	a.isRunning = true
	UpdateTrayServerState()
	return nil
}

// RestartServer stops and then starts the server with current config
func (a *App) RestartServer() error {
	a.serverMux.Lock()
	port := a.port
	a.serverMux.Unlock()

	log.Println("[SERVER] Restarting server...")
	if err := a.StopServer(); err != nil {
		return fmt.Errorf("failed to stop server: %v", err)
	}

	// Small delay to ensure port is released
	time.Sleep(500 * time.Millisecond)

	return a.StartServer(port)
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
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Close base listener first to stop new connections
	if a.baseListener != nil {
		a.baseListener.Close()
	}

	// 2. Shutdown servers
	if a.server != nil {
		if err := a.server.Shutdown(ctx); err != nil {
			log.Printf("[SERVER] HTTPS Shutdown error: %v", err)
			a.server.Close()
		}
	}

	if a.httpServer != nil {
		if err := a.httpServer.Shutdown(ctx); err != nil {
			log.Printf("[SERVER] HTTP Shutdown error: %v", err)
			a.httpServer.Close()
		}
	}

	if a.secondaryServer != nil {
		if err := a.secondaryServer.Shutdown(ctx); err != nil {
			log.Printf("[SERVER] Secondary HTTP Shutdown error: %v", err)
			a.secondaryServer.Close()
		}
	}

	a.isRunning = false
	fmt.Println("[SERVER] All servers stopped")
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

// LogoutAllSessions invalidates every login session stored in SQLite.
func (a *App) LogoutAllSessions() error {
	return a.authMgr.InvalidateAllSessions()
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

// GetUserMemoryRetentionConfig returns the current memory retention policy for a user.
func (a *App) GetUserMemoryRetentionConfig(id string) (mcp.MemoryRetentionConfig, error) {
	return a.authMgr.GetUserMemoryRetentionConfig(id)
}

// SetUserMemoryRetentionConfig updates the memory retention policy for a user.
func (a *App) SetUserMemoryRetentionConfig(id string, cfg mcp.MemoryRetentionConfig) error {
	return a.authMgr.SetUserMemoryRetentionConfig(id, cfg)
}

// SetUserDisabledTools sets the list of disabled tools for a specific user (exposed to Wails)
func (a *App) SetUserDisabledTools(id string, tools []string) error {
	return a.authMgr.SetUserDisabledTools(id, tools)
}

// GetUserDisabledTools returns the list of disabled tools for a specific user (exposed to Wails)
func (a *App) GetUserDisabledTools(id string) ([]string, error) {
	return a.authMgr.GetUserDisabledTools(id)
}

// SetUserDisallowedCommands sets the list of disallowed commands for a specific user (exposed to Wails)
func (a *App) SetUserDisallowedCommands(id string, cmds []string) error {
	return a.authMgr.SetUserDisallowedCommands(id, cmds)
}

// GetUserDisallowedCommands returns the list of disallowed commands for a specific user (exposed to Wails)
func (a *App) GetUserDisallowedCommands(id string) ([]string, error) {
	return a.authMgr.GetUserDisallowedCommands(id)
}

// SetUserDisallowedDirectories sets the list of disallowed directories for a specific user (exposed to Wails)
func (a *App) SetUserDisallowedDirectories(id string, dirs []string) error {
	return a.authMgr.SetUserDisallowedDirectories(id, dirs)
}

// GetUserDisallowedDirectories returns the list of disallowed directories for a specific user (exposed to Wails)
func (a *App) GetUserDisallowedDirectories(id string) ([]string, error) {
	return a.authMgr.GetUserDisallowedDirectories(id)
}

// GetVoiceStyles returns a list of available voice style files (JSON)
func (a *App) GetVoiceStyles() []string {
	var styles []string
	folder := filepath.Join(getTTSAssetsDir(), legacyTTSVoiceStylesDir)
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

// SetServerTTSConfig replaces the persisted TTS engine settings.
func (a *App) SetServerTTSConfig(cfg ServerTTSConfig) {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()

	if cfg.Engine == "" {
		cfg.Engine = "supertonic"
	}
	if cfg.VoiceStyle == "" {
		cfg.VoiceStyle = ttsConfig.VoiceStyle
	}
	if cfg.Speed <= 0 {
		cfg.Speed = ttsConfig.Speed
	}
	if cfg.Threads <= 0 {
		cfg.Threads = ttsConfig.Threads
	}
	if cfg.OSRate <= 0 {
		cfg.OSRate = 1.0
	}
	if cfg.OSPitch <= 0 {
		cfg.OSPitch = 1.0
	}

	ttsConfig = cfg
	a.saveConfig()

	if a.enableTTS && cfg.Engine == "supertonic" {
		fmt.Printf("Reloading TTS engine with %d threads...\n", cfg.Threads)
		go func() {
			if err := InitTTS(getTTSAssetsDir(), cfg.Threads); err != nil {
				fmt.Printf("Failed to reload TTS after config update: %v\n", err)
			}
		}()
	}
}

// SetTTSThreads updates TTS thread count and reloads model
func (a *App) SetTTSThreads(threads int) {
	if threads <= 0 {
		threads = 4
	}
	ttsConfig.Threads = threads
	a.saveConfig()

	if a.enableTTS && ttsConfig.Engine == "supertonic" {
		fmt.Printf("Reloading TTS with %d threads...\n", threads)
		go func() {
			if err := InitTTS(getTTSAssetsDir(), threads); err != nil {
				fmt.Printf("Failed to reload TTS: %v\n", err)
			}
		}()
	}
}

// CheckAssets checks if required assets exist
func (a *App) CheckAssets() bool {
	assetsDir := getTTSAssetsDir()
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

func destroyTTSRuntime() {
	globalTTSMutex.Lock()
	defer globalTTSMutex.Unlock()
	if globalTTS != nil {
		globalTTS.Destroy()
		globalTTS = nil
	}
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
		if ttsConfig.Engine == "os" {
			result.TTSStatus = "ok"
			result.TTSMessage = "Ready (OS TTS)"
			return result
		}

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

func normalizeLLMMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "stateful" {
		return "stateful"
	}
	return "standard"
}

func sanitizeLLMToken(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	return token
}

func normalizeLLMEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimSuffix(endpoint, "/")
	endpoint = strings.TrimSuffix(endpoint, "/v1")
	return endpoint
}

func (a *App) currentLLMRequestConfig() (string, string, string) {
	a.serverMux.Lock()
	endpoint := strings.TrimSuffix(a.llmEndpoint, "/")
	endpoint = strings.TrimSuffix(endpoint, "/v1")
	mode := a.llmMode
	token := strings.TrimSpace(a.llmApiToken)
	a.serverMux.Unlock()
	return normalizeLLMEndpoint(endpoint), sanitizeLLMToken(token), normalizeLLMMode(mode)
}

// FetchAndCacheModels fetches models from the LLM server and caches them
func (a *App) FetchAndCacheModels() ([]byte, error) {
	endpoint, token, mode := a.currentLLMRequestConfig()
	return a.FetchAndCacheModelsWithConfig(endpoint, token, mode)
}

func (a *App) FetchAndCacheModelsWithConfig(endpoint, token, mode string) ([]byte, error) {
	endpoint = normalizeLLMEndpoint(endpoint)
	token = sanitizeLLMToken(token)
	mode = normalizeLLMMode(mode)
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
	endpoint, token, mode := a.currentLLMRequestConfig()
	return a.LoadModelWithConfig(modelID, endpoint, token, mode)
}

func (a *App) LoadModelWithConfig(modelID, endpoint, token, mode string) error {
	endpoint = normalizeLLMEndpoint(endpoint)
	token = sanitizeLLMToken(token)
	mode = normalizeLLMMode(mode)

	loadURL := endpoint + "/v1/models/load"
	if mode == "stateful" {
		loadURL = endpoint + "/api/v1/models/load"
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

func (a *App) UnloadModel(instanceID string) error {
	endpoint, token, mode := a.currentLLMRequestConfig()
	return a.UnloadModelWithConfig(instanceID, endpoint, token, mode)
}

func (a *App) UnloadModelWithConfig(instanceID, endpoint, token, mode string) error {
	endpoint = normalizeLLMEndpoint(endpoint)
	token = sanitizeLLMToken(token)
	mode = normalizeLLMMode(mode)

	unloadURL := endpoint + "/api/v1/models/unload"
	if mode != "stateful" {
		unloadURL = endpoint + "/v1/models/unload"
	}
	fmt.Printf("[UnloadModel] Requesting unload for instance: %s to %s\n", instanceID, unloadURL)

	payload := map[string]interface{}{}
	if strings.TrimSpace(instanceID) != "" {
		payload["instance_id"] = strings.TrimSpace(instanceID)
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", unloadURL, bytes.NewBuffer(body))
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

	fmt.Printf("[UnloadModel] Successfully unloaded instance: %s\n", instanceID)
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
	assetsDir := getWritableTTSAssetsDir()
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

func (a *App) GetWelcomeState() WelcomeState {
	requiresMigration, migrationMessage := detectLegacyStorageNeedsMigration(GetAppDataDir())
	requiresTTSDownload := !a.CheckAssets()
	showModal := a.alwaysShowWelcome || !a.welcomeDismissed || requiresMigration

	state := WelcomeState{
		ShowModal:           showModal,
		RequiresMigration:   requiresMigration,
		MigrationMessage:    migrationMessage,
		RequiresTTSDownload: requiresTTSDownload,
		TTSDownloadMessage:  "",
		CanSkipTTSDownload:  true,
		RestartRecommended:  requiresMigration,
		PrimaryMessage:      "",
		SecondaryMessage:    "",
	}
	if !requiresMigration && !requiresTTSDownload && a.welcomeDismissed && !a.alwaysShowWelcome {
		state.ShowModal = false
	}
	return state
}

func (a *App) DismissWelcome() {
	a.welcomeDismissed = true
	a.saveConfig()
}

func (a *App) SetAlwaysShowWelcome(enabled bool) {
	a.alwaysShowWelcome = enabled
	a.saveConfig()
}

func (a *App) GetAlwaysShowWelcome() bool {
	return a.alwaysShowWelcome
}

func (a *App) SetServerUILanguage(language string) {
	if strings.EqualFold(strings.TrimSpace(language), "en") {
		a.serverUILanguage = "en"
	} else {
		a.serverUILanguage = "ko"
	}
	a.saveConfig()
}

func (a *App) GetServerUILanguage() string {
	if strings.EqualFold(strings.TrimSpace(a.serverUILanguage), "en") {
		return "en"
	}
	return "ko"
}

// GetLicenseText returns the content of LICENSE files
func (a *App) GetLicenseText() string {
	var builder strings.Builder

	// App/Assets License
	assetsLicensePath := filepath.Join(getTTSAssetsDir(), "LICENSE")
	if content, err := os.ReadFile(assetsLicensePath); err == nil {
		builder.WriteString("=== Assets / Model License ===\n")
		builder.WriteString(string(content))
		builder.WriteString("\n\n")
	}

	// ONNX Runtime License
	onnxLicensePath := filepath.Join(getONNXRuntimeDir(), "LICENSE.txt")
	if content, err := os.ReadFile(onnxLicensePath); err == nil {
		builder.WriteString("=== ONNX Runtime License ===\n")
		builder.WriteString(string(content))
		builder.WriteString("\n\n")
	}

	// Third Party Notices
	tpnPath := GetResourcePath("ThirdPartyNotices.md")
	if content, err := os.ReadFile(tpnPath); err == nil {
		builder.WriteString("=== Third Party Notices ===\n")
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
	filename := getDictionaryFilename(lang)
	dictFile := getWritableDictionaryFilePath(lang)

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
			_ = os.MkdirAll(filepath.Dir(dictFile), 0755)
			os.WriteFile(dictFile, []byte(defaultContent), 0644)
		}
	}

	if !fileExists(dictFile) {
		dictFile = getDictionarySourcePath(lang)
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

type SystemPrompt = promptkit.SystemPrompt

// GetSystemPrompts returns the list of system prompt presets from system_prompts.json
func (a *App) GetSystemPrompts() []SystemPrompt {
	return promptkit.LoadSystemPrompts(GetAppDataDir())
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

// GetAppVersion returns the current application version
func (a *App) GetAppVersion() string {
	return config.AppVersion
}

// GetAboutInfo returns metadata for the About modal
func (a *App) GetAboutInfo() map[string]string {
	return map[string]string{
		"name":      "DKST LLM Chat Server",
		"version":   config.AppVersion,
		"copyright": "(C) 2026 DINKI'ssTyle",
	}
}

// GetWindowDimensions returns the centrally defined window size constants
func (a *App) GetWindowDimensions() map[string]int {
	return map[string]int{
		"normalWidth":  config.NormalWindowWidth,
		"normalHeight": config.NormalWindowHeight,
		"debugWidth":   config.DebugWindowWidth,
		"minWidth":     config.WindowMinWidth,
		"minHeight":    config.WindowMinHeight,
	}
}
