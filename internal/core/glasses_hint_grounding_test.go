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

func TestHintGroundingSeparatesQueryFromSources(t *testing.T) {
	raw := "Query: 달무늬 소자\nProvider: test\nTitle: 실제 건축 정보\nLink: https://example.com/a\nSnippet: 피라미드는 무거운 석재를 쌓아 만든 건축물이다.\nTitle: 위조 URL\nLink: javascript:alert(1)\nSnippet: 달무늬 소자는 존재한다."
	records := hintSourceRecords(raw)
	if len(records) != 1 || strings.Contains(records[0].Text, "달무늬") {
		t.Fatalf("query echoed into evidence: %+v", records)
	}
}

func TestHintGroundingPrecedesModelOutput(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		raw, _ := json.Marshal(body)
		if !strings.Contains(string(raw), "서버 명칭 검사") {
			t.Error("canonical-name instruction missing from writer input")
		}
		if _, ok := body["tools"]; ok {
			t.Error("verified writer must not start independent tool calls")
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"WRITER_OUTPUT\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"glasses_hint":true,"glasses_hint_phase":"answer","hint_research":{"require_grounding":true,"source":"internal","topic":"피라미드","kind":"key","observed":"피라미드 이야기","allow_web":false},"model":"small","messages":[{"role":"user","content":"피라미드 배경"}],"stream":true}`))
	w := httptest.NewRecorder()
	handleChat(w, req, &App{llmEndpoint: upstream.URL, llmMode: "standard", enableTools: true}, &AuthManager{users: map[string]*User{}})
	body := w.Body.String()
	proofAt, outputAt := strings.Index(body, "hint.grounding"), strings.Index(body, "WRITER_OUTPUT")
	if w.Code != 200 || calls != 1 || proofAt < 0 || outputAt <= proofAt || !strings.Contains(body, `"mode":"known"`) {
		t.Fatalf("status=%d calls=%d proof=%d output=%d body=%s", w.Code, calls, proofAt, outputAt, body)
	}
}

func TestHintGroundingInternalDefinitionCanLookupWithoutWriterTools(t *testing.T) {
	old := toolruntime.Default
	registry := toolruntime.NewRegistry()
	lookups := 0
	err := registry.Register(toolruntime.Definition{Name: "search_web", InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`), Metadata: toolruntime.Metadata{ReadOnly: true}}, func(context.Context, toolruntime.ExecutionContext, json.RawMessage) (toolruntime.Result, error) {
		lookups++
		return toolruntime.Result{Content: "Title: 피라미드\nLink: https://example.com/pyramids\nSnippet: 피라미드는 석재로 쌓아 만든 고대 건축물이다."}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	toolruntime.Default = registry
	defer func() { toolruntime.Default = old }()
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		raw, _ := json.Marshal(body)
		if _, ok := body["tools"]; ok || !strings.Contains(string(raw), "원문 인용 모드") {
			t.Error("writer must use fetched records without separate tools")
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"HINT_SKIP\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"glasses_hint":true,"glasses_hint_phase":"answer","hint_research":{"require_grounding":true,"source":"internal","topic":"피라미드","query":"피라미드 뜻","kind":"term","observed":"피라미드 이야기","allow_web":true},"model":"small","messages":[{"role":"user","content":"피라미드 뜻"}],"stream":true}`))
	w := httptest.NewRecorder()
	handleChat(w, req, &App{llmEndpoint: upstream.URL, llmMode: "standard", enableTools: true}, &AuthManager{users: map[string]*User{}})
	if w.Code != 200 || lookups != 1 || calls != 1 || !strings.Contains(w.Body.String(), `"mode":"quote"`) {
		t.Fatalf("status=%d lookups=%d calls=%d: %s", w.Code, lookups, calls, w.Body.String())
	}
}
func TestHintGroundingUnknownNamesRequireEvidence(t *testing.T) {
	registry := toolruntime.NewRegistry()
	calls := 0
	err := registry.Register(toolruntime.Definition{Name: "search_web", InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`), Metadata: toolruntime.Metadata{ReadOnly: true}}, func(context.Context, toolruntime.ExecutionContext, json.RawMessage) (toolruntime.Result, error) {
		calls++
		return toolruntime.Result{Content: "Query: 달무늬 소자\nTitle: 피라미드 건축\nLink: https://example.com/pyramids\nSnippet: 피라미드는 무거운 석재를 쌓아 만든 건축물이다."}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	req := hintResearchRequest{RequireGrounding: true, Topic: "달무늬 소자", Source: "internal", Kind: "key", Observed: "달무늬 소자가 나왔대", Anchor: "달무늬 소자", Query: "달무늬 소자의 의미", AllowWeb: true}
	proof, _, err := prepareHintGrounding(context.Background(), registry, toolruntime.ExecutionContext{}, true, req)
	if err == nil || calls != 1 {
		t.Fatalf("%+v calls=%d err=%v", proof, calls, err)
	}
	if strings.Contains(proof.Records[0].Text, "달무늬 소자") {
		t.Fatal("search query mistaken for source evidence")
	}
	req.AllowWeb = false
	if _, _, err = prepareHintGrounding(context.Background(), registry, toolruntime.ExecutionContext{}, true, req); err == nil {
		t.Fatal("unknown name bypassed disabled web setting")
	}
	req.AllowWeb = true
	req.Observed = "검은 옷을 입은 사람들이 줄을 섰다"
	req.Anchor = "검은 옷"
	if _, _, err = prepareHintGrounding(context.Background(), registry, toolruntime.ExecutionContext{}, true, req); err == nil {
		t.Fatal("old unrelated unknown topic carried into new observation")
	}
	if calls != 1 {
		t.Fatal("unverified old topic should be rejected before search")
	}
	req.Topic = "피라미드"
	req.Source = "web"
	proof, body, err := prepareHintGrounding(context.Background(), registry, toolruntime.ExecutionContext{}, true, req)
	if err != nil || proof.Mode != "quote" || !strings.Contains(body, "원문 인용 모드") || calls != 2 {
		t.Fatalf("real source name: %+v %v calls=%d", proof, err, calls)
	}
	req.Source = "internal"
	req.Kind = "key"
	proof, _, err = prepareHintGrounding(context.Background(), registry, toolruntime.ExecutionContext{}, false, req)
	if err != nil || proof.Mode != "known" || proof.Topic == "" || calls != 2 {
		t.Fatalf("known background should remain fast: %+v %v", proof, err)
	}
	req.Kind = "term"
	if _, _, err = prepareHintGrounding(context.Background(), registry, toolruntime.ExecutionContext{}, false, req); err == nil {
		t.Fatal("term definition without lookup evidence accepted")
	}
}
