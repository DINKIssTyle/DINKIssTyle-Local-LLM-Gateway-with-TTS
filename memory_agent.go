package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"dinkisstyle-chat/mcp"
)

// MemoryAnalysisRequest represents the payload for memory extraction
type MemoryAnalysisRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

// AnalyzeConversationForMemory analyzes the recent conversation to extract permanent facts
func (a *App) AnalyzeConversationForMemory(userID string, messages []map[string]interface{}, assistantResponse string) {
	if !a.enableMemory {
		return
	}

	// Heuristic: Only analyze if the conversation is substantial
	// For now, we analyze every turn but with a low priority.
	// In a real system, we might filter short queries.

	// Construct text for analysis
	var conversationText string
	for _, msg := range messages {
		if role, ok := msg["role"].(string); ok {
			if content, ok := msg["content"].(string); ok {
				conversationText += fmt.Sprintf("%s: %s\n", role, content)
			}
		}
	}
	conversationText += fmt.Sprintf("assistant: %s\n", assistantResponse)

	log.Printf("[Smart Memory] 🧠 Analyzing %d chars of conversation for implicit memories...", len(conversationText))

	// Prompt Design
	prompt := fmt.Sprintf(`You are a Memory Manager for an AI assistant.
Analyze the following conversation snippet.
Identify if the USER explicitly stated any NEW, PERMANENT facts about:
- Their preferences (likes/dislikes)
- Personal details (location, job, relationships)
- Future plans (appointments, goals)

IGNORE:
- Questions asked by the user
- Temporary context (e.g., "summarize this file")
- Facts already known or mentioned by the assistant
- Trivial chatter

Conversation:
%s

If NO new permanent facts are found, output ONLY: "NO_FACTS"
If facts are found, output them as a list of simple statements, one per line. Do NOT use bullet points or numbering.
Example:
User likes spicy food
User lives in Busan`, conversationText)

	// Call LLM
	msgs := []Message{
		{Role: "system", Content: "You extract personal facts for long-term memory."},
		{Role: "user", Content: prompt},
	}

	payload := MemoryAnalysisRequest{
		Model:       "gpt-3.5-turbo", // Use a fast/cheap model if possible, or the main model
		Messages:    msgs,
		Temperature: 0.0,
	}

	// Check if we can use the configured model, or default to main
	// Ideally we use a fast "internal" model, but we'll use the configured endpoint.
	// Assuming the endpoint supports the model name or ignores it (like LM Studio often does)
	payload.Model = "local-model"

	respBody, err := a.callLLM(payload)
	if err != nil {
		log.Printf("[Smart Memory] Failed to call LLM: %v", err)
		return
	}

	result := strings.TrimSpace(respBody)
	if result == "NO_FACTS" || result == "" {
		log.Printf("[Smart Memory] No new facts detected.")
		return
	}

	// Process Facts
	log.Printf("[Smart Memory] ✨ Detected potential memories:\n%s", result)
	lines := strings.Split(result, "\n")

	memoryPath, err := mcp.GetUserMemoryFilePath(userID, "personal.md")
	if err != nil {
		log.Printf("[Smart Memory] Error getting memory path: %v", err)
		return
	}

	for _, line := range lines {
		fact := strings.TrimSpace(line)
		if fact == "" || strings.HasPrefix(fact, "NO_FACTS") {
			continue
		}

		// Auto-save
		// Use "remember" action
		// We append a flag [Auto] to distinguish
		res, err := mcp.ManageMemory(memoryPath, "remember", "[Auto_Extracted] "+fact)
		if err != nil {
			log.Printf("[Smart Memory] Failed to save fact '%s': %v", fact, err)
		} else {
			log.Printf("[Smart Memory] ✅ Saved: %s -> %s", fact, res)
		}
	}
}

// Internal helper to call LLM
func (a *App) callLLM(payload interface{}) (string, error) {
	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", a.llmEndpoint+"/v1/chat/completions", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if a.llmApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.llmApiToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Parse standard OpenAI format
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse error: %v", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}

	return result.Choices[0].Message.Content, nil
}
