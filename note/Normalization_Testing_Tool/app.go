/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx             context.Context
	mu              sync.Mutex
	data            AppData
	optimizerCancel context.CancelFunc
	optimizerBusy   bool
}

// AppData holds all persistent data
type AppData struct {
	Settings  AppSettings `json:"settings"`
	Rules     []Rule      `json:"rules"`
	TestCases []TestCase  `json:"testCases"`
}

type AppSettings struct {
	Endpoint    string  `json:"endpoint"`
	ApiKey      string  `json:"apiKey"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"maxTokens"`
}

type Rule struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
	Enabled     bool   `json:"enabled"`
}

type TestCase struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	RawContent string `json:"rawContent"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.loadData()
}

func (a *App) getDataFilePath() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".dinkisstyle", "normalization-tool")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "data.json")
}

func (a *App) loadData() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Default settings
	a.data.Settings = AppSettings{
		Endpoint:    "http://localhost:1234/v1",
		Model:       "gpt-4o",
		Temperature: 0.7,
		MaxTokens:   2048,
	}

	path := a.getDataFilePath()
	if _, err := os.Stat(path); err == nil {
		file, _ := os.ReadFile(path)
		json.Unmarshal(file, &a.data)
	}
	a.data.Settings = sanitizeSettings(a.data.Settings)
}

func (a *App) saveData() {
	path := a.getDataFilePath()
	file, _ := json.MarshalIndent(a.data, "", "  ")
	os.WriteFile(path, file, 0644)
}

// GetSettings returns current settings
func (a *App) GetSettings() AppSettings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return sanitizeSettings(a.data.Settings)
}

// SaveSettings saves new settings
func (a *App) SaveSettings(settings AppSettings) {
	a.mu.Lock()
	a.data.Settings = sanitizeSettings(settings)
	a.mu.Unlock()
	a.saveData()
}

func sanitizeSettings(settings AppSettings) AppSettings {
	settings.Endpoint = strings.TrimRight(strings.TrimSpace(settings.Endpoint), "/")
	settings.ApiKey = strings.TrimSpace(settings.ApiKey)
	settings.Model = strings.TrimSpace(settings.Model)
	if settings.Endpoint == "" {
		settings.Endpoint = "http://localhost:1234/v1"
	}
	if settings.Model == "" {
		settings.Model = "gpt-4o"
	}
	if settings.Temperature < 0 || settings.Temperature > 2 {
		settings.Temperature = 0.7
	}
	if settings.MaxTokens <= 0 {
		settings.MaxTokens = 2048
	}
	return settings
}

// GetRules returns current rules
func (a *App) GetRules() []Rule {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data.Rules
}

// SaveRules saves new rules
func (a *App) SaveRules(rules []Rule) {
	a.mu.Lock()
	a.data.Rules = rules
	a.mu.Unlock()
	a.saveData()
}

// GetTestCases returns current test cases
func (a *App) GetTestCases() []TestCase {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.data.TestCases
}

// SaveTestCases saves new test cases
func (a *App) SaveTestCases(cases []TestCase) {
	a.mu.Lock()
	a.data.TestCases = cases
	a.mu.Unlock()
	a.saveData()
}

// CallOptimizerLLM calls the configured LLM for optimization suggestions
func (a *App) CallOptimizerLLM(systemPrompt string, userPrompt string) (string, error) {
	a.mu.Lock()
	settings := sanitizeSettings(a.data.Settings)
	a.mu.Unlock()

	url := settings.Endpoint + "/chat/completions"
	payload := map[string]interface{}{
		"model": settings.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": settings.Temperature,
		"max_tokens":  settings.MaxTokens,
	}

	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if settings.ApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+settings.ApiKey)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no response from LLM")
}

func (a *App) StartOptimizerLLM(systemPrompt string, userPrompt string) error {
	a.mu.Lock()
	if a.optimizerBusy {
		a.mu.Unlock()
		return fmt.Errorf("optimizer request already in progress")
	}
	settings := sanitizeSettings(a.data.Settings)
	reqCtx, cancel := context.WithCancel(a.ctx)
	a.optimizerCancel = cancel
	a.optimizerBusy = true
	a.mu.Unlock()

	go a.runOptimizerStream(reqCtx, settings, systemPrompt, userPrompt)
	return nil
}

func (a *App) StopOptimizerLLM() error {
	a.mu.Lock()
	cancel := a.optimizerCancel
	running := a.optimizerBusy
	a.mu.Unlock()
	if !running || cancel == nil {
		return nil
	}
	cancel()
	return nil
}

func (a *App) finishOptimizerRequest() {
	a.mu.Lock()
	a.optimizerBusy = false
	a.optimizerCancel = nil
	a.mu.Unlock()
}

func (a *App) runOptimizerStream(ctx context.Context, settings AppSettings, systemPrompt string, userPrompt string) {
	defer a.finishOptimizerRequest()

	payload := map[string]interface{}{
		"model": settings.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": settings.Temperature,
		"max_tokens":  settings.MaxTokens,
		"stream":      true,
	}

	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, settings.Endpoint+"/chat/completions", bytes.NewBuffer(jsonPayload))
	if err != nil {
		runtime.EventsEmit(a.ctx, "optimizer:error", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if settings.ApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+settings.ApiKey)
	}

	runtime.EventsEmit(a.ctx, "optimizer:start")
	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			runtime.EventsEmit(a.ctx, "optimizer:stopped")
			return
		}
		runtime.EventsEmit(a.ctx, "optimizer:error", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		runtime.EventsEmit(a.ctx, "optimizer:error", fmt.Sprintf("LLM API error (HTTP %d): %s", resp.StatusCode, string(body)))
		return
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "text/event-stream") {
		body, _ := io.ReadAll(resp.Body)
		a.emitOptimizerJSONOrError(body)
		return
	}

	reader := bufio.NewReader(resp.Body)
	var fullContent strings.Builder
	var fullReasoning strings.Builder
	var eventLines []string
	completed := false

	flushEvent := func() bool {
		if len(eventLines) == 0 {
			return true
		}
		dataLines := make([]string, 0, len(eventLines))
		for _, line := range eventLines {
			if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		eventLines = eventLines[:0]
		if len(dataLines) == 0 {
			return true
		}
		payload := strings.Join(dataLines, "\n")
		if payload == "[DONE]" {
			completed = true
			runtime.EventsEmit(a.ctx, "optimizer:done", map[string]string{
				"content":   fullContent.String(),
				"reasoning": fullReasoning.String(),
			})
			return false
		}
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			return true
		}
		reasoningChunk, contentChunk := extractOptimizerChunks(msg)
		if reasoningChunk != "" {
			fullReasoning.WriteString(reasoningChunk)
			runtime.EventsEmit(a.ctx, "optimizer:reasoning", reasoningChunk)
		}
		if contentChunk != "" {
			fullContent.WriteString(contentChunk)
			runtime.EventsEmit(a.ctx, "optimizer:content", contentChunk)
		}
		if errValue, ok := msg["error"]; ok && errValue != nil {
			runtime.EventsEmit(a.ctx, "optimizer:error", stringifyOptimizerError(errValue))
			return false
		}
		return true
	}

	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && len(line) == 0 {
			if readErr == io.EOF {
				break
			}
			if ctx.Err() != nil {
				runtime.EventsEmit(a.ctx, "optimizer:stopped")
				return
			}
			runtime.EventsEmit(a.ctx, "optimizer:error", readErr.Error())
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			if !flushEvent() {
				return
			}
		} else {
			eventLines = append(eventLines, trimmed)
		}
		if readErr == io.EOF {
			break
		}
	}

	if ctx.Err() != nil {
		runtime.EventsEmit(a.ctx, "optimizer:stopped")
		return
	}
	if !completed {
		flushEvent()
	}
	if completed {
		return
	}
	runtime.EventsEmit(a.ctx, "optimizer:done", map[string]string{
		"content":   fullContent.String(),
		"reasoning": fullReasoning.String(),
	})
}

func (a *App) emitOptimizerJSONOrError(body []byte) {
	var result struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		runtime.EventsEmit(a.ctx, "optimizer:error", err.Error())
		return
	}
	if len(result.Choices) == 0 {
		runtime.EventsEmit(a.ctx, "optimizer:error", "no response from LLM")
		return
	}
	content := result.Choices[0].Message.Content
	reasoning := result.Choices[0].Message.ReasoningContent
	if reasoning != "" {
		runtime.EventsEmit(a.ctx, "optimizer:reasoning", reasoning)
	}
	runtime.EventsEmit(a.ctx, "optimizer:done", map[string]string{
		"content":   content,
		"reasoning": reasoning,
	})
}

func extractOptimizerChunks(msg map[string]interface{}) (string, string) {
	var reasoningChunk string
	var contentChunk string

	if choices, ok := msg["choices"].([]interface{}); ok && len(choices) > 0 {
		if first, ok := choices[0].(map[string]interface{}); ok {
			if delta, ok := first["delta"].(map[string]interface{}); ok {
				if val, ok := delta["reasoning_content"].(string); ok {
					reasoningChunk = val
				}
				if val, ok := delta["content"].(string); ok {
					contentChunk = val
				}
			}
			if message, ok := first["message"].(map[string]interface{}); ok {
				if reasoningChunk == "" {
					if val, ok := message["reasoning_content"].(string); ok {
						reasoningChunk = val
					}
				}
				if contentChunk == "" {
					if val, ok := message["content"].(string); ok {
						contentChunk = val
					}
				}
			}
		}
	}

	if msgType, ok := msg["type"].(string); ok {
		switch msgType {
		case "reasoning.delta":
			if val, ok := msg["content"].(string); ok {
				reasoningChunk = val
			}
		case "message.delta":
			if val, ok := msg["content"].(string); ok {
				contentChunk = val
			}
		}
	}

	return reasoningChunk, contentChunk
}

func stringifyOptimizerError(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]interface{}:
		if msg, ok := v["message"].(string); ok && strings.TrimSpace(msg) != "" {
			return msg
		}
		if bytes, err := json.Marshal(v); err == nil {
			return string(bytes)
		}
	}
	return "unknown optimizer error"
}

// ValidateSettings checks whether the configured endpoint is reachable and OpenAI-compatible.
func (a *App) ValidateSettings(settings AppSettings) (string, error) {
	settings = sanitizeSettings(settings)
	if settings.Endpoint == "" || settings.Model == "" {
		return "", fmt.Errorf("endpoint and model are required")
	}

	url := settings.Endpoint + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if settings.ApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+settings.ApiKey)
	}

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to reach %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("connection test failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err == nil && len(result.Data) > 0 {
		return fmt.Sprintf("Connected successfully. %d model(s) reported. Current target: %s", len(result.Data), settings.Model), nil
	}

	return fmt.Sprintf("Connected successfully to %s", settings.Endpoint), nil
}

// SyncFromAppJs reads the main frontend/app.js and extracts regex patterns from normalizeMarkdownForRender
func (a *App) SyncFromAppJs() ([]Rule, error) {
	// Try to find the root app.js. Assuming tool is in note/Normalization_Testing_Tool/
	path := filepath.Join("..", "..", "frontend", "app.js")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read app.js: %v", err)
	}

	content := string(data)
	// Simple regex to find .replace(/\/(.*?)\/([gimuy]*)/g, '$1$2- ')
	// We'll search for .replace(/.../, '...') or .replace(/.../, "$1...")
	importRegex := regexp.MustCompile(`\.replace\(\/([\s\S]*?)\/([gimuy]*)\s*,\s*['"]([\s\S]*?)['"]\)`)
	matches := importRegex.FindAllStringSubmatch(content, -1)

	var rules []Rule
	for i, m := range matches {
		if len(m) >= 4 {
			rules = append(rules, Rule{
				Id:          fmt.Sprintf("sync-%d", i),
				Name:        fmt.Sprintf("Imported %d", i+1),
				Pattern:     m[1],
				Replacement: m[3],
				Enabled:     true,
			})
		}
	}

	return rules, nil
}
