package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEvenHubCORS(t *testing.T) {
	for _, tc := range []struct {
		name, origin, path, method, requestedMethod, headers string
		status                                               int
		allowed, forwarded                                   bool
	}{
		{"login preflight", "http://127.0.0.1:50496", "/api/login", "OPTIONS", "POST", "content-type", 204, true, false},
		{"chrome translate preflight", "http://localhost:7890", "/api/translate/chrome", "OPTIONS", "POST", "authorization, content-type", 204, true, false},
		{"chrome prepare preflight", "http://localhost:7890", "/api/translate/chrome/prepare", "OPTIONS", "POST", "authorization, content-type", 204, true, false},
		{"hint preflight", "http://localhost:7890", "/api/glasses/hint-decision", "OPTIONS", "POST", "authorization, content-type", 204, true, false},
		{"bearer preflight", "http://localhost:7890", "/api/chat", "OPTIONS", "POST", "authorization, content-type", 204, true, false},
		{"screenshot preflight", "http://localhost:7890", "/api/computer/screenshots/0123456789ab-00112233445566778899aabb.png", "OPTIONS", "GET", "authorization", 204, true, false},
		{"screenshot read", "http://localhost:7890", "/api/computer/screenshots/0123456789ab-00112233445566778899aabb.png", "GET", "", "", 200, true, true},
		{"screenshot foreign origin", "https://example.com", "/api/computer/screenshots/0123456789ab-00112233445566778899aabb.png", "OPTIONS", "GET", "authorization", 405, false, true},
		{"config read", "http://localhost:7890", "/api/config", "GET", "", "", 200, true, true},
		{"config preflight", "http://localhost:7890", "/api/config", "OPTIONS", "GET", "authorization", 204, true, false},
		{"config write excluded", "http://localhost:7890", "/api/config", "POST", "", "", 200, false, true},
		{"config write preflight excluded", "http://localhost:7890", "/api/config", "OPTIONS", "POST", "content-type", 405, false, true},
		{"IPv6 origin", "http://[::1]:7890", "/api/health", "GET", "", "", 200, true, true},
		{"auth failure readable", "http://127.0.0.1:7890", "/api/models", "GET", "", "", 401, true, true},
		{"remote origin", "https://example.com", "/api/login", "OPTIONS", "POST", "content-type", 405, false, true},
		{"deceptive hostname", "http://localhost.example.com", "/api/health", "GET", "", "", 200, false, true},
		{"opaque origin", "null", "/api/health", "GET", "", "", 200, false, true},
		{"admin API excluded", "http://127.0.0.1:7890", "/api/users", "OPTIONS", "POST", "content-type", 405, false, true},
		{"extra headers rejected", "http://127.0.0.1:7890", "/api/login", "OPTIONS", "POST", "x-admin", 403, true, false},
		{"delete rejected", "http://127.0.0.1:7890", "/api/login", "OPTIONS", "DELETE", "", 403, true, false},
		{"same origin untouched", "", "/api/health", "GET", "", "", 200, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := evenHubCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if tc.allowed && r.Header.Get("Cookie") != "" {
					t.Error("cross-origin cookie forwarded")
				}
				if r.Header.Get("Authorization") != "Bearer test" {
					t.Error("bearer token lost")
				}
				if r.Method == "OPTIONS" {
					w.WriteHeader(405)
				} else if r.URL.Path == "/api/models" {
					w.WriteHeader(401)
				}
			}))
			r := httptest.NewRequest(tc.method, tc.path, nil)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Cookie", "session=test")
			r.Header.Set("Authorization", "Bearer test")
			r.Header.Set("Access-Control-Request-Method", tc.requestedMethod)
			r.Header.Set("Access-Control-Request-Headers", tc.headers)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Errorf("status = %d, want %d", w.Code, tc.status)
			}
			if allowed := w.Header().Get("Access-Control-Allow-Origin") == tc.origin && tc.origin != ""; allowed != tc.allowed {
				t.Errorf("CORS allowed = %v", allowed)
			}
			if called != tc.forwarded {
				t.Errorf("forwarded = %v", called)
			}
			if w.Header().Get("Access-Control-Allow-Credentials") != "" {
				t.Error("credentialed CORS must not be enabled")
			}
		})
	}
}
