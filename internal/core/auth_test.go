package core

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExpandDisabledToolAliases(t *testing.T) {
	got := expandDisabledToolAliases([]string{"personal_memory", "search_web", "search_memory"})
	want := []string{"search_memory", "read_memory", "read_memory_context", "delete_memory", "search_web"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandDisabledToolAliases() = %#v, want %#v", got, want)
	}
}

func TestCollapseDisabledToolsForUI(t *testing.T) {
	got := collapseDisabledToolsForUI([]string{"search_memory", "read_memory_context", "search_web"})
	want := []string{"personal_memory", "search_web"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collapseDisabledToolsForUI() = %#v, want %#v", got, want)
	}
}

func TestMCPSessionBindingCleanupHelpers(t *testing.T) {
	am := NewAuthManager(filepath.Join(t.TempDir(), "users.json"))
	am.mcpSessionBindings["a"] = mcpSessionBinding{TokenHash: "hash-a", UserID: "alice"}
	am.mcpSessionBindings["b"] = mcpSessionBinding{TokenHash: "hash-b", UserID: "bob"}
	am.mcpSessionBindings["c"] = mcpSessionBinding{TokenHash: "hash-a", UserID: "alice"}

	am.removeMCPSessionBindingsByTokenHash("hash-a")
	if len(am.mcpSessionBindings) != 1 {
		t.Fatalf("expected one binding after token cleanup, got %d", len(am.mcpSessionBindings))
	}
	if _, ok := am.mcpSessionBindings["b"]; !ok {
		t.Fatalf("expected non-matching binding to remain")
	}

	am.removeMCPSessionBindingsByUser("bob")
	if len(am.mcpSessionBindings) != 0 {
		t.Fatalf("expected all bindings removed after user cleanup, got %d", len(am.mcpSessionBindings))
	}
}

func TestBuildMCPSessionURL(t *testing.T) {
	app := &App{port: "2806"}
	req := httptest.NewRequest("GET", "https://localhost:2806/api/mcp/session-url", nil)
	req.Host = "127.0.0.1:2806"

	got := buildMCPSessionURL(req, app, "binding123")
	want := "http://127.0.0.1:2807/mcp/sse?mcp_session=binding123"
	if got != want {
		t.Fatalf("buildMCPSessionURL() = %q, want %q", got, want)
	}
}

func TestFindUserByMCPAPIKey(t *testing.T) {
	am := NewAuthManager(filepath.Join(t.TempDir(), "users.json"))
	if err := am.AddUser("alice", "pw", "user"); err != nil {
		t.Fatalf("AddUser() error = %v", err)
	}
	if err := am.SetUserMCPAPIKey("alice", "mcp-secret"); err != nil {
		t.Fatalf("SetUserMCPAPIKey() error = %v", err)
	}

	user, ok := am.FindUserByMCPAPIKey("mcp-secret")
	if !ok || user == nil || user.ID != "alice" {
		t.Fatalf("FindUserByMCPAPIKey() = %#v, %v; want alice, true", user, ok)
	}

	if user, ok := am.FindUserByMCPAPIKey("missing"); ok || user != nil {
		t.Fatalf("FindUserByMCPAPIKey() for missing key = %#v, %v; want nil, false", user, ok)
	}
}

func TestExtractMCPAPIKeyFromRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/mcp/sse", nil)
	req.Header.Set("X-API-Key", "header-key")
	if got := extractMCPAPIKeyFromRequest(req); got != "header-key" {
		t.Fatalf("extractMCPAPIKeyFromRequest(X-API-Key) = %q, want %q", got, "header-key")
	}

	req = httptest.NewRequest(http.MethodGet, "/mcp/sse", nil)
	req.Header.Set("Authorization", "Bearer bearer-key")
	if got := extractMCPAPIKeyFromRequest(req); got != "bearer-key" {
		t.Fatalf("extractMCPAPIKeyFromRequest(Authorization) = %q, want %q", got, "bearer-key")
	}
}
