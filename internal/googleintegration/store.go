// Package googleintegration implements account-scoped, read-only Google access.
package googleintegration

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var serviceScopes = map[string][]string{
	"gmail":    {"https://www.googleapis.com/auth/gmail.readonly"},
	"drive":    {"https://www.googleapis.com/auth/drive.readonly"},
	"calendar": {"https://www.googleapis.com/auth/calendar.events.readonly", "https://www.googleapis.com/auth/calendar.calendarlist.readonly"},
}

type Settings struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret,omitempty"`
	Services     []string `json:"services"`
}
type Status struct {
	ClientID           string   `json:"client_id"`
	HasClientSecret    bool     `json:"has_client_secret"`
	Services           []string `json:"services"`
	AuthorizedServices []string `json:"authorized_services"`
	Connected          bool     `json:"connected"`
	Email              string   `json:"email"`
	Pending            bool     `json:"pending"`
	Message            string   `json:"message"`
}
type record struct {
	Settings     Settings  `json:"settings"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry"`
	Scopes       string    `json:"scopes,omitempty"`
	Email        string    `json:"email,omitempty"`
}
type pendingLogin struct {
	state, verifier, redirect string
	revision                  uint64
	cancel                    func()
}
type Manager struct {
	refreshMu                               sync.Mutex
	mu                                      sync.Mutex
	dir                                     string
	records                                 map[string]record
	revisions                               map[string]uint64
	pending                                 map[string]*pendingLogin
	messages                                map[string]string
	client                                  *http.Client
	apiBase, tokenURL, authURL, userinfoURL string
}

func New(dir string) *Manager {
	return &Manager{dir: dir, revisions: map[string]uint64{}, pending: map[string]*pendingLogin{}, messages: map[string]string{}, client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("Google redirect refused") }}, apiBase: "https://www.googleapis.com", tokenURL: "https://oauth2.googleapis.com/token", authURL: "https://accounts.google.com/o/oauth2/v2/auth", userinfoURL: "https://www.googleapis.com/oauth2/v2/userinfo"}
}
func (m *Manager) loadLocked() error {
	if m.records != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(m.dir, "accounts.enc"))
	if os.IsNotExist(err) {
		m.records = map[string]record{}
		return nil
	}
	if err != nil {
		return err
	}
	key, err := os.ReadFile(filepath.Join(m.dir, "key"))
	if err != nil {
		return fmt.Errorf("read Google credential key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if len(data) < gcm.NonceSize() {
		return fmt.Errorf("invalid Google credential store")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return fmt.Errorf("decrypt Google credentials: %w", err)
	}
	var records map[string]record
	if err = json.Unmarshal(plain, &records); err != nil {
		return err
	}
	if records == nil {
		records = map[string]record{}
	}
	m.records = records
	return nil
}
func (m *Manager) saveLocked() error {
	if err := os.MkdirAll(m.dir, 0700); err != nil {
		return err
	}
	keyPath := filepath.Join(m.dir, "key")
	key, err := os.ReadFile(keyPath)
	if os.IsNotExist(err) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return err
		}
		f, e := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		_, err = f.Write(key)
		ce := f.Close()
		if err != nil {
			return err
		}
		if ce != nil {
			return ce
		}
	} else if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	plain, err := json.Marshal(m.records)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	encrypted := gcm.Seal(nonce, nonce, plain, nil)
	f, err := os.CreateTemp(m.dir, "credentials-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(encrypted)
	}
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err != nil {
		return err
	}
	if ce != nil {
		return ce
	}
	return os.Rename(f.Name(), filepath.Join(m.dir, "accounts.enc"))
}
func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
func authorized(r record, service string) bool {
	for _, scope := range serviceScopes[service] {
		if !contains(splitScopes(r.Scopes), scope) {
			return false
		}
	}
	return len(serviceScopes[service]) > 0 && r.RefreshToken != ""
}
func (m *Manager) Status(user string) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadLocked(); err != nil {
		return Status{}, err
	}
	r := m.records[user]
	s := Status{ClientID: r.Settings.ClientID, HasClientSecret: r.Settings.ClientSecret != "", Services: append([]string{}, r.Settings.Services...), AuthorizedServices: []string{}, Connected: r.RefreshToken != "", Email: r.Email, Pending: m.pending[user] != nil, Message: m.messages[user]}
	for _, service := range []string{"gmail", "drive", "calendar"} {
		if authorized(r, service) {
			s.AuthorizedServices = append(s.AuthorizedServices, service)
		}
	}
	return s, nil
}
func (m *Manager) Configure(user string, settings Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadLocked(); err != nil {
		return err
	}
	services := []string{}
	for _, s := range settings.Services {
		if len(serviceScopes[s]) == 0 {
			return fmt.Errorf("unknown Google service: %s", s)
		}
		if !contains(services, s) {
			services = append(services, s)
		}
	}
	settings.Services = services
	old := m.records[user]
	if settings.ClientID == old.Settings.ClientID && settings.ClientSecret == "" {
		settings.ClientSecret = old.Settings.ClientSecret
	}
	next := old
	if settings.ClientID != old.Settings.ClientID || settings.ClientSecret != old.Settings.ClientSecret {
		next = record{}
	}
	next.Settings = settings
	m.records[user] = next
	if err := m.saveLocked(); err != nil {
		m.records[user] = old
		return err
	}
	m.invalidateLocked(user)
	m.messages[user] = ""
	return nil
}
func (m *Manager) invalidateLocked(user string) {
	m.revisions[user]++
	if p := m.pending[user]; p != nil {
		p.cancel()
		delete(m.pending, user)
	}
}

// Disconnect removes local credentials. It deliberately does not revoke a shared
// Google grant, which could disconnect another gateway account using that grant.
func (m *Manager) Disconnect(user string, remove bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadLocked(); err != nil {
		return err
	}
	old, exists := m.records[user]
	if remove {
		delete(m.records, user)
	} else {
		m.records[user] = record{Settings: old.Settings}
	}
	if err := m.saveLocked(); err != nil {
		if exists {
			m.records[user] = old
		} else {
			delete(m.records, user)
		}
		return err
	}
	m.invalidateLocked(user)
	m.messages[user] = ""
	return nil
}
func (m *Manager) Allowed(user, service string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if user == "" || m.loadLocked() != nil {
		return false
	}
	r := m.records[user]
	return contains(r.Settings.Services, service) && authorized(r, service)
}
