package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveRequestContextUsesResolverPerRequest(t *testing.T) {
	originalResolver := requestContextResolver
	t.Cleanup(func() {
		SetRequestContextResolver(originalResolver)
	})

	SetRequestContextResolver(func(r *http.Request) ToolContext {
		return ToolContext{
			UserID:        r.Header.Get("X-Test-User"),
			EnableMemory:  true,
			DisabledTools: []string{"search_memory"},
		}
	})

	reqA := httptest.NewRequest("GET", "/mcp/sse", nil)
	reqA.Header.Set("X-Test-User", "alice")
	ctxA := ResolveRequestContext(reqA)

	reqB := httptest.NewRequest("GET", "/mcp/sse", nil)
	reqB.Header.Set("X-Test-User", "bob")
	ctxB := ResolveRequestContext(reqB)

	if ctxA.UserID != "alice" {
		t.Fatalf("expected alice context, got %q", ctxA.UserID)
	}
	if ctxB.UserID != "bob" {
		t.Fatalf("expected bob context, got %q", ctxB.UserID)
	}

	ctxA.DisabledTools[0] = "mutated"
	ctxA2 := ResolveRequestContext(reqA)
	if ctxA2.DisabledTools[0] != "search_memory" {
		t.Fatalf("expected cloned context slices, got %q", ctxA2.DisabledTools[0])
	}
}

func TestExecuteToolByNameUsesExplicitContext(t *testing.T) {
	resultA, err := ExecuteToolByName("get_current_location", []byte(`{}`), ToolContext{
		UserID:       "alice",
		LocationInfo: "Seoul, KR",
	})
	if err != nil {
		t.Fatalf("unexpected error for alice: %v", err)
	}
	if resultA != "Seoul, KR" {
		t.Fatalf("expected alice location, got %q", resultA)
	}

	resultB, err := ExecuteToolByName("get_current_location", []byte(`{}`), ToolContext{
		UserID:       "bob",
		LocationInfo: "Busan, KR",
	})
	if err != nil {
		t.Fatalf("unexpected error for bob: %v", err)
	}
	if resultB != "Busan, KR" {
		t.Fatalf("expected bob location, got %q", resultB)
	}
}
