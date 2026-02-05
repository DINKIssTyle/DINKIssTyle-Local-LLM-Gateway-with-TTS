/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"bufio"
	"bytes"
	"dinkisstyle-chat/mcp"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	globalTTS *TextToSpeech
	ttsConfig = ServerTTSConfig{
		VoiceStyle: "M1.json",
		Speed:      1.0,
	}
	// Style Cache
	styleCache = make(map[string]*Style)
	styleMutex sync.Mutex
	// Global TTS Mutex
	globalTTSMutex sync.RWMutex
)

type ServerTTSConfig struct {
	VoiceStyle string  `json:"voiceStyle"`
	Speed      float32 `json:"speed"`
	Threads    int     `json:"threads"`
}

// createServerMux creates the HTTP handler mux for the server
func createServerMux(app *App, authMgr *AuthManager) *http.ServeMux {
	mux := http.NewServeMux()

	// Public endpoints (no auth required)
	mux.HandleFunc("/api/login", handleLogin(authMgr))
	mux.HandleFunc("/api/logout", handleLogout(authMgr))
	mux.HandleFunc("/api/auth/check", handleAuthCheck(authMgr))
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(app.CheckHealth())
	})

	// Protected API endpoints
	mux.HandleFunc("/api/chat", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		handleChat(w, r, app, authMgr)
	}))
	mux.HandleFunc("/api/tts", AuthMiddleware(authMgr, handleTTS))

	// MCP Endpoints (Conditional)
	if app.enableMCP {
		log.Println("[Server] MCP Support Enabled")
		// Import mcp package is required. Ensure imports are correct.
		// We need to add "dinkisstyle-chat/mcp" to imports if not present.
		// Since we cannot easily check imports here, we assume it's imported or will fix it.
		// Actually, standard way is adding import "github.com/.../mcp"
		// But this is main package. mcp is sub package?
		// We created `mcp` folder. so it is `MODULE_NAME/mcp`.
		// Let's assume we need to fix imports first.
		mux.HandleFunc("/mcp/sse", mcp.HandleSSE)
		mux.HandleFunc("/mcp/messages", mcp.HandleMessages)
	}

	mux.HandleFunc("/api/config", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var newCfg struct {
				TTSThreads  int     `json:"tts_threads"`
				ApiEndpoint string  `json:"api_endpoint"`
				ApiToken    *string `json:"api_token"`
				LLMMode     string  `json:"llm_mode"`
				EnableTTS   *bool   `json:"enable_tts"`
				EnableMCP   *bool   `json:"enable_mcp"`
			}
			if err := json.NewDecoder(r.Body).Decode(&newCfg); err == nil {
				// Check for authenticated user
				userID := r.Header.Get("X-User-ID")
				var user *User
				if userID != "" {
					authMgr.mu.Lock()
					user = authMgr.users[userID]
					authMgr.mu.Unlock()
				}

				if user != nil {
					// User-specific save
					updated := false
					if newCfg.ApiEndpoint != "" {
						cleanEndpoint := strings.TrimSuffix(strings.TrimSpace(newCfg.ApiEndpoint), "/")
						cleanEndpoint = strings.TrimSuffix(cleanEndpoint, "/v1")
						user.Settings.ApiEndpoint = &cleanEndpoint
						updated = true
					}
					if newCfg.ApiToken != nil {
						token := strings.TrimSpace(*newCfg.ApiToken)
						if strings.HasPrefix(strings.ToLower(token), "bearer ") {
							token = strings.TrimSpace(token[7:])
						}
						user.Settings.ApiToken = &token
						updated = true
					}
					if newCfg.LLMMode != "" {
						user.Settings.LLMMode = &newCfg.LLMMode
						updated = true
					}
					if newCfg.EnableTTS != nil {
						user.Settings.EnableTTS = newCfg.EnableTTS
						updated = true
					}
					if newCfg.EnableMCP != nil {
						user.Settings.EnableMCP = newCfg.EnableMCP
						updated = true
					}
					// Handle TTS Config partial updates if needed, for now simplistic
					if newCfg.TTSThreads > 0 {
						if user.Settings.TTSConfig == nil {
							user.Settings.TTSConfig = &ServerTTSConfig{}
						}
						user.Settings.TTSConfig.Threads = newCfg.TTSThreads
						updated = true
					}

					if updated {
						if err := authMgr.SaveUsers(); err != nil {
							log.Printf("[handleConfig] Failed to save user settings: %v", err)
						} else {
							log.Printf("[handleConfig] Saved settings for user %s", userID)
						}
					}
				} else {
					// Global config save (Admin or fallback) - Only if no user context or explicitly desired?
					// For now, keeping legacy behavior for unauthenticated or admin might be confusing.
					// Let's assume if X-User-ID is missing (local mode) we save global.
					// If X-User-ID is present, we ONLY save to user.
					if userID == "" {
						if newCfg.TTSThreads > 0 {
							app.SetTTSThreads(newCfg.TTSThreads)
						}
						if newCfg.ApiEndpoint != "" {
							cleanEndpoint := strings.TrimSuffix(strings.TrimSpace(newCfg.ApiEndpoint), "/")
							cleanEndpoint = strings.TrimSuffix(cleanEndpoint, "/v1")
							app.SetLLMEndpoint(cleanEndpoint)
						}
						if newCfg.ApiToken != nil {
							token := strings.TrimSpace(*newCfg.ApiToken)
							if strings.HasPrefix(strings.ToLower(token), "bearer ") {
								token = strings.TrimSpace(token[7:])
							}
							app.SetLLMApiToken(token)
						}
						if newCfg.LLMMode != "" {
							app.SetLLMMode(newCfg.LLMMode)
						}
						if newCfg.EnableTTS != nil {
							app.SetEnableTTS(*newCfg.EnableTTS)
						}
						if newCfg.EnableMCP != nil {
							app.SetEnableMCP(*newCfg.EnableMCP)
						}
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")

		// Prepare response (Merge global + user)
		resp := map[string]interface{}{
			"llm_endpoint": app.llmEndpoint,
			"llm_mode":     app.llmMode,
			"enable_tts":   app.enableTTS,
			"enable_mcp":   app.enableMCP,
			"tts_config":   ttsConfig,
			"has_token":    app.llmApiToken != "",
		}

		// Overlay User Settings
		userID := r.Header.Get("X-User-ID")
		if userID != "" {
			authMgr.mu.RLock()
			user := authMgr.users[userID]
			authMgr.mu.RUnlock()

			if user != nil {
				if user.Settings.ApiEndpoint != nil {
					resp["llm_endpoint"] = *user.Settings.ApiEndpoint
				}
				if user.Settings.LLMMode != nil {
					resp["llm_mode"] = *user.Settings.LLMMode
				}
				if user.Settings.EnableTTS != nil {
					resp["enable_tts"] = *user.Settings.EnableTTS
				}
				if user.Settings.EnableMCP != nil {
					resp["enable_mcp"] = *user.Settings.EnableMCP
				}
				if user.Settings.ApiToken != nil && *user.Settings.ApiToken != "" {
					resp["has_token"] = true
				}
				// Note: We don't return the actul token for security, just has_token status
				// If the user wants to clear it, they send empty string.
				// But we assume if they set it, they know it.
			}
		}

		json.NewEncoder(w).Encode(resp)
	}))
	mux.HandleFunc("/api/tts/styles", AuthMiddleware(authMgr, handleTTSStyles))
	mux.HandleFunc("/v1/chat/completions", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		// Pass authMgr to allow user settings lookup
		handleChat(w, r, app, authMgr)
	}))
	mux.HandleFunc("/api/v1/chat", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		handleChat(w, r, app, authMgr)
	}))
	mux.HandleFunc("/api/models", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		handleModels(w, r, app)
	}))
	mux.HandleFunc("/api/dictionary", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(app.GetTTSDictionary(lang))
	}))
	mux.HandleFunc("/api/prompts", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(app.GetSystemPrompts())
	}))

	// Admin-only endpoints
	mux.HandleFunc("/api/users", AdminMiddleware(authMgr, handleUsers(authMgr)))
	mux.HandleFunc("/api/users/add", AdminMiddleware(authMgr, handleAddUser(authMgr)))
	mux.HandleFunc("/api/users/delete", AdminMiddleware(authMgr, handleDeleteUser(authMgr)))

	// Static file server for frontend (embedded)
	frontendFS, err := fs.Sub(app.assets, "frontend")
	if err != nil {
		log.Fatal("Failed to get frontend FS:", err)
	}
	webFS := http.FileServer(http.FS(frontendFS))

	// Serve web.html at root (Chat UI for web)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			// Serve web.html from embedded FS
			f, err := frontendFS.Open("web.html")
			if err != nil {
				http.Error(w, "web.html not found", http.StatusInternalServerError)
				return
			}
			defer f.Close()

			stat, _ := f.Stat()
			http.ServeContent(w, r, "web.html", stat.ModTime(), f.(io.ReadSeeker))
			return
		}
		webFS.ServeHTTP(w, r)
	})

	return mux
}

