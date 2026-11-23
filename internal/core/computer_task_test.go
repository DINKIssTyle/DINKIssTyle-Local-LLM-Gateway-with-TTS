package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dinkisstyle-chat/internal/chatharness"
	"dinkisstyle-chat/internal/promptkit"
	"dinkisstyle-chat/internal/toolruntime"
)

func fakeDesktopRegistry(t *testing.T, initialActive ...bool) ([]promptkit.ToolDefinition, *int) {
	t.Helper()
	png, err := os.ReadFile(filepath.Join("testdata", "desktop_download_progress.png"))
	if err != nil {
		t.Fatal(err)
	}
	imageURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	gatewayPNG, err := os.ReadFile(filepath.Join("testdata", "desktop_gateway.png"))
	if err != nil {
		t.Fatal(err)
	}
	gatewayImage := "data:image/png;base64," + base64.StdEncoding.EncodeToString(gatewayPNG)
	active := true
	if len(initialActive) > 0 {
		active = initialActive[0]
	}
	old := toolruntime.Default
	r := toolruntime.NewRegistry()
	calls := new(int)
	tools := []promptkit.ToolDefinition{}
	for _, name := range []string{"computer_list_apps", "computer_switch_app", "computer_observe"} {
		name := name
		schema := json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
		description := map[string]string{"computer_list_apps": "List running GUI applications with pid and frontmost status. No arguments.", "computer_switch_app": "Activate a running app using its pid from computer_list_apps. Observe afterward.", "computer_observe": "Observe the current primary display and foreground app, returning a screenshot and observation_id. No arguments."}[name]
		if name == "computer_switch_app" {
			schema = json.RawMessage(`{"type":"object","properties":{"pid":{"type":"integer","minimum":1}},"required":["pid"]}`)
		}
		if err := r.Register(toolruntime.Definition{Name: name, Description: description, InputSchema: schema}, func(_ context.Context, _ toolruntime.ExecutionContext, args json.RawMessage) (toolruntime.Result, error) {
			*calls++
			switch name {
			case "computer_list_apps":
				return toolruntime.Result{Content: fmt.Sprintf(`{"applications":[{"name":"LM Studio","pid":42,"frontmost":%t}]}`, active)}, nil
			case "computer_switch_app":
				var input struct {
					PID int `json:"pid"`
				}
				_ = json.Unmarshal(args, &input)
				if input.PID != 42 {
					return toolruntime.Result{IsError: true}, fmt.Errorf("unknown app PID")
				}
				active = true
				return toolruntime.Result{Content: `{"status":"activation_requested","pid":42}`}, nil
			default:
				if !active {
					return toolruntime.Result{Content: `{"observation_id":"gateway","application":"Gateway","frontmost_pid":7,"windows":[{"title":"Chat"}]}`, ImageURLs: []string{gatewayImage}}, nil
				}
				return toolruntime.Result{Content: `{"observation_id":"screen1","application":"LM Studio","frontmost_pid":42,"windows":[{"title":"Downloads — test-model: 42% (4.2 GB / 10 GB), 8 MB/s"}]}`, ImageURLs: []string{imageURL}}, nil
			}
		}); err != nil {
			t.Fatal(err)
		}
		tools = append(tools, promptkit.ToolDefinition{Name: name, Description: description, InputSchema: schema})
	}
	toolruntime.Default = r
	t.Cleanup(func() { toolruntime.Default = old })
	return tools, calls
}

