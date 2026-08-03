package core

import (
	"testing"

	"dinkisstyle-chat/internal/promptkit"
)

func TestNormalizeBufferedToolMatchUnwrapsAppToolCall(t *testing.T) {
	name, argumentsJSON, _, wrapper := normalizeBufferedToolMatch("", `{"name":"search_web","arguments":{"query":"latest news"}}`)
	if !wrapper {
		t.Fatal("canonical app tool call was not recognized as a wrapper")
	}
	if name != "search_web" {
		t.Fatalf("got tool name %q", name)
	}
	if argumentsJSON != `{"query":"latest news"}` {
		t.Fatalf("wrapper was forwarded instead of arguments: %s", argumentsJSON)
	}
}

func TestParseJSONToolCallAcceptsLocalModelAliases(t *testing.T) {
	tests := []struct {
		raw  string
		name string
		args string
	}{
		{`{"tool":"search_web","arguments":{"query":"MCP 도구 검증 테스트"}}`, "search_web", `{"query":"MCP 도구 검증 테스트"}`},
		{`{"tool":"read_buffered_source","parameters":{"source_id":"src_123","query":"Engine 핵심 기능"}}`, "read_buffered_source", `{"query":"Engine 핵심 기능","source_id":"src_123"}`},
		{"```json\n{\"name\":\"get_current_time\",\"arguments\":{}}\n```", "get_current_time", `{}`},
		{"```json\n{\"tool\":\"get_current_location\",\"params\":{}}\n```\n도구를 실행하겠습니다.", "get_current_location", `{}`},
		{`{"thought":"next","tool_name":"read_help","tool_arguments":{"query":"도구"}}`, "read_help", `{"query":"도구"}`},
	}
	for _, test := range tests {
		name, argumentsJSON, arguments, ok := parseJSONToolCall(test.raw)
		if !ok || name != test.name || argumentsJSON != test.args || arguments == nil {
			t.Fatalf("parseJSONToolCall(%q) = name=%q args=%q parsed=%#v ok=%v", test.raw, name, argumentsJSON, arguments, ok)
		}
	}
}

func TestLooksLikeJSONToolCallPossiblePrefixHoldsSplitFence(t *testing.T) {
	for _, raw := range []string{"`", "``", "```", "```j", "```json"} {
		if !looksLikeJSONToolCallPossiblePrefix(raw) {
			t.Fatalf("split JSON fence prefix was not held: %q", raw)
		}
	}
	if looksLikeJSONToolCallPossiblePrefix("ordinary assistant text") {
		t.Fatal("ordinary assistant text was mistaken for a JSON tool prefix")
	}
}

func TestParseToolCodeCallAcceptsPrintedFunctionSyntax(t *testing.T) {
	raw := `<tool_code>
print(read_buffered_source(source_id="src_18c835144500", query="What is Engine? Features, installation, usage examples, architecture"))
</tool_code>`
	name, argumentsJSON, arguments, ok := parsePromptToolMarkup(raw)
	if !ok || name != "read_buffered_source" || arguments == nil {
		t.Fatalf("tool_code wrapper was not parsed: name=%q args=%q parsed=%#v ok=%v", name, argumentsJSON, arguments, ok)
	}
	if argumentsJSON != `{"query":"What is Engine? Features, installation, usage examples, architecture","source_id":"src_18c835144500"}` {
		t.Fatalf("unexpected tool_code arguments: %s", argumentsJSON)
	}
}

func TestLooksLikeToolMarkupQuarantinesIncompleteJSONWrapper(t *testing.T) {
	raw := `{"tool":"search_web","arguments":{"query":"unfinished"}`
	if !looksLikeToolMarkup(raw) {
		t.Fatalf("incomplete JSON tool wrapper was not quarantined: %s", raw)
	}
}

func TestParseXMLLikeToolCallAcceptsToolSpecificJSON(t *testing.T) {
	name, argumentsJSON, arguments, ok := parseXMLLikeToolCall(`<get_current_time>{}</get_current_time>`)
	if name != "get_current_time" || argumentsJSON != `{}` || arguments == nil || !ok {
		t.Fatalf("unexpected no-arg tool parse: name=%q args=%q parsed=%#v ok=%v", name, argumentsJSON, arguments, ok)
	}

	name, argumentsJSON, _, _ = parseXMLLikeToolCall(`<execute_command>{"command":"uptime"}</execute_command>`)
	if name != "execute_command" || argumentsJSON != `{"command":"uptime"}` {
		t.Fatalf("unexpected command parse: name=%q args=%q", name, argumentsJSON)
	}
}

func TestParseXMLLikeToolCallAcceptsEmptyNoArgumentElement(t *testing.T) {
	name, argumentsJSON, arguments, ok := parseXMLLikeToolCall(`<get_current_time></get_current_time>`)
	if name != "get_current_time" || argumentsJSON != `{}` || arguments == nil || !ok {
		t.Fatalf("unexpected empty-element parse: name=%q args=%q parsed=%#v ok=%v", name, argumentsJSON, arguments, ok)
	}
}

