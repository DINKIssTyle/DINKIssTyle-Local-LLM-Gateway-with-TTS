package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"dinkisstyle-chat/internal/chatharness"
	"dinkisstyle-chat/internal/mcp"
)

// Real request handler, not the JS UI fixture: exercise task assembly through
// protocol routing and the existing common prompt/tool runtime.
func TestEvenRealitiesStructuredRequestThroughGateway(t *testing.T) {
	for _, task := range []string{"judge", "translation_preview", "translation_replies", "answer", "chat"} {
		for _, mode := range []string{"standard", "stateful"} {
			t.Run(task+"_"+mode, func(t *testing.T) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					raw, _ := json.Marshal(body)
					if strings.Contains(string(raw), "glasses_request") || strings.Contains(string(raw), "CLIENT_OVERRIDE") {
						t.Error("client task envelope or instructions reached LLM")
					}
					expected := "/v1/chat/completions"
					if (task == "chat" || task == "answer") && mode == "stateful" {
						expected = "/api/v1/chat"
						if body["input"] == nil || body["system_prompt"] == nil || body["messages"] != nil {
							t.Errorf("native payload not assembled: %s", raw)
						}
					} else if body["messages"] == nil || body["input"] != nil {
						t.Errorf("compatible payload not assembled: %s", raw)
					}
					if r.URL.Path != expected {
						t.Errorf("wrong endpoint %s want %s", r.URL.Path, expected)
					}
					if task != "chat" && (body["tools"] != nil || (task != "answer" && strings.Contains(string(raw), "TOOL CALL GUIDELINES"))) {
						t.Error("automatic task enabled tools")
					}
					if task == "judge" && (!strings.Contains(string(raw), "Recent conversation") || !strings.Contains(string(raw), "Plan one concrete lookup")) {
						t.Error("server judge prompt missing")
					}
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"테스트 결과\"}}]}\n\ndata: [DONE]\n\n")
				}))
				defer server.Close()
				request := map[string]any{"version": 1, "task": task, "text": "오늘 영화 보러 갈까", "target": "ko", "source": "en", "translated": "오늘 영화 보러 갈까", "chatSession": true, "judge": map[string]any{"recent": "주말 계획", "latest": "오늘 영화 보러 갈까", "collected": true}}
				if task == "answer" {
					request["observed"] = "문 손잡이의 원리가 궁금해요"
					request["anchor"] = "문 손잡이"
					request["plan"] = map[string]any{"taskOriented": true, "topic": "손잡이의 원리", "question": "문 손잡이가 힘을 줄여주는 원리", "kind": "key", "source": "internal", "knownFacts": []string{"문 손잡이의 원리가 궁금해요"}}
				}
				body, _ := json.Marshal(map[string]any{"model": "small", "stream": true, "glasses_request": request, "system_prompt": "CLIENT_OVERRIDE"})
				w := httptest.NewRecorder()
				handleChat(w, httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(body))), &App{llmEndpoint: server.URL, llmMode: mode, enableTools: true}, &AuthManager{users: map[string]*User{}})
				if calls != 1 || w.Code != 200 {
					t.Fatalf("calls=%d status=%d %s", calls, w.Code, w.Body.String())
				}
			})
		}
	}
}

