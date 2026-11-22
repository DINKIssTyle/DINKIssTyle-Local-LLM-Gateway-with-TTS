package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGlassesJudgeCanFinishAfterFormerSixSecondDeadline(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(6500 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"task\\\":\\\"손잡이의 지렛대 원리\\\",\\\"anchor\\\":\\\"손잡이\\\",\\\"source\\\":\\\"internal\\\",\\\"kind\\\":\\\"key\\\",\\\"search_query\\\":\\\"\\\"}\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"model":"small","stream":true,"glasses_request":{"version":1,"task":"judge","judge":{"latest":"손잡이를 끝에서 잡으면 왜 힘이 덜 들까요?"}}}`))
	w := httptest.NewRecorder()
	handleChat(w, req, &App{llmEndpoint: upstream.URL, llmMode: "standard"}, &AuthManager{users: map[string]*User{}})
	if w.Code != 200 || !strings.Contains(w.Body.String(), "지렛대") || strings.Contains(w.Body.String(), `"error"`) {
		t.Fatalf("valid delayed judgment lost: %d %s", w.Code, w.Body.String())
	}
}

func TestGlassesJudgeReportsTransportFailureInsteadOfSuccessfulEmptyAnswer(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprint(deadline), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "4096")
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"task\\\":\"}}]}\n\n")
				w.(http.Flusher).Flush()
				if deadline {
					<-r.Context().Done()
				}
			}))
			defer upstream.Close()
			req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"model":"small","stream":true,"glasses_request":{"version":1,"task":"judge","judge":{"latest":"손잡이 원리"}}}`))
			if deadline {
				ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
				defer cancel()
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()
			handleChat(w, req, &App{llmEndpoint: upstream.URL, llmMode: "standard"}, &AuthManager{users: map[string]*User{}})
			want := "judge_stream_error"
			if deadline {
				want = "judge_timeout"
			}
			if !strings.Contains(w.Body.String(), want) || strings.Contains(w.Body.String(), `"type":"request.complete"`) {
				t.Fatalf("partial answer reported as a successful decision: %s", w.Body.String())
			}
		})
	}
}

// Opt-in check through the real gateway and the configured judge connection.
// Only synthetic conversation is sent to the model; no memory/search is used.
func TestGlassesJudgeConfiguredModelLive(t *testing.T) {
	configPath, model := os.Getenv("DKST_HINT_JUDGE_LIVE_CONFIG"), os.Getenv("DKST_HINT_JUDGE_LIVE_MODEL_ROUTE")
	if configPath == "" || model == "" {
		t.Skip("configured model route not specified")
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg AppConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	app := &App{llmEndpoint: cfg.LLMEndpoint, llmApiToken: cfg.LLMApiToken, llmMode: cfg.LLMMode, llmConnections: cfg.LLMConnections}
	for _, speech := range []string{"문 손잡이의 끝부분을 잡으면 왜 더 작은 힘으로 문을 열 수 있을까요?", "맥북 에어 M4의 13인치와 15인치 무게가 얼마나 다른지 궁금해요."} {
		t.Run(speech, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"model": model, "stream": true, "glasses_request": map[string]any{"version": 1, "task": "judge", "judge": map[string]any{"latest": speech}, "preferWeb": true, "target": "ko"}})
			req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(body)))
			w := httptest.NewRecorder()
			started := time.Now()
			handleChat(w, req, app, &AuthManager{users: map[string]*User{}})
			var final string
			for _, line := range strings.Split(w.Body.String(), "\n") {
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				var data map[string]any
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &data) != nil {
					continue
				}
				if data["error"] != nil {
					t.Fatalf("judge error after %s: %v", time.Since(started), data["error"])
				}
				if text, ok := data["final_assistant_content"].(string); ok {
					final = text
				}
			}
			final = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(final), "```json"), "```"))
			var decision struct{ Task, Anchor, Source string }
			if err := json.Unmarshal([]byte(final), &decision); err != nil || decision.Task == "" || decision.Anchor == "" || !strings.Contains(speech, decision.Anchor) {
				t.Fatalf("no actionable grounded decision after %s (HTTP %d): %q", time.Since(started), w.Code, final)
			}
			t.Logf("grounded %s plan completed in %s", decision.Source, time.Since(started).Round(time.Millisecond))
		})
	}
}

func TestGlassesJudgeHasNoRuntimeOrTools(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("wrong judge API: %s", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "small" || body["max_tokens"] != float64(1024) || body["store"] != false {
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
		if kwargs, ok := body["chat_template_kwargs"].(map[string]any); ok && kwargs["enable_thinking"] == false {
			t.Error("hint deliberation was forcibly disabled")
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
