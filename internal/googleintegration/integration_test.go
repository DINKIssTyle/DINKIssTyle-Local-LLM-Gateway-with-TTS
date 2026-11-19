package googleintegration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func mockGoogle(t *testing.T) (*Manager, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			if r.Form.Get("grant_type") == "authorization_code" && r.Form.Get("code_verifier") == "" {
				t.Error("missing PKCE verifier")
			}
			scopes := []string{"https://www.googleapis.com/auth/userinfo.email"}
			for _, s := range []string{"gmail", "drive", "calendar"} {
				scopes = append(scopes, serviceScopes[s]...)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "private-access", "refresh_token": "private-refresh", "expires_in": 3600, "scope": strings.Join(scopes, " ")})
		case "/identity":
			_ = json.NewEncoder(w).Encode(map[string]string{"email": "test@example.com"})
		default:
			calls.Add(1)
			if r.Header.Get("Authorization") != "Bearer private-access" {
				t.Error("wrong account token")
			}
			switch r.URL.Path {
			case "/gmail/v1/users/me/messages":
				io.WriteString(w, `{"messages":[{"id":"m1"}],"nextPageToken":"next"}`)
			case "/gmail/v1/users/me/messages/m1":
				io.WriteString(w, `{"id":"m1","threadId":"thread1","snippet":"preview","payload":{"mimeType":"multipart/alternative","headers":[{"name":"Subject","value":"Meeting"}],"parts":[{"mimeType":"text/html","body":{"data":"PHA-SFRNTDwvcD4"}},{"mimeType":"text/plain","body":{"data":"UGxhaW4gYm9keQ"}}]}}`)
			case "/drive/v3/files":
				if !strings.Contains(r.URL.Query().Get("q"), "fullText contains '") {
					t.Error("missing Drive search")
				}
				io.WriteString(w, `{"files":[{"id":"doc1","name":"Example","mimeType":"application/vnd.google-apps.document"}]}`)
			case "/drive/v3/files/doc1":
				io.WriteString(w, `{"id":"doc1","name":"Example","mimeType":"application/vnd.google-apps.document"}`)
			case "/drive/v3/files/doc1/export":
				if r.URL.Query().Get("mimeType") != "text/plain" {
					t.Error("wrong export")
				}
				io.WriteString(w, "문서 내용")
			case "/calendar/v3/users/me/calendarList":
				io.WriteString(w, `{"items":[{"id":"primary","primary":true,"timeZone":"Asia/Seoul"}]}`)
			case "/calendar/v3/calendars/primary/events":
				if r.URL.Query().Get("singleEvents") != "true" {
					t.Error("recurrence not expanded")
				}
				io.WriteString(w, `{"items":[{"id":"event1","start":{"date":"2026-11-18"},"end":{"date":"2026-11-19"}}]}`)
			case "/calendar/v3/calendars/primary/events/event1":
				io.WriteString(w, `{"id":"event1","summary":"Meeting"}`)
			default:
				http.NotFound(w, r)
			}
		}
	}))
	t.Cleanup(server.Close)
	m := New(t.TempDir())
	m.apiBase = server.URL
	m.tokenURL = server.URL + "/token"
	m.userinfoURL = server.URL + "/identity"
	return m, &calls
}
func connect(t *testing.T, m *Manager, user string, services []string) {
	t.Helper()
	if err := m.Configure(user, Settings{ClientID: "desktop.apps.googleusercontent.com", ClientSecret: "private-client-secret", Services: services}); err != nil {
		t.Fatal(err)
	}
	link, err := m.Begin(user)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(link)
	q := u.Query()
	m.mu.Lock()
	p := m.pending[user]
	m.mu.Unlock()
	t.Cleanup(p.cancel)
	challenge := sha256.Sum256([]byte(p.verifier))
	if q.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(challenge[:]) || q.Get("code_challenge_method") != "S256" {
		t.Fatal("invalid PKCE")
	}
	callback := p.redirect + "?" + url.Values{"state": {q.Get("state")}, "code": {"test-code"}}.Encode()
	resp, err := http.Get(callback)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("callback %d: %s", resp.StatusCode, b)
	}
}
func TestOAuthToolsPersistenceAndAccountIsolation(t *testing.T) {
	m, calls := mockGoogle(t)
	connect(t, m, "alice", []string{"gmail", "drive", "calendar"})
	status, err := m.Status("alice")
	if err != nil || !status.Connected || status.Email != "test@example.com" {
		t.Fatalf("status %+v: %v", status, err)
	}
	raw, _ := json.Marshal(status)
	if strings.Contains(string(raw), "private-") {
		t.Fatal("credentials exposed in status")
	}
	for _, file := range []string{"accounts.enc", "key"} {
		data, err := os.ReadFile(filepath.Join(m.dir, file))
		if err != nil || strings.Contains(string(data), "private-") {
			t.Fatalf("unencrypted credentials: %s", file)
		}
		info, _ := os.Stat(filepath.Join(m.dir, file))
		if info.Mode().Perm() != 0600 {
			t.Fatalf("unsafe permissions: %s", file)
		}
	}
	for _, tc := range []struct{ name, args, want string }{
		{"gmail_search", `{"query":"subject:Meeting"}`, "next"}, {"gmail_read", `{"id":"m1"}`, "Plain body"},
		{"google_drive_search", `{"query":"Example"}`, "doc1"}, {"google_drive_read", `{"id":"doc1"}`, "문서 내용"},
		{"google_calendar_list", `{}`, "Asia/Seoul"}, {"google_calendar_search", `{"time_min":"2026-11-18T00:00:00+09:00","time_max":"2026-11-19T00:00:00+09:00"}`, "2026-11-18"}, {"google_calendar_read", `{"id":"event1","calendar_id":"primary"}`, "Meeting"},
	} {
		out, err := m.Execute(context.Background(), "alice", tc.name, json.RawMessage(tc.args))
		if err != nil || !strings.Contains(out, tc.want) {
			t.Fatalf("%s: %s (%v)", tc.name, out, err)
		}
	}
	before := calls.Load()
	if _, err := m.Execute(context.Background(), "bob", "gmail_read", json.RawMessage(`{"id":"m1"}`)); err == nil {
		t.Fatal("cross-account read succeeded")
	}
	if calls.Load() != before {
		t.Fatal("unauthorized read reached Google")
	}
	reloaded := New(m.dir)
	if !reloaded.Allowed("alice", "gmail") || reloaded.Allowed("bob", "gmail") {
		t.Fatal("persisted account isolation failed")
	}
	m.mu.Lock()
	r := m.records["alice"]
	r.Expiry = time.Now().Add(-time.Hour)
	m.records["alice"] = r
	m.mu.Unlock()
	if _, err := m.Execute(context.Background(), "alice", "gmail_read", json.RawMessage(`{"id":"m1"}`)); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if err := m.Configure("alice", Settings{ClientID: status.ClientID, Services: []string{"drive"}}); err != nil {
		t.Fatal(err)
	}
	if m.Allowed("alice", "gmail") {
		t.Fatal("disabled Gmail remains allowed")
	}
	if _, err := m.Execute(context.Background(), "alice", "gmail_read", json.RawMessage(`{"id":"m1"}`)); err == nil {
		t.Fatal("stale conversation bypassed permissions")
	}
	if err := m.Disconnect("alice", false); err != nil {
		t.Fatal(err)
	}
	if m.Allowed("alice", "drive") {
		t.Fatal("disconnected account remains allowed")
	}
}
func TestOAuthInvalidStateAndChangedSettings(t *testing.T) {
	m, _ := mockGoogle(t)
	if err := m.Configure("alice", Settings{ClientID: "desktop.apps.googleusercontent.com", ClientSecret: "secret", Services: []string{"gmail"}}); err != nil {
		t.Fatal(err)
	}
	link, err := m.Begin("alice")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(link)
	redirect := u.Query().Get("redirect_uri")
	m.mu.Lock()
	p := m.pending["alice"]
	m.mu.Unlock()
	t.Cleanup(p.cancel)
	resp, err := http.Get(redirect + "?state=wrong&code=test")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatal("invalid state accepted")
	}
	if err = m.Configure("alice", Settings{ClientID: "desktop.apps.googleusercontent.com", Services: []string{"drive"}}); err != nil {
		t.Fatal(err)
	}
	if err = m.finish(context.Background(), "alice", p, url.Values{"code": {"test"}}); err == nil {
		t.Fatal("old OAuth flow changed new settings")
	}
	if m.Allowed("alice", "gmail") {
		t.Fatal("old flow connected Gmail")
	}
}
func TestPartialConsentAndCorruptStoreFailClosed(t *testing.T) {
	m, _ := mockGoogle(t)
	connect(t, m, "alice", []string{"gmail", "drive"})
	m.mu.Lock()
	r := m.records["alice"]
	r.Scopes = serviceScopes["gmail"][0]
	m.records["alice"] = r
	m.mu.Unlock()
	if m.Allowed("alice", "drive") || !m.Allowed("alice", "gmail") {
		t.Fatal("partial grant not enforced")
	}
	if err := m.Disconnect("alice", true); err != nil {
		t.Fatal(err)
	}
	reloaded := New(m.dir)
	if reloaded.Allowed("alice", "gmail") {
		t.Fatal("deleted account reappeared")
	}
	if err := os.WriteFile(filepath.Join(m.dir, "accounts.enc"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	broken := New(m.dir)
	if _, err := broken.Status("alice"); err == nil {
		t.Fatal("corruption silently reset credentials")
	}
	if broken.Allowed("alice", "gmail") {
		t.Fatal("corrupt store allowed access")
	}
}

func TestDisableDuringSlowSearchStopsFollowupRequests(t *testing.T) {
	m, _ := mockGoogle(t)
	connect(t, m, "alice", []string{"gmail"})
	entered, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gmail/v1/users/me/messages" {
			t.Error("disabled search continued to read message")
		}
		close(entered)
		<-release
		io.WriteString(w, `{"messages":[{"id":"m1"}]}`)
	}))
	defer server.Close()
	m.apiBase = server.URL
	finished := make(chan error, 1)
	go func() {
		_, err := m.Execute(context.Background(), "alice", "gmail_search", json.RawMessage(`{"query":"meeting"}`))
		finished <- err
	}()
	<-entered
	configured := make(chan error, 1)
	go func() {
		configured <- m.Configure("alice", Settings{ClientID: "desktop.apps.googleusercontent.com", Services: []string{}})
	}()
	select {
	case err := <-configured:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("slow network request blocked permission changes")
	}
	close(release)
	if err := <-finished; err == nil {
		t.Fatal("search used disabled grant for its follow-up request")
	}
}

