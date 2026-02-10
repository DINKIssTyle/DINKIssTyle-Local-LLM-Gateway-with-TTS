/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"dinkisstyle-chat/mcp"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

	// Global App Instance (for handlers to access app methods)
	globalApp *App
)

// Structured Tool Call Support
type StructuredToolCall struct {
	Thought       string      `json:"thought"`
	ToolName      string      `json:"tool_name"`
	ToolArguments interface{} `json:"tool_arguments"`
}

// ensureSelfSignedCert check if cert.pem and key.pem exist in AppDataDir, if not create them.
func ensureSelfSignedCert(appDataDir string) (string, string, error) {
	certPath := filepath.Join(appDataDir, "cert.pem")
	keyPath := filepath.Join(appDataDir, "key.pem")

	// If both files exist, return them
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return certPath, keyPath, nil
		}
	}

	log.Println("[HTTPS] Generating self-signed certificate...")

	// Generate a new private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %v", err)
	}

	// Create a certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // 1 year validity

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"DINKI'ssTyle Local LLM Gateway"},
			CommonName:   "localhost",
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add local IP and localhost to SANs
	template.IPAddresses = append(template.IPAddresses, net.ParseIP("127.0.0.1"), net.ParseIP("::1"))
	template.DNSNames = append(template.DNSNames, "localhost")

	// Create the certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		return "", "", fmt.Errorf("failed to create certificate: %v", err)
	}

	// Write cert.pem
	certOut, err := os.Create(certPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to open cert.pem for writing: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", "", fmt.Errorf("failed to write data to cert.pem: %v", err)
	}
	if err := certOut.Close(); err != nil {
		return "", "", fmt.Errorf("error closing cert.pem: %v", err)
	}

	// Write key.pem
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", "", fmt.Errorf("failed to open key.pem for writing: %v", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("unable to marshal private key: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return "", "", fmt.Errorf("failed to write data to key.pem: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		return "", "", fmt.Errorf("error closing key.pem: %v", err)
	}

	log.Printf("[HTTPS] Certificate generated at %s", certPath)
	return certPath, keyPath, nil
}

// preloadUserMemory reads the user's memory files (static.md and personal.md) into a string
// Returns formatted string or empty if no files exist.
func preloadUserMemory(userID string) string {
	var sb strings.Builder

	// 1. Load Static Memory (Read-Only)
	staticPath, err := mcp.GetUserMemoryFilePath(userID, "static.md")
	if err == nil {
		if data, err := os.ReadFile(staticPath); err == nil && len(data) > 0 {
			content := strings.TrimSpace(string(data))
			if content != "" {
				sb.WriteString("[Static Memory (Read-Only)]\n")
				sb.WriteString(content)
				sb.WriteString("\n\n")
			}
		}
	}

	// 2. Load Personal Memory (Writable)
	personalPath, err := mcp.GetUserMemoryFilePath(userID, "personal.md")
	if err == nil {
		if data, err := os.ReadFile(personalPath); err == nil && len(data) > 0 {
			content := strings.TrimSpace(string(data))
			if content != "" {
				sb.WriteString("[Personal Memory]\n")
				sb.WriteString(content)
			}
		}
	}

	finalContent := strings.TrimSpace(sb.String())
	if finalContent == "" {
		return ""
	}

	// Truncate if too large to avoid context overflow (max ~10000 chars)
	maxLen := 10000
	if len(finalContent) > maxLen {
		finalContent = finalContent[:maxLen] + "\n... (memory truncated)"
	}

	log.Printf("[preloadUserMemory] Loaded %d bytes of combined memory for user %s", len(finalContent), userID)
	return finalContent
}

type ServerTTSConfig struct {
	VoiceStyle string  `json:"voiceStyle"`
	Speed      float32 `json:"speed"`
	Threads    int     `json:"threads"`
}

