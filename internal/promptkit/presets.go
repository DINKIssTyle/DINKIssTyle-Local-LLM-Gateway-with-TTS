package promptkit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const DefaultPromptText = "You are a helpful AI assistant."

// SystemPrompt represents a single system prompt preset.
type SystemPrompt struct {
	Title  string `json:"title"`
	Prompt string `json:"prompt"`
}

// LoadSystemPrompts loads system prompt presets from the application data directory.
func LoadSystemPrompts(appDataDir string) []SystemPrompt {
	promptsFile := filepath.Join(appDataDir, "system_prompts.json")

	if _, err := os.Stat(promptsFile); os.IsNotExist(err) {
		defaultPrompts := []SystemPrompt{
			{Title: "Default", Prompt: DefaultPromptText},
		}
		if data, err := json.MarshalIndent(defaultPrompts, "", "  "); err == nil {
			_ = os.WriteFile(promptsFile, data, 0644)
		}
	}

	content, err := os.ReadFile(promptsFile)
	if err != nil {
		fmt.Printf("[Prompts] Failed to read system_prompts.json: %v\n", err)
		return []SystemPrompt{{Title: "Default", Prompt: DefaultPromptText}}
	}

	var prompts []SystemPrompt
	if err := json.Unmarshal(content, &prompts); err != nil {
		fmt.Printf("[Prompts] Failed to parse system_prompts.json: %v\n", err)
		return []SystemPrompt{{Title: "Default", Prompt: DefaultPromptText}}
	}

	if len(prompts) == 0 {
		return []SystemPrompt{{Title: "Default", Prompt: DefaultPromptText}}
	}

	return prompts
}
