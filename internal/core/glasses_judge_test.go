package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGlassesJudgeHasNoRuntimeOrTools(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("wrong judge API: %s", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "small" || body["max_tokens"] != float64(192) || body["store"] != false {
			t.Errorf("judge controls: %+v", body)
		}
		raw, _ := json.Marshal(body)
		for _, bad := range []string{"tools", "previous_response_id", "hint_research", "OLD_MEMORY", "GLASSES REQUEST CONTEXT", "TOOL CALL GUIDELINES"} {
			if strings.Contains(string(raw), bad) {
				t.Errorf("judge leaked %s", bad)
			}
		}
		if !strings.Contains(string(raw), "CURRENT_SHORT_TASK") {
			t.Error("lost classifier prompt")
		}
		if body["chat_template_kwargs"].(map[string]any)["enable_thinking"] != false {
			t.Error("thinking enabled")
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"hint\\\":false}\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"glasses_hint":true,"glasses_hint_phase":"judge","hint_research":{"source":"web","query":"must not search"},"model":"small","messages":[{"role":"system","content":"CURRENT_SHORT_TASK"},{"role":"user","content":"피라미드"}],"tools":[{"type":"function","function":{"name":"execute_command"}}],"max_tokens":9999,"stream":true,"previous_response_id":"resp_existing_web"}`))
	w := httptest.NewRecorder()
	handleChat(w, req, &App{llmEndpoint: server.URL, llmMode: "stateful", enableTools: true}, &AuthManager{users: map[string]*User{}})
	if w.Code != 200 || calls != 1 {
		t.Fatalf("%d calls=%d %s", w.Code, calls, w.Body.String())
	}
}

func TestGlassesAdvisoryRetainsCandidatesWhenEmbeddingDisabled(t *testing.T) {
	no := false
	auth := &AuthManager{users: map[string]*User{"advice": {ID: "advice", Settings: UserSettings{EmbeddingConfig: &EmbeddingModelConfig{Enabled: no}}}}}
	req := httptest.NewRequest("POST", "/api/glasses/hint-decision", strings.NewReader(`{"advisory":true,"text":"청동기 시대에는 청동으로 도구를 만들었대."}`))
	req.Header.Set("X-User-ID", "advice")
	w := httptest.NewRecorder()
	handleGlassesHintDecision(auth)(w, req)
	var decision glassesHintDecision
	_ = json.Unmarshal(w.Body.Bytes(), &decision)
	if w.Code != 200 || decision.Hint || len(decision.Candidates) == 0 {
		t.Fatalf("%d %+v", w.Code, decision)
	}
}

// Opt-in full gateway-to-LM-Studio test; credentials remain in the existing config file.
func TestGlassesJudgeLocalGemma(t *testing.T) {
	configPath, fixturePath := os.Getenv("DKST_HINT_JUDGE_LIVE_CONFIG"), os.Getenv("DKST_HINT_JUDGE_FIXTURES")
	if configPath == "" || fixturePath == "" {
		t.Skip("local model/config fixtures not specified")
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg AppConfig
	if err = json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name string         `json:"name"`
		Body map[string]any `json:"body"`
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		if fixture.Name != "product" && fixture.Name != "daily" {
			continue
		}
		fixture.Body["model"] = "gemma-4-e2b-it-mlx"
		body, _ := json.Marshal(fixture.Body)
		req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(body)))
		w := httptest.NewRecorder()
		handleChat(w, req, &App{llmEndpoint: cfg.LLMEndpoint, llmApiToken: cfg.LLMApiToken, llmMode: "stateful", enableTools: true}, &AuthManager{users: map[string]*User{}})
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", fixture.Name, w.Code, w.Body.String())
		}
		var answer strings.Builder
		for _, line := range strings.Split(w.Body.String(), "\n") {
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			var data struct {
				Type    string `json:"type"`
				Content string `json:"content"`
				Error   any    `json:"error"`
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(payload), &data) != nil {
				continue
			}
			if data.Error != nil {
				t.Fatalf("%s: stream error %v", fixture.Name, data.Error)
			}
			if data.Type == "message.delta" {
				answer.WriteString(data.Content)
			} else if len(data.Choices) > 0 {
				answer.WriteString(data.Choices[0].Delta.Content)
			}
		}
		var decision struct {
			Hint     bool   `json:"hint"`
			Evidence string `json:"evidence"`
			Topic    string `json:"topic"`
		}
		if err = json.Unmarshal([]byte(answer.String()), &decision); err != nil {
			t.Fatalf("%s: %v %q", fixture.Name, err, answer.String())
		}
		if decision.Hint != (fixture.Name == "product") {
			t.Fatalf("%s: %+v", fixture.Name, decision)
		}
		if decision.Hint && !strings.Contains(decision.Evidence, "맥북 에어 M4") {
			t.Fatal(decision)
		}
		t.Logf("%s: hint=%v topic=%q", fixture.Name, decision.Hint, decision.Topic)
	}
}
