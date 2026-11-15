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
		t.Run(task, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" {
					t.Errorf("must be stateless %s", r.URL.Path)
				}
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
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
			req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"glasses_client":true,"glasses_task":"`+task+`","model":"translate","stream":true,"max_tokens":99999,"messages":[{"role":"user","content":"Translate hello"}],"tools":[{"type":"function","function":{"name":"execute_command"}}],"previous_response_id":"resp_old"}`))
			w := httptest.NewRecorder()
			handleChat(w, req, &App{llmEndpoint: upstream.URL, llmMode: "stateful", enableTools: true}, &AuthManager{users: map[string]*User{}})
			if w.Code != 200 || !strings.Contains(w.Body.String(), "translated") {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
}
