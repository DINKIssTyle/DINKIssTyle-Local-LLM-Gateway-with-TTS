package promptkit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStatefulFallbackUsesToolSpecificXML(t *testing.T) {
	prompt := BuildRuntimeInstructions(RuntimeInstructionsInput{
		Tools: []ToolDefinition{{
			Name:        "get_current_time",
			Description: "Get current time",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		}},
	})

	if !strings.Contains(prompt, `<get_current_time>{}</get_current_time>`) {
		t.Fatalf("tool-specific example missing: %s", prompt)
	}
	if strings.Contains(prompt, `<tool_call>`) {
		t.Fatalf("reserved wrapper leaked into stateful fallback prompt: %s", prompt)
	}
	if !strings.Contains(prompt, "same language as the user's current request") {
		t.Fatalf("response-language preservation rule missing: %s", prompt)
	}
	if !strings.Contains(prompt, "BULK TOOL TEST RULE") || !strings.Contains(prompt, "continue automatically") {
		t.Fatalf("bulk tool diagnostic rule missing: %s", prompt)
	}
	if !strings.Contains(prompt, "FRESHNESS SOURCE QUALITY RULE") || !strings.Contains(prompt, "primary-source") {
		t.Fatalf("freshness source-quality rule missing: %s", prompt)
	}
	if !strings.Contains(prompt, "TOOL DECISION DEADLINE") || !strings.Contains(prompt, "invoke it immediately") {
		t.Fatalf("reasoning-loop prevention rule missing: %s", prompt)
	}
	if !strings.Contains(prompt, "CURRENT REQUEST BOUNDARY RULE") || !strings.Contains(prompt, "exact page title/keyword") {
		t.Fatalf("current-turn context boundary rule missing: %s", prompt)
	}
}

func TestPlatformCommandGuidanceIsRuntimeSpecific(t *testing.T) {
	tests := []struct {
		environment string
		want        string
		notWant     string
	}{
		{"- Operating System: darwin", "ps axo", "Get-Process"},
		{"- Operating System: linux", "ps -eo", "sysctl -n kern.boottime"},
		{"- Operating System: windows", "Get-Process", "ps axo"},
	}
	for _, test := range tests {
		guidance := platformCommandGuidance(test.environment)
		if !strings.Contains(guidance, test.want) || strings.Contains(guidance, test.notWant) {
			t.Fatalf("unexpected platform guidance for %q: %q", test.environment, guidance)
		}
	}
}

func TestBuildCompactTaskInstructionsRoutesOnlySimpleTransformations(t *testing.T) {
	for _, input := range []string{
		"다음 문장을 자연스러운 영어로만 번역해 주세요.",
		"이 문장을 정중한 업무 문장으로 다듬어 주세요.",
		"Rewrite this sentence more concisely.",
	} {
		if prompt := BuildCompactTaskInstructions(input); !strings.Contains(prompt, compactTaskMarker) {
			t.Fatalf("compact transformation was not detected: %q", input)
		}
	}
	for _, input := range []string{
		"물의 화학식이 무엇인가요?",
		"비 오는 날 고양이 이야기를 써 주세요.",
		"그 장점을 두 문장으로 설명해 주세요.",
	} {
		if prompt := BuildCompactTaskInstructions(input); prompt != "" {
			t.Fatalf("ordinary request was classified as a compact transformation: %q => %q", input, prompt)
		}
	}
}

func TestNativeToolPromptUsesOnlyRelevantPolicyPacks(t *testing.T) {
	prompt := BuildRuntimeInstructions(RuntimeInstructionsInput{
		UseNativeTools:  true,
		EnvironmentInfo: "- Operating System: darwin\n- Current Working Directory: /tmp/example\n",
		Tools:           []ToolDefinition{{Name: "search_web"}, {Name: "search_web_multi"}},
	})
	for _, expected := range []string{"provider-native", "WEB:", "exactly two complementary queries", toolGuidelineEndMarker} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("native prompt omitted %q:\n%s", expected, prompt)
		}
	}
	for _, forbidden := range []string{"COMMAND:", "MEMORY:", "HELP:", "AVAILABLE APP TOOLS:", "ENVIRONMENT INFO:", "Current Working Directory", "ps axo"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("irrelevant policy pack %q was included:\n%s", forbidden, prompt)
		}
	}
	if len(prompt) > 3200 {
		t.Fatalf("native web prompt regressed above compact budget: chars=%d", len(prompt))
	}
}

func TestStripToolGuidelinesPreservesBasePromptAndMemory(t *testing.T) {
	content := "Base system prompt.\n\n" + toolGuidelineMarker + "\ntransient tool rules\n" + toolGuidelineEndMarker + "\n\n### MEMORY CONTEXT ###\nAurora"
	reqMap := map[string]interface{}{
		"messages": []interface{}{map[string]interface{}{"role": "system", "content": content}},
	}
	if !StripToolGuidelines(reqMap) {
		t.Fatal("tool guidelines were not removed")
	}
	messages := reqMap["messages"].([]interface{})
	stripped := messages[0].(map[string]interface{})["content"].(string)
	for _, expected := range []string{"Base system prompt.", "### MEMORY CONTEXT ###", "Aurora"} {
		if !strings.Contains(stripped, expected) {
			t.Fatalf("stripping lost %q: %s", expected, stripped)
		}
	}
	if strings.Contains(stripped, toolGuidelineMarker) || strings.Contains(stripped, "transient tool rules") {
		t.Fatalf("tool block survived stripping: %s", stripped)
	}
}
