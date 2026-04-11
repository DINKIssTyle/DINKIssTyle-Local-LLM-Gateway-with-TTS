/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dinkisstyle-chat/internal/mcp"

	"golang.org/x/crypto/bcrypt"
)

// UserSettings holds user-specific overrides
type UserSettings struct {
	ApiEndpoint           *string                    `json:"api_endpoint,omitempty"`
	ApiToken              *string                    `json:"api_token,omitempty"`
	SecondaryModel        *string                    `json:"secondary_model,omitempty"`
	LLMMode               *string                    `json:"llm_mode,omitempty"`
	ContextStrategy       *string                    `json:"context_strategy,omitempty"`
	EnableTTS             *bool                      `json:"enable_tts,omitempty"`
	EnableMCP             *bool                      `json:"enable_mcp,omitempty"`
	EnableMemory          *bool                      `json:"enable_memory,omitempty"`
	StatefulTurnLimit     *int                       `json:"stateful_turn_limit,omitempty"`
	StatefulCharBudget    *int                       `json:"stateful_char_budget,omitempty"`
	StatefulTokenBudget   *int                       `json:"stateful_token_budget,omitempty"`
	TTSConfig             *ServerTTSConfig           `json:"tts_config,omitempty"`
	EmbeddingConfig       *EmbeddingModelConfig      `json:"embedding_config,omitempty"`
	MemoryRetention       *mcp.MemoryRetentionConfig `json:"memory_retention,omitempty"`
	DisabledTools         []string                   `json:"disabled_tools,omitempty"`
	DisallowedCommands    []string                   `json:"disallowed_commands,omitempty"`
	DisallowedDirectories []string                   `json:"disallowed_directories,omitempty"`
}

// User represents a user account
type User struct {
	ID           string       `json:"id"`
	PasswordHash string       `json:"password_hash"`
	Role         string       `json:"role"` // "admin" or "user"
	CreatedAt    string       `json:"created_at"`
	Settings     UserSettings `json:"settings"`
}

// Session represents an active user session
type Session struct {
	UserID    string
	ExpiresAt time.Time
}

// AuthManager handles user authentication
type AuthManager struct {
	users     map[string]*User
	usersFile string
	mu        sync.RWMutex
	sessionMu sync.RWMutex
}

var disabledToolAliasGroups = map[string][]string{
	"personal_memory": {
		"search_memory",
		"read_memory",
		"read_memory_context",
		"delete_memory",
	},
}

func expandDisabledToolAliases(tools []string) []string {
	if len(tools) == 0 {
		return nil
	}

	seen := make(map[string]bool, len(tools))
	expanded := make([]string, 0, len(tools))
	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool == "" {
			continue
		}
		if members, ok := disabledToolAliasGroups[tool]; ok {
			for _, member := range members {
				if seen[member] {
					continue
				}
				seen[member] = true
				expanded = append(expanded, member)
			}
			continue
		}
		if seen[tool] {
			continue
		}
		seen[tool] = true
		expanded = append(expanded, tool)
	}
	return expanded
}

func collapseDisabledToolsForUI(tools []string) []string {
	if len(tools) == 0 {
		return []string{}
	}

	seen := make(map[string]bool, len(tools))
	collapsed := make([]string, 0, len(tools))
	disabled := make(map[string]bool, len(tools))
	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool == "" {
			continue
		}
		disabled[tool] = true
	}

	for alias, members := range disabledToolAliasGroups {
		anyMemberDisabled := disabled[alias]
		for _, member := range members {
			if disabled[member] {
				anyMemberDisabled = true
				break
			}
		}
		if anyMemberDisabled {
			seen[alias] = true
			collapsed = append(collapsed, alias)
		}
	}

outer:
	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool == "" || seen[tool] {
			continue
		}
		for _, members := range disabledToolAliasGroups {
			for _, member := range members {
				if tool == member {
					continue outer
				}
			}
		}
		seen[tool] = true
		collapsed = append(collapsed, tool)
	}

	return collapsed
}

// NewAuthManager creates a new AuthManager
func NewAuthManager(usersFile string) *AuthManager {
	am := &AuthManager{
		users:     make(map[string]*User),
		usersFile: usersFile,
	}
	am.LoadUsers()

	return am
}

