package core

import (
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
