// Created by DINKIssTyle on 2026. Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestContextIsolation(t *testing.T) {
	// 1. Setup resolver that returns different UserIDs based on token
	SetRequestContextResolver(func(r *http.Request) ToolContext {
		token := r.URL.Query().Get("token")
		return ToolContext{UserID: "user_" + token}
	})
	defer SetRequestContextResolver(nil)

	// 2. Validate ResolveRequestContext
	req1 := httptest.NewRequest("GET", "/mcp?token=abc", nil)
	ctx1 := ResolveRequestContext(req1)
	if ctx1.UserID != "user_abc" {
		t.Errorf("Expected UserID user_abc, got %s", ctx1.UserID)
	}

	req2 := httptest.NewRequest("GET", "/mcp?token=def", nil)
	ctx2 := ResolveRequestContext(req2)
	if ctx2.UserID != "user_def" {
		t.Errorf("Expected UserID user_def, got %s", ctx2.UserID)
	}
}

func TestExecuteToolByNameWithExplicitContext(t *testing.T) {
	// Test if ExecuteToolByName respects the passed context
	ctx := ToolContext{
		UserID:        "test_user",
		EnableMemory:  true,
		DisabledTools: []string{"search_web"},
	}

	// This tool is disabled in ctx, so it should fail
	_, err := ExecuteToolByName(ctx, "search_web", []byte(`{"query": "test"}`))
	if err == nil {
		t.Fatal("Expected error for disabled tool, got nil")
	}
	expectedErr := "tool 'search_web' is disabled for this user"
	if err.Error() != expectedErr {
		t.Errorf("Expected error %q, got %q", expectedErr, err.Error())
	}

	// This tool is not disabled, should proceed
	ctx.DisabledTools = []string{}
	// get_current_time is a safe tool that doesn't require hooks
	_, err = ExecuteToolByName(ctx, "get_current_time", []byte(`{}`))
	if err != nil {
		t.Errorf("Expected no error for get_current_time, got %v", err)
	}
}
