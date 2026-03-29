/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// App struct
type App struct {
	ctx  context.Context
	mu   sync.Mutex
	data AppData
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
	return a.data.Settings
}

// SaveSettings saves new settings
func (a *App) SaveSettings(settings AppSettings) {
	a.mu.Lock()
	a.data.Settings = settings
	a.mu.Unlock()
	a.saveData()
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
	settings := a.data.Settings
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