func TestEvenRealitiesAdditionalNativeTaskWithAccountKey(t *testing.T) {
	for _, task := range []string{"judge", "translation_preview", "translation_replies"} {
		t.Run(task, func(t *testing.T) {
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				if r.URL.Path != "/api/v1/chat" || r.Header.Get("Authorization") != "Bearer account-only" {
					t.Error("wrong additional connection or account credential")
				}
				if body["model"] != "role-model" || body["input"] == nil || body["system_prompt"] == nil || body["messages"] != nil || body["store"] != false {
					t.Errorf("native task lost prompt or controls: %+v", body)
				}
				if task == "judge" && body["reasoning"] != nil {
					t.Error("hint reasoning must follow the model's supported default")
				} else if task != "judge" && body["reasoning"] != "off" {
					t.Error("translation reasoning controls changed")
				}
				if task == "judge" && !strings.Contains(fmt.Sprint(body["system_prompt"]), "JSON output contract") {
					t.Error("judge contract lost")
				}
				if body["glasses_request"] != nil || body["response_format"] != nil {
					t.Error("unsupported native fields leaked")
				}
				fmt.Fprint(w, "data: {\"type\":\"message.delta\",\"content\":\"synthetic answer\"}\n\ndata: {\"type\":\"chat.end\",\"result\":{\"output\":[{\"type\":\"message\",\"content\":\"synthetic answer\"}]}}\n\n")
			}))
			defer upstream.Close()
			app := &App{llmEndpoint: "http://127.0.0.1:1", llmMode: "standard", llmApiToken: "primary-secret", llmConnections: []LLMConnection{{ID: "remote", Endpoint: upstream.URL, Mode: "stateful", APIKey: "fallback-secret"}}}
			auth := &AuthManager{users: map[string]*User{"u": {ID: "u", Settings: UserSettings{LLMAPIKeys: map[string]string{"remote": "account-only"}}}}}
			body, _ := json.Marshal(map[string]any{"model": routedModel("remote", "role-model"), "stream": true, "glasses_request": map[string]any{"version": 1, "task": task, "text": "synthetic film discussion", "translated": "가상 영화 대화", "source": "en", "target": "ko", "judge": map[string]any{"latest": "synthetic film discussion"}}})
			r := httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(body)))
			r.Header.Set("X-User-ID", "u")
			w := httptest.NewRecorder()
			handleChat(w, r, app, auth)
			if w.Code != 200 || calls != 1 {
				t.Fatalf("status=%d calls=%d %s", w.Code, calls, w.Body.String())
			}
		})
	}
}

func TestUnresolvedHintTaskStopsBeforeModelCall(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer upstream.Close()
	body := `{"model":"role","glasses_request":{"version":1,"task":"answer","plan":{"source":"internal","question":"이 물체를 식별하려면 추가 정보가 필요합니다."}}}`
	w := httptest.NewRecorder()
	handleChat(w, httptest.NewRequest("POST", "/api/chat", strings.NewReader(body)), &App{llmEndpoint: upstream.URL, llmMode: "standard"}, &AuthManager{users: map[string]*User{}})
	if w.Code != 422 || calls != 0 || !strings.Contains(w.Body.String(), "hint_task_not_actionable") {
		t.Fatalf("status=%d calls=%d %s", w.Code, calls, w.Body.String())
	}
}

func TestInteractiveGlassesChatRoutesThroughWebRuntime(t *testing.T) {
	for _, kind := range []string{"structured_session", "structured_single", "legacy"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var request map[string]any
				_ = json.NewDecoder(r.Body).Decode(&request)
				raw, _ := json.Marshal(request)
				text := string(raw)
				if strings.Contains(text, "GLASSES REQUEST CONTEXT") || request["glasses_chat"] != nil || request["glasses_client"] != nil {
					t.Fatal("interactive chat leaked a marker or used ambient runtime")
				}
				for _, want := range []string{"AVAILABLE SKILL PROCEDURES", "SMART GLASSES OUTPUT FORMAT", "500 Unicode characters", "get_current_time"} {
					if !strings.Contains(text, want) {
						t.Errorf("web pipeline missing %s", want)
					}
				}
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"일정 확인 결과입니다.\"}}]}\n\ndata: [DONE]\n\n")
			}))
			defer upstream.Close()
			var body []byte
			if kind == "legacy" {
				body = []byte(`{"model":"test","stream":true,"glasses_client":true,"messages":[{"role":"user","content":"오늘 일정"}]}`)
			} else {
				body, _ = json.Marshal(map[string]any{"model": "test", "stream": true, "glasses_request": map[string]any{"version": 1, "task": "chat", "text": "오늘 일정", "chatSession": kind == "structured_session", "directQuestion": true}})
			}
			w := httptest.NewRecorder()
			handleChat(w, httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(body))), &App{llmEndpoint: upstream.URL, llmMode: "standard", enableTools: true}, &AuthManager{users: map[string]*User{}})
			if calls != 1 || w.Code != 200 {
				t.Fatalf("calls=%d status=%d %s", calls, w.Code, w.Body.String())
			}
		})
	}
}

