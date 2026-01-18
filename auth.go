/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents a user account
type User struct {
	ID           string `json:"id"`
	PasswordHash string `json:"password_hash"`
	Role         string `json:"role"` // "admin" or "user"
	CreatedAt    string `json:"created_at"`
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

	am.users[id] = &User{
		ID:           id,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    time.Now().Format(time.RFC3339),
	}

	// Save immediately
	am.mu.Unlock()
	err = am.SaveUsers()
	am.mu.Lock()
	return err
}

// DeleteUser removes a user
func (am *AuthManager) DeleteUser(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	delete(am.users, id)

	am.mu.Unlock()
	err := am.SaveUsers()
	am.mu.Lock()
	return err
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