// LoadUsers loads users from JSON file
func (am *AuthManager) LoadUsers() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	data, err := os.ReadFile(am.usersFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet
		}
		return err
	}

	var users []*User
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}

	am.users = make(map[string]*User)
	for _, u := range users {
		am.users[u.ID] = u
	}
	return nil
}

// SaveUsers saves users to JSON file
func (am *AuthManager) SaveUsers() error {
	am.mu.RLock()
	defer am.mu.RUnlock()

	users := make([]*User, 0, len(am.users))
	for _, u := range am.users {
		users = append(users, u)
	}

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(am.usersFile, data, 0600)
}

// saveUsersLocked saves users to JSON file (assumes lock is already held)
func (am *AuthManager) saveUsersLocked() error {
	users := make([]*User, 0, len(am.users))
	for _, u := range am.users {
		users = append(users, u)
	}

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	dir := filepath.Dir(am.usersFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(am.usersFile, data, 0600)
}

// AddUser adds a new user
func (am *AuthManager) AddUser(id, password, role string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	return am.addUserLocked(id, password, role)
}

func (am *AuthManager) HasUsers() bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return len(am.users) > 0
}

func (am *AuthManager) InitializeAdmin(id, password string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if len(am.users) > 0 {
		return fmt.Errorf("users already initialized")
	}
	return am.addUserLocked(id, password, "admin")
}

