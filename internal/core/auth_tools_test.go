package core

import (
	"dinkisstyle-chat/internal/toolruntime"
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizePersistedSettingsKeepsEnableTools(t *testing.T) {
	var settings UserSettings
	if err := json.Unmarshal([]byte(`{"enable_tools":false}`), &settings); err != nil {
		t.Fatal(err)
	}

	settings = normalizePersistedSettings(settings)
	if settings.EnableTools == nil || *settings.EnableTools {
		t.Fatalf("tool setting was not preserved: %#v", settings.EnableTools)
	}

	encoded, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"enable_tools":false`) {
		t.Fatalf("unexpected persisted settings: %s", encoded)
	}
}

func TestComputerToolToggleDisablesEveryDesktopTool(t *testing.T) {
	expanded := expandDisabledToolAliases([]string{"computer_use"})
	if len(expanded) != len(toolruntime.ComputerToolNames) {
		t.Fatalf("incomplete desktop policy: %v", expanded)
	}
	set := map[string]bool{}
	for _, name := range expanded {
		set[name] = true
	}
	for _, name := range toolruntime.ComputerToolNames {
		if !set[name] {
			t.Fatalf("missing desktop tool: %s", name)
		}
	}
	if collapsed := collapseDisabledToolsForUI(expanded); len(collapsed) != 1 || collapsed[0] != "computer_use" {
		t.Fatalf("incorrect UI toggle: %v", collapsed)
	}
}
