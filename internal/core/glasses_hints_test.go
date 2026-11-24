package core

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"dinkisstyle-chat/internal/mcp"
	"dinkisstyle-chat/internal/toolruntime"
	"path/filepath"
)

func TestGlassesHintSelection(t *testing.T) {
	for _, tc := range []struct {
		name, selected, sensitivity string
		scores                      map[string]float64
		want                        bool
	}{
		{"clear explanation", "explain", "balanced", map[string]float64{"explain": .8, "none": .2}, true},
		{"ordinary conversation", "none", "active", map[string]float64{"none": .9, "reply": .1}, false},
		{"ambiguous quiet", "reply", "quiet", map[string]float64{"reply": .51, "none": .49}, false},
		{"quiet", "check", "quiet", map[string]float64{"check": .60, "none": .2, "explain": .2}, false},
		{"active", "recall", "active", map[string]float64{"recall": .40, "none": .30, "explain": .30}, true},
		{"passive window near tie", "none", "balanced", map[string]float64{"none": .307, "explain": .296, "check": .235}, true},
		{"strong no hint", "none", "balanced", map[string]float64{"none": .90, "explain": .10}, false},
		{"knowledge topic", "background", "balanced", map[string]float64{"background": .8, "none": .15, "explain": .05}, true},
		{"missing scores", "check", "balanced", nil, false},
		{"invalid category", "execute_command", "active", map[string]float64{"execute_command": 1}, false},
		{"invalid score", "check", "balanced", map[string]float64{"check": math.NaN(), "none": 0}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := selectGlassesHint(jevReply{Selected: tc.selected, Probabilities: tc.scores}, tc.sensitivity)
			if d.Hint != tc.want {
				t.Fatalf("decision %#v", d)
			}
		})
	}
	for _, text := range []string{"응, 맞아.", "네", "yes", "   "} {
		if !hintFiller(text) {
			t.Fatalf("filler: %q", text)
		}
	}
	if hintFiller("묵시적 갱신이 무슨 뜻이지?") {
		t.Fatal("substantive utterance filtered")
	}
}
func TestGlassesHintReadOnlyTools(t *testing.T) {
	disabled := readOnlyHintTools([]string{"search_web"})
	ctx := toolruntime.ExecutionContext{EnableMemory: true, DisabledTools: disabled}
	for _, tool := range toolruntime.Default.List(ctx) {
		if !tool.Metadata.ReadOnly || tool.Metadata.SideEffecting {
			t.Fatalf("unsafe hint tool: %s", tool.Name)
		}
	}
	for _, name := range []string{"execute_command", "delete_memory"} {
		_, err := toolruntime.Default.Call(context.Background(), ctx, name, json.RawMessage(`{}`))
		if err == nil || !strings.Contains(err.Error(), "disabled") {
			t.Fatalf("call %s: %v", name, err)
		}
	}
}
func TestGlassesHintInput(t *testing.T) {
	auth := &AuthManager{}
	for _, tc := range []struct {
		body, method string
		status       int
	}{
		{`{"text":"네"}`, "POST", 200},
		{`{"text":""}`, "POST", 400},
		{`{"text":"설명해줘","sensitivity":"unknown"}`, "POST", 400},
		{`{`, "POST", 400},
		{`{"text":"네"}`, "GET", 405},
	} {
		req := httptest.NewRequest(tc.method, "/api/glasses/hint-decision", strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		handleGlassesHintDecision(auth)(w, req)
		if w.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.body, w.Code, w.Body.String())
		}
	}
}
func TestGlassesHintLocalJev(t *testing.T) {
	python, model := os.Getenv("DKST_JEV_TEST_PYTHON"), os.Getenv("DKST_JEV_TEST_MODEL")
	if python == "" || model == "" {
		t.Skip("local assets not specified")
	}
	worker, err := startJevEncoder(python, model, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer worker.Close()
	for _, text := range []string{"피라미드", "청동기 시대", "이번에 피라미드 영상을 봤어.\n거기서 청동기 시대 얘기가 나오더라고.\n응 그렇구나.", "마추픽추 사진을 보고 있어.", "응, 맞아. 오늘도 수고했어.", "계약서에 묵시적 갱신이라고 써있는데 그게 무슨 뜻이지?", "저번에 그 담당자랑 언제 만나기로 했더라?", "고마워, 내일 보자.", "이 제안은 정중하게 거절하고 싶은데 뭐라고 답해야 좋을까?", "유동성 공급이라는 게 뭔지 잘 모르겠네.", "계약서는 묵시적 갱신 조항이 적용된대.\n그게 어떤 의미인지 잘 모르겠는데.\n응 그렇구나.", "이번 자료는 유동성커버리지비율을 기준으로 작성됐습니다.\n다음 분기부터 이 수치로 평가할 예정입니다.\n네, 알겠습니다."} {
		started := time.Now()
		reply, err := worker.request(map[string]any{"op": "choice", "text": "RECENT conversation window:\n" + text, "criteria": glassesHintCriteria, "instructions": glassesHintInstructions}, 10*time.Second)
		if err != nil {
			t.Fatal(err)
		}

		if !selectGlassesHint(reply, "balanced").Hint {
			background, err := worker.request(map[string]any{"op": "choice", "text": text, "criteria": glassesBackgroundCriteria, "instructions": glassesBackgroundInstructions}, 10*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("BACKGROUND %s: selected=%s scores=%v decision=%+v", text, background.Selected, background.Probabilities, selectBackgroundHint(background, "balanced"))
		}
		t.Logf("%s: selected=%s scores=%v decision=%+v elapsed=%v", text, reply.Selected, reply.Probabilities, selectGlassesHint(reply, "balanced"), time.Since(started))
		if _, ok := glassesHintCriteria[reply.Selected]; !ok {
			t.Fatalf("invalid category: %s", reply.Selected)
		}
	}
}

func TestGlassesHintChatRouting(t *testing.T) {
	var upstream map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&upstream); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"짧은 힌트\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	app := &App{llmEndpoint: server.URL, llmMode: "standard", enableTools: true}
	auth := &AuthManager{users: map[string]*User{}}
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"glasses_hint":true,"model":"test","messages":[{"role":"user","content":"묵시적 갱신이란?"}],"stream":true,"previous_response_id":"resp_webchat"}`))
	req.Header.Set("X-User-ID", "hint-test")
	w := httptest.NewRecorder()
	handleChat(w, req, app, auth)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "짧은 힌트") {
		t.Fatalf("chat %d: %s", w.Code, w.Body.String())
	}
	if _, ok := upstream["glasses_hint"]; ok {
		t.Fatal("private hint control leaked upstream")
	}
	if _, ok := upstream["previous_response_id"]; ok {
		t.Fatal("hint linked to ordinary chat response")
	}
	for _, raw := range upstream["tools"].([]interface{}) {
		name := raw.(map[string]interface{})["function"].(map[string]interface{})["name"].(string)
		for _, tool := range toolruntime.Default.List(toolruntime.ExecutionContext{EnableMemory: true}) {
			if tool.Name == name && (!tool.Metadata.ReadOnly || tool.Metadata.SideEffecting) {
				t.Fatalf("unsafe advertised tool: %s", name)
			}
		}
	}
}

func TestGlassesBackgroundSelection(t *testing.T) {
	for _, tc := range []struct {
		score                 float64
		selected, sensitivity string
		want                  bool
	}{
		{.9, "topic", "balanced", true}, {.7, "topic", "quiet", false}, {.7, "topic", "balanced", true},
		{.53, "topic", "active", true}, {.2, "none", "balanced", false}, {math.NaN(), "topic", "active", false},
	} {
		decision := selectBackgroundHint(jevReply{Selected: tc.selected, Probabilities: map[string]float64{"topic": tc.score}}, tc.sensitivity)
		if decision.Hint != tc.want {
			t.Fatalf("%+v: %+v", tc, decision)
		}
	}
}

func TestHintEvidenceRespectsSettingsAndInjectsReferences(t *testing.T) {
	registry := toolruntime.NewRegistry()
	calls := 0
	err := registry.Register(toolruntime.Definition{Name: "search_web", InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`), Metadata: toolruntime.Metadata{ReadOnly: true}}, func(ctx context.Context, exec toolruntime.ExecutionContext, args json.RawMessage) (toolruntime.Result, error) {
		calls++
		var query map[string]string
		_ = json.Unmarshal(args, &query)
		if query["query"] != "피라미드의 건립 목적" {
			t.Errorf("wrong research query: %v", query)
		}
		return toolruntime.Result{Content: "출처: https://example.com/museum 피라미드는 왕실 무덤으로 건립되었다."}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	research := hintResearchRequest{Query: "피라미드의 건립 목적", Source: "web"}
	exec := toolruntime.ExecutionContext{UserID: "hint-evidence"}
	if _, _, err := loadHintEvidence(context.Background(), registry, exec, false, research); err == nil {
		t.Fatal("tools setting bypassed")
	}
	if _, _, err := loadHintEvidence(context.Background(), registry, toolruntime.ExecutionContext{DisabledTools: []string{"search_web"}}, true, research); err == nil {
		t.Fatal("disabled tool bypassed")
	}
	if calls != 0 {
		t.Fatal("search executed while disabled")
	}
	evidence, _, err := loadHintEvidence(context.Background(), registry, exec, true, research)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !strings.Contains(evidence, "<hint_evidence") || !strings.Contains(evidence, "왕실 무덤") {
		t.Fatal(evidence)
	}
	for _, body := range []string{`{"input":"조사 질문에 답하세요","model":"test"}`, `{"messages":[{"role":"user","content":"조사 질문에 답하세요"}]}`} {
		enriched, err := appendHintEvidence([]byte(body), evidence)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(enriched), "왕실 무덤") {
			t.Fatal(string(enriched))
		}
	}
	if _, _, err := loadHintEvidence(context.Background(), registry, exec, true, hintResearchRequest{Source: "memory", Query: "과거 약속"}); err == nil {
		t.Fatal("memory setting bypassed")
	}
}

