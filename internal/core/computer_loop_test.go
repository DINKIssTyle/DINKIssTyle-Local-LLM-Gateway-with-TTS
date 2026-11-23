package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dinkisstyle-chat/internal/toolruntime"
)

// Exercise the real gateway loop with a fake desktop. No native capture,
// application activation, or keyboard/mouse input occurs in this test.
func TestComputerLoopCanObserveRepeatedlyAndSwitchBack(t *testing.T) {
	for _, bounded := range []bool{false, true} {
		t.Run(fmt.Sprintf("budget_%t", bounded), func(t *testing.T) {
			registry := toolruntime.NewRegistry()
			observations, switches := 0, 0
			for _, name := range []string{"computer_observe", "computer_switch_app"} {
				name := name
				err := registry.Register(toolruntime.Definition{Name: name, Description: "Fake desktop tool", InputSchema: json.RawMessage(`{"type":"object","properties":{"pid":{"type":"integer"}}}`)}, func(_ context.Context, _ toolruntime.ExecutionContext, args json.RawMessage) (toolruntime.Result, error) {
					if name == "computer_observe" {
						observations++
						return toolruntime.Result{Content: fmt.Sprintf(`{"observation_id":"capture%d"}`, observations), ImageURLs: []string{"data:image/png;base64,c2NyZWVu"}}, nil
					}
					switches++
					return toolruntime.Result{Content: `{"status":"activation_requested"}`}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			old := toolruntime.Default
			toolruntime.Default = registry
			defer func() { toolruntime.Default = old }()
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				raw, _ := json.Marshal(body)
				if calls > 0 && strings.Count(string(raw), "data:image/png;base64,") != 1 {
					t.Error("gateway lost or accumulated desktop screenshots")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				if (!bounded && calls == 11) || (bounded && body["tools"] == nil) {
					calls++
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"DESKTOP_DONE\"}}]}\n\ndata: [DONE]\n\n")
					return
				}
				name, args := "computer_observe", "{}"
				if !bounded && calls%2 == 1 {
					name = "computer_switch_app"
					args = fmt.Sprintf(`{"pid":%d}`, 42+(calls/2%2)*57)
				}
				event := map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprintf("call%d", calls), "type": "function", "function": map[string]any{"name": name, "arguments": args}}}}}}}
				encoded, _ := json.Marshal(event)
				calls++
				fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", encoded)
			}))
			defer upstream.Close()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"model":"vision","stream":true,"messages":[{"role":"user","content":"앱을 번갈아 전환하고 화면을 확인해줘"}]}`))
			handleChat(recorder, request, &App{llmEndpoint: upstream.URL, llmMode: "standard", enableTools: true}, &AuthManager{users: map[string]*User{}})
			if bounded {
				if calls != 32 || observations != 31 {
					t.Fatalf("desktop budget failed: responses=%d captures=%d", calls, observations)
				}
			} else if calls != 12 || observations != 6 || switches != 5 {
				t.Fatalf("desktop calls were deduplicated or stopped early: responses=%d captures=%d switches=%d", calls, observations, switches)
			}
			if !strings.Contains(recorder.Body.String(), "DESKTOP_DONE") {
				t.Fatal("final answer lost")
			}
		})
	}
}

func TestGemmaWeatherPlainToolCallExecutesInsteadOfEndingAsText(t *testing.T) {
	old := toolruntime.Default
	registry := toolruntime.NewRegistry()
	toolruntime.Default = registry
	t.Cleanup(func() { toolruntime.Default = old })
	lookups := 0
	if err := registry.Register(toolruntime.Definition{Name: "search_web", Description: "Look up weather", Metadata: toolruntime.Metadata{ReadOnly: true}, InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)}, func(_ context.Context, _ toolruntime.ExecutionContext, args json.RawMessage) (toolruntime.Result, error) {
		lookups++
		if !strings.Contains(string(args), "논산 내일 아침 날씨") {
			t.Errorf("lost follow-up location: %s", args)
		}
		return toolruntime.Result{Content: "Title: 논산 예보\nLink: https://example.com/weather\nSnippet: 내일 아침 6도"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/event-stream")
		text := "search_web\nquery: 논산 내일 아침 날씨"
		if requests > 1 {
			text = "논산 내일 아침 예보는 6도입니다."
		}
		for _, char := range text {
			event, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": string(char)}}}})
			fmt.Fprintf(w, "data: %s\n\n", event)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()
	body := `{"model":"gemma-4-e2b-it-mlx","stream":true,"messages":[{"role":"user","content":"내일 아침 날씨"},{"role":"assistant","content":"어느 지역인가요?"},{"role":"user","content":"논산"}]}`
	recorder := httptest.NewRecorder()
	handleChat(recorder, httptest.NewRequest("POST", "/api/chat", strings.NewReader(body)), &App{llmEndpoint: upstream.URL, llmMode: "standard", enableTools: true, computerTaskPlanning: true}, &AuthManager{users: map[string]*User{}})
	if lookups != 1 || requests != 2 || !strings.Contains(recorder.Body.String(), "6") {
		t.Fatalf("weather lookup did not execute: lookups=%d requests=%d output=%s", lookups, requests, recorder.Body.String())
	}
}