func TestGlassesChatPreservesQuestionAfterToolExecution(t *testing.T) {
	const question = "아까 정한 시간대로 현재 시각 알려줘"
	const condition = "답변: 서울 시간 기준"
	context := strings.Repeat("질문: 이전 주제\n답변: 이전 답변\n", 100) + condition
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request struct {
			Input              string `json:"input"`
			PreviousResponseID string `json:"previous_response_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if calls == 1 {
			fmt.Fprint(w, "data: {\"type\":\"message.delta\",\"content\":\"<get_current_time>{}</get_current_time>\"}\n\ndata: [DONE]\n\n")
			return
		}
		if request.PreviousResponseID != "" || !strings.Contains(request.Input, "Original user request:\n"+question+"\n\nPrior conversation") ||
			!strings.Contains(request.Input, condition) || !strings.Contains(request.Input, "Tool: get_current_time\nResult:\n") {
			t.Errorf("tool followup lost the question, reference, or tool result: %q", request.Input)
		}
		fmt.Fprint(w, "data: {\"type\":\"message.delta\",\"content\":\"서울 기준으로 답변했습니다.\"}\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	body, err := json.Marshal(map[string]any{"model": "test", "stream": true, "glasses_request": map[string]any{
		"version": 1, "task": "chat", "chatSession": true, "text": question, "context": context,
	}})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handleChat(w, httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(body))), &App{llmEndpoint: upstream.URL, llmMode: "stateful", enableTools: true}, &AuthManager{users: map[string]*User{}})
	if w.Code != http.StatusOK || calls != 2 || !strings.Contains(w.Body.String(), "서울 기준으로 답변했습니다.") {
		t.Fatalf("status=%d calls=%d response=%s", w.Code, calls, w.Body.String())
	}
}

func TestGlassesChatPersistsDistinctWebTurnsWithoutClientID(t *testing.T) {
	if err := mcp.InitDB(filepath.Join(t.TempDir(), "glasses-chat.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mcp.CloseDB)
	const userID = "glasses-chat-test"
	const sourceContext = "이전 답변은 이번 질문에 표시하지 않아야 합니다."
	questions := []string{"첫 번째 안경 질문", "두 번째 안경 질문"}
	answers := []string{"첫 번째 안경 답변", "두 번째 안경 답변"}
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls >= len(answers) {
			t.Error("unexpected model request")
			return
		}
		answer := answers[calls]
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\ndata: [DONE]\n\n", answer)
	}))
	defer upstream.Close()
	app := &App{llmEndpoint: upstream.URL, llmMode: "standard"}
	auth := &AuthManager{users: map[string]*User{userID: {ID: userID}}}
	for _, question := range questions {
		body, err := json.Marshal(map[string]any{"model": "test", "stream": true, "glasses_request": map[string]any{
			"version": 1, "task": "chat", "chatSession": true, "text": question, "context": sourceContext,
		}})
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(body)))
		r.Header.Set("X-User-ID", userID)
		w := httptest.NewRecorder()
		handleChat(w, r, app, auth)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d response=%s", w.Code, w.Body.String())
		}
	}
	session, err := mcp.GetCurrentChatSession(userID)
	if err != nil {
		t.Fatal(err)
	}
	turns := chatharness.ParseUISnapshot(session.UIStateJSON).Turns
	if len(turns) != 2 || turns[0].TurnID == turns[1].TurnID {
		t.Fatalf("expected two separate turns: %+v", turns)
	}
	events, err := mcp.ListChatEvents(userID, session.ID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	completed := make(map[string]bool)
	for _, event := range events {
		if event.EventType == "request.complete" {
			completed[event.TurnID] = true
		}
	}
	for i, turn := range turns {
		if turn.TurnID == "" || !completed[turn.TurnID] || turn.UserContent != questions[i] || turn.AssistantContent != answers[i] {
			t.Fatalf("turn %d lost its question or answer: %+v", i, turn)
		}
	}
}