func TestDesktopTaskRejectsPrematureCompletionAndContinues(t *testing.T) {
	_, calls := fakeDesktopRegistry(t)
	reviews := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		content := `{"desktop_required":true,"kind":"desktop_work","goal":"Inspect current screen progress","steps":[{"title":"Read progress","done_when":"Progress visible","requires_screen":true}]}`
		if r.Header.Get("X-DKST-Task-Phase") == "route" {
			content = `{"mode":"desktop","request_evidence":"Inspect current screen progress"}`
		}
		if r.Header.Get("X-DKST-Task-Phase") == "review" {
			reviews++
			switch reviews {
			case 1:
				content = `{"answer":"I checked the progress","steps":{"1":{"status":"completed","evidence_ids":[1]}},"action_type":"completed","findings":["I checked the progress"]}`
			case 2:
				raw, _ := json.Marshal(body)
				if !strings.Contains(string(raw), "nonexistent") {
					t.Error("missing review repair feedback")
				}
				content = `{"next_tool":"computer_observe","arguments":{},"next_step":1,"steps":{"1":{"status":"pending","evidence_ids":[]}},"action_type":"continue"}`
			default:
				content = `{"answer":"42%","steps":{"1":{"status":"completed","evidence_ids":[1]}},"action_type":"completed","findings":["42%"]}`
			}
		}
		if r.Header.Get("X-DKST-Task-Phase") == "review" {
			content = `{"decision":` + content + `}`
		}
		if r.Header.Get("X-DKST-Task-Phase") == "verify" {
			content = `{"verified":true,"reason":"42% is visible"}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
	}))
	defer server.Close()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"model":"test","stream":true,"messages":[{"role":"user","content":"Inspect current screen progress"}]}`))
	handleChat(recorder, request, &App{llmEndpoint: server.URL, llmMode: "standard", enableTools: true, computerTaskPlanning: true}, &AuthManager{users: map[string]*User{}})
	output := recorder.Body.String()
	if !strings.Contains(output, "42%") || strings.Contains(output, "I checked the progress") || *calls != 1 || reviews != 3 || strings.Count(output, `"type":"task.progress"`) < 3 {
		t.Fatalf("gateway task did not recover and verify: tools=%d reviews=%d output=%s", *calls, reviews, output)
	}
}