func (am *AuthManager) addUserLocked(id, password, role string) error {
	id = strings.TrimSpace(id)
	role = strings.TrimSpace(role)
	if id == "" {
		return fmt.Errorf("user id is required")
	}
	if password == "" {
		return fmt.Errorf("password is required")
	}
	if role != "admin" && role != "user" {
		return fmt.Errorf("invalid role: must be 'admin' or 'user'")
	}
	if _, exists := am.users[id]; exists {
		return fmt.Errorf("user already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	enableMCP := true
	enableTTS := true
	voiceStyle := "F1"
	speed := float32(1.1)
	threads := 2
	engine := "supertonic"
	osRate := float32(1.0)
	osPitch := float32(1.0)

	am.users[id] = &User{
		ID:           id,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    time.Now().Format(time.RFC3339),
		Settings: UserSettings{
			EnableMCP:    &enableMCP,
			EnableTTS:    &enableTTS,
			EnableMemory: &enableMCP,
			TTSConfig: &ServerTTSConfig{
				Engine:     engine,
				VoiceStyle: voiceStyle,
				Speed:      speed,
				Threads:    threads,
				OSRate:     osRate,
				OSPitch:    osPitch,
			},
			DisallowedCommands: []string{
				"rm", "rmdir", "unlink", "dd", "mkfs", "mkfs.ext4", "mkfs.xfs", "mkfs.apfs",
				"fsck", "fsck.ext4", "fsck.xfs", "fsck_apfs", "mount", "umount", "chmod", "chown", "chgrp",
				"kill", "killall", "pkill", "shutdown", "reboot", "poweroff", "init", "telinit",
				"systemctl", "service", "crontab", "at", "curl", "wget", "scp", "rsync",
				"nc", "ncat", "ssh", "sudo", "su", "visudo", "useradd", "userdel", "usermod",
				"groupadd", "groupdel", "passwd", "export", "env", "set", "unset", "source",
				"exec", "nohup", "screen", "tmux", "history", "alias", "unalias",
			},
		},
	}

	return am.saveUsersLocked()
}

// DeleteUser removes a user
func (am *AuthManager) DeleteUser(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	delete(am.users, id)
	if err := mcp.DeleteAuthSessionsByUser(id); err != nil {
		return err
	}

	// Save while still holding lock
	return am.saveUsersLocked()
}

// Authenticate validates credentials and returns a session token.
func (am *AuthManager) Authenticate(id, password string, rememberMe bool, userAgent, clientAddr string) (string, error) {
	am.mu.RLock()
	user, exists := am.users[id]
	am.mu.RUnlock()

	if !exists {
		return "", nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", nil
	}

	// Generate session token
	token := generateToken()
	tokenHash := hashToken(token)
	expiresAt := time.Now().Add(24 * time.Hour)
	if rememberMe {
		expiresAt = time.Now().Add(30 * 24 * time.Hour)
	}

	if err := mcp.PurgeExpiredAuthSessions(time.Now()); err != nil {
		return "", err
	}
	if err := mcp.InsertAuthSession(id, tokenHash, rememberMe, userAgent, clientAddr, expiresAt); err != nil {
		return "", err
	}

	return token, nil
}

// ValidateSession checks if a session token is valid
func (am *AuthManager) ValidateSession(token string) (*User, bool) {
	tokenHash := hashToken(token)
	session, err := mcp.GetAuthSessionByTokenHash(tokenHash)
	if err != nil {
		return nil, false
	}
	if time.Now().After(session.ExpiresAt) {
		_ = mcp.DeleteAuthSession(tokenHash)
		return nil, false
	}
	_ = mcp.TouchAuthSession(tokenHash, time.Now())

	am.mu.RLock()
	user := am.users[session.UserID]
	am.mu.RUnlock()

	return user, user != nil
}

// InvalidateSession removes a session
func (am *AuthManager) InvalidateSession(token string) {
	_ = mcp.DeleteAuthSession(hashToken(token))
}

func (am *AuthManager) InvalidateAllSessions() error {
	return mcp.DeleteAllAuthSessions()
}

func (am *AuthManager) InvalidateAllSessionsForUser(id string) error {
	return mcp.DeleteAuthSessionsByUser(id)
}

// GetUsers returns list of users (without passwords)
func (am *AuthManager) GetUsers() []map[string]string {
	am.mu.RLock()
	defer am.mu.RUnlock()

	users := make([]map[string]string, 0, len(am.users))
	for _, u := range am.users {
		users = append(users, map[string]string{
			"id":         u.ID,
			"role":       u.Role,
			"created_at": u.CreatedAt,
		})
	}
	return users
}

// GetUserDetail returns detailed info for a specific user (without password)
func (am *AuthManager) GetUserDetail(id string) (map[string]interface{}, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	user, exists := am.users[id]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	// Return user info with masked API token
	apiToken := ""
	if user.Settings.ApiToken != nil && *user.Settings.ApiToken != "" {
		// Mask the token, show only last 4 characters
		token := *user.Settings.ApiToken
		if len(token) > 4 {
			apiToken = "••••••••" + token[len(token)-4:]
		} else {
			apiToken = "••••"
		}
	}

	return map[string]interface{}{
		"id":             user.ID,
		"role":           user.Role,
		"created_at":     user.CreatedAt,
		"has_api_key":    user.Settings.ApiToken != nil && *user.Settings.ApiToken != "",
		"api_key_masked": apiToken,
		"disabled_tools": user.Settings.DisabledTools,
	}, nil
}

// UpdatePassword changes a user's password (admin action, no current password required)
func (am *AuthManager) UpdatePassword(id, newPassword string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, exists := am.users[id]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hash)
	if err := mcp.DeleteAuthSessionsByUser(id); err != nil {
		return err
	}

	return am.saveUsersLocked()
}

// UpdateUserRole changes a user's role (admin/user)
func (am *AuthManager) UpdateUserRole(id, role string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, exists := am.users[id]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Validate role
	if role != "admin" && role != "user" {
		return fmt.Errorf("invalid role: must be 'admin' or 'user'")
	}

	user.Role = role

	return am.saveUsersLocked()
}

// SetUserApiToken sets the API token for a specific user
func (am *AuthManager) SetUserApiToken(id, token string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, exists := am.users[id]
	if !exists {
		return fmt.Errorf("user not found")
	}

	user.Settings.ApiToken = &token

	return am.saveUsersLocked()
}

// GetUserApiToken returns the API token for a specific user (full token, not masked)
func (am *AuthManager) GetUserApiToken(id string) (string, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	user, exists := am.users[id]
	if !exists {
		return "", fmt.Errorf("user not found")
	}

	if user.Settings.ApiToken == nil {
		return "", nil
	}

	return *user.Settings.ApiToken, nil
}

func (am *AuthManager) GetUserMemoryRetentionConfig(id string) (mcp.MemoryRetentionConfig, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	user, exists := am.users[id]
	if !exists {
		return mcp.MemoryRetentionConfig{}, fmt.Errorf("user not found")
	}
	if user.Settings.MemoryRetention == nil {
		return mcp.DefaultMemoryRetentionConfig(), nil
	}
	return *user.Settings.MemoryRetention, nil
}

func (am *AuthManager) ResolveUserMemoryRetentionConfig(id string) mcp.MemoryRetentionConfig {
	am.mu.RLock()
	defer am.mu.RUnlock()

	user, exists := am.users[id]
	if !exists || user.Settings.MemoryRetention == nil {
		return mcp.DefaultMemoryRetentionConfig()
	}
	return *user.Settings.MemoryRetention
}

func (am *AuthManager) SetUserMemoryRetentionConfig(id string, cfg mcp.MemoryRetentionConfig) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, exists := am.users[id]
	if !exists {
		return fmt.Errorf("user not found")
	}

	normalized := mcp.DefaultMemoryRetentionConfig()
	if cfg.CoreDays >= 0 {
		normalized.CoreDays = cfg.CoreDays
	}
	if cfg.WorkingDays >= 0 {
		normalized.WorkingDays = cfg.WorkingDays
	}
	if cfg.EphemeralDays >= 0 {
		normalized.EphemeralDays = cfg.EphemeralDays
	}
	user.Settings.MemoryRetention = &normalized

	return am.saveUsersLocked()
}

