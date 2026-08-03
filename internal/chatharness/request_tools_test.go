package chatharness

import (
	"encoding/json"
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
