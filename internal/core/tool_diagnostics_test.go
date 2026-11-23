package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dinkisstyle-chat/internal/toolruntime"
)

func TestBulkDiagnosticScope(t *testing.T) {
	for _, tc := range []struct {
		text, prior string
		want        bool
	}{
		{"모든 도구를", "도구들을 하나씩 테스트 해줘", true},
		{"모든 도구 테스트", "", true},
		{"모든 도구 테스트 방법", "", false},
		{"모든 도구를", "일본 뉴스 검색", false},
		{"핑 테스트", "도구들을 하나씩 테스트 해줘", false},
	} {
		if got := isBulkDiagnostic(tc.text, tc.prior); got != tc.want {
			t.Errorf("isBulkDiagnostic(%q, %q) = %v", tc.text, tc.prior, got)
		}
	}
}

func TestPingDiagnosticExecutesBoundedCommand(t *testing.T) {
	for input, want := range map[string]string{
		"핑 테스트":                  "google.com",
		"구글에 핑 테스트해봐":            "google.com",
		"ping 127.0.0.1":         "127.0.0.1",
		"핑 테스트 방법":               "",
		"ping google.com;whoami": "",
		"google.com 핑 테스트를 설명해줘": "",
	} {
		if target := diagnosticPingTarget(input); target != want {
			t.Errorf("diagnosticPingTarget(%q) = %q, want %q", input, target, want)
		}
	}
	registry := toolruntime.NewRegistry()
	registerDiagnosticTestTool(t, registry, "execute_command", toolruntime.Metadata{SideEffecting: true}, func(_ context.Context, _ toolruntime.ExecutionContext, args json.RawMessage) (toolruntime.Result, error) {
		if !strings.Contains(string(args), "ping ") || !strings.Contains(string(args), " google.com") {
			t.Errorf("wrong arguments %s", args)
		}
		return toolruntime.Result{Content: "3 packets transmitted, 3 received"}, nil
	})
	var events []string
	emit := func(kind, _, _ string, _ map[string]interface{}) { events = append(events, kind) }
	answer := runPingDiagnostic(context.Background(), registry, toolruntime.ExecutionContext{}, emit, "google.com")
	if !strings.Contains(answer, "3 packets transmitted, 3 received") || strings.Join(events, ",") != "tool_call.start,tool_call.arguments,tool_call.success" {
		t.Fatalf("result or events missing: %q %v", answer, events)
	}
}

func TestBulkDiagnosticUsesFixturesAndReportsFailures(t *testing.T) {
	registry := toolruntime.NewRegistry()
	calls := 0
	for _, name := range []string{"get_current_time", "search_web", "computer_type", "get_current_location"} {
		registerDiagnosticTestTool(t, registry, name, toolruntime.Metadata{SideEffecting: name == "computer_type"}, func(_ context.Context, _ toolruntime.ExecutionContext, args json.RawMessage) (toolruntime.Result, error) {
			calls++
			switch name {
			case "computer_type", "get_current_location":
				t.Errorf("unexpected call to %s", name)
			case "search_web":
				if !strings.Contains(string(args), "IANA example domains") {
					t.Errorf("unexpected diagnostic search %s", args)
				}
				return toolruntime.Result{}, errors.New("provider unavailable")
			}
			return toolruntime.Result{Content: "test response"}, nil
		})
	}
	emit := func(string, string, string, map[string]interface{}) {}
	answer := runBulkToolDiagnostic(context.Background(), registry, toolruntime.ExecutionContext{DisabledTools: []string{"get_current_location"}}, emit)
	if calls != 2 || !strings.Contains(answer, "get_current_time` · 응답 확인") ||
		!strings.Contains(answer, "search_web` · 실패 · provider unavailable") ||
		!strings.Contains(answer, "computer_type` · 건너뜀") || strings.Contains(answer, "get_current_location") {
		t.Fatalf("unexpected diagnostic result (%d calls): %s", calls, answer)
	}
}

func registerDiagnosticTestTool(t *testing.T, registry *toolruntime.Registry, name string, metadata toolruntime.Metadata, handler toolruntime.Handler) {
	t.Helper()
	if err := registry.Register(toolruntime.Definition{
		Name: name, InputSchema: json.RawMessage(`{"type":"object"}`), Metadata: metadata,
	}, handler); err != nil {
		t.Fatal(err)
	}
}

func TestChatDiagnosticRequestsDoNotFallThroughToModel(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("diagnostic was forwarded to the model")
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer upstream.Close()
	app := &App{llmEndpoint: upstream.URL, llmMode: "standard"}
	for _, body := range []string{
		`{"model":"gemma-4-e2b-it-mlx","stream":true,"messages":[{"role":"user","content":"핑 테스트"}]}`,
		`{"model":"gemma-4-e2b-it-mlx","stream":true,"messages":[{"role":"user","content":"도구들을 하나씩 테스트 해줘"},{"role":"assistant","content":"어떤 도구를 테스트할까요?"},{"role":"user","content":"모든 도구를"}]}`,
	} {
		w := httptest.NewRecorder()
		handleChat(w, httptest.NewRequest("POST", "/api/chat", strings.NewReader(body)), app, &AuthManager{users: map[string]*User{}})
		if !strings.Contains(w.Body.String(), "도구 사용이 꺼져 있어 진단을 실행하지 못했습니다") {
			t.Fatalf("missing disabled diagnostic response: %s", w.Body.String())
		}
	}
}
