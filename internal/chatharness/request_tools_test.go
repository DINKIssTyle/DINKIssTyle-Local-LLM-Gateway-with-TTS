package chatharness

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"dinkisstyle-chat/internal/promptkit"
	"dinkisstyle-chat/internal/toolruntime"
)

func testToolDefinitions() []promptkit.ToolDefinition {
	return []promptkit.ToolDefinition{{
		Name:        "get_current_time",
		Description: "Get current time",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}}
}

func allAppToolDefinitions() []promptkit.ToolDefinition {
	definitions := toolruntime.Default.List(toolruntime.ExecutionContext{EnableMemory: true})
	tools := make([]promptkit.ToolDefinition, 0, len(definitions))
	for _, definition := range definitions {
		tools = append(tools, promptkit.ToolDefinition{
			Name:        definition.Name,
			Description: definition.Description,
			InputSchema: definition.InputSchema,
		})
	}
	return tools
}

func TestPrepareRequestAddsNativeToolsForOpenAICompatibleMode(t *testing.T) {
	prepared, err := PrepareRequest(RequestInput{
		Body:        []byte(`{"model":"test","messages":[{"role":"user","content":"time?"}],"tools":[{"type":"function","function":{"name":"unsafe"}}],"stream":true}`),
		LLMMode:     "standard",
		EnableTools: true,
		Tools:       testToolDefinitions(),
	})
	if err != nil {
		t.Fatal(err)
	}
	tools, ok := prepared.ReqMap["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("native tools not added: %#v", prepared.ReqMap["tools"])
	}
	function := tools[0].(map[string]interface{})["function"].(map[string]interface{})
	if function["name"] != "get_current_time" {
		t.Fatalf("caller-supplied tool was not replaced by app catalog: %#v", function)
	}
	if _, exists := prepared.ReqMap["integrations"]; exists {
		t.Fatal("legacy MCP integration was added")
	}
	if prepared.ReqMap["tool_choice"] != "auto" {
		t.Fatalf("unexpected tool_choice: %#v", prepared.ReqMap["tool_choice"])
	}
	if prepared.ReqMap["parallel_tool_calls"] != false {
		t.Fatalf("parallel tool calls were not disabled: %#v", prepared.ReqMap["parallel_tool_calls"])
	}
}

