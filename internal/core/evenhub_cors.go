package core

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Packaged Even Hub apps are served from a phone-local WebView origin whose
// port changes between launches. Only expose the plugin's APIs to loopback
// origins, and use explicit bearer tokens rather than ambient session cookies.
func evenHubCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !evenHubAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		if origin == "" || !evenHubLoopbackOrigin(origin) {
			next.ServeHTTP(w, r)
			return
		}
		// Only opt the read into Even Hub CORS; preserve normal config writes.
		if r.URL.Path == "/api/config" && r.Method != http.MethodGet &&
			!(r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") == http.MethodGet) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		if r.Method == http.MethodOptions {
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			method := r.Header.Get("Access-Control-Request-Method")
			if method != http.MethodGet && method != http.MethodPost {
				http.Error(w, "CORS method not allowed", http.StatusForbidden)
				return
			}
			for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
				switch strings.ToLower(strings.TrimSpace(header)) {
				case "", "authorization", "content-type":
				default:
					http.Error(w, "CORS header not allowed", http.StatusForbidden)
					return
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		request := r.Clone(r.Context())
		request.Header.Del("Cookie")
		next.ServeHTTP(w, request)
	})
}

func evenHubAPIPath(path string) bool {
	switch path {
	case "/api/apple/status", "/api/stt/apple/prepare", "/api/stt/apple/stop", "/api/translate/apple", "/api/translate/apple/prepare", "/api/stt/chrome/stop", "/api/stt/chrome/prepare", "/api/translate/chrome", "/api/translate/chrome/prepare", "/api/glasses/hint-decision", "/api/config", "/api/login", "/api/auth/check", "/api/health", "/api/models",
		"/api/stt/transcribe", "/api/stt/preview", "/api/stt/warmup", "/api/stt/status", "/api/stt/transcripts", "/api/chat", "/api/tts":
		return true
	default:
		return false
	}
}

func evenHubLoopbackOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" ||
		(u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
