package toolruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"dinkisstyle-chat/internal/mcp"
)

func TestDefaultRegistryListsAndCallsTool(t *testing.T) {
	definitions := Default.List(ExecutionContext{})
	found := false
	for _, definition := range definitions {
		if definition.Name == "get_current_time" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("get_current_time is missing from the app tool registry")
	}

	result, err := Default.Call(context.Background(), ExecutionContext{}, "get_current_time", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("get_current_time failed: %v", err)
	}
	if strings.TrimSpace(result.Content) == "" {
		t.Fatal("get_current_time returned empty content")
	}
}

func TestDefaultRegistryDoesNotExposeUnavailableTerminalHostTools(t *testing.T) {
	foundExecuteCommand := false
	for _, definition := range Default.List(ExecutionContext{EnableMemory: true}) {
		if definition.Name == "send_keys" || definition.Name == "read_terminal_tail" {
			t.Fatalf("gateway exposed unavailable terminal-host tool %q", definition.Name)
		}
		if definition.Name == "execute_command" {
			foundExecuteCommand = true
		}
	}
	if !foundExecuteCommand {
		t.Fatal("gateway did not expose the host shell execute_command tool")
	}
}

func TestRegistryAppliesRequestScopedPolicy(t *testing.T) {
	execCtx := ExecutionContext{DisabledTools: []string{"get_current_time"}}
	for _, definition := range Default.List(execCtx) {
		if definition.Name == "get_current_time" {
			t.Fatal("disabled tool was exposed")
		}
	}
	if _, err := Default.Call(context.Background(), execCtx, "get_current_time", json.RawMessage(`{}`)); err == nil {
		t.Fatal("disabled tool call unexpectedly succeeded")
	}
}

func TestRegistryHidesMemoryToolsWhenMemoryDisabled(t *testing.T) {
	for _, definition := range Default.List(ExecutionContext{EnableMemory: false}) {
		if definition.Name == "search_memory" {
			t.Fatal("memory tool exposed while memory is disabled")
		}
	}
}

func TestRegistryValidatesRequiredArguments(t *testing.T) {
	if _, err := Default.Call(context.Background(), ExecutionContext{}, "search_web", json.RawMessage(`{}`)); err == nil {
		t.Fatal("missing required argument unexpectedly accepted")
	}
}

func TestRegistryValidatesArgumentTypesAndArrayBounds(t *testing.T) {
	for _, arguments := range []string{
		`{"queries":"one | two"}`,
		`{"queries":["one"]}`,
		`{"queries":["one","two","three"]}`,
		`{"queries":["one",2]}`,
	} {
		if _, err := Default.Call(context.Background(), ExecutionContext{}, "search_web_multi", json.RawMessage(arguments)); err == nil {
			t.Fatalf("invalid search_web_multi arguments were accepted: %s", arguments)
		}
	}
}

func TestRegistryNormalizesCommonLocalModelArgumentAliases(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
		wantName  string
		wantJSON  string
	}{
		{name: "read_help", arguments: `{"topic":"인증서"}`, wantName: "read_help", wantJSON: `"query":"인증서"`},
		{name: "read_web_page", arguments: `{"link":"https://example.com"}`, wantName: "read_web_page", wantJSON: `"url":"https://example.com"`},
		{name: "namu_wiki", arguments: `{"query":"대한민국"}`, wantName: "namu_wiki", wantJSON: `"keyword":"대한민국"`},
		{name: "read_memory", arguments: `{"memory_id":"216"}`, wantName: "read_memory", wantJSON: `"memory_id":216`},
		{name: "execute_command", arguments: `{"cmd":"pwd"}`, wantName: "execute_command", wantJSON: `"command":"pwd"`},
	}
	for _, test := range tests {
		gotName, gotArguments := mcp.NormalizeToolCall(test.name, []byte(test.arguments))
		if gotName != test.wantName || !strings.Contains(string(gotArguments), test.wantJSON) {
			t.Fatalf("NormalizeToolCall(%s) = %s %s", test.name, gotName, gotArguments)
		}
	}
}

func TestRegistryKeepsConcurrentExecutionContextsIsolated(t *testing.T) {
	registry := NewRegistry()
	err := registry.Register(Definition{
		Name:        "who_am_i",
		Description: "Return the request user",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(_ context.Context, execCtx ExecutionContext, _ json.RawMessage) (Result, error) {
		return Result{Content: execCtx.UserID}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		userID := fmt.Sprintf("user-%d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, callErr := registry.Call(context.Background(), ExecutionContext{UserID: userID}, "who_am_i", json.RawMessage(`{}`))
			if callErr != nil {
				errs <- callErr
				return
			}
			if result.Content != userID {
				errs <- fmt.Errorf("context leak: got %q, want %q", result.Content, userID)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestRegistryContainsHandlerPanic(t *testing.T) {
	registry := NewRegistry()
	err := registry.Register(Definition{
		Name:        "panic_tool",
		Description: "panic",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(context.Context, ExecutionContext, json.RawMessage) (Result, error) {
		panic("boom")
	})
	if err != nil {
		t.Fatal(err)
	}

	result, callErr := registry.Call(context.Background(), ExecutionContext{}, "panic_tool", json.RawMessage(`{}`))
	if callErr == nil || !strings.Contains(callErr.Error(), "panicked") || !result.IsError {
		t.Fatalf("panic was not contained: result=%#v err=%v", result, callErr)
	}
}

func TestExecuteCommandReportsShellFailure(t *testing.T) {
	result, err := Default.Call(
		context.Background(),
		ExecutionContext{},
		"execute_command",
		json.RawMessage(`{"command":"__dinkisstyle_command_that_does_not_exist__"}`),
	)
	if err == nil {
		t.Fatalf("failed shell command was reported as success: %#v", result)
	}
	if !result.IsError || !strings.Contains(result.Content, "Command failed") {
		t.Fatalf("shell failure details were not preserved: result=%#v err=%v", result, err)
	}
}