func TestDesktopTaskPlannerLeavesOrdinaryConversationAlone(t *testing.T) {
	tools, calls := fakeDesktopRegistry(t)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-DKST-Task-Phase") != "route" {
			t.Error("ordinary chat entered planning")
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"mode\":\"chat\",\"request_evidence\":\"\"}"}}]}`)
	}))
	defer s.Close()
	handled, _ := runDesktopTask(context.Background(), desktopTaskModel{endpoint: s.URL}, "안녕", "", tools, toolruntime.ExecutionContext{}, func(string, map[string]any) { t.Error("non-desktop request created task") })
	if handled || *calls != 0 {
		t.Fatal("ordinary conversation triggered desktop work")
	}
}

func TestDesktopActionAvailabilityTracksObservedState(t *testing.T) {
	tools, _ := fakeDesktopRegistry(t)
	task := chatharness.DesktopTask{Kind: "desktop_work"}
	names := func(receipts []chatharness.DesktopReceipt) map[string]bool {
		set := map[string]bool{}
		for _, tool := range desktopAvailableTools(tools, task, receipts) {
			set[tool.Name] = true
		}
		return set
	}
	if first := names(nil); len(first) != 1 || !first["computer_observe"] {
		t.Fatal("desktop must start by observing")
	}
	receipts := []chatharness.DesktopReceipt{{ID: 1, Tool: "computer_observe", Success: true, HasImage: true}}
	if names(receipts)["computer_switch_app"] {
		t.Fatal("app switch accepted before PID discovery")
	}
	receipts = append(receipts, chatharness.DesktopReceipt{ID: 2, Tool: "computer_list_apps", Success: true, Result: `{"applications":[{"pid":42}]}`})
	if !names(receipts)["computer_switch_app"] || names(receipts)["computer_list_apps"] {
		t.Fatal("PID discovery did not advance available actions")
	}
	receipts = append(receipts, chatharness.DesktopReceipt{ID: 3, Tool: "computer_switch_app", Success: true})
	if next := names(receipts); len(next) != 1 || !next["computer_observe"] {
		t.Fatal("app switch must be followed by observation")
	}
}

func TestInspectionNeverExecutesNarrationAsTypedText(t *testing.T) {
	tools, calls := fakeDesktopRegistry(t)
	typed := 0
	inputSchema := json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"},"observation_id":{"type":"string"}},"required":["text","observation_id"]}`)
	if err := toolruntime.Default.Register(toolruntime.Definition{Name: "computer_type", InputSchema: inputSchema}, func(context.Context, toolruntime.ExecutionContext, json.RawMessage) (toolruntime.Result, error) {
		typed++
		return toolruntime.Result{Content: "typed"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	tools = append(tools, promptkit.ToolDefinition{Name: "computer_type", InputSchema: inputSchema})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := `{"mode":"desktop","request_evidence":"LM Studio 다운로드 상태 확인","text_input_evidence":"","target_application":"LM Studio"}`
		switch r.Header.Get("X-DKST-Task-Phase") {
		case "plan":
			content = `{"desktop_required":true,"kind":"desktop_work","goal":"진행률 확인","steps":[{"title":"진행률 읽기","done_when":"진행률 표시","requires_screen":true}]}`
		case "review":
			content = `{"decision":{"action_type":"continue","next_tool":"computer_type","arguments":{"text":"크롬 브라우저 창을 찾아야 합니다","observation_id":"screen1"},"next_step":1,"steps":{"1":{"status":"pending","evidence_ids":[]}}}}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
	}))
	defer server.Close()
	handled, answer := runDesktopTask(context.Background(), desktopTaskModel{endpoint: server.URL}, "LM Studio 다운로드 상태 확인", "", tools, toolruntime.ExecutionContext{}, func(string, map[string]any) {})
	if !handled || typed != 0 || *calls != 0 || !strings.Contains(answer, "중단") {
		t.Fatalf("inspection typed narration: typed=%d actions=%d answer=%s", typed, *calls, answer)
	}
	state := chatharness.DesktopTask{Kind: "desktop_work"}
	allowed := desktopAvailableTools(tools, state, []chatharness.DesktopReceipt{{ID: 1, Tool: "computer_observe", Success: true, HasImage: true}})
	for _, tool := range allowed {
		if tool.Name == "computer_type" {
			t.Fatal("typing exposed to inspection")
		}
	}
}

func TestIndependentVerificationRejectsGenericCompletion(t *testing.T) {
	tools, _ := fakeDesktopRegistry(t)
	verifies := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		content := `{"mode":"desktop","request_evidence":"진행률 확인","target_application":"LM Studio","text_input_evidence":""}`
		switch r.Header.Get("X-DKST-Task-Phase") {
		case "plan":
			content = `{"desktop_required":true,"kind":"desktop_work","goal":"진행률 확인","steps":[{"title":"진행률 읽기","done_when":"진행률 표시","requires_screen":true}]}`
		case "review":
			raw, _ := json.Marshal(body)
			if !strings.Contains(string(raw), "screen1") {
				content = `{"decision":{"action_type":"continue","next_tool":"computer_observe","arguments":{},"next_step":1,"steps":{"1":{"status":"pending","evidence_ids":[]}}}}`
			} else {
				content = `{"decision":{"action_type":"completed","answer":"명확한 정보가 화면에 표시되었습니다","findings":["명확한 정보가 화면에 표시되었습니다"],"steps":{"1":{"status":"completed","evidence_ids":[1]}}}}`
			}
		case "verify":
			verifies++
			content = `{"verified":false,"reason":"진행률 수치를 답하지 않았습니다"}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
	}))
	defer server.Close()
	last := chatharness.DesktopTask{}
	_, answer := runDesktopTask(context.Background(), desktopTaskModel{endpoint: server.URL}, "진행률 확인", "", tools, toolruntime.ExecutionContext{}, func(event string, payload map[string]any) {
		if event == "task.progress" {
			last = payload["task"].(chatharness.DesktopTask)
		}
	})
	if verifies != 4 || last.Status == "completed" || last.Steps[0].Status == "completed" || !strings.Contains(answer, "중단") {
		t.Fatalf("generic claim completed task: verifies=%d state=%+v answer=%s", verifies, last, answer)
	}
}