func TestGmailSearchDefaultsToRecentInboxAndPreservesFilters(t *testing.T) {
	m, _ := mockGoogle(t)
	connect(t, m, "alice", []string{"gmail"})
	for _, tc := range []struct{ args, want string }{
		{`{}`, "in:inbox"},
		{`{"query":"  ","limit":5}`, "in:inbox"},
		{`{"query":"is:unread from:someone@example.com","page_token":"page2"}`, "is:unread from:someone@example.com"},
	} {
		t.Run(tc.want+tc.args, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("q"); got != tc.want {
					t.Errorf("query = %q, want %q", got, tc.want)
				}
				if strings.Contains(tc.args, "page2") && r.URL.Query().Get("pageToken") != "page2" {
					t.Error("pagination lost")
				}
				io.WriteString(w, `{"messages":[]}`)
			}))
			defer server.Close()
			m.apiBase = server.URL
			out, err := m.Execute(context.Background(), "alice", "gmail_search", json.RawMessage(tc.args))
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Query string `json:"query"`
			}
			if err = json.Unmarshal([]byte(out), &result); err != nil || result.Query != tc.want {
				t.Fatalf("applied filter missing: %s (%v)", out, err)
			}
		})
	}
}

func TestDriveAndCalendarDefaultSearches(t *testing.T) {
	m, _ := mockGoogle(t)
	connect(t, m, "alice", []string{"drive", "calendar"})
	for _, tc := range []struct{ name, args string }{
		{"google_drive_search", `{}`},
		{"google_drive_search", `{"query":"a'b","page_token":"next"}`},
		{"google_calendar_search", `{}`},
		{"google_calendar_search", `{"period":"today"}`},
		{"google_calendar_search", `{"period":"tomorrow","time_zone":"America/New_York"}`},
	} {
		t.Run(tc.name+tc.args, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				switch r.URL.Path {
				case "/drive/v3/files":
					if q.Get("orderBy") != "modifiedTime desc" || q.Get("pageSize") != "10" {
						t.Error("recent file order or limit missing")
					}
					if tc.args == `{}` && q.Get("q") != "trashed = false and mimeType != 'application/vnd.google-apps.folder'" {
						t.Error("default list still requires keyword")
					}
					if tc.args != `{}` && (!strings.Contains(q.Get("q"), `a\'b`) || q.Get("pageToken") != "next") {
						t.Error("explicit filter or pagination lost")
					}
					io.WriteString(w, `{"files":[],"nextPageToken":"page2"}`)
				case "/calendar/v3/users/me/calendarList":
					io.WriteString(w, `{"items":[{"id":"primary","primary":true,"timeZone":"Asia/Seoul"}]}`)
				case "/calendar/v3/users/me/calendarList/primary":
					io.WriteString(w, `{"timeZone":"Asia/Seoul"}`)
				case "/calendar/v3/calendars/primary/events":
					start, e1 := time.Parse(time.RFC3339, q.Get("timeMin"))
					end, e2 := time.Parse(time.RFC3339, q.Get("timeMax"))
					if e1 != nil || e2 != nil || !end.After(start) {
						t.Error("missing resolved time range")
					}
					if q.Get("orderBy") != "startTime" || q.Get("singleEvents") != "true" {
						t.Error("event ordering/recurrence changed")
					}
					if strings.Contains(tc.args, "today") && (start.Hour() != 0 || end.Hour() != 0) {
						t.Error("today is not a full local day")
					}
					io.WriteString(w, `{"items":[],"nextPageToken":"page2"}`)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			m.apiBase = server.URL
			out, err := m.Execute(context.Background(), "alice", tc.name, json.RawMessage(tc.args))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "page2") {
				t.Fatal("pagination lost")
			}
			if tc.name == "google_calendar_search" && !strings.Contains(out, "effective_range") {
				t.Fatal("resolved period missing from result")
			}
		})
	}
}
