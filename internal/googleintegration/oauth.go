package googleintegration

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func splitScopes(s string) []string { return strings.Fields(s) }
func randomValue() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b), err
}

// Begin uses a short-lived loopback listener, state and PKCE. No callback is
// exposed on the gateway's public HTTP/HTTPS listener.
func (m *Manager) Begin(user string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadLocked(); err != nil {
		return "", err
	}
	r := m.records[user]
	if r.Settings.ClientID == "" || r.Settings.ClientSecret == "" {
		return "", fmt.Errorf("configure a Google Desktop OAuth client ID and client secret first")
	}
	if len(r.Settings.Services) == 0 {
		return "", fmt.Errorf("select at least one Google service")
	}
	state, err := randomValue()
	if err != nil {
		return "", err
	}
	verifier, err := randomValue()
	if err != nil {
		return "", err
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	m.invalidateLocked(user)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	p := &pendingLogin{state: state, verifier: verifier, redirect: "http://" + ln.Addr().String() + "/oauth2/callback", revision: m.revisions[user], cancel: cancel}
	m.pending[user] = p
	m.messages[user] = ""
	var server *http.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/callback", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if req.Method != http.MethodGet || req.URL.Query().Get("state") != p.state {
			http.Error(w, "Invalid authorization callback.", 400)
			return
		}
		err := m.finish(ctx, user, p, req.URL.Query())
		if err != nil {
			http.Error(w, "Google connection failed. Return to the gateway for details.", 400)
		} else {
			fmt.Fprint(w, "Google connected. You can close this window and return to the gateway.\nGoogle 연결이 완료되었습니다. 게이트웨이로 돌아가세요.")
		}
		m.mu.Lock()
		if m.pending[user] == p {
			delete(m.pending, user)
			if err != nil {
				m.messages[user] = err.Error()
			} else {
				m.messages[user] = ""
			}
		}
		m.mu.Unlock()
		cancel()
	})
	server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(ln) }()
	go func() {
		<-ctx.Done()
		_ = server.Close()
		m.mu.Lock()
		if m.pending[user] == p {
			delete(m.pending, user)
			m.messages[user] = "Google sign-in expired. Please connect again."
		}
		m.mu.Unlock()
	}()
	scopes := []string{"https://www.googleapis.com/auth/userinfo.email"}
	for _, s := range r.Settings.Services {
		scopes = append(scopes, serviceScopes[s]...)
	}
	challenge := sha256.Sum256([]byte(verifier))
	q := url.Values{"client_id": {r.Settings.ClientID}, "redirect_uri": {p.redirect}, "response_type": {"code"}, "scope": {strings.Join(scopes, " ")}, "state": {state}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "code_challenge_method": {"S256"}, "access_type": {"offline"}, "prompt": {"consent select_account"}}
	return m.authURL + "?" + q.Encode(), nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

func (m *Manager) token(ctx context.Context, values url.Values) (tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", m.tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := m.client.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("Google token request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return tokenResponse{}, googleTokenError(resp.StatusCode, resp.Body)
	}
	var t tokenResponse
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&t); err != nil {
		return t, fmt.Errorf("invalid Google token response")
	}
	if t.AccessToken == "" {
		return t, fmt.Errorf("Google returned no access token")
	}
	return t, nil
}
func (m *Manager) finish(ctx context.Context, user string, p *pendingLogin, q url.Values) error {
	if q.Get("error") != "" {
		return fmt.Errorf("Google permission was not granted; please connect again")
	}
	if q.Get("code") == "" {
		return fmt.Errorf("Google returned no authorization code")
	}
	m.mu.Lock()
	if m.pending[user] != p || m.revisions[user] != p.revision {
		m.mu.Unlock()
		return fmt.Errorf("Google connection settings changed; connect again")
	}
	r := m.records[user]
	m.mu.Unlock()
	t, err := m.token(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {q.Get("code")}, "client_id": {r.Settings.ClientID}, "client_secret": {r.Settings.ClientSecret}, "redirect_uri": {p.redirect}, "code_verifier": {p.verifier}})
	if err != nil {
		return err
	}
	if t.RefreshToken == "" {
		return fmt.Errorf("Google returned no offline access; reconnect and grant access")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", m.userinfoURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+t.AccessToken)
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("could not verify the Google account")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("could not verify the Google account")
	}
	var identity struct {
		Email string `json:"email"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&identity); err != nil || identity.Email == "" {
		return fmt.Errorf("Google returned no account email")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pending[user] != p || m.revisions[user] != p.revision {
		return fmt.Errorf("Google connection settings changed; connect again")
	}
	old := m.records[user]
	r.AccessToken = t.AccessToken
	r.RefreshToken = t.RefreshToken
	r.Expiry = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	r.Scopes = t.Scope
	r.Email = identity.Email
	m.records[user] = r
	if err = m.saveLocked(); err != nil {
		m.records[user] = old
		return err
	}
	return nil
}

// snapshot checks current account policy without holding the store lock during
// network requests, so service changes remain responsive during a slow search.
func (m *Manager) snapshot(user, service string) (record, uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadLocked(); err != nil {
		return record{}, 0, err
	}
	r := m.records[user]
	if user == "" || !contains(r.Settings.Services, service) || !authorized(r, service) {
		return record{}, 0, fmt.Errorf("Google %s is not connected and enabled for this gateway account", service)
	}
	return r, m.revisions[user], nil
}

// get only reads known Google API paths. Neither URLs nor gateway account IDs
// can be supplied by the LLM. Every request checks the current service grant.
func (m *Manager) get(ctx context.Context, user, service, path string, q url.Values) ([]byte, error) {
	r, _, err := m.snapshot(user, service)
	if err != nil {
		return nil, err
	}
	if time.Until(r.Expiry) < time.Minute {
		if err := m.refresh(ctx, user, service); err != nil {
			return nil, err
		}
		r, _, err = m.snapshot(user, service)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", m.apiBase+path+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.AccessToken)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Google %s request failed", service)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, googleAPIError(service, resp.StatusCode, resp.Body)
	}
	const max = 4 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if len(body) > max {
		return nil, fmt.Errorf("Google response exceeds 4 MiB; narrow the search or select a smaller document")
	}
	return body, nil
}

func (m *Manager) refresh(ctx context.Context, user, service string) error {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	r, revision, err := m.snapshot(user, service)
	if err != nil {
		return err
	}
	if time.Until(r.Expiry) >= time.Minute {
		return nil
	}
	t, err := m.token(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {r.RefreshToken}, "client_id": {r.Settings.ClientID}, "client_secret": {r.Settings.ClientSecret}})
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.revisions[user] != revision {
		return fmt.Errorf("Google connection settings changed; retry if this service is still enabled")
	}
	old := m.records[user]
	r.AccessToken = t.AccessToken
	r.Expiry = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	if t.RefreshToken != "" {
		r.RefreshToken = t.RefreshToken
	}
	if t.Scope != "" {
		r.Scopes = t.Scope
	}
	m.records[user] = r
	if err = m.saveLocked(); err != nil {
		m.records[user] = old
		return err
	}
	if !authorized(r, service) {
		return fmt.Errorf("Google permissions changed; reconnect this service")
	}
	return nil
}
