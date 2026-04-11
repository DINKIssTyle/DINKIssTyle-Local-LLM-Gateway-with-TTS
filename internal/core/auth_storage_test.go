package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dinkisstyle-chat/internal/mcp"
)

func TestAuthManagerMigratesLegacyUsersJSONIntoDBWithoutTTSSettings(t *testing.T) {
	mcp.CloseDB()

	root := t.TempDir()
	usersFile := filepath.Join(root, "users.json")
	legacy := []map[string]interface{}{
		{
			"id":            "alice",
			"password_hash": "hash-1",
			"role":          "admin",
			"created_at":    "2026-04-11T10:00:00Z",
			"settings": map[string]interface{}{
				"api_endpoint":        "http://127.0.0.1:1234",
				"api_token":           "secret-token",
				"secondary_model":     "gpt-4.1-mini",
				"llm_mode":            "stateful",
				"context_strategy":    "retrieval",
				"enable_tts":          true,
				"enable_mcp":          true,
				"enable_memory":       true,
				"stateful_turn_limit": 12,
				"embedding_config": map[string]interface{}{
					"provider": "local",
					"modelId":  "multilingual-e5-small",
					"enabled":  true,
				},
				"memory_retention": map[string]interface{}{
					"coreDays":      0,
					"workingDays":   7,
					"ephemeralDays": 14,
				},
				"disallowed_commands": []string{"rm"},
				"tts_config": map[string]interface{}{
					"engine":     "supertonic",
					"voiceStyle": "F1",
					"speed":      1.1,
					"threads":    2,
				},
			},
		},
	}

	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy users: %v", err)
	}
	if err := os.WriteFile(usersFile, data, 0600); err != nil {
		t.Fatalf("write legacy users file: %v", err)
	}

	am := NewAuthManager(usersFile)
	if _, ok := am.users["alice"]; !ok {
		t.Fatalf("expected legacy users.json to be readable before DB init")
	}

	dbPath := filepath.Join(root, "memory.db")
	if err := mcp.InitDB(dbPath); err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer mcp.CloseDB()

	if err := am.LoadUsers(); err != nil {
		t.Fatalf("reload users from DB-backed storage: %v", err)
	}

	settingsJSON, err := mcp.GetAccountSettingsJSON("alice")
	if err != nil {
		t.Fatalf("load migrated settings json: %v", err)
	}

	var stored map[string]interface{}
	if err := json.Unmarshal([]byte(settingsJSON), &stored); err != nil {
		t.Fatalf("unmarshal migrated settings json: %v", err)
	}

	if got := stored["api_token"]; got != "secret-token" {
		t.Fatalf("expected api_token to migrate, got %#v", got)
	}
	if _, exists := stored["enable_tts"]; exists {
		t.Fatalf("expected enable_tts to be omitted from DB settings_json")
	}
	if _, exists := stored["tts_config"]; exists {
		t.Fatalf("expected tts_config to be omitted from DB settings_json")
	}

	user := am.users["alice"]
	if user == nil {
		t.Fatalf("expected alice to be loaded from DB")
	}
	if user.Settings.ApiToken == nil || *user.Settings.ApiToken != "secret-token" {
		t.Fatalf("expected api_token to round-trip through DB")
	}
	if user.Settings.EnableTTS != nil {
		t.Fatalf("expected enable_tts not to be loaded from DB-backed settings")
	}
	if user.Settings.TTSConfig != nil {
		t.Fatalf("expected tts_config not to be loaded from DB-backed settings")
	}
}
