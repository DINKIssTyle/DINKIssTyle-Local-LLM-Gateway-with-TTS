package core

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerDoesNotExposeLegacyMCPTransport(t *testing.T) {
	auth := NewAuthManager(filepath.Join(t.TempDir(), "users.json"))
	mux := createServerMux(&App{authMgr: auth}, auth)

	for _, path := range []string{"/mcp/sse", "/mcp/messages"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("legacy transport %s returned %d, want 404", path, recorder.Code)
		}
	}
}

func TestServerLivenessDoesNotRequireLLM(t *testing.T) {
	for _, method := range []string{"GET", "HEAD", "POST"} {
		recorder := httptest.NewRecorder()
		handleServerLiveness(recorder, httptest.NewRequest(method, "/api/health/live", nil))
		want := http.StatusNoContent
		if method == "POST" {
			want = http.StatusMethodNotAllowed
		}
		if recorder.Code != want || recorder.Body.Len() != 0 {
			t.Fatalf("%s: status=%d body=%q", method, recorder.Code, recorder.Body.String())
		}
		if method != "POST" && recorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("liveness response must not be cached")
		}
	}
}

func TestReconnectPollingNeverQueriesModelCatalog(t *testing.T) {
	calls := 0
	previousTransport := http.DefaultTransport
	http.DefaultTransport = healthTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"test-model"}]}`)), Header: make(http.Header)}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = previousTransport })
	auth := NewAuthManager(filepath.Join(t.TempDir(), "users.json"))
	mux := createServerMux(&App{authMgr: auth, llmEndpoint: "http://model.invalid"}, auth)
	for i := 0; i < 20; i++ {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/health/live", nil))
		if recorder.Code != http.StatusNoContent {
			t.Fatal(recorder.Code)
		}
	}
	if calls != 0 {
		t.Fatalf("reconnect polling made %d model requests", calls)
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/health", nil))
	if calls != 1 {
		t.Fatalf("explicit health check made %d model requests, want 1", calls)
	}
}

type healthTestTransport func(*http.Request) (*http.Response, error)

func (f healthTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