// createServerMux creates the HTTP handler mux for the server
func createServerMux(app *App, authMgr *AuthManager) *http.ServeMux {
	globalApp = app // Initialize the global instance for all handlers
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
	// MCP Endpoints (Always Enabled if server runs)
	log.Println("[Server] MCP Support Active")
	mux.HandleFunc("/mcp/sse", mcp.HandleSSE)
	mux.HandleFunc("/mcp/messages", mcp.HandleMessages)

	mux.HandleFunc("/api/config", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var newCfg struct {
				TTSThreads   int     `json:"tts_threads"`
				ApiEndpoint  string  `json:"api_endpoint"`
				ApiToken     *string `json:"api_token"`
				LLMMode      string  `json:"llm_mode"`
				EnableTTS    *bool   `json:"enable_tts"`
				EnableMCP    *bool   `json:"enable_mcp"`
				EnableMemory *bool   `json:"enable_memory"`
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
					if newCfg.EnableMemory != nil {
						user.Settings.EnableMemory = newCfg.EnableMemory
						updated = true
						// Sync to MCP context
						// We need disallowed lists here too, but handleConfig is partial update.
						// Let's retrieve full user settings to be safe.
						mcp.SetContext(user.ID, *newCfg.EnableMemory, user.Settings.DisabledTools, "", user.Settings.DisallowedCommands, user.Settings.DisallowedDirectories)
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
						if newCfg.EnableMemory != nil {
							// Global memory toggle is removed, handled per-user
						}
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")

		// Prepare response (Merge global + user)
		resp := map[string]interface{}{
			"llm_endpoint":  app.llmEndpoint,
			"llm_mode":      app.llmMode,
			"enable_tts":    app.enableTTS,
			"enable_mcp":    app.enableMCP,
			"enable_memory": false, // Global default is false, overridden by user settings below
			"tts_config":    ttsConfig,
			"has_token":     app.llmApiToken != "",
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
				if user.Settings.EnableMemory != nil {
					resp["enable_memory"] = *user.Settings.EnableMemory
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
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodPost {
		// Handle Model Load Request
		var req struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.Model == "" {
			http.Error(w, "Model ID required", http.StatusBadRequest)
			return
		}

		if err := app.LoadModel(req.Model); err != nil {
			log.Printf("[handleModels] Load failed: %v", err)
			http.Error(w, fmt.Sprintf("Failed to load model: %v", err), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Model loaded"})
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	enableMemory := false // Default to false (Secure by default for unauthenticated)

	var disabledTools []string
	var locationInfo string
	var disallowedCmds []string
	var disallowedDirs []string

	// Override with User Settings
	userID := r.Header.Get("X-User-ID")
	// Extract Client Location
	locationInfo = r.Header.Get("X-User-Location")

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
			if user.Settings.EnableMemory != nil {
				enableMemory = *user.Settings.EnableMemory
			} else {
				enableMemory = true // Default to true for authenticated users
			}
			if user.Settings.DisabledTools != nil {
				disabledTools = user.Settings.DisabledTools
			}
			if user.Settings.DisallowedCommands != nil {
				disallowedCmds = user.Settings.DisallowedCommands
			}
			if user.Settings.DisallowedDirectories != nil {
				disallowedDirs = user.Settings.DisallowedDirectories
			}
		}
	}

	// Set MCP Context for this user interaction
	// This ensures that when LM Studio calls back to MCP, it has the correct context
	mcp.SetContext(userID, enableMemory, disabledTools, locationInfo, disallowedCmds, disallowedDirs)
	log.Printf("[handleChat-DEBUG] userID=%s, enableMemory=%v, disabledTools=%v, Location=%s, DisallowedCmds=%v, DisallowedDirs=%v", userID, enableMemory, disabledTools, locationInfo, disallowedCmds, disallowedDirs)

	// Sanitize endpoint: Remove trailing slash and optional /v1 suffix if user included it
	endpoint := strings.TrimRight(endpointRaw, "/")
	endpoint = strings.TrimSuffix(endpoint, "/v1")
	token := strings.TrimSpace(tokenRaw)

	// Sanitize token: Remove "Bearer " prefix if user pasted it
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}

	var reqMap map[string]interface{}
	// Always unmarshal body into reqMap to prevent nil panics later in the turn loop
	json.Unmarshal(body, &reqMap)

	// Inject MCP integration if enabled AND NOT IN STANDARD MODE
	// Standard Mode (OpenAI compliant) with 'integrations' field might trigger strict auth in LM Studio.
	if enableMCP && llmMode != "standard" {
		if reqMap != nil {
			// Optimization for LM Studio Stateful mode:
			// If we are in stateful mode and have a previous_response_id, we can skip redundant injections
			// because LM Studio remembers the context (system_prompt, integrations, etc.) in the chat thread.
			isStatefulTurn := false
			if llmMode == "stateful" {
				if pid, ok := reqMap["previous_response_id"].(string); ok && pid != "" {
					isStatefulTurn = true
					log.Println("[handleChat] Detected follow-up stateful turn, skipping redundant system prompt (maintaining tools)")
				}
			}

			if enableMCP {
				targetMCP := "mcp/dinkisstyle-gateway"
				var integrations []string
				if existing, ok := reqMap["integrations"].([]interface{}); ok {
					for _, v := range existing {
						if str, ok := v.(string); ok {
							integrations = append(integrations, str)
						}
					}
				}

				hasMCP := false
				for _, v := range integrations {
					if v == targetMCP {
						hasMCP = true
						break
					}
				}

				if !hasMCP {
					integrations = append(integrations, targetMCP)
					reqMap["integrations"] = integrations
					// Important: internal body update will happen after system prompt injection
				}
			}

			// EXTRA SAFEGUARD: Inject System Prompt instruction for cleaner Tool Calls
			// Qwen/VL models often mess up XML tags (nested or unclosed).
			if !isStatefulTurn {
				var envInfo string
				if cwd, err := os.Getwd(); err == nil {
					envInfo += fmt.Sprintf("- Current Working Directory: %s\n", cwd)
				}
				if home, err := os.UserHomeDir(); err == nil {
					desktop := filepath.Join(home, "Desktop")
					envInfo += fmt.Sprintf("- Desktop Path: %s\n", desktop)
				}

				// We use a marker to detect if we already injected instructions
				instrMarker := "### TOOL CALL GUIDELINES ###"
				extraInstr := mcp.SystemPromptToolUsage(envInfo)

				if enableMemory {
					preloadedMemory := preloadUserMemory(userID)
					extraInstr += mcp.SystemPromptMemoryTemplate(preloadedMemory)
				}

				foundSystem := false
				// Case A: Standard mode (OpenAI style)
				if messages, ok := reqMap["messages"].([]interface{}); ok {
					// 🚀 SLIDING WINDOW: Limit to last 10 messages to prevent context overflow
					maxMessages := 10
					if len(messages) > maxMessages {
						log.Printf("[handleChat] Truncating chat history: %d -> %d messages", len(messages), maxMessages)

						// Preserve system message if it exists at the beginning
						var systemMsg interface{}
						if len(messages) > 0 {
							if m, ok := messages[0].(map[string]interface{}); ok {
								if role, ok := m["role"].(string); ok && role == "system" {
									systemMsg = messages[0]
								}
							}
						}

						// Take the last maxMessages
						truncated := messages[len(messages)-maxMessages:]

						// If we had a system message at the start and it's not in the truncated set now, re-add it
						systemInTruncated := false
						if systemMsg != nil {
							for _, msg := range truncated {
								if msg == systemMsg {
									systemInTruncated = true
									break
								}
							}
							if !systemInTruncated {
								truncated = append([]interface{}{systemMsg}, truncated...)
							}
						}
						messages = truncated
						reqMap["messages"] = messages
					}

					for i, msg := range messages {
						if m, ok := msg.(map[string]interface{}); ok {
							if role, ok := m["role"].(string); ok && role == "system" {
								content, _ := m["content"].(string)
								// Prevent duplicate injection
								if !strings.Contains(content, instrMarker) {
									m["content"] = content + extraInstr
									messages[i] = m
								}
								foundSystem = true
								break
							}
						}
					}
					if !foundSystem {
						newMsg := map[string]interface{}{
							"role":    "system",
							"content": "You are a helpful assistant." + extraInstr,
						}
						reqMap["messages"] = append([]interface{}{newMsg}, messages...)
						foundSystem = true
					}
				}

				// Case B: Stateful mode (LM Studio style)
				// LM Studio might append system_prompt every turn if we keep sending it.
				if sp, ok := reqMap["system_prompt"].(string); ok {
					// Prevent duplicate injection
					if !strings.Contains(sp, instrMarker) {
						// Check if this turn has previous state.
						// If it does, we might want to ONLY send the extra instructions if something changed,
						// but for now, we'll just ensure we don't duplicate the marker.
						reqMap["system_prompt"] = sp + extraInstr
					}
					foundSystem = true
				}

				if foundSystem {
					log.Println("[handleChat] Injected or deduplicated System Prompt instructions")
				}
			}

			// Final Body update if we changed anything
			if newBody, err := json.Marshal(reqMap); err == nil {
				body = newBody
			}
		}
	} else {
		log.Printf("[handleChat] MCP injection skipped (EnableMCP=%v, Mode=%s)", enableMCP, llmMode)
	}

	// Set the LLM URL based on mode
	if llmMode == "stateful" {
		llmURL = endpoint + "/api/v1/chat"
	} else {
		llmURL = endpoint + "/v1/chat/completions"
	}
	log.Printf("[handleChat] User: %s, Mode: %s, Endpoint: %s, URL: %s", userID, llmMode, endpoint, llmURL)

	// Determine Model ID for Tool Pattern Lookup
	var modelID string
	var tmpModel struct {
		Model string `json:"model"`
	}
	json.Unmarshal(body, &tmpModel)
	modelID = tmpModel.Model

	// Set SSE headers ONCE before turn loop
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Shared turn-state variables
	fullResponse := ""
	memoryLogged := false
	var messagesForMemory []map[string]interface{}
	needsCorrection := false
	var badContentCapture string
	var lastResponseID string // Captured from chat.end for stateful chaining

	// --- TURN LOOP START ---
	// We allow up to 10 turns (tool call cycles) per request
	for turn := 0; turn < 10; turn++ {
		toolExecutedThisTurn := false
		var lastToolName string
		var lastToolArgsStr string
		var lastSavedBufferForTurn string

		// Variables for tool execution (shared between Regex and JSON modes)
		var toolName string
		var toolArgsStr string

		// Use r.Context() to propagate cancellation from frontend
		// Note: 'body' must be updated if we loop
		req, err := http.NewRequestWithContext(r.Context(), "POST", llmURL, bytes.NewReader(body))
		if err != nil {
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}

		req.Header.Set("Content-Type", "application/json")

		// Check if token is effectively empty or just "bearer", OR IS A MASKED VALUE
		token = strings.TrimSpace(token)
		isMasked := strings.HasPrefix(token, "***") || strings.HasSuffix(token, "...")
		if token != "" && !isMasked {
			req.Header.Set("Authorization", "Bearer "+token)
		} else {
			// Default to lm-studio (standard, no hacks).
			// If this fails with 401, we will handle the error response to guide the user.
			log.Printf("[handleChat] Empty/Invalid/Masked Token detected ('%s'), using Default: lm-studio", token)
			req.Header.Set("Authorization", "Bearer lm-studio")
		}

		client := &http.Client{Timeout: 5 * time.Minute}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("LLM request failed: %v", err)
			if turn == 0 {
				http.Error(w, fmt.Sprintf("LLM connection failed: %v", err), http.StatusBadGateway)
			}
			return
		}
		// Close body at end of this turn
		// We will close it explicitly after the scanner instead of using defer in a loop

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			errorMsg := string(bodyBytes)
			log.Printf("LLM error response: %s", errorMsg)

			// Check for specific LM Studio auth error
			// We return a text starting with "LM_STUDIO_AUTH_ERROR:" so frontend can localize it.
			if resp.StatusCode == http.StatusUnauthorized || strings.Contains(errorMsg, "invalid_api_key") || strings.Contains(errorMsg, "Malformed LM Studio API token") {
				// Frontend will detect this prefix and show translated message
				http.Error(w, "LM_STUDIO_AUTH_ERROR: "+errorMsg, resp.StatusCode)
				return
			}

			// Check for MCP Permission Error (403)
			if resp.StatusCode == http.StatusForbidden && strings.Contains(errorMsg, "Permission denied to use plugin") {
				http.Error(w, "LM_STUDIO_MCP_ERROR: "+errorMsg, resp.StatusCode)
				return
			}

			// Check for Context Overflow Error
			if strings.Contains(errorMsg, "Context size has been exceeded") || strings.Contains(errorMsg, "context_length_exceeded") {
				log.Printf("[handleChat] LM Studio Context Limit Reached. Informing user.")
				// Return a specialized error that the frontend can handle specifically
				http.Error(w, "LM_STUDIO_CONTEXT_ERROR: Context limit reached. Please clear the chat and try again.", resp.StatusCode)
				return
			}

			http.Error(w, fmt.Sprintf("LLM error: %s", errorMsg), resp.StatusCode)
			return
		}

		// Tool Pattern Logic
		toolPattern := app.GetToolPattern(modelID)
		var toolRegex = (*regexp.Regexp)(nil)

		// Buffer for handling split tags (e.g., "<", "tool", "_call>")
		var partialTagBuffer string

		// Flag if we are in "buffering mode" (waiting for complete tool call)
		isBuffering := false
		var buffer string             // Declare buffer here
		var bufferingThreshold = 8000 // Buffer size (increased for large JSON args)

		if toolPattern != nil {
			if regexStr, ok := toolPattern["regex"]; ok {
				var err error
				toolRegex, err = regexp.Compile(regexStr)
				if err != nil {
					log.Printf("[handleChat] Invalid regex for model %s: %v", modelID, err)
					toolPattern = nil // Disable if invalid
				} else {
					isBuffering = true
					log.Printf("[handleChat] Enabled Custom Tool Parsing for model: %s", modelID)
				}
			}
		}

		// Extract messages for Memory Analysis (if enabled)
		if enableMemory {
			log.Printf("[handleChat-DEBUG] Request Body Snippet: %s", string(body)[:min(len(body), 500)])

			var tmpBody struct {
				Messages []map[string]interface{} `json:"messages"`
				Input    string                   `json:"input"` // Support single input field
			}
			if err := json.Unmarshal(body, &tmpBody); err == nil {
				if len(tmpBody.Messages) > 0 {
					messagesForMemory = tmpBody.Messages
					log.Printf("[handleChat-DEBUG] Extracted %d messages for memory", len(messagesForMemory))
				} else if tmpBody.Input != "" {
					// Construct a single user message from Input
					messagesForMemory = []map[string]interface{}{
						{"role": "user", "content": tmpBody.Input},
					}
					log.Printf("[handleChat-DEBUG] Extracted 'input' as user message for memory")
				} else {
					log.Printf("[handleChat-DEBUG] No 'messages' or 'input' found in body")
				}
			} else {
				log.Printf("[handleChat-DEBUG] Failed to extract messages for memory: %v", err)
			}
		}

		scanner := bufio.NewScanner(resp.Body)
		log.Println("[handleChat-DEBUG] Starting response scanner loop")
		for scanner.Scan() {
			line := scanner.Text()

			// Log first few lines to debug stream format
			if len(fullResponse) < 100 && len(fullResponse) > 0 {
				// Don't log every line forever, just start
			}

			// CUSTOM PARSING LOGIC & SSE Handling
			// We process every line to handle both standard OpenAI and the custom format seen in logs
			trimmedLine := strings.TrimSpace(line)

			// Skip empty lines or comment lines (unless we need event types, which we might for the custom format)
			if trimmedLine == "" {
				continue
			}

			// Check for data: prefix
			if strings.HasPrefix(trimmedLine, "data: ") {
				dataStr := strings.TrimPrefix(trimmedLine, "data: ")

				// 1. Check for Standard OpenAI [DONE]
				if dataStr == "[DONE]" {
					log.Println("[handleChat-DEBUG] [DONE] signal received (Standard)")
					// If buffering for tools, flush any remaining buffer as content before sending DONE
					if toolPattern != nil && isBuffering && len(buffer) > 0 {
						payload := map[string]interface{}{
							"choices": []interface{}{
								map[string]interface{}{
									"delta": map[string]string{
										"content": buffer,
									},
								},
							},
						}
						jsonBytes, _ := json.Marshal(payload)
						fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
						fullResponse += buffer
						buffer = ""
					}

					// Just forward the DONE
					fmt.Fprintf(w, "%s\n\n", line)
					flusher.Flush()

					if enableMemory {
						checkAndLogMemoryWith(userID, messagesForMemory, fullResponse, &memoryLogged, modelID)
					}
					continue
				}

				// 2. Parse JSON
				var chunk map[string]interface{}
				if err := json.Unmarshal([]byte(dataStr), &chunk); err == nil {

					// --- A. Handle Custom Format (type: "message.delta", etc) ---
					if msgType, ok := chunk["type"].(string); ok {
						if msgType == "message.delta" {
							if content, ok := chunk["content"].(string); ok {
								fullResponse += content

								// 🔍 Self-Evolution for Custom Format
								if toolPattern == nil && len(content) > 5 {
									lc := strings.ToLower(content)
									if strings.Contains(lc, "<|") ||
										strings.Contains(lc, "function") ||
										strings.Contains(lc, "tool") ||
										strings.Contains(lc, "execute") ||
										(strings.Contains(lc, "{\"") && strings.Contains(lc, "args")) {
										log.Printf("[handleChat] Invalid tool pattern detected in content (Custom Format): %s", content)
										needsCorrection = true
										badContentCapture = content // Capture the snippet for the prompt
									}
								}

								// Forward to client identical to source
								fmt.Fprintf(w, "%s\n\n", line)
								flusher.Flush()
								continue
							}
						} else if msgType == "chat.end" || msgType == "message.end" {
							log.Printf("[handleChat-DEBUG] Custom End Signal Received: %s", msgType)

							// Capture response_id for chaining in stateful mode
							// Line is like: event: chat.end\ndata: {"result": {"response_id": "..."}}
							// We need to decode "data" part.
							// The 'line' variable here is the raw SSE line, e.g. "data: {...}"
							if strings.HasPrefix(line, "data: ") {
								jsonPart := strings.TrimPrefix(line, "data: ")
								var endPayload map[string]interface{}
								if err := json.Unmarshal([]byte(jsonPart), &endPayload); err == nil {
									if res, ok := endPayload["result"].(map[string]interface{}); ok {
										if rid, ok := res["response_id"].(string); ok {
											lastResponseID = rid
											log.Printf("[handleChat] Captured response_id for chaining: %s", lastResponseID)
										}
									}
								}
							}

							// Forward
							fmt.Fprintf(w, "%s\n\n", line)
							flusher.Flush()
							if enableMemory {
								checkAndLogMemoryWith(userID, messagesForMemory, fullResponse, &memoryLogged, modelID)
							}
							continue
						} else {
							// Forward other events (start, progress, etc)
							fmt.Fprintf(w, "%s\n\n", line)
							flusher.Flush()
							continue
						}
					}

					// --- B. Handle Tool Pattern Logic (if enabled and buffering) ---
					if toolPattern != nil && isBuffering {
						// Extract content for buffering
						var content string
						if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
							if choice, ok := choices[0].(map[string]interface{}); ok {
								if delta, ok := choice["delta"].(map[string]interface{}); ok {
									if c, ok := delta["content"].(string); ok {
										content = c
									} else if rc, ok := delta["reasoning_content"].(string); ok {
										content = rc
									} else if r, ok := delta["reasoning"].(string); ok {
										content = r
									}
								} else if message, ok := choice["message"].(map[string]interface{}); ok {
									if c, ok := message["content"].(string); ok {
										content = c
									}
								}
							}
						}
						buffer += content

						// Check Regex
						matches := toolRegex.FindStringSubmatch(buffer)
						if len(matches) > 2 {
							// Match Found! (Group 1: Tool Name, Group 2: Args)
							toolName = matches[1]
							toolArgsStr = matches[2]

							// 🛠️ Command-R / GPT-OSS Name Extraction
							// If the tool name (Group 1) looks like a Command-R prefix, try to extract real name from "to=..."
							if strings.Contains(toolName, "<|channel|>") {
								reName := regexp.MustCompile(`to=([a-zA-Z0-9_]+)`)
								nameMatch := reName.FindStringSubmatch(toolName)
								if len(nameMatch) > 1 {
									toolName = nameMatch[1]
									// If generic "functions", we expect the JSON to have the real name (handled by Smart Parsing below)
								}
							}

							// Smart Parsing: Check if G2 is actually a wrapper object with "name" and "arguments"
							var wrapper struct {
								Name      string      `json:"name"`
								Arguments interface{} `json:"arguments"`
							}

							isWrapper := false
							if err := json.Unmarshal([]byte(toolArgsStr), &wrapper); err == nil {
								if wrapper.Name != "" && wrapper.Arguments != nil {
									toolName = wrapper.Name
									isWrapper = true
									log.Printf("[handleChat] Detected Wrapper JSON format. Extracted Tool: %s", toolName)
								}
							}

							log.Printf("[handleChat] Custom Tool Pattern Matched! Tool: %s", toolName)

							// 🛠️ Mark for execution after stream ends
							toolExecutedThisTurn = true
							lastToolName = toolName
							lastToolArgsStr = toolArgsStr

							// 1. Emit start event
							startEvt := map[string]string{
								"type": "tool_call.start",
								"tool": toolName,
							}
							startBytes, _ := json.Marshal(startEvt)
							fmt.Fprintf(w, "data: %s\n\n", string(startBytes))

							// 2. Emit arguments event
							if isWrapper {
								argsEvt := map[string]interface{}{
									"type":      "tool_call.arguments",
									"tool":      toolName,
									"arguments": wrapper.Arguments,
								}
								argsBytes, _ := json.Marshal(argsEvt)
								fmt.Fprintf(w, "data: %s\n\n", string(argsBytes))
							} else {
								fmt.Fprintf(w, "data: {\"type\": \"tool_call.arguments\", \"tool\": \"%s\", \"arguments\": %s}\n\n", toolName, toolArgsStr)
							}

							// 3. Clear Buffer & Stop Buffering
							buffer = ""
							isBuffering = false
							flusher.Flush()
							continue // Tool call handled, move to next line
						}

						// If buffer too long, assume no match and flush
						if len(buffer) > bufferingThreshold {
							// 🔍 Self-Evolution Check
							lowerBuf := strings.ToLower(buffer)
							if (strings.Contains(lowerBuf, "function") || strings.Contains(lowerBuf, "call") || strings.Contains(lowerBuf, "tool")) &&
								(strings.Contains(lowerBuf, "{") && strings.Contains(lowerBuf, "}")) {
								log.Printf("[handleChat] Invalid tool pattern detected in buffer: %s", buffer)
								needsCorrection = true
								badContentCapture = buffer
							}

							// Flush buffer as regular content
							payload := map[string]interface{}{
								"choices": []interface{}{
									map[string]interface{}{
										"delta": map[string]string{
											"content": buffer,
										},
									},
								},
							}
							jsonBytes, _ := json.Marshal(payload)
							fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
							fullResponse += buffer // Add to full response
							buffer = ""
							flusher.Flush()
						}
						continue // Buffering logic handled, move to next line
					}

					// --- C. Handle Standard OpenAI Format (if not custom and not tool-buffered) ---
					if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]interface{}); ok {
							if delta, ok := choice["delta"].(map[string]interface{}); ok {
								if c, ok := delta["content"].(string); ok {
									fullResponse += c
								}
							}
						}
					}

					// --- D. Self-Evolution for non-buffered models ---
					// This block was previously in an `else if strings.HasPrefix(strings.TrimSpace(line), "data: ")`
					// It should now be integrated here, after content extraction but before forwarding.
					if toolPattern == nil { // Only if no tool pattern is active
						var content string
						if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
							if choice, ok := choices[0].(map[string]interface{}); ok {
								if delta, ok := choice["delta"].(map[string]interface{}); ok {
									if c, ok := delta["content"].(string); ok {
										content = c
									} else if rc, ok := delta["reasoning_content"].(string); ok {
										content = rc
									} else if r, ok := delta["reasoning"].(string); ok {
										content = r
									}
								}
							}
						}

						// 🛠️ Structured Output Support: Force buffering if start of JSON object is detected
						if !isBuffering && len(fullResponse) < 50 && strings.HasPrefix(strings.TrimSpace(content), "{") {
							log.Printf("[handleChat] Detected potential JSON start. Switching to buffering mode.")
							isBuffering = true
							buffer = content
							flusher.Flush()
							continue
						}

						// 🧪 Special Handling for Command-R / GPT-OSS Format (<|channel|>)
						if !isBuffering && (strings.Contains(content, "<|channel|>") || strings.Contains(content, "<|tool_code|>") || strings.Contains(content, "<tool_call>")) {
							log.Printf("[handleChat] Detected Command-R/GPT-OSS/Qwen Tool Call Pattern. Switching to buffering mode.")
							isBuffering = true
							buffer = content

							if strings.Contains(content, "<tool_call>") {
								// Qwen Style: <tool_call>{JSON}</tool_call>
								toolPattern = map[string]string{"format": "qwen"}
								// Regex: Group 1 (Dummy/Name) + Group 2 (JSON)
								toolRegex = regexp.MustCompile(`(?s)(<tool_call>)\s*(\{[\s\S]*?\})\s*</tool_call>`)
							} else {
								// Command-R / GPT-OSS Style
								// Define a regex that captures the prefix as Group 1 (ignored) and the JSON as Group 2
								// Pattern: <|channel|>...<|message|> { JSON }
								toolPattern = map[string]string{"format": "command-r"}
								toolRegex = regexp.MustCompile(`(?s)(<\|channel\|>.*?<\|message\|>)\s*(\{[\s\S]*\})`)
							}
							flusher.Flush()
							continue
						}

						// 🔍 Self-Evolution for non-buffered models
						if len(content) > 5 {
							lc := strings.ToLower(content)
							// Broaden trigger keywords: <|, function, tool, execute, {"name":, and tool names
							hasToolName := strings.Contains(lc, "search_web") ||
								strings.Contains(lc, "personal_memory") ||
								strings.Contains(lc, "read_user_document") ||
								strings.Contains(lc, "read_web_page")

							if strings.Contains(lc, "<|") ||
								strings.Contains(lc, "function") ||
								strings.Contains(lc, "tool") ||
								strings.Contains(lc, "execute") ||
								hasToolName ||
								(strings.Contains(lc, "{\"") && strings.Contains(lc, "args")) {

								// 🧬 Anti-Recursion & Meta-Content Filter
								// We want to skip learning if the content is:
								// 1. A regex definition (has "(?s)", "regex")
								// 2. A code block explanation of a tool call (has "tool_call[")
								// 3. A JSON definition inside a markdown block (often starts with ```json) - though hard to detect in fragments
								// 4. Broken/Partial JSON that is clearly just text discussion
								if strings.Contains(lc, "(?s)") ||
									strings.Contains(lc, "regex") ||
									strings.Contains(lc, "tool_call[") || // Catch array/map access style which is code, not natural language
									strings.Contains(lc, "tool_call [") ||
									strings.Contains(lc, "```") {
									log.Printf("[handleChat] Self-Evolution trigger skipped: detected meta-content (regex/code)")
								} else {
									// Double check: if it's just a tool name mention without execution context, skip
									// Real execution usually involves JSON-like structure "{" or special tokens "<|"
									isRealExecution := strings.Contains(lc, "<|") || (strings.Contains(lc, "{") && strings.Contains(lc, ":"))

									if isRealExecution {
										log.Printf("[handleChat] Invalid tool pattern detected in content: %s", content)
										// 🛡️ REPLACEMENT: Instead of Self-Evolution (which loops), mark for Self-Correction
										needsCorrection = true
										badContentCapture = content // Capture the snippet for the prompt
									} else {
										// If it's just a word "tool" or "function" without structure, ignore
									}
								}
							}
						}
					}
				}

				// Forward the line (if not already continued by custom format or tool logic)
				fmt.Fprintf(w, "%s\n\n", line)
				flusher.Flush()

			} else {
				// Forward non-data lines (e.g. event: ...)
				fmt.Fprintf(w, "%s\n\n", line)
				flusher.Flush()
			}
		}

		resp.Body.Close() // Explicit close after scanner is done with this turn

		if err := scanner.Err(); err != nil {
			log.Printf("[handleChat] Stream scanner error: %v", err)
		}

		// 🛠️ Structured Output Support (JSON)
		// Check if buffer looks like a complete JSON object from a Structured Output model
		// Pattern: {"thought": "...", "tool_name": "...", "tool_arguments": ...}
		if isBuffering || (strings.HasPrefix(strings.TrimSpace(buffer), "{") && strings.Contains(buffer, "\"tool_name\"")) {
			var structTool StructuredToolCall
			if err := json.Unmarshal([]byte(buffer), &structTool); err == nil {
				if structTool.ToolName != "" {
					log.Printf("[handleChat] Detected Structured JSON Tool Call: %s", structTool.ToolName)
					toolName = structTool.ToolName

					// Convert arguments back to JSON string for consistent handling
					if argsBytes, err := json.Marshal(structTool.ToolArguments); err == nil {
						toolArgsStr = string(argsBytes)
					} else {
						toolArgsStr = "{}"
					}

					toolExecutedThisTurn = true
					lastToolName = toolName
					lastToolArgsStr = toolArgsStr

					// Emit events to frontend
					startEvt := map[string]string{
						"type": "tool_call.start",
						"tool": toolName,
					}
					startBytes, _ := json.Marshal(startEvt)
					fmt.Fprintf(w, "data: %s\n\n", string(startBytes))

					argsEvt := map[string]interface{}{
						"type":      "tool_call.arguments",
						"tool":      toolName,
						"arguments": structTool.ToolArguments,
					}
					argsBytes, _ := json.Marshal(argsEvt)
					fmt.Fprintf(w, "data: %s\n\n", string(argsBytes))

					// Clear buffer and stop any further buffering
					buffer = ""
					isBuffering = false
					flusher.Flush()
					continue
				}
			}
		}

		// 🛠️ FINAL BUFFER FLUSH: If we were buffering and the stream ended, flush what's left.
		if isBuffering && len(buffer) > 0 {
			log.Printf("[handleChat] Final buffer flush triggered (Stream End)")
			payload := map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"delta": map[string]string{
							"content": buffer,
						},
					},
				},
			}
			jsonBytes, _ := json.Marshal(payload)
			fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
			fullResponse += buffer
			lastSavedBufferForTurn = buffer // Save for history before clearing
			buffer = ""
			flusher.Flush()
		} else if len(partialTagBuffer) > 0 {
			// Flush partial tag buffer if stream ends
			payload := map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"delta": map[string]string{
							"content": partialTagBuffer,
						},
					},
				},
			}
			jsonBytes, _ := json.Marshal(payload)
			fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
			fullResponse += partialTagBuffer
			partialTagBuffer = ""
			flusher.Flush()
		}

		log.Printf("[handleChat-DEBUG] turn %d processing complete. Total response len: %d", turn, len(fullResponse))

		// 🛡️ TOOL EXECUTION & LOOP LOGIC
		if toolExecutedThisTurn {
			log.Printf("[handleChat] Turn %d detected Tool Call: %s. Executing...", turn, lastToolName)

			// 1. Execute Tool
			result, err := mcp.ExecuteToolByName(lastToolName, []byte(lastToolArgsStr), userID, enableMemory, disabledTools)
			var toolResultEvt map[string]interface{}
			if err != nil {
				log.Printf("[handleChat] Tool Execution Failed: %v", err)
				toolResultEvt = map[string]interface{}{
					"type":   "tool_call.failure",
					"tool":   lastToolName,
					"reason": err.Error(),
				}
				result = fmt.Sprintf("Error executing tool %s: %v", lastToolName, err)
			} else {
				log.Printf("[handleChat] Tool Execution Success.")
				toolResultEvt = map[string]interface{}{
					"type": "tool_call.success",
					"tool": lastToolName,
				}
			}

			// Emit Result Event to Frontend
			resBytes, _ := json.Marshal(toolResultEvt)
			fmt.Fprintf(w, "data: %s\n\n", string(resBytes))
			flusher.Flush()

			// 2. Prepare Follow-up Request
			// We feed the result back as a hidden user message or a tool response if the model supports it.
			// For consistency across modes, we'll use a simulated message.

			if llmMode == "stateful" {
				// Stateful: use previous_response_id of the JUST FINISHED assistant turn
				if lastResponseID == "" {
					log.Printf("[handleChat] WARNING: No lastResponseID captured for turn %d. Multi-turn might break.", turn)
				}
				reqMap = map[string]interface{}{
					"model":                modelID,
					"input":                fmt.Sprintf("Tool Result (%s): %s", lastToolName, result),
					"previous_response_id": lastResponseID,
					"stream":               true,
				}
			} else {
				// Standard: Append Assistant turn and Tool Result turn
				msgs, _ := reqMap["messages"].([]interface{})
				// Add what the assistant just said (the tool call) - use the saved buffer
				msgs = append(msgs, map[string]interface{}{
					"role":    "assistant",
					"content": lastSavedBufferForTurn,
				})
				// Add the result
				msgs = append(msgs, map[string]interface{}{
					"role":    "user",
					"content": fmt.Sprintf("Tool Result (%s): %s", lastToolName, result),
				})
				reqMap["messages"] = msgs
			}

			// Reinject integrations for the next turn
			if enableMCP {
				reqMap["integrations"] = []string{"mcp/dinkisstyle-gateway"}
			}

			// Update body for next turn
			body, _ = json.Marshal(reqMap)
			continue // Start next turn loop
		}

		// If no tool executed, we are done with all turns
		break
	} // --- TURN LOOP END ---

	// 🛡️ SELF-CORRECTION TRIGGER (Only if we didn't loop or at the very end)
	if needsCorrection && badContentCapture != "" {
		log.Printf("[handleChat] Triggering Self-Correction for invalid tool format...")

		// Prepare Correction Request
		correctionPrompt := mcp.SelfCorrectionPromptTemplate(badContentCapture)
		var correctionReq map[string]interface{}

		// Determine if we are in stateful mode or standard mode
		// We need to re-parse body or re-use reqMap if available.
		// Since reqMap was local to an if block earlier, we might not have it here.
		// We will reconstruct a minimal valid request based on llmMode.

		if llmMode == "stateful" {
			// Stateful: just send input and previous_response_id
			// We need the response ID from the JUST FINISHED stream.
			// It was in the 'chat.end' event: "response_id": "resp_..."
			// However, capturing it from the stream is hard without parsing every chunk JSON.
			// Fallback: Just send a new message with the SAME previous_response_id as the original request,
			// effectively branching or continuing.
			// But original 'reqMap' is not in scope here. We need to parse 'body' again or lift 'reqMap' scope.
			// Since parsing is cheap, let's re-parse 'body' to get previous_id.
			var tempMap map[string]interface{}
			if err := json.Unmarshal(body, &tempMap); err == nil {
				// Use the lastResponseID from the just-completed turn if available
				// This chains the correction AFTER the bad response.
				correctionReq = map[string]interface{}{
					"model":       modelID,
					"input":       correctionPrompt, // Just the prompt
					"stream":      true,
					"temperature": 0.1,
				}

				if enableMCP {
					correctionReq["integrations"] = []string{"mcp/dinkisstyle-gateway"}
				}

				if lastResponseID != "" {
					correctionReq["previous_response_id"] = lastResponseID
				} else {
					// Fallback: fork from original parent
					if pid, ok := tempMap["previous_response_id"].(string); ok && pid != "" {
						correctionReq["previous_response_id"] = pid
					}
				}
			}
		} else {
			// Standard/Stateless: Send full messages array with correction appended
			var tempMap map[string]interface{}
			if err := json.Unmarshal(body, &tempMap); err == nil {
				if msgs, ok := tempMap["messages"].([]interface{}); ok {
					newMessages := append(msgs, map[string]interface{}{
						"role":    "assistant",
						"content": fullResponse,
					})
					newMessages = append(newMessages, map[string]interface{}{
						"role":    "system",
						"content": correctionPrompt,
					})

					correctionReq = map[string]interface{}{
						"model":       modelID,
						"messages":    newMessages,
						"stream":      true,
						"temperature": 0.1,
					}
					if enableMCP {
						correctionReq["integrations"] = []string{"mcp/dinkisstyle-gateway"}
					}
				}
			}
		}

		if correctionReq != nil {
			jsonPayload, _ := json.Marshal(correctionReq)

			// Use 'url' which is defined in handleChat scope (we need to verify this variable name)
			// Looking at code, 'url' variable holds the endpoint.
			// If 'url' is not available, we reconstruct it:
			targetURL := app.llmEndpoint
			if !strings.HasSuffix(targetURL, "/v1/chat/completions") && !strings.HasSuffix(targetURL, "/api/v1/chat") {
				// Basic fix, though precise path depends on mode.
				// Ideally we use a variable that holds the valid endpoint used earlier.
				// Let's assume 'app.llmEndpoint' + appropriate suffix if needed, or better:
				// The variable 'url' IS usually defined in handleChat. Let's check previous context.
				// In `handleChat`:
				// url := app.llmEndpoint
				// So we can use 'url'.
			}
			// Force valid URL for safety if 'url' is not in scope of this block (it should be)
			// Actually, to be safe against scope issues, we use 'app.llmEndpoint' and fix path.
			reqUrl := app.llmEndpoint
			if llmMode == "stateful" && !strings.Contains(reqUrl, "chat") {
				reqUrl = strings.TrimSuffix(reqUrl, "/") + "/api/v1/chat"
			} else if !strings.Contains(reqUrl, "chat") {
				reqUrl = strings.TrimSuffix(reqUrl, "/") + "/v1/chat/completions"
			}

			req, _ := http.NewRequest("POST", reqUrl, bytes.NewBuffer(jsonPayload))
			req.Header.Set("Content-Type", "application/json")
			if app.llmApiToken != "" {
				req.Header.Set("Authorization", "Bearer "+app.llmApiToken)
			}

			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				correctionScanner := bufio.NewScanner(resp.Body)
				for correctionScanner.Scan() {
					line := correctionScanner.Text()
					if strings.HasPrefix(line, "data: ") {
						fmt.Fprintf(w, "%s\n\n", line)
						flusher.Flush()
					}
				}
			} else {
				log.Printf("[handleChat] Self-Correction Request Failed: %v", err)
			}
		}
	}

	// 🔍 FALLBACK logging
	if enableMemory && len(messagesForMemory) > 0 && fullResponse != "" && !memoryLogged {
		log.Printf("[handleChat] Fallback Memory Logging Triggered")
		go logChatToHistory(userID, messagesForMemory, fullResponse, modelID)
	}
}

