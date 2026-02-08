package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// Message struct for LLM communication
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// EvolutionRequest represents the payload to ask the LLM for a regex
type EvolutionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

// LearnToolPattern attempts to generate a regex for an unrecognized tool call format
func (a *App) LearnToolPattern(modelID string, sampleLine string) {
	// Prevent infinite loops or spamming: Check if we already have a pattern or recently tried
	if a.GetToolPattern(modelID) != nil {
		return // Already has a pattern, maybe it's just a bad generation?
	}

	log.Printf("[Self-Evolution] 🧬 Analyzing potential missed tool call for model %s: %s", modelID, sampleLine)

	// Construct the prompt
	prompt := fmt.Sprintf(`You are an expert at Go Regular Expressions and LLM Tool Calling patterns.
I have a log line from an LLM that appears to be a tool call, but my current parser missed it.
The line is:
%s

Please generate a single Go-compatible Regular Expression (regexp) to capture:
- Group 1: The Tool Name
- Group 2: The JSON Arguments (or the content containing the arguments)

Examples of what I want to capture:
Line: "Function call: search_web({query:'test'})" -> Regex: "Function call: (\w+)\((.*)\)"

REQUIREMENTS:
1. Return ONLY the regex string. Do not wrap in markdown or code blocks.
2. The regex must be robust enough to handle slight variations.
3. Use (?i) for case insensitivity if appropriate.
4. Do not explain your reasoning. Just the regex.`, sampleLine)

	// Send request to the configured LLM endpoint
	// We use the SAME endpoint the user is using, assuming it can handle concurrent requests or queue them.
	msgs := []Message{
		{Role: "system", Content: "You are a coding assistant optimized for Go regex generation."},
		{Role: "user", Content: prompt},
	}

	payload := EvolutionRequest{
		Model:       modelID, // Ask the same model to understand its own output, or use a default if available
		Messages:    msgs,
		Temperature: 0.1, // Deterministic
	}

	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", a.llmEndpoint+"/v1/chat/completions", bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("[Self-Evolution] Failed to create request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if a.llmApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.llmApiToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Self-Evolution] Failed to query LLM: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[Self-Evolution] Failed to parse LLM response: %v", err)
		return
	}

	if len(result.Choices) == 0 {
		return
	}

	generatedRegex := strings.TrimSpace(result.Choices[0].Message.Content)
	// Clean up markdown if present
	generatedRegex = strings.TrimPrefix(generatedRegex, "```regex")
	generatedRegex = strings.TrimPrefix(generatedRegex, "```go")
	generatedRegex = strings.TrimPrefix(generatedRegex, "```")
	generatedRegex = strings.TrimSuffix(generatedRegex, "```")
	generatedRegex = strings.TrimSpace(generatedRegex)

	log.Printf("[Self-Evolution] 🧬 Proposed Regex: %s", generatedRegex)

	// Validate Regex
	re, err := regexp.Compile(generatedRegex)
	if err != nil {
		log.Printf("[Self-Evolution] Invalid regex generated: %v", err)
		return
	}

	// Test against the sample line
	matches := re.FindStringSubmatch(sampleLine)
	if len(matches) > 2 {
		log.Printf("[Self-Evolution] ✅ SUCCESS! Regex matched. Group1 (Tool): '%s', Group2 (Args): '%s'", matches[1], matches[2])

		// Update Config
		if a.toolPatterns == nil {
			a.toolPatterns = make(map[string]map[string]string)
		}

		// Create new pattern entry
		newPattern := map[string]string{
			"regex":      generatedRegex,
			"format":     "auto_generated", // Mark as auto-generated
			"created_at": time.Now().Format(time.RFC3339),
			"sample":     sampleLine,
		}

		a.toolPatterns[modelID] = newPattern

		// Save to config.json
		// We need to load standard config, update it, and save.
		// Since 'saveConfig' uses 'a.toolPatterns', we can just call it if we update the struct field.
		// However, we need to be careful about concurrency if multiple requests happen.
		// For now, let's just try to save.

		// NOTE: logic to save config is in app.go's saveConfig which reads from 'a' fields.
		// We might need to ensure 'saveConfig' works correctly with the map.

		// Let's create a temporary config to save just to be safe or use existing method
		// Re-using exiting SaveConfig which exposes `a.toolPatterns`

		// We need to trigger a save. But SaveConfig is usually internal or triggered by UI.
		// Let's implement a specific internal save.

		a.saveToolPatterns()

	} else {
		log.Printf("[Self-Evolution] ❌ Regex did not match the sample line. Discarding.")
	}
}

// Helper to save just the tool patterns or full config
func (a *App) saveToolPatterns() {
	// Re-read current config to avoid overwriting other fields?
	// Or just trust current state? App state should be source of truth.

	// Create a minimal struct to read/write just to avoid messing up other things if we can,
	// but standard app.go logic overwrites everything with current app state.
	// Let's try to reuse the existing save logic mechanisms if possible,
	// but `saveConfig` is private in `loadConfig` context usually?
	// app.go has `saveConfig` which writes `config.json`.
	// Let's check `app.go` again to see if `saveConfig` is available or if I need to duplicate it/expose it.

	// Assuming I can't easily call private `saveConfig` if it's not a method of *App.
	// Looking at previous `view_file` of `app.go`:
	// `func (a *App) saveConfig(cfgPath string)` seems to indicate it might be a method?
	// If not, I'll implement a local saver here.

	cfgPath := "config.json"

	// Read existing to preserve comments/order? JSON doesn't preserve comments anyway.
	file, err := os.ReadFile(cfgPath)
	var cfg AppConfig
	if err == nil {
		json.Unmarshal(file, &cfg)
	}

	// Update patterns
	cfg.ToolPatterns = a.toolPatterns

	// Write back
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	log.Printf("[Self-Evolution] 💾 Configuration saved with new pattern for %s", "updated_model")
}
