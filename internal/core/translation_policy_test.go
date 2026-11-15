package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGlassesTranslationPolicy(t *testing.T) {
	for _, task := range []string{"translation_preview", "translation", "translation_replies"} {
		for _, nativeInput := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/native_input=%t", task, nativeInput), func(t *testing.T) {
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/v1/chat/completions" {
						t.Errorf("must be stateless %s", r.URL.Path)
					}
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					messages, ok := body["messages"].([]any)
					if !ok || len(messages) == 0 {
						http.Error(w, "'messages' field is required", 400)
						return
					}
					if messages[len(messages)-1].(map[string]any)["content"] != "Translate hello" {
						t.Errorf("lost user prompt: %v", messages)
					}
					if nativeInput && messages[0].(map[string]any)["content"] != "Recommend bilingual replies" {
						t.Errorf("lost system prompt: %v", messages)
					}
					for _, key := range []string{"input", "system_prompt"} {
						if _, exists := body[key]; exists {
							t.Errorf("native-only field leaked: %s", key)
						}
					}
					raw, _ := json.Marshal(body)
					for _, key := range []string{"glasses_task", "tools", "previous_response_id", "OLD_MEMORY", "TOOL CALL GUIDELINES"} {
						if strings.Contains(string(raw), key) {
							t.Errorf("translation leaked %s", key)
						}
					}
					if body["store"] != false || body["chat_template_kwargs"].(map[string]any)["enable_thinking"] != false {
						t.Error("translation controls missing")
					}
					expected := float64(1024)
					if task == "translation_replies" {
						expected = 384
					}
					if body["max_tokens"] != expected {
						t.Errorf("budget %v", body["max_tokens"])
					}
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"translated\"}}]}\n\ndata: [DONE]\n\n")
				}))
				defer upstream.Close()
				payload := map[string]any{"glasses_client": true, "glasses_task": task, "model": "translate", "stream": true, "max_tokens": 99999, "previous_response_id": "resp_old", "tools": []any{map[string]any{"type": "function", "function": map[string]any{"name": "execute_command"}}}}
				if nativeInput {
					payload["input"] = "Translate hello"
					payload["system_prompt"] = "Recommend bilingual replies"
				} else {
					payload["messages"] = []any{map[string]any{"role": "user", "content": "Translate hello"}}
				}
				encoded, _ := json.Marshal(payload)
				req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(encoded)))
				w := httptest.NewRecorder()
				handleChat(w, req, &App{llmEndpoint: upstream.URL, llmMode: "stateful", enableTools: true}, &AuthManager{users: map[string]*User{}})
				if w.Code != 200 || !strings.Contains(w.Body.String(), "translated") {
					t.Fatalf("%d %s", w.Code, w.Body.String())
				}
			})
		}
	}
}