// Helper to dedup memory check code
func checkAndLogMemoryWith(userID string, messages []map[string]interface{}, response string, msgLogged *bool, modelID string) {
	if *msgLogged {
		return
	}
	if len(messages) > 0 && response != "" {
		log.Printf("[handleChat] Stream End Trigger. Logging to memory...")
		go logChatToHistory(userID, messages, response, modelID)
		*msgLogged = true
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

// logChatToHistory appends the latest chat turn to a log file for async processing.
func logChatToHistory(userID string, messages []map[string]interface{}, assistantResponse string, modelID string) {
	log.Printf("[AsyncMemory] logChatToHistory called for user: %s, model: %s, msgs: %d", userID, modelID, len(messages))

	// 1. Find the last user message
	var lastUserMsg string
	// Iterate backwards to find the most recent user message
	for i := len(messages) - 1; i >= 0; i-- {
		if role, ok := messages[i]["role"].(string); ok && role == "user" {
			if content, ok := messages[i]["content"].(string); ok {
				lastUserMsg = content
				break
			}
		}
	}

	if lastUserMsg == "" {
		log.Printf("[AsyncMemory] No user message found in history. Skipping.")
		return // No user message to log?
	}

	// 2. Create Log Entry
	entry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"user":      lastUserMsg,
		"assistant": assistantResponse,
		"model":     modelID,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("[AsyncMemory] Failed to marshal log entry: %v", err)
		return
	}

	// 3. Append to File
	logPath, err := mcp.GetUserMemoryFilePath(userID, "chat_history.log")
	if err != nil {
		log.Printf("[AsyncMemory] Failed to get log path: %v", err)
		return
	}

	log.Printf("[AsyncMemory] Writing to: %s", logPath)

	// Ensure directory exists (GetUserMemoryFilePath calls GetUserMemoryDir which creates the dir)
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		log.Printf("[AsyncMemory] Failed to create log directory: %v", err)
		return
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[AsyncMemory] Failed to open log file: %v", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(string(data) + "\n"); err != nil {
		log.Printf("[AsyncMemory] Failed to write to log file: %v", err)
	} else {
		log.Printf("[AsyncMemory] Successfully wrote log entry.")
	}
}
