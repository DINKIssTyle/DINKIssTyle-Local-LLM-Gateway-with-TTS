/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"bufio"
	"bytes"
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

	// Protected API endpoints
	mux.HandleFunc("/api/chat", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		handleChat(w, r, app.llmEndpoint)
	}))
	mux.HandleFunc("/api/tts", AuthMiddleware(authMgr, handleTTS))
	mux.HandleFunc("/api/config", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var newCfg struct {
				TTSThreads int `json:"tts_threads"`
			}
			if err := json.NewDecoder(r.Body).Decode(&newCfg); err == nil && newCfg.TTSThreads > 0 {
				app.SetTTSThreads(newCfg.TTSThreads)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"llm_endpoint": app.llmEndpoint,
			"enable_tts":   app.enableTTS,
			"tts_config":   ttsConfig,
		})
	}))
	mux.HandleFunc("/api/tts/styles", AuthMiddleware(authMgr, handleTTSStyles))

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

// handleChat proxies chat requests to LM Studio with SSE streaming
func handleChat(w http.ResponseWriter, r *http.Request, llmEndpoint string) {
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

	llmURL := llmEndpoint + "/v1/chat/completions"
	// Use r.Context() to propagate cancellation from frontend
	req, err := http.NewRequestWithContext(r.Context(), "POST", llmURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer lm-studio")

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
	wavData, _, err := globalTTS.Call(req.Text, req.Lang, style, steps, speed, req.ChunkSize)
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
	w.Write(audioBytes)
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
			styles = append(styles, f.Name())
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