// SetUserDisabledTools sets the list of disabled tools for a specific user
func (am *AuthManager) SetUserDisabledTools(id string, tools []string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, exists := am.users[id]
	if !exists {
		return fmt.Errorf("user not found")
	}

	user.Settings.DisabledTools = expandDisabledToolAliases(tools)
	return am.saveUsersLocked()
}

// GetUserDisabledTools returns the list of disabled tools for a specific user
func (am *AuthManager) GetUserDisabledTools(id string) ([]string, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	user, exists := am.users[id]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	if user.Settings.DisabledTools == nil {
		return []string{}, nil
	}
	return collapseDisabledToolsForUI(user.Settings.DisabledTools), nil
}

// generateToken creates a random session token
func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func getClientAddress(r *http.Request) string {
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			parts := strings.Split(value, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
		return value
	}

	host := strings.TrimSpace(r.RemoteAddr)
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func extractSessionTokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie("session"); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return strings.TrimSpace(cookie.Value)
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		token := strings.TrimSpace(authHeader[len("Bearer "):])
		if token != "" {
			return token
		}
	}

	headerToken := strings.TrimSpace(r.Header.Get("X-Session-Token"))
	if headerToken != "" {
		return headerToken
	}

	return ""
}

// AuthMiddleware wraps an http handler with authentication
func AuthMiddleware(am *AuthManager, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractSessionTokenFromRequest(r)
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, valid := am.ValidateSession(token)
		if !valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Store user in context (simplified - just header)
		r.Header.Set("X-User-ID", user.ID)
		r.Header.Set("X-User-Role", user.Role)
		next(w, r)
	}
}

// AdminMiddleware requires admin role
func AdminMiddleware(am *AuthManager, next http.HandlerFunc) http.HandlerFunc {
	return AuthMiddleware(am, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-User-Role") != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

// SetUserDisallowedCommands sets the list of disallowed commands for a specific user
func (am *AuthManager) SetUserDisallowedCommands(id string, cmds []string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, exists := am.users[id]
	if !exists {
		return fmt.Errorf("user not found")
	}

	user.Settings.DisallowedCommands = cmds
	return am.saveUsersLocked()
}

// GetUserDisallowedCommands returns the list of disallowed commands for a specific user
func (am *AuthManager) GetUserDisallowedCommands(id string) ([]string, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	user, exists := am.users[id]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	if user.Settings.DisallowedCommands == nil {
		return []string{}, nil
	}
	return user.Settings.DisallowedCommands, nil
}

// SetUserDisallowedDirectories sets the list of disallowed directories for a specific user
func (am *AuthManager) SetUserDisallowedDirectories(id string, dirs []string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, exists := am.users[id]
	if !exists {
		return fmt.Errorf("user not found")
	}

	user.Settings.DisallowedDirectories = dirs
	return am.saveUsersLocked()
}

// GetUserDisallowedDirectories returns the list of disallowed directories for a specific user
func (am *AuthManager) GetUserDisallowedDirectories(id string) ([]string, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	user, exists := am.users[id]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	if user.Settings.DisallowedDirectories == nil {
		return []string{}, nil
	}
	return user.Settings.DisallowedDirectories, nil
}
