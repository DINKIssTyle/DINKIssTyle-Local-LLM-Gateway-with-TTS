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
		if _, ok := body["tools"]; ok || !strings.Contains(string(raw), "selection:") {
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
	if err != nil || proof.Mode != "quote" || !strings.Contains(body, "selection:") || calls != 2 {
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

func TestHintGroundingConversationNeedsNoSearchOrDictionary(t *testing.T) {
	req := hintResearchRequest{RequireGrounding: true, Source: "conversation", Kind: "schedule", Topic: "회의 준비", Observed: "금요일 오후 3시에 회의합니다. 장소는 2층입니다. 노트북을 가져오세요.", Anchor: "금요일 오후 3시"}
	proof, prompt, err := prepareHintGrounding(context.Background(), toolruntime.NewRegistry(), toolruntime.ExecutionContext{}, false, req)
	if err != nil || proof.Mode != "conversation" || proof.Kind != "schedule" || proof.Records[0].Text != req.Observed || !strings.Contains(prompt, "points") {
		t.Fatalf("%+v %v", proof, err)
	}
	req.Kind = "term"
	if _, _, err = prepareHintGrounding(context.Background(), toolruntime.NewRegistry(), toolruntime.ExecutionContext{}, false, req); err == nil {
		t.Fatal("term verification bypassed through summary path")
	}
	req.Kind = "recap"
	req.Anchor = "없는 구절"
	if _, _, err = prepareHintGrounding(context.Background(), toolruntime.NewRegistry(), toolruntime.ExecutionContext{}, false, req); err == nil {
		t.Fatal("unobserved summary anchor accepted")
	}
}

func TestHintGroundingConversationAPIKeepsOperationAndRawEvidence(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		raw, _ := json.Marshal(body)
		if _, ok := body["tools"]; ok || !strings.Contains(string(raw), "장소는 2층") || !strings.Contains(string(raw), "Conversation source data") || !strings.Contains(string(raw), "type=summary/schedule/check") || strings.Contains(string(raw), "rather than summarize the transcript") {
			t.Errorf("summary writer lost source or gained tools: %s", raw)
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"HINT_SKIP\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"glasses_hint":true,"glasses_hint_phase":"answer","hint_research":{"require_grounding":true,"source":"conversation","topic":"회의 준비","kind":"schedule","observed":"금요일 오후 3시에 회의합니다. 장소는 2층입니다.","anchor":"금요일 오후 3시","allow_web":false},"model":"small","messages":[{"role":"user","content":"일정 정리"}],"stream":true}`))
	w := httptest.NewRecorder()
	handleChat(w, req, &App{llmEndpoint: upstream.URL, llmMode: "standard"}, &AuthManager{users: map[string]*User{}})
	if w.Code != 200 || calls != 1 || !strings.Contains(w.Body.String(), `"mode":"conversation"`) || !strings.Contains(w.Body.String(), `"kind":"schedule"`) {
		t.Fatalf("%d calls=%d %s", w.Code, calls, w.Body.String())
	}
}

func TestHintQuoteSpansAndTaskFirstSelection(t *testing.T) {
	text := "첫 번째 문장은 실제 자료에서 읽은 문장입니다. 수치는 1.5미터로 표현되어 있습니다. 마지막 설명도 실제 자료의 일부입니다."
	spans := hintQuoteSpans(text)
	if len(spans) != 3 || !strings.Contains(spans[1], "1.5미터") {
		t.Fatalf("source sentence boundaries: %#v", spans)
	}
	for _, span := range spans {
		if !strings.Contains(text, span) {
			t.Fatal("source text was rewritten")
		}
	}
	registry := toolruntime.NewRegistry()
	err := registry.Register(toolruntime.Definition{Name: "search_web", InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`)}, func(context.Context, toolruntime.ExecutionContext, json.RawMessage) (toolruntime.Result, error) {
		return toolruntime.Result{Content: "Title: 물리학의 고고학 활용\nLink: https://example.com/archaeology\nSnippet: 고고학에서는 물리학적 측정으로 시료의 특성을 비교합니다."}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	query := "고고학에서 물리학이 활용되는 구체적인 사례"
	proof, instructions, err := prepareHintGrounding(context.Background(), registry, toolruntime.ExecutionContext{}, true, hintResearchRequest{Source: "internal", Kind: "term", Topic: "물리학", Query: query, Observed: "고고학에서는 물리학을 활용합니다", Anchor: "물리학", AllowWeb: true})
	if err != nil || proof.Task != query || len(proof.Records) != 1 || len(proof.Records[0].Spans) != 1 {
		t.Fatalf("missing numbered source proof: %+v %v", proof, err)
	}
	for _, wanted := range []string{query, "selection:", "an integer choice number", "generic definition is not an answer", "Topic overlap alone is not enough"} {
		if !strings.Contains(instructions, wanted) {
			t.Errorf("task-first selector instruction missing: %s", wanted)
		}
	}
	if strings.Contains(instructions, `"support_quote":"`) {
		t.Fatal("selector still asks the model to reconstruct quotes")
	}
}

func TestHintSelectorReplacesConflictingWriterFormat(t *testing.T) {
	proof := hintGrounding{Version: 1, Mode: "quote", Topic: "물리학", Source: "web", Records: []hintSourceRecord{{Title: "활용", URL: "https://example.com/a", Text: "검증용 자료에서 선택할 문장입니다.", Spans: []string{"검증용 자료에서 선택할 문장입니다."}}}}
	req := hintResearchRequest{Query: "고고학에서 물리학을 활용하는 사례", Observed: "고고학에서는 물리학을 이용합니다", Kind: "term"}
	for _, native := range []bool{false, true} {
		body := map[string]any{"model": "role-model", "stream": true, "messages": []map[string]string{{"role": "system", "content": "OLD_FREE_FORM_FACT_INSTRUCTION"}, {"role": "user", "content": "old task"}}}
		if native {
			delete(body, "messages")
			body["input"] = "old task"
			body["system_prompt"] = "OLD_FREE_FORM_FACT_INSTRUCTION"
		}
		raw, _ := json.Marshal(body)
		updated, err := prepareHintSelectionBody(raw, proof, req)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(updated), "OLD_FREE_FORM_FACT_INSTRUCTION") || !strings.Contains(string(updated), req.Query) || !strings.Contains(string(updated), "selection:") {
			t.Fatalf("selector still conflicts with old format: %s", updated)
		}
		var payload map[string]any
		_ = json.Unmarshal(updated, &payload)
		if payload["model"] != "role-model" || payload["stream"] != true {
			t.Fatal("model/transport routing changed")
		}
		if !strings.Contains(string(updated), proof.Records[0].URL) {
			t.Fatal("source URL missing from quality review input")
		}
		if native {
			if payload["max_output_tokens"] != float64(1024) {
				t.Fatal("Native selector budget missing")
			}
		} else if payload["max_tokens"] != float64(1024) {
			t.Fatal("standard selector budget missing")
		}
	}
}

func TestContextualHintSearchUsesTaskNotLiteralCueName(t *testing.T) {
	registry := toolruntime.NewRegistry()
	calls := 0
	query := "현재 한국 극장 관객 수 순위와 집계 기준일"
	err := registry.Register(toolruntime.Definition{Name: "search_web", InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`)}, func(_ context.Context, _ toolruntime.ExecutionContext, args json.RawMessage) (toolruntime.Result, error) {
		calls++
		var input map[string]string
		_ = json.Unmarshal(args, &input)
		if calls == 1 && input["query"] != query {
			t.Errorf("self-contained task changed into keyword search: %q", input["query"])
		}
		return toolruntime.Result{Content: "Title: 가상 일별 관객 수 집계\nLink: https://example.com/chart\nSnippet: 가상 기준일의 국내 극장 관객 수 통계입니다. 가상 작품 A가 관객 수 집계에서 첫 번째입니다."}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	req := hintResearchRequest{TaskOriented: true, RequireGrounding: true, Source: "web", Topic: query, Kind: "key", Query: query, Observed: "주말에 극장에서 영화 볼까 해요", Anchor: "영화", AllowWeb: true}
	proof, _, err := prepareHintGrounding(context.Background(), registry, toolruntime.ExecutionContext{}, true, req)
	if err != nil || !proof.TaskOriented || proof.Mode != "quote" || len(proof.Records) != 1 {
		t.Fatalf("task-based chart lookup failed: %+v %v", proof, err)
	}
	for _, native := range []bool{false, true} {
		payload := map[string]any{"model": "role", "stream": true, "messages": []map[string]string{{"role": "user", "content": "old"}}}
		if native {
			delete(payload, "messages")
			payload["input"] = "old"
		}
		raw, _ := json.Marshal(payload)
		selected, err := prepareHintSelectionBody(raw, proof, req)
		if err != nil || !strings.Contains(string(selected), "Booking rate and admissions are different") || !strings.Contains(string(selected), "local sentence numbers") {
			t.Fatalf("context selection policy lost: %s %v", selected, err)
		}
	}
	req.Topic = "새로운 정보 과제"
	req.Anchor = "피라미드" // Fabricated evidence cannot override actual speech.
	if _, _, err = prepareHintGrounding(context.Background(), registry, toolruntime.ExecutionContext{}, true, req); err == nil || calls != 1 {
		t.Fatal("unrelated old topic bypassed conversation anchoring")
	}
	req.Topic = "영화"
	req.Anchor = "영화"
	req.TaskOriented = false
	if _, _, err = prepareHintGrounding(context.Background(), registry, toolruntime.ExecutionContext{}, true, req); err == nil {
		t.Fatal("legacy proper-name verification was relaxed")
	}
}

func TestContextTaskStableInternalAndFreshLookup(t *testing.T) {
	req := hintResearchRequest{TaskOriented: true, RequireGrounding: true, Source: "internal", Topic: "손잡이 원리", Kind: "key", Query: "문 손잡이가 힘을 줄여주는 원리", Observed: "문 손잡이가 돌아가는 이유가 궁금해요", Anchor: "문 손잡이", AllowWeb: false}
	proof, _, err := prepareHintGrounding(context.Background(), toolruntime.NewRegistry(), toolruntime.ExecutionContext{}, false, req)
	if err != nil || proof.Mode != "known" || !proof.TaskOriented {
		t.Fatalf("stable internal help gated by dictionary: %+v %v", proof, err)
	}
	req.Query = "현재 한국 박스오피스 관객 수 순위"
	req.Topic = req.Query
	if _, _, err = prepareHintGrounding(context.Background(), toolruntime.NewRegistry(), toolruntime.ExecutionContext{}, false, req); err == nil {
		t.Fatal("current rankings must not be generated from internal knowledge without a source")
	}
	req.Query = "특정 생소한 단어의 정의"
	req.Kind = "term"
	if _, _, err = prepareHintGrounding(context.Background(), toolruntime.NewRegistry(), toolruntime.ExecutionContext{}, false, req); err == nil {
		t.Fatal("uncertain definitions bypassed source lookup")
	}
}
