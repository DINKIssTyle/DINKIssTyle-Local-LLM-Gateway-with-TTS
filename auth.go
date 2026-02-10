/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// UserSettings holds user-specific overrides
type UserSettings struct {
	ApiEndpoint           *string          `json:"api_endpoint,omitempty"`
	ApiToken              *string          `json:"api_token,omitempty"`
	LLMMode               *string          `json:"llm_mode,omitempty"`
	EnableTTS             *bool            `json:"enable_tts,omitempty"`
	EnableMCP             *bool            `json:"enable_mcp,omitempty"`
	EnableMemory          *bool            `json:"enable_memory,omitempty"`
	TTSConfig             *ServerTTSConfig `json:"tts_config,omitempty"`
	DisabledTools         []string         `json:"disabled_tools,omitempty"`
	DisallowedCommands    []string         `json:"disallowed_commands,omitempty"`
	DisallowedDirectories []string         `json:"disallowed_directories,omitempty"`
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
	sessions  map[string]*Session
	usersFile string
	mu        sync.RWMutex
	sessionMu sync.RWMutex
}

// NewAuthManager creates a new AuthManager
func NewAuthManager(usersFile string) *AuthManager {
	am := &AuthManager{
		users:     make(map[string]*User),
		sessions:  make(map[string]*Session),
		usersFile: usersFile,
	}
	am.LoadUsers()

	// Create default admin if no users exist
	if len(am.users) == 0 {
		am.AddUser("admin", "admin", "admin")
	}

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

	if _, exists := am.users[id]; exists {
		return nil // User already exists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Default User Settings
	// User requested defaults:
	// Enable MCP: true
	// Enable TTS: true
	// TTS Config: Voice F1, Speed 1.1, Threads 2
	enableMCP := true
	enableTTS := true
	voiceStyle := "F1" // F1.json usually
	speed := float32(1.1)
	threads := 2

	am.users[id] = &User{
		ID:           id,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    time.Now().Format(time.RFC3339),
		Settings: UserSettings{
			// Initialize with pointers to defaults
			EnableMCP:    &enableMCP,
			EnableTTS:    &enableTTS,
			EnableMemory: &enableMCP, // Default to same as MCP for new users, or false? Let's default true if new user.
			TTSConfig: &ServerTTSConfig{
				VoiceStyle: voiceStyle,
				Speed:      speed,
				Threads:    threads,
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

	// Save while still holding lock
	return am.saveUsersLocked()
}

// DeleteUser removes a user
func (am *AuthManager) DeleteUser(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	delete(am.users, id)

	// Save while still holding lock
	return am.saveUsersLocked()
}

// Authenticate validates credentials and returns a session token
func (am *AuthManager) Authenticate(id, password string) (string, error) {
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

	am.sessionMu.Lock()
	am.sessions[token] = &Session{
		UserID:    id,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	am.sessionMu.Unlock()

	return token, nil
}

// ValidateSession checks if a session token is valid
func (am *AuthManager) ValidateSession(token string) (*User, bool) {
	am.sessionMu.RLock()
	session, exists := am.sessions[token]
	am.sessionMu.RUnlock()

	if !exists || time.Now().After(session.ExpiresAt) {
		return nil, false
	}

	am.mu.RLock()
	user := am.users[session.UserID]
	am.mu.RUnlock()

	return user, user != nil
}

// InvalidateSession removes a session
func (am *AuthManager) InvalidateSession(token string) {
	am.sessionMu.Lock()
	delete(am.sessions, token)
	am.sessionMu.Unlock()
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

// SetUserDisabledTools sets the list of disabled tools for a specific user
func (am *AuthManager) SetUserDisabledTools(id string, tools []string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, exists := am.users[id]
	if !exists {
		return fmt.Errorf("user not found")
	}

	user.Settings.DisabledTools = tools
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
	return user.Settings.DisabledTools, nil
}

// generateToken creates a random session token
func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// AuthMiddleware wraps an http handler with authentication
func AuthMiddleware(am *AuthManager, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, valid := am.ValidateSession(cookie.Value)
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