func TestDesktopRouteFailureFallsBackWithoutPlanning(t *testing.T) {
	tools, calls := fakeDesktopRegistry(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-DKST-Task-Phase") != "route" {
			t.Error("route failure entered planning")
		}
		http.Error(w, "schema unsupported", http.StatusBadRequest)
	}))
	defer upstream.Close()
	handled, answer := runDesktopTask(context.Background(), desktopTaskModel{endpoint: upstream.URL}, "내일 아침 날씨", "", tools, toolruntime.ExecutionContext{}, func(string, map[string]any) { t.Error("task event emitted before desktop authorization") })
	if handled || answer != "" || *calls != 0 {
		t.Fatal("routing failure intercepted ordinary chat")
	}
}

func TestDesktopInputCandidate(t *testing.T) {
	for _, question := range []string{"남은 메모리량", "계속 비가 내리나?", "내일 아침 날씨", "논산"} {
		if desktopInputCandidate(question) {
			t.Errorf("ordinary request reached desktop routing: %q", question)
		}
	}
	for _, question := range []string{"현재 화면 내용", "LM Studio에서 다운로드 상태 확인", "크롬 탭 목록"} {
		if !desktopInputCandidate(question) {
			t.Errorf("desktop request skipped routing: %q", question)
		}
	}
}

func TestWeatherRequestBypassesDesktopPlanningThroughGateway(t *testing.T) {
	for _, mode := range []string{"standard", "stateful"} {
		for _, routeFailure := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/route_failure_%t", mode, routeFailure), func(t *testing.T) {
				_, desktopCalls := fakeDesktopRegistry(t)
				schema := json.RawMessage(`{"type":"object","properties":{}}`)
				if err := toolruntime.Default.Register(toolruntime.Definition{Name: "search_web", Description: "Weather lookup", InputSchema: schema}, func(context.Context, toolruntime.ExecutionContext, json.RawMessage) (toolruntime.Result, error) {
					return toolruntime.Result{Content: "weather"}, nil
				}); err != nil {
					t.Fatal(err)
				}
				routes, chats := 0, 0
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					phase := r.Header.Get("X-DKST-Task-Phase")
					if phase == "route" {
						routes++
						if routeFailure {
							http.Error(w, "schema unsupported", 400)
							return
						}
						// Reproduce the local Qwen template: schema JSON lands in reasoning_content.
						fmt.Fprint(w, `{"choices":[{"message":{"content":"","reasoning_content":"{\"mode\":\"chat\",\"request_evidence\":\"\"}"}}]}`)
						return
					}
					if phase != "" {
						t.Errorf("weather entered desktop %s", phase)
						http.Error(w, "unexpected desktop planning", 500)
						return
					}
					chats++
					var request map[string]any
					_ = json.NewDecoder(r.Body).Decode(&request)
					raw, _ := json.Marshal(request)
					if strings.Contains(string(raw), "computer_observe") || strings.Contains(string(raw), "#### builtin:computer-use") {
						t.Error("desktop tools/procedure leaked into ordinary weather request")
					}
					if !strings.Contains(string(raw), "search_web") {
						t.Error("normal lookup tool was removed")
					}
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"WEATHER_NORMAL_CHAT\"}}]}\n\ndata: [DONE]\n\n")
				}))
				defer upstream.Close()
				body := `{"model":"qwen-test","stream":true,"messages":[{"role":"user","content":"내일 아침 날씨"}]}`
				if mode == "stateful" {
					body = `{"model":"qwen-test","stream":true,"input":"내일 아침 날씨"}`
				}
				recorder := httptest.NewRecorder()
				handleChat(recorder, httptest.NewRequest("POST", "/api/chat", strings.NewReader(body)), &App{llmEndpoint: upstream.URL, llmMode: mode, enableTools: true, computerTaskPlanning: true}, &AuthManager{users: map[string]*User{}})
				output := recorder.Body.String()
				if routes != 0 || chats != 1 || *desktopCalls != 0 || strings.Contains(output, `"type":"task.progress"`) || !strings.Contains(output, "WEATHER_NORMAL_CHAT") {
					t.Fatalf("weather intercepted: routes=%d chats=%d desktop=%d output=%s", routes, chats, *desktopCalls, output)
				}
			})
		}
	}
}

