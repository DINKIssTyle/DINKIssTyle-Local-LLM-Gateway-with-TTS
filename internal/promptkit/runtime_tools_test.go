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