func TestCompletedToolNamesFollowAdvertisedCatalogOrder(t *testing.T) {
	tools := []promptkit.ToolDefinition{{Name: "get_current_time"}, {Name: "get_current_location"}, {Name: "read_help"}}
	completed := completedToolNames(tools, map[string]int{"read_help": 1, "get_current_time": 2})
	if len(completed) != 2 || completed[0] != "get_current_time" || completed[1] != "read_help" {
		t.Fatalf("unexpected completed tool order: %#v", completed)
	}
}

func TestParsePromptToolMarkupAcceptsLegacyWrapper(t *testing.T) {
	name, argumentsJSON, _, ok := parsePromptToolMarkup(`<tool_call>{"name":"get_current_time","arguments":{}}</tool_call>`)
	if !ok || name != "get_current_time" || argumentsJSON != `{}` {
		t.Fatalf("legacy wrapper was not normalized: name=%q args=%q ok=%v", name, argumentsJSON, ok)
	}
}

func TestClassifyPromptToolMarkupHandlesSplitOpeningTag(t *testing.T) {
	tools := []promptkit.ToolDefinition{{Name: "get_current_time"}}
	if recognized, possible := classifyPromptToolMarkup(`<get_current_`, tools); recognized || !possible {
		t.Fatalf("split tag classification failed: recognized=%v possible=%v", recognized, possible)
	}
	if recognized, possible := classifyPromptToolMarkup(`<get_current_time>{}`, tools); !recognized || !possible {
		t.Fatalf("complete opening tag classification failed: recognized=%v possible=%v", recognized, possible)
	}
	if !hasPromptToolClosingTag(`</get_current_time>`, tools) {
		t.Fatal("known closing tag was not detected")
	}
	if recognized, possible := classifyPromptToolMarkup(`<tool_`, tools); recognized || !possible {
		t.Fatalf("split tool_code tag classification failed: recognized=%v possible=%v", recognized, possible)
	}
	if recognized, possible := classifyPromptToolMarkup(`<tool_code>print(`, tools); !recognized || !possible {
		t.Fatalf("tool_code tag classification failed: recognized=%v possible=%v", recognized, possible)
	}
}

func TestLMStudioEndPayloadToolMarkupIsRecognized(t *testing.T) {
	payload := map[string]interface{}{
		"result": map[string]interface{}{
			"output": []interface{}{map[string]interface{}{
				"type":    "message",
				"content": `<execute_command>{"command":"sysctl -n kern.boottime"}</execute_command>`,
			}},
		},
	}
	content := extractFinalAssistantContent(payload)
	name, argumentsJSON, _, ok := parsePromptToolMarkup(content)
	tools := []promptkit.ToolDefinition{{Name: "execute_command"}}
	if !ok || !isRegisteredPromptTool(name, tools) || argumentsJSON != `{"command":"sysctl -n kern.boottime"}` {
		t.Fatalf("terminal tool call in chat.end was not recognized: name=%q args=%q ok=%v", name, argumentsJSON, ok)
	}
}

func TestParseFunctionLikeToolCall(t *testing.T) {
	raw := `read_buffered_source(source_id="src_18c83936c25763f0", query="게시글의 주요 내용과 운영자의 사과/해명 내용을 요약해 주세요.")`
	name, argumentsJSON, arguments, ok := parsePromptToolMarkup(raw)
	if !ok || name != "read_buffered_source" {
		t.Fatalf("function-like tool call was not parsed: name=%q args=%q ok=%v", name, argumentsJSON, ok)
	}
	args, _ := arguments.(map[string]interface{})
	if args["source_id"] != "src_18c83936c25763f0" || args["query"] != "게시글의 주요 내용과 운영자의 사과/해명 내용을 요약해 주세요." {
		t.Fatalf("function-like arguments were not preserved: %#v", args)
	}

	tools := []promptkit.ToolDefinition{{Name: "read_buffered_source"}}
	if recognized, possible := classifyPromptToolMarkup(`read_buffered_`, tools); recognized || !possible {
		t.Fatalf("split function prefix was not held: recognized=%v possible=%v", recognized, possible)
	}
	if recognized, _ := classifyPromptToolMarkup(raw, tools); !recognized {
		t.Fatal("complete function-like tool call was not recognized")
	}
}

func TestLooksLikeToolMarkupRejectsOrphanedClosingTags(t *testing.T) {
	raw := `{"query":"논산 오늘 저녁 날씨","source_id":null}</read_buffered_source>
{"memory_id":216,"chunk_index":0}</read_memory_context>
{"memory_id":215}</read_memory>`
	if !looksLikeToolMarkup(raw) {
		t.Fatal("orphaned tool closing tags were not recognized as hidden protocol markup")
	}
}
