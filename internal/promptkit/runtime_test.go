package promptkit

import (
	"strings"
	"testing"
)

func TestInjectPromptAddsSystemMessageWhenMissing(t *testing.T) {
	reqMap := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
		},
	}

	if !InjectPrompt(reqMap, "\n\n"+ToolGuidelineMarker()+"\nrule") {
		t.Fatalf("expected prompt injection to report success")
	}

	messages := reqMap["messages"].([]interface{})
	if len(messages) != 2 {
		t.Fatalf("expected prepended system message, got %d messages", len(messages))
	}
	system := messages[0].(map[string]interface{})
	if system["role"] != "system" {
		t.Fatalf("expected first message to be system, got %v", system["role"])
	}
	if !strings.Contains(system["content"].(string), ToolGuidelineMarker()) {
		t.Fatalf("expected injected marker in system content")
	}
}

func TestInjectPromptDoesNotDuplicateExistingMarker(t *testing.T) {
	existing := "base\n\n" + ToolGuidelineMarker() + "\nkeep"
	reqMap := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "system", "content": existing},
			map[string]interface{}{"role": "user", "content": "hello"},
		},
	}

	if !InjectPrompt(reqMap, "\n\n"+ToolGuidelineMarker()+"\nnew") {
		t.Fatalf("expected prompt injection to report success")
	}

	got := reqMap["messages"].([]interface{})[0].(map[string]interface{})["content"].(string)
	if got != existing {
		t.Fatalf("expected existing system prompt to remain unchanged, got %q", got)
	}
}

func TestInjectPromptUpdatesSystemPromptFieldWithoutDuplicating(t *testing.T) {
	reqMap := map[string]interface{}{
		"system_prompt": "base prompt",
	}

	InjectPrompt(reqMap, "\n\n"+ToolGuidelineMarker()+"\nrule")
	InjectPrompt(reqMap, "\n\n"+ToolGuidelineMarker()+"\nrule")

	got := reqMap["system_prompt"].(string)
	if strings.Count(got, ToolGuidelineMarker()) != 1 {
		t.Fatalf("expected exactly one marker after repeated injections, got %q", got)
	}
}

func TestInjectPromptReturnsFalseForEmptyInstructions(t *testing.T) {
	reqMap := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
		},
	}

	if InjectPrompt(reqMap, "   ") {
		t.Fatalf("expected empty instructions to skip injection")
	}
}

func TestTruncateMessagesTruncatesLongHistoryAndPreservesSystemMessage(t *testing.T) {
	longContent := strings.Repeat("a", 11050)
	messages := []interface{}{
		map[string]interface{}{"role": "system", "content": "base system"},
		map[string]interface{}{"role": "user", "content": longContent},
	}
	for i := 0; i < 15; i++ {
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": strings.Repeat("m", 1200),
		})
	}

	got := truncateMessages(messages)
	if len(got) > 11 {
		t.Fatalf("expected truncated history to stay within 11 messages including system, got %d", len(got))
	}
	if got[0].(map[string]interface{})["role"] != "system" {
		t.Fatalf("expected preserved system message at front")
	}

	firstUser := messages[1].(map[string]interface{})["content"].(string)
	if !strings.Contains(firstUser, "truncated for context optimization") {
		t.Fatalf("expected original long message to be truncated in place")
	}
	if len(firstUser) > 10064 {
		t.Fatalf("expected truncated message length to stay near cap, got %d", len(firstUser))
	}
}

func TestBuildRuntimeInstructionsIncludesMemoryAndEnvironment(t *testing.T) {
	got := BuildRuntimeInstructions(RuntimeInstructionsInput{
		EnvironmentInfo:       "- Operating System: darwin\n- Preferred Shell: /bin/zsh\n",
		ModelID:               "test-model",
		UseNativeIntegrations: true,
		MemorySnapshot:        "likes tea",
		ActiveContext:         "recently asked about weather",
	})

	if !strings.Contains(got, ToolGuidelineMarker()) {
		t.Fatalf("expected tool guideline marker in runtime instructions")
	}
	if !strings.Contains(got, "### MEMORY CONTEXT ###") {
		t.Fatalf("expected memory section in runtime instructions")
	}
	if !strings.Contains(got, "ENVIRONMENT INFO:") {
		t.Fatalf("expected environment info in runtime instructions")
	}
}

func TestBuildRuntimeInstructionsAddsGemmaSpecificRules(t *testing.T) {
	got := BuildRuntimeInstructions(RuntimeInstructionsInput{
		ModelID:               "gemma-4-foo",
		UseNativeIntegrations: true,
	})

	if !strings.Contains(got, "GEMMA-4 RULE") {
		t.Fatalf("expected gemma-specific rules in runtime instructions")
	}
}

func TestBuildRuntimeInstructionsOmitsToolGuidanceWhenNativeIntegrationsDisabled(t *testing.T) {
	got := BuildRuntimeInstructions(RuntimeInstructionsInput{
		ModelID:               "gemma-4-foo",
		UseNativeIntegrations: false,
		MemorySnapshot:        "likes tea",
		ActiveContext:         "recently asked about weather",
	})

	if strings.Contains(got, ToolGuidelineMarker()) {
		t.Fatalf("expected no tool guideline marker when native integrations are disabled")
	}
	if strings.Contains(got, "<tool_call>") {
		t.Fatalf("expected no textual tool-call guidance when native integrations are disabled")
	}
	if strings.Contains(got, "search_web") || strings.Contains(got, "execute_command") {
		t.Fatalf("expected no MCP tool names when native integrations are disabled")
	}
	if !strings.Contains(got, "### MEMORY CONTEXT ###") {
		t.Fatalf("expected passive memory context to still be injected")
	}
}
