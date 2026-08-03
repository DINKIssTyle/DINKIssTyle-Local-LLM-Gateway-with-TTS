package core

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