func TestLiveDesktopTaskPlannerWithFakeDesktop(t *testing.T) {
	if os.Getenv("DKST_LIVE_DESKTOP_EVAL") != "1" {
		t.Skip("opt-in local model evaluation; desktop is always fake")
	}
	wasEnabled := isDebugTraceCollectorEnabled()
	setDebugTraceCollectorEnabled(true)
	t.Cleanup(func() {
		for _, entry := range getDebugTraceEntriesSnapshot() {
			if entry.Stage == "task.review_rejected" {
				t.Logf("review rejection: %+v", entry.Details)
			}
		}
		setDebugTraceCollectorEnabled(wasEnabled)
	})
	home, _ := os.UserHomeDir()
	raw, err := os.ReadFile(filepath.Join(home, "Library/Application Support/DKST LLM Chat Server/config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Endpoint string `json:"llmEndpoint"`
		Token    string `json:"llmApiToken"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	if config.Endpoint != "http://127.0.0.1:8094" {
		t.Fatal("live evaluation requires the observed local LM Studio endpoint")
	}
	tools, calls := fakeDesktopRegistry(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	model := os.Getenv("DKST_LIVE_DESKTOP_MODEL")
	if model == "" {
		model = "gemma-4-e2b-it-mlx"
	}
	if model != "gemma-4-e2b-it-mlx" && model != "qwen3.6-35b-a3b-uncensored-heretic-mlx" {
		t.Fatal("unexpected evaluation model")
	}
	handled, answer := runDesktopTask(ctx, desktopTaskModel{endpoint: config.Endpoint, token: config.Token, model: model}, "LM Studio에서 다운로드 중인 작업이 얼마나 진행되었는지 확인해봐", "", tools, toolruntime.ExecutionContext{}, func(event string, payload map[string]any) {
		if event == "task.progress" {
			raw, _ := json.Marshal(payload)
			t.Log(string(raw))
		}
		if event == "tool_call.start" {
			t.Logf("fake tool: %s", payload["tool"])
		}
	})
	if !handled || !strings.Contains(answer, "42") || *calls < 4 || *calls > 8 {
		t.Fatalf("live task failed: handled=%v answer=%q fake_actions=%d", handled, answer, *calls)
	}
	t.Logf("verified answer=%s; fake desktop actions=%d", answer, *calls)
}

func TestLiveDesktopIntentRouting(t *testing.T) {
	if os.Getenv("DKST_LIVE_DESKTOP_ROUTE") != "1" {
		t.Skip("opt-in local model routing only; no desktop actions")
	}
	wasEnabled := isDebugTraceCollectorEnabled()
	setDebugTraceCollectorEnabled(true)
	t.Cleanup(func() { setDebugTraceCollectorEnabled(wasEnabled) })
	home, _ := os.UserHomeDir()
	raw, err := os.ReadFile(filepath.Join(home, "Library/Application Support/DKST LLM Chat Server/config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Endpoint string `json:"llmEndpoint"`
		Token    string `json:"llmApiToken"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	if config.Endpoint != "http://127.0.0.1:8094" {
		t.Fatal("unexpected local endpoint")
	}
	for _, model := range []string{"gemma-4-e2b-it-mlx", "qwen3.6-35b-a3b-uncensored-heretic-mlx"} {
		for _, tc := range []struct {
			question string
			desktop  bool
		}{{"내일 아침 날씨", false}, {"안녕", false}, {"컴퓨터 유즈가 뭔지 설명해줘", false}, {"LM Studio에서 다운로드 중인 작업이 얼마나 진행되었는지 확인해봐", true}} {
			t.Run(model+"/"+tc.question, func(t *testing.T) {
				m := desktopTaskModel{endpoint: config.Endpoint, token: config.Token, model: model}
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				defer cancel()
				got := m.route(ctx, tc.question, "user: LM Studio 다운로드 진행률 확인\nassistant: 확인을 완료했습니다.")
				if got != tc.desktop {
					t.Fatalf("desktop route=%v want=%v", got, tc.desktop)
				}
			})
		}
	}
}

func TestRecentInboxDefaultKeepsSpecificRequestsInNormalRouting(t *testing.T) {
	for _, question := range []string{"이메일 확인", "최근 메일 확인", "Check my email!"} {
		if !isRecentInboxRequest(question) {
			t.Errorf("missed bare inbox request %q", question)
		}
	}
	for _, question := range []string{"어제 이메일 확인", "이메일 확인 방법 알려줘", "메일 보내줘", "왜이래.."} {
		if isRecentInboxRequest(question) {
			t.Errorf("bare inbox default swallowed %q", question)
		}
	}
}

func TestDesktopWindowPresenceUsesFirstObservationWithoutModelOrActivation(t *testing.T) {
	for _, tc := range []struct {
		name, windows, answer, status string
	}{
		{"logged Chrome window", `[{"owner":"DKST LLM Chat","pid":4790},{"owner":"Google Chrome","pid":1361}]`, "주 디스플레이에 Google Chrome 창이 있습니다.", "completed"},
		{"no Chrome window", `[{"owner":"DKST LLM Chat","pid":4790}]`, "창이 없습니다.", "completed"},
		{"missing metadata", `null`, "유효한 창 목록을 받지 못했습니다.", "blocked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := toolruntime.Default
			registry := toolruntime.NewRegistry()
			toolruntime.Default = registry
			t.Cleanup(func() { toolruntime.Default = old })
			var calls int
			schema := json.RawMessage(`{"type":"object"}`)
			err := registry.Register(toolruntime.Definition{Name: "computer_observe", InputSchema: schema}, func(context.Context, toolruntime.ExecutionContext, json.RawMessage) (toolruntime.Result, error) {
				calls++
				return toolruntime.Result{Content: `{"frontmost_pid":4790,"windows":` + tc.windows + `}`, ImageURLs: []string{"test observation"}}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("window presence should use the observation without calling the model")
				w.WriteHeader(http.StatusBadRequest)
			}))
			defer upstream.Close()
			body := `{"model":"gemma-4-e2b-it-mlx","stream":true,"messages":[{"role":"user","content":"컴퓨터 화면에 크롬브라우저가 있나?"}]}`
			w := httptest.NewRecorder()
			handleChat(w, httptest.NewRequest("POST", "/api/chat", strings.NewReader(body)), &App{llmEndpoint: upstream.URL, llmMode: "standard", enableTools: true, computerTaskPlanning: true}, &AuthManager{users: map[string]*User{}})
			if stream := w.Body.String(); calls != 1 || !strings.Contains(stream, tc.answer) || !strings.Contains(stream, `"status":"`+tc.status+`"`) {
				t.Fatalf("unexpected result after %d observations: %s", calls, stream)
			}
		})
	}
	for _, question := range []string{"화면에 로그인 버튼이 있나?", "화면에 다운로드 진행률이 보여?", "크롬 창을 열어줘"} {
		if _, ok := desktopWindowPresenceTarget(question); ok {
			t.Errorf("treated %q as window presence", question)
		}
	}
}
