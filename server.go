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
)

// preloadUserMemory reads the user's memory file and returns its content for injection.
// This ensures the LLM has access to stored user preferences even without tool calls.
// Now uses the index for efficiency instead of raw log.
// Returns empty string if memory is disabled, file doesn't exist, or is empty.
func preloadUserMemory(userID string, enableMemory bool) string {
	if !enableMemory {
		return ""
	}

	// Try index-based summary first (efficient)
	summary := mcp.GetIndexSummaryForPrompt(userID)
	if summary != "" {
		log.Printf("[preloadUserMemory] Loaded index summary for user %s (%d chars)", userID, len(summary))
		return summary
	}

	// Fallback: read raw file and try to build index
	filePath, err := mcp.GetUserMemoryPath(userID)
	if err != nil {
		log.Printf("[preloadUserMemory] Failed to get path for user %s: %v", userID, err)
		return ""
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "" // No memory yet
		}
		log.Printf("[preloadUserMemory] Failed to read memory for user %s: %v", userID, err)
		return ""
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return ""
	}

	// Build index from raw file for future use
	go func() {
		if _, err := mcp.RebuildAndSaveIndex(userID); err != nil {
			log.Printf("[preloadUserMemory] Background index build failed: %v", err)
		}
	}()

	// Truncate if too large to avoid context overflow (max ~4000 chars)
	maxLen := 4000
	if len(content) > maxLen {
		content = content[:maxLen] + "\n... (memory truncated)"
	}

	log.Printf("[preloadUserMemory] Loaded %d bytes of raw memory for user %s", len(content), userID)
	return content
}

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
						mcp.SetContext(user.ID, *newCfg.EnableMemory)
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
							app.SetEnableMemory(*newCfg.EnableMemory)
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
			"enable_memory": app.enableMemory,
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
	enableMemory := app.enableMemory // Default global setting

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
			if user.Settings.EnableMemory != nil {
				enableMemory = *user.Settings.EnableMemory
			}
		}
	}

	// Set MCP Context for this user interaction
	// This ensures that when LM Studio calls back to MCP, it has the correct context
	mcp.SetContext(userID, enableMemory)

	// Sanitize endpoint: Remove trailing slash and optional /v1 suffix if user included it
	endpoint := strings.TrimRight(endpointRaw, "/")
	endpoint = strings.TrimSuffix(endpoint, "/v1")
	token := strings.TrimSpace(tokenRaw)

	// Sanitize token: Remove "Bearer " prefix if user pasted it
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}

	// Inject MCP integration if enabled AND NOT IN STANDARD MODE
	// Standard Mode (OpenAI compliant) with 'integrations' field might trigger strict auth in LM Studio.
	if enableMCP && llmMode != "standard" {
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
				// Important: Must update body for the request
				if newBody, err := json.Marshal(reqMap); err == nil {
					body = newBody
					log.Println("[handleChat] Injected MCP integration into request")
				} else {
					log.Printf("[handleChat] Failed to marshal new body with MCP: %v", err)
				}
			} else {
				log.Println("[handleChat] MCP integration already present")
			}
		} else {
			log.Printf("[handleChat] Failed to unmarshal body for MCP injection: %v", err)
		}

		// EXTRA SAFEGUARD: Inject System Prompt instruction for cleaner Tool Calls
		// Qwen/VL models often mess up XML tags (nested or unclosed).
		if err := json.Unmarshal(body, &reqMap); err == nil {
			if messages, ok := reqMap["messages"].([]interface{}); ok {
				foundSystem := false

				// Preload user memory for injection
				preloadedMemory := preloadUserMemory(userID, enableMemory)

				for i, msg := range messages {
					if m, ok := msg.(map[string]interface{}); ok {
						if role, ok := m["role"].(string); ok && role == "system" {
							// Append instruction to existing system prompt
							if content, ok := m["content"].(string); ok {
								newContent := content + "\n\nIMPORTANT: When using tools, output a SINGLE valid <tool_call> block. Do NOT nest tool_call tags. Ensure strict XML syntax."
								newContent += "\n- CURRENT_TIME: " + time.Now().Format("2006-01-02 15:04:05 Monday")
								if enableMemory {
									if preloadedMemory != "" {
										// Inject actual memory content
										newContent += "\n\n=== USER'S SAVED MEMORY (already loaded) ===\n" + preloadedMemory + "\n=== END OF SAVED MEMORY ===\n"
										newContent += "Use this information to personalize your responses. To update memory, use `personal_memory` (action='upsert', content='Key: Value')."
									} else {
										// No saved memory yet
										newContent += "\n- USER_MEMORY: Empty or not yet created. When user shares facts about themselves, use `personal_memory` (action='upsert', content='Key: Value') to save them."
									}
								}
								m["content"] = newContent
								messages[i] = m
								foundSystem = true
								break
							}
						}
					}
				}

				// If no system prompt found, prepend one
				if !foundSystem {
					instr := "You are a helpful assistant. IMPORTANT: For tools, use a SINGLE <tool_call> block. No nesting."
					instr += "\n- CURRENT_TIME: " + time.Now().Format("2006-01-02 15:04:05 Monday")
					if enableMemory {
						if preloadedMemory != "" {
							// Inject actual memory content
							instr += "\n\n=== USER'S SAVED MEMORY (already loaded) ===\n" + preloadedMemory + "\n=== END OF SAVED MEMORY ===\n"
							instr += "Use this information to personalize your responses. To update memory, use `personal_memory` (action='upsert', content='Key: Value')."
						} else {
							// No saved memory yet
							instr += "\n- USER_MEMORY: Empty or not yet created. When user shares facts about themselves, use `personal_memory` (action='upsert', content='Key: Value') to save them."
						}
					}
					newMsg := map[string]interface{}{
						"role":    "system",
						"content": instr,
					}
					messages = append([]interface{}{newMsg}, messages...)
					foundSystem = true
				}

				if foundSystem {
					reqMap["messages"] = messages
					if newBody, err := json.Marshal(reqMap); err == nil {
						body = newBody
						if preloadedMemory != "" {
							log.Println("[handleChat] Injected preloaded memory into System Prompt")
						} else {
							log.Println("[handleChat] Injected Tool Safety Instruction (no memory to preload)")
						}
					}
				}
			}
		}
	} else {
		log.Printf("[handleChat] MCP injection skipped (EnableMCP=%v, Mode=%s)", enableMCP, llmMode)
	}

	// DEBUG LOG
	log.Printf("[handleChat] User: %s, Mode: %s, Endpoint: %s, HasToken: %v", userID, llmMode, endpoint, token != "")

	if llmMode == "stateful" {
		llmURL = endpoint + "/api/v1/chat"
	} else {
		llmURL = endpoint + "/v1/chat/completions"
	}
	log.Printf("[handleChat] Proxying to LLM URL: %s", llmURL)

	// Use r.Context() to propagate cancellation from frontend
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
		http.Error(w, fmt.Sprintf("LLM connection failed: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

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

		http.Error(w, fmt.Sprintf("LLM error: %s", errorMsg), resp.StatusCode)
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

	// Text-based tool call detection state
	var accumulatedContent strings.Builder
	toolCallPattern := regexp.MustCompile(`\{"name"\s*:\s*"([^"]+)"\s*,\s*"arguments"\s*:\s*(\{[^}]+\})\}`)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Filter out model-specific reasoning tags if it's a data line
		if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
			// Strip common internal reasoning/channel/special tags observed in models (DeepSeek, Qwen, etc.)
			// We use a regex that handles both <|token|> and <|channel|>style<|message|> patterns.
			// Specifically targeting: <|channel|>, <|message|>, <|start|>, <|end|>, <|thought|>, etc.
			re := regexp.MustCompile(`(?i)<\|(?:channel|message|start|end|thought|analysis|final|start_header_id|end_header_id|assistant|user|system)[^>]*\|?>`)
			line = re.ReplaceAllString(line, "")

			// Try to extract content from SSE data for tool call detection
			dataContent := strings.TrimPrefix(line, "data: ")
			if dataContent != "" && dataContent != "[DONE]" {
				var sseData map[string]interface{}
				if json.Unmarshal([]byte(dataContent), &sseData) == nil {
					// Extract content from choices[0].delta.content or choices[0].message.content
					if choices, ok := sseData["choices"].([]interface{}); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]interface{}); ok {
							if delta, ok := choice["delta"].(map[string]interface{}); ok {
								if content, ok := delta["content"].(string); ok {
									accumulatedContent.WriteString(content)
								}
							}
							if msg, ok := choice["message"].(map[string]interface{}); ok {
								if content, ok := msg["content"].(string); ok {
									accumulatedContent.WriteString(content)
								}
							}
						}
					}
					// Also check for output array format (LM Studio stateful)
					if outputs, ok := sseData["output"].([]interface{}); ok {
						for _, output := range outputs {
							if outMap, ok := output.(map[string]interface{}); ok {
								if outType, ok := outMap["type"].(string); ok && outType == "message" {
									if content, ok := outMap["content"].(string); ok {
										accumulatedContent.WriteString(content)
									}
								}
							}
						}
					}
				}
			}
		}

		fmt.Fprintf(w, "%s\n\n", line)
		flusher.Flush()

		// Check for text-based tool call pattern in accumulated content
		accumulated := accumulatedContent.String()
		if matches := toolCallPattern.FindStringSubmatch(accumulated); len(matches) == 3 {
			toolName := matches[1]
			toolArgs := matches[2]
			log.Printf("[handleChat] Detected text-based tool call: %s with args: %s", toolName, toolArgs)

			// Execute tool
			result, err := mcp.ExecuteToolByName(toolName, []byte(toolArgs), userID, enableMemory)
			if err != nil {
				log.Printf("[handleChat] Text-based tool call error: %v", err)
				result = fmt.Sprintf("Error executing tool: %v", err)
			}

			// Send tool result as SSE event
			toolResultEvent := map[string]interface{}{
				"type": "tool_call.result",
				"tool": toolName,
				"result": map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": result},
					},
				},
			}
			resultBytes, _ := json.Marshal(toolResultEvent)
			fmt.Fprintf(w, "data: %s\n\n", string(resultBytes))
			flusher.Flush()
			log.Printf("[handleChat] Sent text-based tool result for: %s", toolName)

			// Clear accumulated content after processing to avoid duplicate triggers
			accumulatedContent.Reset()
		}
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