func TestPrepareRequestUsesPromptToolsForLMStudioStatefulMode(t *testing.T) {
	prepared, err := PrepareRequest(RequestInput{
		Body:        []byte(`{"model":"test","input":"time?","system_prompt":"You are helpful.","integrations":["mcp/dinkisstyle-gateway"],"stream":true}`),
		LLMMode:     "stateful",
		EnableTools: true,
		Tools:       testToolDefinitions(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := prepared.ReqMap["integrations"]; exists {
		t.Fatal("legacy MCP integration survived request preparation")
	}
	if _, exists := prepared.ReqMap["tools"]; exists {
		t.Fatal("LM Studio native /api/v1/chat received unsupported custom tools")
	}
	systemPrompt, _ := prepared.ReqMap["system_prompt"].(string)
	if !strings.Contains(systemPrompt, "AVAILABLE APP TOOLS") || !strings.Contains(systemPrompt, "get_current_time") {
		t.Fatalf("app tool catalog missing from LM Studio prompt: %q", systemPrompt)
	}
}

func TestEveryAppToolIsAdvertisedInBothProviderModes(t *testing.T) {
	definitions := allAppToolDefinitions()
	if len(definitions) == 0 {
		t.Fatal("app tool catalog is empty")
	}

	standard, err := PrepareRequest(RequestInput{
		Body:        []byte(`{"model":"test","messages":[{"role":"user","content":"test"}],"stream":true}`),
		LLMMode:     "standard",
		EnableTools: true,
		Tools:       definitions,
	})
	if err != nil {
		t.Fatal(err)
	}
	nativeTools, _ := standard.ReqMap["tools"].([]interface{})
	if len(nativeTools) != len(definitions) {
		t.Fatalf("standard mode advertised %d tools, want %d", len(nativeTools), len(definitions))
	}
	if standard.ReqMap["max_tokens"] != defaultToolTurnMaxTokens {
		t.Fatalf("native tool turn output budget missing: %#v", standard.ReqMap["max_tokens"])
	}
	nativeNames := make(map[string]bool, len(nativeTools))
	for _, rawTool := range nativeTools {
		tool, _ := rawTool.(map[string]interface{})
		function, _ := tool["function"].(map[string]interface{})
		name, _ := function["name"].(string)
		if name == "" || function["parameters"] == nil {
			t.Fatalf("invalid native tool definition: %#v", rawTool)
		}
		nativeNames[name] = true
	}

	stateful, err := PrepareRequest(RequestInput{
		Body:        []byte(`{"model":"test","input":"test","system_prompt":"You are helpful.","stream":true}`),
		LLMMode:     "stateful",
		EnableTools: true,
		Tools:       definitions,
	})
	if err != nil {
		t.Fatal(err)
	}
	systemPrompt, _ := stateful.ReqMap["system_prompt"].(string)
	for _, definition := range definitions {
		if !nativeNames[definition.Name] {
			t.Errorf("standard mode omitted %s", definition.Name)
		}
		if !strings.Contains(systemPrompt, "- "+definition.Name+":") {
			t.Errorf("stateful mode omitted %s", definition.Name)
		}
	}
}

func TestPrepareRequestRemovesLegacyIntegrationWhenToolsDisabled(t *testing.T) {
	prepared, err := PrepareRequest(RequestInput{
		Body:    []byte(`{"model":"test","messages":[{"role":"user","content":"hello"}],"integrations":["mcp/dinkisstyle-gateway"],"tools":[{"type":"function","function":{"name":"unsafe"}}],"tool_choice":"auto","stream":true}`),
		LLMMode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := prepared.ReqMap["integrations"]; exists {
		t.Fatal("legacy integration survived while app tools were disabled")
	}
	if strings.Contains(string(prepared.Body), "dinkisstyle-gateway") {
		t.Fatalf("legacy integration remained in prepared body: %s", prepared.Body)
	}
	for _, key := range []string{"tools", "tool_choice", "parallel_tool_calls", "functions", "function_call"} {
		if _, exists := prepared.ReqMap[key]; exists {
			t.Fatalf("caller-supplied provider control %q survived while app tools were disabled", key)
		}
	}
}

func TestPrepareRequestInjectsSkillsWhenToolsAndMemoryAreDisabled(t *testing.T) {
	const skillPrompt = "\n\n### ACTIVE SKILLS ###\n#### builtin:weather\nRead current weather.\n### END ACTIVE SKILLS ###\n"
	prepared, err := PrepareRequest(RequestInput{
		Body:              []byte(`{"model":"test","messages":[{"role":"user","content":"weather"}],"stream":true}`),
		LLMMode:           "standard",
		SkillInstructions: skillPrompt,
	})
	if err != nil {
		t.Fatal(err)
	}
	messages, _ := prepared.ReqMap["messages"].([]interface{})
	if len(messages) < 2 {
		t.Fatalf("system message was not injected: %#v", messages)
	}
	system, _ := messages[0].(map[string]interface{})["content"].(string)
	if !strings.Contains(system, "builtin:weather") || !prepared.InjectedPrompt {
		t.Fatalf("skill prompt was not injected: %q", system)
	}
	if _, exists := prepared.ReqMap["tools"]; exists {
		t.Fatal("skill injection granted tools while tools were disabled")
	}
}

func TestPrepareRequestInjectsCompactPolicyForSimpleRewriteWithoutTools(t *testing.T) {
	prepared, err := PrepareRequest(RequestInput{
		Body:    []byte(`{"model":"test","messages":[{"role":"user","content":"이 문장을 정중하고 간결하게 다듬어 주세요."}],"stream":true}`),
		LLMMode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	messages, _ := prepared.ReqMap["messages"].([]interface{})
	if len(messages) < 2 {
		t.Fatalf("compact system policy was not injected: %#v", messages)
	}
	system, _ := messages[0].(map[string]interface{})["content"].(string)
	if !strings.Contains(system, "COMPACT TASK OUTPUT") || !strings.Contains(system, "Return only the requested result") {
		t.Fatalf("compact output policy is missing: %q", system)
	}
	if _, exists := prepared.ReqMap["tools"]; exists {
		t.Fatal("compact output policy granted tools")
	}
}

func TestPrepareRequestDoesNotInjectCompactPolicyForOrdinaryConversation(t *testing.T) {
	prepared, err := PrepareRequest(RequestInput{
		Body:    []byte(`{"model":"test","messages":[{"role":"user","content":"비 오는 날 고양이 이야기를 두 문장으로 써 주세요."}],"stream":true}`),
		LLMMode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.InjectedPrompt || strings.Contains(string(prepared.Body), "COMPACT TASK OUTPUT") {
		t.Fatalf("ordinary conversation received compact transformation policy: %s", prepared.Body)
	}
}

func TestPrepareRequestInjectsNewSkillOnStatefulFollowup(t *testing.T) {
	prepared, err := PrepareRequest(RequestInput{
		Body:              []byte(`{"model":"test","input":"현재 날씨","system_prompt":"Base","previous_response_id":"resp_123","stream":true}`),
		LLMMode:           "stateful",
		ContextStrategy:   "stateful",
		EnableTools:       true,
		SkillInstructions: "\n\n### ACTIVE SKILLS ###\nWeather workflow.\n### END ACTIVE SKILLS ###\n",
		Tools:             testToolDefinitions(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.IsStatefulFollowup {
		t.Fatal("request was not recognized as a stateful follow-up")
	}
	system, _ := prepared.ReqMap["system_prompt"].(string)
	if !strings.Contains(system, "Weather workflow") {
		t.Fatalf("skill was not injected on stateful follow-up: %q", system)
	}
	if strings.Contains(system, "AVAILABLE APP TOOLS") {
		t.Fatalf("transient tool catalog was redundantly injected on a stateful follow-up: %q", system)
	}
}

func TestPrepareToolFollowupPreservesChatCompletionToolCallID(t *testing.T) {
	req, _, err := PrepareToolFollowupRequest(ToolFollowupInput{
		LLMMode:          "standard",
		ModelID:          "test",
		ToolName:         "get_current_time",
		ToolArguments:    `{}`,
		ToolCallID:       "call_123",
		ToolResult:       "now",
		OriginalUserText: "현재 시간은?",
		ReqMap: map[string]interface{}{
			"model":    "test",
			"messages": []interface{}{map[string]interface{}{"role": "user", "content": "time?"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	messages := req["messages"].([]interface{})
	toolMessage := messages[len(messages)-1].(map[string]interface{})
	if toolMessage["role"] != "tool" || toolMessage["tool_call_id"] != "call_123" {
		t.Fatalf("tool result linkage was not preserved: %#v", toolMessage)
	}
}

func TestCompactToolResultPreservesOriginalKoreanLanguage(t *testing.T) {
	result := CompactToolResult("execute_command", "boot time: today", "현재 시스템 부트업 시간")
	if !strings.Contains(result, "한국어로 답하세요") || !strings.Contains(result, "영어로 전환하지 마세요") {
		t.Fatalf("Korean response-language guard is missing: %q", result)
	}
	if !strings.Contains(result, "NOT A USER MESSAGE") {
		t.Fatalf("tool-result boundary marker is missing: %q", result)
	}
}

func TestShouldFinalizeAfterWebSearch(t *testing.T) {
	directResult := "Search Guidance\nRecommended Next Action: answer_from_search_if_sufficient\n---\nTitle: News"
	if !ShouldFinalizeAfterWebSearch("search_web", directResult, false, false) {
		t.Fatal("ordinary successful web search did not request a final answer")
	}
	pageReadResult := "Search Guidance\nRecommended Next Action: read_top_result_if_more_detail_is_needed"
	if ShouldFinalizeAfterWebSearch("search_web", pageReadResult, false, false) {
		t.Fatal("search result that recommends a page read was finalized too early")
	}
	if ShouldFinalizeAfterWebSearch("search_web", directResult, true, false) {
		t.Fatal("deep research was finalized after its first search")
	}
	if !ShouldFinalizeAfterWebSearch("search_web", "No results found or parsing failed.", false, false) {
		t.Fatal("ordinary failed search was allowed to repeat instead of reporting insufficient evidence")
	}
	if !ShouldFinalizeAfterWebSearch("read_buffered_source", "Relevant Excerpts:\nverified details", false, false) {
		t.Fatal("focused buffered-source read did not force the final answer")
	}
	if ShouldFinalizeAfterWebSearch("read_buffered_source", "Evidence Quality Warning: no_authoritative_or_reputable_source", false, false) {
		t.Fatal("weak buffered evidence prevented an authoritative refinement search")
	}
	if ShouldFinalizeAfterWebSearch("search_web", "Recommended Next Action: refine_search_for_authoritative_source", false, false) {
		t.Fatal("weak search evidence was finalized before authoritative refinement")
	}
	if ShouldFinalizeAfterWebSearch("read_web_page", "Buffered Web Source Saved", false, false) {
		t.Fatal("page buffering was finalized before the focused source read")
	}
}

func TestFailedWebSearchWithoutEvidenceIsFailClosed(t *testing.T) {
	providerErr := errors.New("all web search providers failed")
	if !ShouldFailClosedWebSearch("search_web", providerErr, 0) {
		t.Fatal("failed search without evidence was allowed to continue into an LLM answer")
	}
	if ShouldFailClosedWebSearch("search_web_multi", providerErr, 1) {
		t.Fatal("partial web evidence was incorrectly discarded")
	}
	if ShouldFailClosedWebSearch("get_current_time", providerErr, 0) {
		t.Fatal("non-web tool error was classified as a failed web search")
	}

	answer := BuildWebSearchFailureAnswer("현재 미국 뉴스", providerErr.Error())
	for _, expected := range []string{"실시간 웹 검색에 실패", "내부 지식으로 최신 정보를 대신 작성하지 않았습니다", providerErr.Error()} {
		if !strings.Contains(answer, expected) {
			t.Fatalf("fail-closed answer omitted %q:\n%s", expected, answer)
		}
	}
	for _, forbidden := range []string{"검증된 사실", "Verified Facts", "2024년 대선"} {
		if strings.Contains(answer, forbidden) {
			t.Fatalf("fail-closed answer contained unsupported claim %q:\n%s", forbidden, answer)
		}
	}
}

func TestContextualFollowupAndMissingSearchQueryRecovery(t *testing.T) {
	for _, input := range []string{"자녀들 정보", "탐크루즈 자녀요", "What about his children?"} {
		if !IsLikelyContextualFollowup(input) {
			t.Fatalf("contextual follow-up was not recognized: %q", input)
		}
	}
	if IsLikelyContextualFollowup("스페인 난민 사태의 최신 뉴스를 자세히 조사해 주세요") {
		t.Fatal("standalone research request was classified as an elliptical follow-up")
	}

	recent := "Recent Turn 1\nUser: 탐크루즈 생년월일\nAssistant: 톰 크루즈는 1962년 7월 3일생입니다."
	repaired, ok := RepairMissingSearchToolArguments("search_web", `{}`, "자녀들 정보", recent)
	if !ok {
		t.Fatal("missing search query was not repaired")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(repaired), &payload); err != nil {
		t.Fatal(err)
	}
	query, _ := payload["query"].(string)
	if !strings.Contains(query, "탐크루즈 생년월일") || !strings.Contains(query, "자녀들 정보") {
		t.Fatalf("repaired query lost conversational subject: %q", query)
	}
	unchanged, repairedExisting := RepairMissingSearchToolArguments("search_web", `{"query":"Tom Cruise children"}`, "자녀들 정보", recent)
	if repairedExisting || unchanged != `{"query":"Tom Cruise children"}` {
		t.Fatalf("valid model arguments were overwritten: %q", unchanged)
	}
	refined, refinedFamily := RefineFamilySearchToolArguments("search_web", unchanged, "자녀들 정보")
	if !refinedFamily || !strings.Contains(refined, "adopted biological children relationship") {
		t.Fatalf("family search did not verify relationship type: %q", refined)
	}
	if secondPass, changed := RefineFamilySearchToolArguments("search_web", refined, "자녀들 정보"); changed || secondPass != refined {
		t.Fatalf("family search refinement was not idempotent: %q", secondPass)
	}
	standalone, repairedStandalone := RepairMissingSearchToolArguments("search_web", `{}`, "훈민정음의 창제 원리 검색", recent)
	if !repairedStandalone || strings.Contains(standalone, "탐크루즈") {
		t.Fatalf("standalone request was contaminated by the prior subject: %q", standalone)
	}

	memRepaired, memOk := RepairMissingSearchToolArguments("search_memory", `{}`, "당신의 이름 확인", recent)
	if !memOk || !strings.Contains(memRepaired, `"query":"당신의 이름 확인"`) {
		t.Fatalf("search_memory missing query was not repaired: %q", memRepaired)
	}

	factRepaired, factOk := RepairMissingSearchToolArguments("save_user_fact", `{}`, "나를 주인님이라고 부르기로 한거", recent)
	if !factOk || !strings.Contains(factRepaired, `"fact_value":"나를 주인님이라고 부르기로 한거"`) || !strings.Contains(factRepaired, `"fact_key":"user_fact"`) {
		t.Fatalf("save_user_fact missing arguments were not repaired: %q", factRepaired)
	}
}

func TestRefineContextualFollowupSearchQuery(t *testing.T) {
	titanicRecent := "Recent Turn 1\nUser: 타이타닉에서 구출 된 총 인원은 몇명입니까?\nAssistant: 타이타닉호 구출 인원 정보..."

	// Case 1: Short follow-up without subject -> Enriched with "타이타닉"
	repaired, ok := RefineContextualFollowupSearchQuery("search_web", `{"query":"생존자"}`, "생존자로 검색하면 나오지 않을까요?", titanicRecent)
	if !ok || !strings.Contains(repaired, "타이타닉") || !strings.Contains(repaired, "생존자") {
		t.Fatalf("follow-up query was not enriched: %q (ok=%v)", repaired, ok)
	}

	// Case 2: Short follow-up without subject -> Enriched with "타이타닉"
	repaired2, ok2 := RefineContextualFollowupSearchQuery("search_web", `{"query":"총 몇명이 탑승"}`, "총 몇명이 탑승했는데요?", titanicRecent)
	if !ok2 || !strings.Contains(repaired2, "타이타닉") || !strings.Contains(repaired2, "총 몇명이 탑승") {
		t.Fatalf("follow-up query was not enriched: %q (ok=%v)", repaired2, ok2)
	}

	// Case 3: Topic Shift / Independent query -> Must NOT be contaminated!
	independent, ok3 := RefineContextualFollowupSearchQuery("search_web", `{"query":"오늘 서울 날씨"}`, "오늘 서울 날씨 알려줘", titanicRecent)
	if ok3 || strings.Contains(independent, "타이타닉") {
		t.Fatalf("independent query was contaminated: %q (ok=%v)", independent, ok3)
	}
}

func TestMissingReadWebPageURLRecoveryUsesOnlyCurrentRequest(t *testing.T) {
	request := "이 페이지를 읽어주세요: https://example.com/weather/current"
	repaired, ok := RepairMissingReadWebPageArguments("read_web_page", `{}`, request)
	if !ok {
		t.Fatal("missing read_web_page URL was not recovered")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(repaired), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["url"] != "https://example.com/weather/current" {
		t.Fatalf("unexpected recovered URL: %#v", payload["url"])
	}

	unchanged, changed := RepairMissingReadWebPageArguments("read_web_page", `{"url":"https://example.com/original"}`, request)
	if changed || unchanged != `{"url":"https://example.com/original"}` {
		t.Fatalf("valid model URL was overwritten: %q", unchanged)
	}
	if _, changed := RepairMissingReadWebPageArguments("read_web_page", `{}`, "앞에서 말한 페이지를 읽어주세요"); changed {
		t.Fatal("a URL was invented for a request without an explicit current-turn URL")
	}
	if _, changed := RepairMissingReadWebPageArguments("search_web", `{}`, request); changed {
		t.Fatal("read-page recovery changed a different tool")
	}
}

func TestNamuwikiExactTitleUsesCurrentTurnInsteadOfPriorRequest(t *testing.T) {
	tests := []struct {
		request string
		want    string
	}{
		{"나무위키에서 훈민정음 검색", "훈민정음"},
		{"나무위키에서 '훈민정음'을 찾아 주세요", "훈민정음"},
		{"훈민정음을 나무위키에서 검색해줘", "훈민정음"},
		{"Please look up Hunminjeongeum on Namuwiki", "Hunminjeongeum"},
	}
	for _, test := range tests {
		refined, changed := RefineExactLookupToolArguments(
			"namu_wiki",
			`{"keyword":"오늘 날짜 나무위키에서 훈민정음 검색"}`,
			test.request,
		)
		if !changed {
			t.Fatalf("contaminated keyword was not refined for %q", test.request)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(refined), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["keyword"] != test.want {
			t.Fatalf("exact title for %q = %#v, want %q", test.request, payload["keyword"], test.want)
		}
	}

	unchanged, changed := RefineExactLookupToolArguments("namu_wiki", `{"keyword":"훈민정음"}`, "이어서 더 알려줘")
	if changed || unchanged != `{"keyword":"훈민정음"}` {
		t.Fatalf("elliptical request unexpectedly overwrote the model-resolved title: %q", unchanged)
	}

	recent := "Turn -1\nUser: 오늘 날짜\nAssistant: 오늘은 2026년 8월 4일입니다."
	repaired, ok := RepairMissingSearchToolArguments("namu_wiki", `{}`, "나무위키에서 훈민정음 검색", recent)
	if !ok {
		t.Fatal("missing Namuwiki keyword was not repaired")
	}
	refined, ok := RefineExactLookupToolArguments("namu_wiki", repaired, "나무위키에서 훈민정음 검색")
	if !ok || !strings.Contains(refined, `"keyword":"훈민정음"`) || strings.Contains(refined, "오늘 날짜") {
		t.Fatalf("screenshot regression retained the completed date request: %q", refined)
	}
}

func TestFreshnessSensitiveWebRequestAndCrossCheckPrompt(t *testing.T) {
	if !IsFreshnessSensitiveWebRequest("스페인 난민 사태 최신 뉴스") {
		t.Fatal("Korean latest-news request was not recognized as freshness-sensitive")
	}
	if IsFreshnessSensitiveWebRequest("Go에서 인터페이스를 설명해 주세요") {
		t.Fatal("stable technical question was classified as freshness-sensitive")
	}

	req, _, err := PrepareToolFollowupRequest(ToolFollowupInput{
		LLMMode:                    "standard",
		ToolName:                   "search_web",
		ToolResult:                 "Title: Example\nLink: https://example.com/news",
		ToolCallID:                 "call_freshness",
		ToolArguments:              `{"query":"스페인 난민 뉴스"}`,
		OriginalUserText:           "스페인 난민 사태 최신 뉴스",
		RequireFreshnessCrossCheck: true,
		ReqMap: map[string]interface{}{
			"model":    "test",
			"messages": []interface{}{map[string]interface{}{"role": "user", "content": "news"}},
			"tools":    []interface{}{map[string]interface{}{"type": "function"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := req["tools"]; !exists {
		t.Fatal("freshness cross-check removed tools before the second search")
	}
	messages := req["messages"].([]interface{})
	content, _ := messages[len(messages)-1].(map[string]interface{})["content"].(string)
	for _, expected := range []string{"freshness-sensitive request", "exactly one additional web search", "Do not repeat the previous query", "source links"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("freshness cross-check prompt omitted %q:\n%s", expected, content)
		}
	}
}

func TestFreshnessSingleSearchUpgradesToParallelCrossCheck(t *testing.T) {
	name, arguments, upgraded := UpgradeFreshnessSearchToolCall(
		"search_web",
		`{"query":"2026 AI model news"}`,
		"오늘 최신 AI 모델 뉴스를 알려 주세요",
	)
	if !upgraded || name != "search_web_multi" {
		t.Fatalf("fresh search was not upgraded: name=%q arguments=%s", name, arguments)
	}
	var payload struct {
		Queries []string `json:"queries"`
	}
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Queries) != 2 || payload.Queries[0] == payload.Queries[1] || !strings.Contains(payload.Queries[1], "official primary source") {
		t.Fatalf("upgraded queries are not complementary: %#v", payload.Queries)
	}
	if stableName, stableArgs, changed := UpgradeFreshnessSearchToolCall("search_web", `{"query":"Go interfaces"}`, "Go 인터페이스를 설명해 주세요"); changed || stableName != "search_web" || stableArgs == "" {
		t.Fatalf("stable request was unexpectedly upgraded: %q %s", stableName, stableArgs)
	}
}

func TestWeakWebEvidenceRequestsOneSingleAuthoritativeRefinement(t *testing.T) {
	result := CompactToolResult(
		"search_web_multi",
		"Recommended Next Action: refine_search_for_authoritative_source\nEvidence Quality Warning: no_authoritative_or_reputable_source",
		"오늘 최신 AI 모델 뉴스를 알려 주세요",
	)
	for _, expected := range []string{"exactly one refined search_web call", "not search_web_multi", "current year shown by CURRENT_TIME", "Do not read or summarize this buffered source"} {
		if !strings.Contains(result, expected) {
			t.Fatalf("weak-evidence refinement prompt omitted %q:\n%s", expected, result)
		}
	}
}

func TestCompactWebEvidenceMarksDayOnlyDatesWithoutChangingFullDates(t *testing.T) {
	result := CompactToolResult(
		"search_web",
		"Snippet: 사태 발생 이틀째인 31일 귀환했다. 기사 게시일은 8월 1일이다.",
		"최신 뉴스를 알려 주세요",
	)
	if !strings.Contains(result, "31일 [month unverified in source]") {
		t.Fatalf("day-only date was not annotated:\n%s", result)
	}
	if !strings.Contains(result, "8월 1일") || strings.Contains(result, "8월 1일 [month unverified") {
		t.Fatalf("full month/day date was changed:\n%s", result)
	}
}

func TestWebEvidenceSourcesAreExtractedAndAppended(t *testing.T) {
	result := "Title: First report\nLink: https://news.example/first\nSource Quality: reputable_news\n---\nTitle: Second report\nLink: https://news.example/second\nSource Quality: reputable_news"
	sources := ExtractWebEvidenceSources(result, 5)
	if len(sources) != 2 || sources[0].Title != "First report" || sources[1].URL != "https://news.example/second" {
		t.Fatalf("unexpected evidence sources: %#v", sources)
	}
	answer := AppendMissingWebEvidenceSources("확인된 내용입니다.", "최신 뉴스를 알려주세요", sources)
	for _, expected := range []string{"검색 출처:", "[First report](https://news.example/first)", "[Second report](https://news.example/second)"} {
		if !strings.Contains(answer, expected) {
			t.Fatalf("source appendix omitted %q:\n%s", expected, answer)
		}
	}
	if duplicated := AppendMissingWebEvidenceSources(answer, "최신 뉴스", sources); duplicated != answer {
		t.Fatalf("existing source links were appended twice:\n%s", duplicated)
	}
}

func TestFreshnessSourceAppendPrefersHighConfidenceEvidence(t *testing.T) {
	sources := []WebEvidenceSource{
		{Title: "SEO roundup", URL: "https://example.tistory.com/ai"},
		{Title: "Official release", URL: "https://openai.com/index/release"},
	}
	answer := AppendMissingWebEvidenceSources("확인된 최신 소식입니다.", "오늘 최신 AI 뉴스를 알려 주세요", sources)
	if !strings.Contains(answer, "https://openai.com/index/release") {
		t.Fatalf("official evidence was omitted:\n%s", answer)
	}
	if strings.Contains(answer, "example.tistory.com") {
		t.Fatalf("weak discovery lead was appended beside official evidence:\n%s", answer)
	}
}

func TestWebEvidenceExtractionAcceptsBufferedPageURL(t *testing.T) {
	sources := ExtractWebEvidenceSources("Title: Go 1.25 Release Notes\nURL: https://go.dev/doc/go1.25\nSummary: official", 2)
	if len(sources) != 1 || sources[0].URL != "https://go.dev/doc/go1.25" {
		t.Fatalf("buffered page URL was not extracted: %#v", sources)
	}
}

func TestToolSnapshotShowsParallelQueriesAndEvidence(t *testing.T) {
	snapshot := SessionUISnapshot{ToolCards: map[string]SessionToolCardSnapshot{}}
	updateToolSnapshot(&snapshot, "turn-1", "tool_call.start", map[string]interface{}{"tool": "search_web_multi"})
	updateToolSnapshot(&snapshot, "turn-1", "tool_call.arguments", map[string]interface{}{
		"tool": "search_web_multi",
		"arguments": map[string]interface{}{
			"queries": []interface{}{"first angle", "second angle"},
		},
	})
	updateToolSnapshot(&snapshot, "turn-1", "tool_call.success", map[string]interface{}{
		"tool": "search_web_multi",
		"evidence": []interface{}{
			map[string]interface{}{"title": "Report", "url": "https://news.example/report"},
		},
	})
	history := snapshot.ToolCards["turn-1"].History
	if len(history) != 3 {
		t.Fatalf("expected two query stages and one evidence entry, got %#v", history)
	}
	if history[0].Detail != "first angle" || history[1].Detail != "second angle" || !strings.Contains(history[2].Detail, "https://news.example/report") {
		t.Fatalf("unexpected tool history: %#v", history)
	}
}

func TestRecoveredToolArgumentsReplaceOriginalHistoryEntry(t *testing.T) {
	snapshot := SessionUISnapshot{ToolCards: map[string]SessionToolCardSnapshot{}}
	updateToolSnapshot(&snapshot, "turn-repair", "tool_call.start", map[string]interface{}{"tool": "search_web"})
	updateToolSnapshot(&snapshot, "turn-repair", "tool_call.arguments", map[string]interface{}{
		"tool": "search_web", "arguments": map[string]interface{}{"query": "이영애 배우 자녀"},
	})
	updateToolSnapshot(&snapshot, "turn-repair", "tool_call.arguments", map[string]interface{}{
		"tool": "search_web", "arguments": map[string]interface{}{"query": "이영애 배우 자녀 adopted biological children relationship"}, "recovered": true,
	})
	history := snapshot.ToolCards["turn-repair"].History
	if len(history) != 1 || !strings.Contains(history[0].Detail, "adopted biological") {
		t.Fatalf("recovered arguments were appended instead of replacing the original: %#v", history)
	}
}

func TestFinalAnswerOnlyFollowupRemovesNativeTools(t *testing.T) {
	req, _, err := PrepareToolFollowupRequest(ToolFollowupInput{
		LLMMode:          "standard",
		ToolName:         "search_web",
		ToolResult:       "Title: Example\nLink: https://example.com",
		ToolCallID:       "call_search",
		ToolArguments:    `{"query":"example"}`,
		OriginalUserText: "최신 뉴스를 검색해 주세요",
		FinalAnswerOnly:  true,
		ReqMap: map[string]interface{}{
			"model":               "test",
			"messages":            []interface{}{map[string]interface{}{"role": "user", "content": "news"}},
			"tools":               []interface{}{map[string]interface{}{"type": "function"}},
			"tool_choice":         "auto",
			"parallel_tool_calls": false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tools", "tool_choice", "parallel_tool_calls"} {
		if _, exists := req[key]; exists {
			t.Fatalf("final-answer follow-up retained provider control %q", key)
		}
	}
	messages := req["messages"].([]interface{})
	toolMessage := messages[len(messages)-1].(map[string]interface{})
	content, _ := toolMessage["content"].(string)
	for _, expected := range []string{"CURRENT APPLICATION DATE:", "later date is future", "evidence-gathering phase is complete", "Do not call, print, or describe any tool", "using only the supplied evidence", "could not be verified", "한국어로 답하세요"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("final-answer guard omitted %q:\n%s", expected, content)
		}
	}
}

func TestReasoningOnlyFinalRecoveryDisablesReasoningAndTools(t *testing.T) {
	stateful, _, err := PrepareReasoningOnlyFinalRequest("stateful", "qwen", "resp_123", "이영애 자녀", "쌍둥이 아들과 딸이라는 검색 근거", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := stateful["reasoning"]; exists {
		t.Fatalf("stateful recovery used an unsupported reasoning control: %#v", stateful)
	}
	messages := stateful["messages"].([]interface{})
	content := messages[len(messages)-1].(map[string]interface{})["content"].(string)
	if !strings.Contains(content, "이영애 자녀") || !strings.Contains(content, "쌍둥이 아들과 딸") {
		t.Fatalf("stateful recovery lost the original request: %#v", stateful)
	}

	standard, _, err := PrepareReasoningOnlyFinalRequest("standard", "qwen", "", "이영애 자녀", "검색 근거", map[string]interface{}{
		"model": "qwen", "messages": []interface{}{}, "tools": []interface{}{map[string]interface{}{"type": "function"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := standard["reasoning_effort"]; exists {
		t.Fatalf("standard recovery used an unsupported reasoning control: %#v", standard)
	}
	if _, exists := standard["tools"]; exists {
		t.Fatalf("standard recovery retained tools: %#v", standard)
	}
}

func TestSummarizeReasoningEvidenceExtractsConclusionsAndDrafts(t *testing.T) {
	longReasoning := strings.Repeat("분석 단계 1: 사용자의 요청에 대해 여러 가능성을 고민합니다. ", 100) +
		"\nLet's refine the Korean response:\n쿠시 왕국은 고대 아프리카의 번성했던 왕국입니다.\n결론적으로 이집트를 정복한 역사적 사실이 있습니다."

	summary := SummarizeReasoningEvidence(longReasoning)
	if !strings.Contains(summary, "쿠시 왕국은 고대 아프리카의 번성했던 왕국입니다") {
		t.Fatalf("reasoning summary lost the vital conclusion draft: %s", summary)
	}
	if len(summary) >= len(longReasoning) {
		t.Fatalf("reasoning summary was not compacted: %d >= %d", len(summary), len(longReasoning))
	}
}

func TestHarvestFinalAnswerFromReasoning(t *testing.T) {
	reasoningOutput := `This meets all requirements. Output matches. Proceeds.
Final Output Generation.
[Output] -> 제공된 검색 결과에 따르면, 타이타닉호 침몰 사고 당시 최연소 생존자는 **밀비나 딘(Millvina Dean)**입니다. 그녀는 배가 침몰하던 당시 생후 9주밖에 되지 않은 영아였으며, 2009년에 세상을 떠난 마지막 생존자 중 한 명으로 기록되어 있습니다.
출처: [Maestrovirtuale.com](https://example.com)`

	harvested, ok := HarvestFinalAnswerFromReasoning(reasoningOutput)
	if !ok {
		t.Fatalf("expected successful harvesting from reasoning text")
	}
	if !strings.Contains(harvested, "밀비나 딘(Millvina Dean)") || !strings.Contains(harvested, "Maestrovirtuale.com") {
		t.Fatalf("harvested content was missing vital content: %q", harvested)
	}
}

func TestSingleSearchRefinementForcesThenResetsNativeToolChoice(t *testing.T) {
	providerTools := []interface{}{
		map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "search_web"}},
		map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "read_web_page"}},
	}
	reqMap := map[string]interface{}{
		"model":       "test",
		"messages":    []interface{}{map[string]interface{}{"role": "user", "content": "news"}},
		"tools":       providerTools,
		"tool_choice": "auto",
	}
	forced, _, err := PrepareToolFollowupRequest(ToolFollowupInput{
		LLMMode: "standard", ToolName: "search_web_multi", ToolResult: "weak",
		ToolCallID: "call_1", ToolArguments: `{"queries":["a","b"]}`,
		OriginalUserText: "latest news", ReqMap: reqMap, SingleSearchRefinement: true, ProviderTools: providerTools,
	})
	if err != nil {
		t.Fatal(err)
	}
	forcedTools, _ := forced["tools"].([]interface{})
	if len(forcedTools) != 1 || forced["tool_choice"] != "auto" {
		t.Fatalf("single refinement did not expose only search_web: tools=%#v choice=%#v", forcedTools, forced["tool_choice"])
	}
	forcedFunction := forcedTools[0].(map[string]interface{})["function"].(map[string]interface{})
	if forcedFunction["name"] != "search_web" {
		t.Fatalf("wrong refinement tool exposed: %#v", forcedFunction)
	}
	reset, _, err := PrepareToolFollowupRequest(ToolFollowupInput{
		LLMMode: "standard", ToolName: "search_web", ToolResult: "strong",
		ToolCallID: "call_2", ToolArguments: `{"query":"official"}`,
		OriginalUserText: "latest news", ReqMap: forced, ProviderTools: providerTools,
	})
	if err != nil {
		t.Fatal(err)
	}
	if reset["tool_choice"] != "auto" {
		t.Fatalf("forced tool choice was not reset: %#v", reset["tool_choice"])
	}
	resetTools, _ := reset["tools"].([]interface{})
	if len(resetTools) != 2 {
		t.Fatalf("full provider tool catalog was not restored: %#v", resetTools)
	}
}

func TestBulkToolTestRequestKeepsFollowupRunning(t *testing.T) {
	request := "사용 가능한 Tool들을 하나씩 모두 테스트해보세요"
	if !IsBulkToolTestRequest(request) {
		t.Fatal("explicit Korean bulk tool test was not detected")
	}
	if IsBulkToolTestRequest("현재 시간을 도구로 확인해 주세요") {
		t.Fatal("ordinary single-tool request was classified as a bulk test")
	}

	req, _, err := PrepareToolFollowupRequest(ToolFollowupInput{
		LLMMode:          "standard",
		ToolName:         "get_current_time",
		ToolResult:       "now",
		ToolCallID:       "call_bulk",
		ToolArguments:    `{}`,
		OriginalUserText: request,
		CompletedTools:   []string{"get_current_time"},
		AvailableTools:   []string{"get_current_time", "get_current_location", "read_help"},
		ReqMap: map[string]interface{}{
			"model":    "test",
			"messages": []interface{}{map[string]interface{}{"role": "user", "content": request}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	messages := req["messages"].([]interface{})
	toolMessage := messages[len(messages)-1].(map[string]interface{})
	content, _ := toolMessage["content"].(string)
	for _, expected := range []string{"explicit bulk tool diagnostic", "get_current_time, get_current_location, read_help", "Continue immediately", "native function-call mechanism", "Never print a JSON object"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("bulk progress instruction omitted %q:\n%s", expected, content)
		}
	}
}