func TestHintEvidenceUsesSourceFocusedQueryOnlyForWeb(t *testing.T) {
	registry := toolruntime.NewRegistry()
	var queries []string
	for _, name := range []string{"search_web", "search_memory"} {
		err := registry.Register(toolruntime.Definition{Name: name, InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)}, func(_ context.Context, _ toolruntime.ExecutionContext, args json.RawMessage) (toolruntime.Result, error) {
			var input map[string]string
			_ = json.Unmarshal(args, &input)
			queries = append(queries, input["query"])
			return toolruntime.Result{Content: "Title: 자료\nLink: https://example.com/manual\nSnippet: 검증할 자료입니다."}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	research := hintResearchRequest{Source: "web", Query: "내 메모에 저장한 장치의 최대 입력 전력", Anchor: "내 메모에서 장치 전력을 찾아줘", Observed: "내 메모에서 장치 전력을 찾아줘", SearchQuery: "장치 제조사 사용 설명서 입력 전력"}
	exec := toolruntime.ExecutionContext{EnableMemory: true}
	for _, source := range []string{"web", "memory"} {
		research.Source = source
		if _, _, err := loadHintEvidence(context.Background(), registry, exec, true, research); err != nil {
			t.Fatal(err)
		}
	}
	if len(queries) != 2 || queries[0] != research.SearchQuery || queries[1] != research.Query {
		t.Fatalf("search query replaced the task or leaked into memory: %v", queries)
	}
	research.Source, research.SearchQuery = "web", strings.Repeat("가", 301)
	if _, _, err := loadHintEvidence(context.Background(), registry, exec, true, research); err == nil || len(queries) != 2 {
		t.Fatal("oversized model query reached a tool")
	}
}

func TestHintPlanningHasNoTools(t *testing.T) {
	var upstream map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&upstream)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"HINT_SKIP\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"glasses_hint":true,"glasses_hint_phase":"plan","hint_research":{"source":"web","query":"should not search"},"model":"test","messages":[{"role":"user","content":"피라미드의 추가 정보 조사 계획"}],"stream":true}`))
	w := httptest.NewRecorder()
	handleChat(w, req, &App{llmEndpoint: server.URL, llmMode: "standard", enableTools: true}, &AuthManager{users: map[string]*User{}})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, key := range []string{"tools", "hint_research", "glasses_hint_phase", "glasses_hint"} {
		if _, ok := upstream[key]; ok {
			t.Fatalf("planning leaked %s", key)
		}
	}
}

func TestHintLiveWebEvidence(t *testing.T) {
	if os.Getenv("DKST_HINT_LIVE_WEB") != "1" {
		t.Skip("live web not requested")
	}
	if err := mcp.InitDB(filepath.Join(t.TempDir(), "hint-web.db")); err != nil {
		t.Fatal(err)
	}
	defer mcp.CloseDB()
	evidence, _, err := loadHintEvidence(context.Background(), toolruntime.Default, toolruntime.ExecutionContext{UserID: "hint-live", EnableMemory: true}, true, hintResearchRequest{Source: "web", Query: "Egyptian pyramids purpose royal tomb museum"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(evidence, "http") {
		t.Fatal("no verifiable source in web evidence")
	}
	if path := os.Getenv("DKST_HINT_EVIDENCE_OUT"); path != "" {
		if err := os.WriteFile(path, []byte(evidence), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("Live search evidence received: %d chars", len([]rune(evidence)))
}

func TestAutomaticGlassesRequestsIsolateGeneralChatAndPreserveStorage(t *testing.T) {
	if err := mcp.InitDB(filepath.Join(t.TempDir(), "glasses-context.db")); err != nil {
		t.Fatal(err)
	}
	defer mcp.CloseDB()
	memoryID, err := mcp.InsertMemory("glasses-user", "User: 피라미드 OLD_TV_MEMORY translation transcript\nAssistant: OLD_TRANSLATION_REPLY")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := `{"messages":[{"user_content":"OLD_WEB_CHAT","assistant_content":"OLD_WEB_REPLY"}]}`
	if _, err := mcp.UpsertChatSession(mcp.ChatSessionEntry{UserID: "glasses-user", SessionKey: "default", UIStateJSON: snapshot, LastResponseID: "resp_existing_web", Status: "idle"}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var incoming map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&incoming)
		raw, _ := json.Marshal(incoming)
		for _, bad := range []string{"OLD_TV_MEMORY", "OLD_TRANSLATION_REPLY", "OLD_WEB_CHAT", "OLD_WEB_REPLY", "previous_response_id", "glasses_client"} {
			if strings.Contains(string(raw), bad) {
				t.Errorf("unrelated data leaked: %s", bad)
			}
		}
		if !strings.Contains(string(raw), "GLASSES REQUEST CONTEXT") {
			t.Error("missing dedicated prompt profile")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"새 정보\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	yes := true
	strategy := "retrieval"
	auth := &AuthManager{users: map[string]*User{"glasses-user": {ID: "glasses-user", Settings: UserSettings{EnableMemory: &yes, ContextStrategy: &strategy}}}}
	app := &App{llmEndpoint: server.URL, llmMode: "standard", enableTools: true}
	for _, controls := range []map[string]interface{}{
		{"glasses_hint": true, "glasses_hint_phase": "plan"},
		{"glasses_hint": true, "glasses_hint_phase": "answer", "hint_research": map[string]string{"source": "internal", "query": "피라미드 건립 목적"}},
		{"glasses_hint": true, "glasses_hint_phase": "direct"},
	} {
		controls["model"] = "test"
		controls["messages"] = []map[string]string{{"role": "system", "content": "CURRENT_GLASSES_TASK"}, {"role": "user", "content": "피라미드 건립 목적을 알려줘"}}
		body, _ := json.Marshal(controls)
		req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(body)))
		req.Header.Set("X-User-ID", "glasses-user")
		w := httptest.NewRecorder()
		handleChat(w, req, app, auth)
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
	entry, err := mcp.GetCurrentChatSession("glasses-user")
	if err != nil {
		t.Fatal(err)
	}
	if entry.LastResponseID != "resp_existing_web" || entry.UIStateJSON != snapshot {
		t.Fatal("glasses request modified the web session")
	}
	if _, err := mcp.ReadMemory("glasses-user", memoryID); err != nil {
		t.Fatal("existing memory was changed or deleted")
	}
	memories, err := mcp.SearchMemoriesByRecent("glasses-user", 20)
	if err != nil || len(memories) != 1 {
		t.Fatalf("glasses response was auto-saved as memory: %d %v", len(memories), err)
	}
}

func TestHintFocusSelectionGroundedInTranscript(t *testing.T) {
	text := "資料は EBITDA を使うそうです。\n네, 알겠습니다.\n새 맥북 에어 M4 모델 얘기도 나왔어\n응 그렇구나."
	candidates := hintFocusCandidates(text)
	if len(candidates) != 2 {
		t.Fatalf("candidates: %#v", candidates)
	}
	reply := jevReply{Selected: "focus_1", Probabilities: map[string]float64{"focus_1": .7, "focus_0": .2, "none": .1}}
	if got := focusHintChoice(reply, candidates, "balanced"); got != candidates[1] {
		t.Fatalf("focus %q", got)
	}
	reply.Selected = "focus_99"
	if focusHintChoice(reply, candidates, "active") != "" {
		t.Fatal("out of range selection")
	}
	reply.Selected = "focus_1"
	reply.Probabilities["none"] = .8
	if focusHintChoice(reply, candidates, "balanced") != "" {
		t.Fatal("strong none must not become a topic")
	}
	reply.Probabilities["none"] = math.NaN()
	if focusHintChoice(reply, candidates, "active") != "" {
		t.Fatal("invalid scores")
	}
}

func TestGlassesHintFocusLocalJev(t *testing.T) {
	python, model := os.Getenv("DKST_JEV_TEST_PYTHON"), os.Getenv("DKST_JEV_TEST_MODEL")
	if python == "" || model == "" {
		t.Skip("local assets not specified")
	}
	worker, err := startJevEncoder(python, model, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer worker.Close()
	for _, text := range []string{"점심 먹었어.\n자료에 EBITDA가 나왔어.\n네, 알겠습니다.", "오늘도 수고했어.\n맥북 에어 M4 얘기야.\n응 그렇구나.", "점심 맛있었어.\n내일 만나자."} {
		candidates := hintFocusCandidates(text)
		criteria := map[string]string{"none": "맞장구·인사·단순 개인 일정뿐이며 공개 지식이나 확인할 정보가 없음"}
		for i, sentence := range candidates {
			criteria[fmt.Sprintf("focus_%d", i)] = sentence
		}
		reply, err := worker.request(map[string]any{"op": "choice", "text": text, "criteria": criteria, "instructions": hintFocusQuery + hintFocusInstructions}, 6*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		focus := focusHintChoice(reply, candidates, "balanced")
		if focus != "" {
			verified, err := worker.request(map[string]any{"op": "choice", "text": focus, "criteria": glassesBackgroundCriteria, "instructions": glassesBackgroundInstructions}, 6*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("verify selected=%s scores=%v", verified.Selected, verified.Probabilities)
			if !selectBackgroundHint(verified, "balanced").Hint {
				focus = ""
			}
		}
		if (strings.Contains(text, "EBITDA") || strings.Contains(text, "맥북")) && focus == "" {
			t.Error("missed specific topic")
		}
		if strings.Contains(text, "점심 맛있었어") && focus != "" {
			t.Error("ordinary conversation opened research")
		}
		t.Logf("focus text=%q selected=%s scores=%v focus=%q", text, reply.Selected, reply.Probabilities, focus)
	}
}

func TestGlassesHintShortTopicPipeline(t *testing.T) {
	for _, tc := range []struct {
		text  string
		shown []string
		want  string
	}{
		{"피라미드 얘기야\n응 그렇구나", nil, "피라미드 얘기야"},
		{"피라미드 얘기야\n청동기 시대", []string{"피라미드 얘기야"}, "청동기 시대"},
	} {
		calls := 0
		decision, err := decideLegacyGlassesHint(glassesHintRequest{Text: tc.text, Shown: tc.shown}, func(text string, criteria map[string]string, instructions string) (jevReply, error) {
			calls++
			if _, focus := criteria["focus_0"]; focus {
				t.Fatal("filler/shown spans should have been removed before choice")
			}
			if text != tc.want {
				t.Fatalf("classifier got %q want focused %q", text, tc.want)
			}
			return jevReply{Selected: "background", Probabilities: map[string]float64{"background": .8, "none": .1}}, nil
		})
		if err != nil || !decision.Hint || decision.Focus != tc.want || calls != 1 {
			t.Fatalf("%+v calls=%d err=%v", decision, calls, err)
		}
	}
	long := "피라미드 " + strings.Repeat("이런 건축물이 있다고 하더라고 ", 20)
	if candidates := hintFocusCandidates(long); len(candidates) == 0 || !strings.Contains(candidates[0], "피라미드") {
		t.Fatal("long utterance silently lost its topic")
	}
}

func TestGlassesHintShortTopicLocalJev(t *testing.T) {
	python, model := os.Getenv("DKST_JEV_TEST_PYTHON"), os.Getenv("DKST_JEV_TEST_MODEL")
	if python == "" || model == "" {
		t.Skip("local assets not specified")
	}
	worker, err := startJevEncoder(python, model, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer worker.Close()
	for _, tc := range []struct {
		text  string
		shown []string
		want  bool
	}{
		{"피라미드", nil, true}, {"청동기 시대", nil, true},
		{"자료에 EBITDA가 나왔어\n네, 알겠습니다", nil, true},
		{"피라미드 얘기야\n맥북 에어 M4 무게 얘기야", []string{"피라미드 얘기야"}, true},
		{"점심 맛있었어\n내일 만나자", nil, false},
	} {
		start := time.Now()
		calls := 0
		d, err := decideGlassesHint(glassesHintRequest{Text: tc.text, Shown: tc.shown}, func(text string, criteria map[string]string, instructions string) (jevReply, error) {
			calls++
			return worker.request(map[string]any{"op": "choice", "text": text, "criteria": criteria, "instructions": instructions}, 5*time.Second)
		})
		t.Logf("text=%q hint=%v focus=%q category=%s calls=%d elapsed=%v", tc.text, d.Hint, d.Focus, d.Category, calls, time.Since(start))
		if err != nil || d.Hint != tc.want {
			t.Errorf("decision=%+v err=%v", d, err)
		}
	}
}

func TestGlassesHintContextAndFillers(t *testing.T) {
	for _, text := range []string{"아 진짜?", "수고하셨습니다!", "잠시만요.", "hmm", "thanks"} {
		if !hintFiller(text) {
			t.Fatalf("expected filler: %q", text)
		}
	}
	var seenInput string
	_, err := decideGlassesHint(glassesHintRequest{
		Text:    "그거 무게가 얼마나 나가지?",
		Context: "맥북 에어 M4 얘기 중이었어",
	}, func(text string, criteria map[string]string, instructions string) (jevReply, error) {
		seenInput = text
		return jevReply{Selected: "check", Probabilities: map[string]float64{"check": 0.7, "none": 0.3}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seenInput, "맥북 에어 M4") || !strings.Contains(seenInput, "그거 무게가") {
		t.Fatalf("expected prior context to be included for referential utterance, got %q", seenInput)
	}
}