// handleLogin processes login requests
func handleLogin(am *AuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ID         string `json:"id"`
			Password   string `json:"password"`
			RememberMe bool   `json:"remember_me"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		token, err := am.Authenticate(req.ID, req.Password)
		if err != nil || token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
			return
		}

		// Set session cookie
		maxAge := 86400 // 24 hours default
		if req.RememberMe {
			maxAge = 86400 * 30 // 30 days
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   maxAge,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// handleLogout processes logout requests
func handleLogout(am *AuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err == nil {
			am.InvalidateSession(cookie.Value)
		}

		http.SetCookie(w, &http.Cookie{
			Name:   "session",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// handleAuthCheck checks if user is authenticated
func handleAuthCheck(am *AuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
			return
		}

		user, valid := am.ValidateSession(cookie.Value)
		if !valid {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": true,
			"user_id":       user.ID,
			"role":          user.Role,
		})
	}
}

// handleUsers returns list of users
func handleUsers(am *AuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(am.GetUsers())
	}
}

// handleAddUser adds a new user
func handleAddUser(am *AuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ID       string `json:"id"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if req.Role == "" {
			req.Role = "user"
		}

		if err := am.AddUser(req.ID, req.Password, req.Role); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// handleDeleteUser removes a user
func handleDeleteUser(am *AuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if err := am.DeleteUser(req.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// handleModels proxies model list requests to LLM server
func handleModels(w http.ResponseWriter, r *http.Request, app *App) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Try to fetch fresh models
	bodyBytes, err := app.FetchAndCacheModels()
	if err != nil {
		log.Printf("[handleModels] Fetch failed: %v", err)

		// Fallback to cache if available
		cached := app.GetCachedModels()
		if cached != nil {
			log.Printf("[handleModels] Returning cached models (fallback)")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Model-Source", "cache-fallback")
			w.Write(cached)
			return
		}

		// No cache and fetch failed
		http.Error(w, fmt.Sprintf("Failed to fetch models: %v", err), http.StatusBadGateway)
		return
	}

	// Success
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Model-Source", "live")
	w.Write(bodyBytes)
}

// handleChat proxies chat requests to LM Studio with SSE streaming
// handleChat proxies chat requests to LM Studio with SSE streaming
func handleChat(w http.ResponseWriter, r *http.Request, app *App, authMgr *AuthManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var llmURL string

	// Default configuration (Global)
	endpointRaw := app.llmEndpoint
	tokenRaw := app.llmApiToken
	llmMode := app.llmMode
	enableMCP := app.enableMCP

	// Override with User Settings
	userID := r.Header.Get("X-User-ID")
	if userID != "" {
		authMgr.mu.RLock()
		user := authMgr.users[userID]
		authMgr.mu.RUnlock()
		if user != nil {
			if user.Settings.ApiEndpoint != nil {
				endpointRaw = *user.Settings.ApiEndpoint
			}
			if user.Settings.ApiToken != nil {
				tokenRaw = *user.Settings.ApiToken
			}
			if user.Settings.LLMMode != nil {
				llmMode = *user.Settings.LLMMode
			}
			if user.Settings.EnableMCP != nil {
				enableMCP = *user.Settings.EnableMCP
			}
		}
	}

	// Sanitize endpoint: Remove trailing slash and optional /v1 suffix if user included it
	endpoint := strings.TrimSuffix(endpointRaw, "/")
	endpoint = strings.TrimSuffix(endpoint, "/v1")
	token := strings.TrimSpace(tokenRaw)

	// Sanitize token: Remove "Bearer " prefix if user pasted it
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}

	// Inject MCP integration if enabled
	if enableMCP {
		var reqMap map[string]interface{}
		if err := json.Unmarshal(body, &reqMap); err == nil {
			var integrations []interface{}
			if existing, ok := reqMap["integrations"].([]interface{}); ok {
				integrations = existing
			}

			// Add our MCP server if not present
			targetMCP := "mcp/dinkisstyle-gateway"
			hasMCP := false
			for _, v := range integrations {
				if str, ok := v.(string); ok && str == targetMCP {
					hasMCP = true
					break
				}
			}

			if !hasMCP {
				integrations = append(integrations, targetMCP)
				reqMap["integrations"] = integrations
				if newBody, err := json.Marshal(reqMap); err == nil {
					body = newBody
					log.Println("[handleChat] Injected MCP integration into request")
				}
			}
		}
	}

	// DEBUG LOG
	log.Printf("[handleChat] User: %s, Mode: %s, Endpoint: %s, HasToken: %v", userID, llmMode, endpoint, token != "")

	if llmMode == "stateful" {
		llmURL = endpoint + "/api/v1/chat"
	} else {
		llmURL = endpoint + "/v1/chat/completions"
	}

	// Use r.Context() to propagate cancellation from frontend
	req, err := http.NewRequestWithContext(r.Context(), "POST", llmURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		if len(token) > 4 {
			log.Printf("[handleChat] Using Token: %s...", token[:4])
		} else {
			log.Printf("[handleChat] Using Token (short)")
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("LLM request failed: %v", err)
		http.Error(w, fmt.Sprintf("LLM connection failed: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("LLM error response: %s", string(bodyBytes))
		http.Error(w, fmt.Sprintf("LLM error: %s", string(bodyBytes)), resp.StatusCode)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		fmt.Fprintf(w, "%s\n\n", line)
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Stream error: %v", err)
	}
}

// handleTTS converts text to speech using Supertonic
func handleTTS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Text       string  `json:"text"`
		Lang       string  `json:"lang"`
		ChunkSize  int     `json:"chunkSize"`
		VoiceStyle string  `json:"voiceStyle"`
		Speed      float32 `json:"speed"`
		Format     string  `json:"format"` // "wav" or "mp3"
		Steps      int     `json:"steps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Lang == "" {
		req.Lang = "ko"
	}
	if req.Format == "" {
		req.Format = "wav" // Default to WAV for backward compatibility
	}

	// Check if TTS is initialized
	globalTTSMutex.RLock()
	ttsInstance := globalTTS
	globalTTSMutex.RUnlock()

	if ttsInstance == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "not_available",
			"message": "TTS not initialized. Check assets folder.",
		})
		return
	}

	// Load voice style from config or request
	styleName := ttsConfig.VoiceStyle
	if req.VoiceStyle != "" {
		styleName = req.VoiceStyle
	}
	if !strings.HasSuffix(styleName, ".json") {
		styleName += ".json"
	}
	voiceStylePath := GetResourcePath(filepath.Join("assets", "voice_styles", styleName))

	// Check Cache
	styleMutex.Lock()
	style, found := styleCache[styleName]
	styleMutex.Unlock()

	if !found {
		// Load if not in cache
		loadedStyle, err := LoadVoiceStyle(voiceStylePath)
		if err != nil {
			log.Printf("Failed to load voice style %s: %v", styleName, err)
			http.Error(w, "Failed to load voice style", http.StatusInternalServerError)
			return
		}

		styleMutex.Lock()
		// Double check locking (standard double-checked locking pattern not strictly needed for this scale but safe)
		if cached, ok := styleCache[styleName]; ok {
			loadedStyle.Destroy() // discard duplicate
			style = cached
		} else {
			styleCache[styleName] = loadedStyle
			style = loadedStyle
		}
		styleMutex.Unlock()
		log.Printf("Loaded and cached voice style: %s", styleName)
	}

	// Do NOT destroy style here, it is cached for lifetime of app (or until explicit clear)
	// defer style.Destroy() <--- REMOVED

	// Generate speech using configured speed
	speed := ttsConfig.Speed
	if req.Speed > 0 {
		speed = req.Speed
	}
	steps := 5
	if req.Steps > 0 {
		steps = req.Steps
		if steps > 50 {
			steps = 50
		}
	}
	globalTTSMutex.RLock()
	if globalTTS == nil {
		globalTTSMutex.RUnlock()
		http.Error(w, "TTS not initialized", http.StatusInternalServerError)
		return
	}
	// Use globalTTS directly while holding the lock to prevent destruction
	wavData, _, err := globalTTS.Call(r.Context(), req.Text, req.Lang, style, steps, speed, req.ChunkSize)
	sampleRate := globalTTS.SampleRate
	globalTTSMutex.RUnlock()

	if err != nil {
		log.Printf("TTS failed: %v", err)
		http.Error(w, "TTS generation failed", http.StatusInternalServerError)
		return
	}

	// Generate audio bytes in requested format
	audioBytes, contentType, err := GenerateAudio(wavData, sampleRate, req.Format)
	if err != nil {
		log.Printf("Audio generation failed: %v", err)
		http.Error(w, "Audio generation failed", http.StatusInternalServerError)
		return
	}

	// Return audio
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(audioBytes)))

	startTransfer := time.Now()
	n, err := w.Write(audioBytes)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	elapsedTransfer := time.Since(startTransfer)

	if err != nil {
		log.Printf("[TTS] Network transfer failed after %d bytes: %v", n, err)
	} else {
		log.Printf("[TTS] Network transfer complete: %d bytes sent in %v", n, elapsedTransfer)
	}
}

// handleTTSStyles returns list of available voice styles
func handleTTSStyles(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(GetResourcePath(filepath.Join("assets", "voice_styles")))
	if err != nil {
		http.Error(w, "Failed to read styles directory", http.StatusInternalServerError)
		return
	}

	var styles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
			styles = append(styles, strings.TrimSuffix(f.Name(), ".json"))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(styles)
}

// Global TTS instance

// InitTTS initializes the TTS engine
func InitTTS(assetsDir string, threads int) error {
	onnxDir := assetsDir + "/onnx"

	// Check if TTS files exist
	if _, err := os.Stat(onnxDir + "/vocoder.onnx"); os.IsNotExist(err) {
		log.Println("TTS assets not found, TTS disabled")
		return nil
	}

	// Initialize ONNX Runtime (idempotent, safe to call multiple times or check internal flag)
	if err := InitializeONNXRuntime(); err != nil {
		return fmt.Errorf("failed to initialize ONNX Runtime: %w", err)
	}

	// Load TTS config
	cfg, err := LoadTTSConfig(onnxDir)
	if err != nil {
		return fmt.Errorf("failed to load TTS config: %w", err)
	}

	// Load TTS models
	// Note: Loading takes time, do it before acquiring lock
	tts, err := LoadTextToSpeech(onnxDir, cfg, threads)
	if err != nil {
		return fmt.Errorf("failed to load TTS: %w", err)
	}

	// Swap instances
	globalTTSMutex.Lock()
	defer globalTTSMutex.Unlock()

	if globalTTS != nil {
		globalTTS.Destroy()
	}

	globalTTS = tts
	log.Printf("TTS initialized successfully (Threads: %d)", threads)
	return nil
}
