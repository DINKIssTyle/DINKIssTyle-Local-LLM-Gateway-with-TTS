/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"dinkisstyle-chat/mcp"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	globalTTS *TextToSpeech
	ttsConfig = ServerTTSConfig{
		VoiceStyle: "M1.json",
		Speed:      1.0,
	}
	// Style Cache
	styleCache = make(map[string]*Style)
	styleMutex sync.Mutex
	// Global TTS Mutex
	globalTTSMutex sync.RWMutex

	// Global App Instance (for handlers to access app methods)
	globalApp *App

	currentChatCancels  sync.Map
	savedTurnTitleTasks sync.Map
)

type savedTurnTitleTask struct {
	cancel context.CancelFunc
}

// Structured Tool Call Support
type StructuredToolCall struct {
	Thought       string      `json:"thought"`
	ToolName      string      `json:"tool_name"`
	ToolArguments interface{} `json:"tool_arguments"`
}

func compactText(input string, limit int) string {
	input = strings.TrimSpace(input)
	if limit <= 0 || len([]rune(input)) <= limit {
		return input
	}
	runes := []rune(input)
	return strings.TrimSpace(string(runes[:limit])) + "... (truncated)"
}

func currentChatCancelKey(userID string) string {
	return strings.TrimSpace(userID) + ":default"
}

func registerCurrentChatCancel(userID string, cancel context.CancelFunc) {
	if cancel == nil || strings.TrimSpace(userID) == "" {
		return
	}
	currentChatCancels.Store(currentChatCancelKey(userID), cancel)
}

func unregisterCurrentChatCancel(userID string) {
	if strings.TrimSpace(userID) == "" {
		return
	}
	currentChatCancels.Delete(currentChatCancelKey(userID))
}

func cancelCurrentChat(userID string) bool {
	if strings.TrimSpace(userID) == "" {
		return false
	}
	value, ok := currentChatCancels.Load(currentChatCancelKey(userID))
	if !ok {
		return false
	}
	cancel, ok := value.(context.CancelFunc)
	if !ok || cancel == nil {
		return false
	}
	cancel()
	return true
}

type savedTurnTitleOptions struct {
	ModelID     string
	APIToken    string
	Temperature float64
	LLMMode     string
}

type chatSessionToolHistorySnapshot struct {
	Tool   string `json:"tool"`
	Detail string `json:"detail"`
}

type chatSessionToolCardSnapshot struct {
	State    string                           `json:"state"`
	Summary  string                           `json:"summary"`
	Args     interface{}                      `json:"args,omitempty"`
	ToolName string                           `json:"tool_name"`
	History  []chatSessionToolHistorySnapshot `json:"history,omitempty"`
}

type chatSessionUISnapshot struct {
	ToolCards map[string]chatSessionToolCardSnapshot `json:"tool_cards"`
}

func cleanSavedTurnTitleContext(input string, limit int) string {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	input = regexp.MustCompile("```[\\s\\S]*?```").ReplaceAllString(input, " ")
	input = regexp.MustCompile("<think>[\\s\\S]*?</think>").ReplaceAllString(input, " ")
	input = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`).ReplaceAllString(input, "$1")
	input = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(input, "$1")
	input = regexp.MustCompile(`(?m)^\s*#{1,6}\s*`).ReplaceAllString(input, "")
	input = regexp.MustCompile(`(?m)^\s*[-*+]\s+`).ReplaceAllString(input, "")
	input = regexp.MustCompile(`(?m)^\s*\d+[.)]\s+`).ReplaceAllString(input, "")
	input = regexp.MustCompile(`(?m)^\s*>+\s*`).ReplaceAllString(input, "")
	input = strings.NewReplacer(
		"|", " ",
		"`", "",
		"**", "",
		"__", "",
		"~~", "",
		"---", " ",
		"***", " ",
	).Replace(input)
	input = regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(input), " ")
	return compactText(input, limit)
}

func parseChatSessionUISnapshot(raw string) chatSessionUISnapshot {
	snapshot := chatSessionUISnapshot{
		ToolCards: map[string]chatSessionToolCardSnapshot{},
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return snapshot
	}
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return chatSessionUISnapshot{ToolCards: map[string]chatSessionToolCardSnapshot{}}
	}
	if snapshot.ToolCards == nil {
		snapshot.ToolCards = map[string]chatSessionToolCardSnapshot{}
	}
	return snapshot
}

func encodeChatSessionUISnapshot(snapshot chatSessionUISnapshot) string {
	if snapshot.ToolCards == nil {
		snapshot.ToolCards = map[string]chatSessionToolCardSnapshot{}
	}
	bytes, err := json.Marshal(snapshot)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func estimateStatefulTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	runeCount := len([]rune(trimmed))
	return (runeCount + 3) / 4
}

func extractChatInputText(reqMap map[string]interface{}) string {
	if reqMap == nil {
		return ""
	}
	if input, ok := reqMap["input"].(string); ok {
		return strings.TrimSpace(input)
	}
	if items, ok := reqMap["input"].([]interface{}); ok {
		var parts []string
		for _, item := range items {
			obj, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			itemType, _ := obj["type"].(string)
			switch itemType {
			case "text":
				if content, ok := obj["content"].(string); ok && strings.TrimSpace(content) != "" {
					parts = append(parts, strings.TrimSpace(content))
				}
			case "input_text":
				if text, ok := obj["text"].(string); ok && strings.TrimSpace(text) != "" {
					parts = append(parts, strings.TrimSpace(text))
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	if messages, ok := reqMap["messages"].([]interface{}); ok {
		for i := len(messages) - 1; i >= 0; i-- {
			msg, ok := messages[i].(map[string]interface{})
			if !ok {
				continue
			}
			role, _ := msg["role"].(string)
			if role != "user" {
				continue
			}
			if content, ok := msg["content"].(string); ok {
				return strings.TrimSpace(content)
			}
		}
	}
	return ""
}

func compactToolSnapshotDetail(args interface{}, summary string) string {
	if s, ok := args.(string); ok && strings.TrimSpace(s) != "" {
		return compactText(strings.TrimSpace(s), 220)
	}
	if args != nil {
		if bytes, err := json.Marshal(args); err == nil {
			return compactText(strings.TrimSpace(string(bytes)), 220)
		}
	}
	return compactText(strings.TrimSpace(summary), 220)
}

func updateChatSessionToolSnapshot(snapshot *chatSessionUISnapshot, turnID, eventType string, payload interface{}) {
	if snapshot == nil || strings.TrimSpace(turnID) == "" {
		return
	}
	if snapshot.ToolCards == nil {
		snapshot.ToolCards = map[string]chatSessionToolCardSnapshot{}
	}

	card := snapshot.ToolCards[turnID]
	payloadMap, _ := payload.(map[string]interface{})
	if payloadMap == nil {
		payloadMap = map[string]interface{}{}
	}

	toolName, _ := payloadMap["tool"].(string)
	summary, _ := payloadMap["reason"].(string)
	args := payloadMap["arguments"]

	switch eventType {
	case "tool_call.start":
		card.State = "running"
		card.ToolName = strings.TrimSpace(toolName)
		card.Summary = ""
	case "tool_call.arguments":
		card.State = "running"
		if strings.TrimSpace(toolName) != "" {
			card.ToolName = strings.TrimSpace(toolName)
		}
		card.Args = args
		detail := compactToolSnapshotDetail(args, summary)
		if detail != "" {
			entry := chatSessionToolHistorySnapshot{
				Tool:   strings.TrimSpace(card.ToolName),
				Detail: detail,
			}
			if entry.Tool == "" {
				entry.Tool = "Tool"
			}
			last := chatSessionToolHistorySnapshot{}
			if len(card.History) > 0 {
				last = card.History[len(card.History)-1]
			}
			if last.Tool != entry.Tool || last.Detail != entry.Detail {
				card.History = append(card.History, entry)
			}
		}
	case "tool_call.success":
		card.State = "success"
		if strings.TrimSpace(toolName) != "" {
			card.ToolName = strings.TrimSpace(toolName)
		}
		card.Summary = ""
	case "tool_call.failure":
		card.State = "failure"
		if strings.TrimSpace(toolName) != "" {
			card.ToolName = strings.TrimSpace(toolName)
		}
		card.Summary = strings.TrimSpace(summary)
	}

	snapshot.ToolCards[turnID] = card
}

func normalizeSavedTurnTemperature(temp float64) float64 {
	if temp < 0 {
		return 0
	}
	if temp > 2 {
		return 2
	}
	return temp
}

func parseSavedTurnTitleOptionsFromBody(r *http.Request) (savedTurnTitleOptions, error) {
	var opts savedTurnTitleOptions
	if r == nil || r.Body == nil {
		return opts, nil
	}

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return opts, err
	}
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(rawBody))

	if len(bytes.TrimSpace(rawBody)) == 0 {
		return opts, nil
	}

	var payload struct {
		ModelID     string   `json:"model_id"`
		APIToken    string   `json:"api_token"`
		Temperature *float64 `json:"temperature"`
		LLMMode     string   `json:"llm_mode"`
	}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return opts, nil
	}

	opts.ModelID = strings.TrimSpace(payload.ModelID)
	opts.APIToken = strings.TrimSpace(payload.APIToken)
	opts.LLMMode = strings.TrimSpace(payload.LLMMode)
	if payload.Temperature != nil {
		opts.Temperature = normalizeSavedTurnTemperature(*payload.Temperature)
	}
	return opts, nil
}

func parseSavedTurnTitleFromJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	candidates := []string{raw}
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```JSON")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
		candidates = append([]string{raw}, candidates...)
	}
	if match := regexp.MustCompile(`(?s)\{.*\}`).FindString(raw); match != "" && match != raw {
		candidates = append([]string{match}, candidates...)
	}

	for _, candidate := range candidates {
		var payload struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal([]byte(candidate), &payload); err == nil {
			title := strings.TrimSpace(payload.Title)
			if title != "" {
				return title
			}
		}
	}

	return ""
}

func compactToolResult(toolName, result string) string {
	result = strings.TrimSpace(result)
	if result == "" {
		return fmt.Sprintf("Tool Result (%s): [empty]", toolName)
	}

	return fmt.Sprintf("Tool Result (%s):\n%s", toolName, compactText(result, 1200))
}

// getActiveCertPaths returns the paths to the active certificate and key pair.
func getActiveCertPaths(appDataDir string, certDomain string) (string, string, bool) {
	// 1. Try `{certDomain}.crt` / `{certDomain}.key`
	crtPath := filepath.Join(appDataDir, certDomain+".crt")
	crtKeyPath := filepath.Join(appDataDir, certDomain+".key")
	if _, err := os.Stat(crtPath); err == nil {
		if _, err := os.Stat(crtKeyPath); err == nil {
			return crtPath, crtKeyPath, true
		}
	}

	// 2. Try `{certDomain}.pem` / `{certDomain}.key`
	pemPath := filepath.Join(appDataDir, certDomain+".pem")
	pemKeyPath := filepath.Join(appDataDir, certDomain+".key")
	if _, err := os.Stat(pemPath); err == nil {
		if _, err := os.Stat(pemKeyPath); err == nil {
			return pemPath, pemKeyPath, true
		}
	}

	// 3. Fallback to default `cert.pem` / `key.pem`
	return filepath.Join(appDataDir, "cert.pem"), filepath.Join(appDataDir, "key.pem"), false
}

// ensureSelfSignedCert check if certificate and key exist in AppDataDir, if not create them.
func ensureSelfSignedCert(appDataDir string, certDomain string) (string, string, error) {
	certPath, keyPath, isDomainSpecific := getActiveCertPaths(appDataDir, certDomain)

	if isDomainSpecific {
		log.Printf("[HTTPS] Domain-specific custom certificate detected (%s). Using it.", filepath.Base(certPath))
		return certPath, keyPath, nil
	}

	// If both files exist, check if the domain matches
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			// Validate existing certificate CN
			certData, err := os.ReadFile(certPath)
			if err == nil {
				block, _ := pem.Decode(certData)
				if block != nil && block.Type == "CERTIFICATE" {
					cert, err := x509.ParseCertificate(block.Bytes)
					if err == nil {
						// Check if it's an auto-generated certificate
						isAutoGenerated := false
						for _, org := range cert.Subject.Organization {
							if org == "DINKI'ssTyle Local LLM Gateway" {
								isAutoGenerated = true
								break
							}
						}

						if isAutoGenerated {
							if cert.Subject.CommonName == certDomain && cert.IsCA {
								return certPath, keyPath, nil
							}
							log.Printf("[HTTPS] Auto-generated certificate upgrade/mismatch required. Regenerating...")
						} else {
							// User provided a custom certificate
							log.Printf("[HTTPS] Custom certificate detected. Using it without regeneration.")
							return certPath, keyPath, nil
						}
					}
				}
			}
		}
	}

	log.Println("[HTTPS] Generating self-signed certificate...")

	// Generate a new private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %v", err)
	}

	// Create a certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // 1 year validity

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"DINKI'ssTyle Local LLM Gateway"},
			CommonName:   certDomain,
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Add local IP and localhost to SANs
	template.IPAddresses = append(template.IPAddresses, net.ParseIP("127.0.0.1"), net.ParseIP("::1"))
	template.DNSNames = append(template.DNSNames, "localhost")
	if certDomain != "localhost" && certDomain != "127.0.0.1" {
		template.DNSNames = append(template.DNSNames, certDomain)
	}

	// Create the certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		return "", "", fmt.Errorf("failed to create certificate: %v", err)
	}

	// Write cert.pem
	certOut, err := os.Create(certPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to open cert.pem for writing: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", "", fmt.Errorf("failed to write data to cert.pem: %v", err)
	}
	if err := certOut.Close(); err != nil {
		return "", "", fmt.Errorf("error closing cert.pem: %v", err)
	}

	// Write key.pem
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", "", fmt.Errorf("failed to open key.pem for writing: %v", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("unable to marshal private key: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return "", "", fmt.Errorf("failed to write data to key.pem: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		return "", "", fmt.Errorf("error closing key.pem: %v", err)
	}

	log.Printf("[HTTPS] Certificate generated at %s", certPath)
	return certPath, keyPath, nil
}

// preloadUserMemory has been removed as it was part of the legacy file-based memory system.
// System context is now managed exclusively through the new SQLite Agentic RAG system and tools.

// callLLMInternal makes a background request to the LLM for summary/validation
func callLLMInternal(ctx context.Context, prompt string, opts savedTurnTitleOptions) string {
	if globalApp == nil || globalApp.llmEndpoint == "" {
		AddDebugTrace("saved-turn-title", "llm.skipped", "Skipped title generation request because LLM endpoint is empty", nil)
		return ""
	}

	endpoint := strings.TrimRight(globalApp.llmEndpoint, "/")
	endpoint = strings.TrimSuffix(endpoint, "/v1")

	modelID := strings.TrimSpace(opts.ModelID)
	if modelID == "" {
		modelID = "local-model"
	}

	llmMode := strings.TrimSpace(opts.LLMMode)
	if llmMode == "" {
		llmMode = strings.TrimSpace(globalApp.llmMode)
	}
	if llmMode == "" {
		llmMode = "standard"
	}

	systemPrompt := "You are a master at writing concise saved-chat titles. Return only valid JSON in the form {\"title\":\"...\"}. No markdown fences. No explanations."
	var (
		reqURL  string
		payload map[string]interface{}
	)
	if llmMode == "stateful" {
		reqURL = fmt.Sprintf("%s/api/v1/chat", endpoint)
		payload = map[string]interface{}{
			"model":         modelID,
			"temperature":   normalizeSavedTurnTemperature(opts.Temperature),
			"stream":        true,
			"system_prompt": systemPrompt,
			"input":         prompt,
		}
	} else {
		reqURL = fmt.Sprintf("%s/v1/chat/completions", endpoint)
		payload = map[string]interface{}{
			"model":       modelID,
			"temperature": normalizeSavedTurnTemperature(opts.Temperature),
			"max_tokens":  120,
			"messages": []map[string]interface{}{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": prompt},
			},
			"stream": true,
		}
	}

	AddDebugTrace("saved-turn-title", "llm.request", "Prepared saved turn title LLM request", map[string]interface{}{
		"model":       modelID,
		"mode":        llmMode,
		"temperature": normalizeSavedTurnTemperature(opts.Temperature),
		"endpoint":    reqURL,
		"has_api_key": strings.TrimSpace(opts.APIToken) != "" || strings.TrimSpace(globalApp.llmApiToken) != "",
		"__payload":   payload,
	})

	jsonPayload, _ := json.Marshal(payload)
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		AddDebugTrace("saved-turn-title", "llm.error", "Failed to build title request", map[string]interface{}{
			"error": err,
		})
		return ""
	}

	req.Header.Set("Content-Type", "application/json")
	if opts.APIToken != "" || globalApp.llmApiToken != "" {
		token := strings.TrimSpace(opts.APIToken)
		if token == "" {
			token = strings.TrimSpace(globalApp.llmApiToken)
		}
		if strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = token[7:]
		}
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		req.Header.Set("Authorization", "Bearer lm-studio")
	}

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		AddDebugTrace("saved-turn-title", "llm.error", "Title request failed", map[string]interface{}{
			"error": err,
		})
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		AddDebugTrace("saved-turn-title", "llm.error", "Title request returned non-OK status", map[string]interface{}{
			"status_code": resp.StatusCode,
			"__payload":   string(bodyBytes),
		})
		return ""
	}

	var (
		contentBuilder strings.Builder
		lastChunk      map[string]interface{}
	)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}
		dataStr := strings.TrimPrefix(line, "data: ")
		if dataStr == "[DONE]" {
			break
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
			continue
		}
		lastChunk = chunk

		if msgType, ok := chunk["type"].(string); ok && msgType == "message.delta" {
			if content, ok := chunk["content"].(string); ok && content != "" {
				contentBuilder.WriteString(content)
			}
			continue
		}

		if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if delta, ok := choice["delta"].(map[string]interface{}); ok {
					if content, ok := delta["content"].(string); ok && content != "" {
						contentBuilder.WriteString(content)
					}
				} else if msg, ok := choice["message"].(map[string]interface{}); ok {
					if content, ok := msg["content"].(string); ok && content != "" {
						contentBuilder.WriteString(content)
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		AddDebugTrace("saved-turn-title", "llm.error", "Failed while reading title stream", map[string]interface{}{
			"error": err,
		})
		return ""
	}

	rawContent := strings.TrimSpace(contentBuilder.String())
	AddDebugTrace("saved-turn-title", "llm.response", "Received saved turn title LLM response", map[string]interface{}{
		"model":       modelID,
		"mode":        llmMode,
		"content":     compactText(rawContent, 220),
		"content_len": len([]rune(rawContent)),
		"__payload":   lastChunk,
	})
	if rawContent != "" {
		return rawContent
	}

	AddDebugTrace("saved-turn-title", "llm.empty", "Title response did not contain assistant content", map[string]interface{}{
		"model": modelID,
		"mode":  llmMode,
	})
	return ""
}

func savedTurnTitleTaskKey(userID string, turnID int64) string {
	return fmt.Sprintf("%s:%d", strings.TrimSpace(userID), turnID)
}

func isSavedTurnTitleProcessing(userID string, turnID int64) bool {
	_, ok := savedTurnTitleTasks.Load(savedTurnTitleTaskKey(userID, turnID))
	return ok
}

func markSavedTurnEntriesProcessing(userID string, entries []mcp.SavedTurnEntry) []mcp.SavedTurnEntry {
	for i := range entries {
		entries[i].Processing = isSavedTurnTitleProcessing(userID, entries[i].ID)
	}
	return entries
}

func startSavedTurnTitleTask(userID string, turnID int64, fn func(ctx context.Context)) bool {
	key := savedTurnTitleTaskKey(userID, turnID)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	task := &savedTurnTitleTask{cancel: cancel}
	if _, loaded := savedTurnTitleTasks.LoadOrStore(key, task); loaded {
		cancel()
		return false
	}

	go func() {
		defer func() {
			cancel()
			savedTurnTitleTasks.Delete(key)
		}()
		fn(ctx)
	}()
	return true
}

func cancelSavedTurnTitleTask(userID string, turnID int64) bool {
	value, ok := savedTurnTitleTasks.Load(savedTurnTitleTaskKey(userID, turnID))
	if !ok {
		return false
	}
	task, ok := value.(*savedTurnTitleTask)
	if !ok || task == nil || task.cancel == nil {
		return false
	}
	task.cancel()
	return true
}

func savedTurnQueuedStatus(started bool) string {
	if started {
		return "processing"
	}
	return "noop"
}

type ServerTTSConfig struct {
	VoiceStyle string  `json:"voiceStyle"`
	Speed      float32 `json:"speed"`
	Threads    int     `json:"threads"`
}

// createServerMux creates the HTTP handler mux for the server
func createServerMux(app *App, authMgr *AuthManager) *http.ServeMux {
	globalApp = app // Initialize the global instance for all handlers
	mux := http.NewServeMux()

	// Public endpoints (no auth required)
	mux.HandleFunc("/api/login", handleLogin(authMgr))
	mux.HandleFunc("/api/logout", handleLogout(authMgr))
	mux.HandleFunc("/api/auth/check", handleAuthCheck(authMgr))
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(app.CheckHealth())
	})

	// Protected API endpoints
	mux.HandleFunc("/api/chat", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		handleChat(w, r, app, authMgr)
	}))
	mux.HandleFunc("/api/chat-session/current", AuthMiddleware(authMgr, handleCurrentChatSession()))
	mux.HandleFunc("/api/chat-session/events", AuthMiddleware(authMgr, handleChatSessionEvents()))
	mux.HandleFunc("/api/chat-session/stop", AuthMiddleware(authMgr, handleStopCurrentChat()))
	mux.HandleFunc("/api/chat-session/clear", AuthMiddleware(authMgr, handleClearCurrentChat()))
	mux.HandleFunc("/api/tts", AuthMiddleware(authMgr, handleTTS))
	mux.HandleFunc("/api/last-session", AuthMiddleware(authMgr, handleLastSession()))
	mux.HandleFunc("/api/saved-turns", AuthMiddleware(authMgr, handleSavedTurns()))
	mux.HandleFunc("/api/saved-turns/title-refresh", AuthMiddleware(authMgr, handleSavedTurnTitleRefresh()))

	// MCP Endpoints (Conditional)
	// MCP Endpoints (Always Enabled if server runs)
	log.Println("[Server] MCP Support Active")
	mux.HandleFunc("/mcp/sse", mcp.HandleSSE)
	mux.HandleFunc("/mcp/messages", mcp.HandleMessages)

	// Certificate Download Endpoint
	mux.HandleFunc("/api/cert/download", func(w http.ResponseWriter, r *http.Request) {
		certPath, _, _ := getActiveCertPaths(GetAppDataDir(), app.GetCertDomain())
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			http.Error(w, "Certificate not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(certPath)))
		w.Header().Set("Content-Type", "application/x-x509-ca-cert")
		http.ServeFile(w, r, certPath)
	})

	mux.HandleFunc("/api/config", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var newCfg struct {
				TTSThreads          int     `json:"tts_threads"`
				ApiEndpoint         string  `json:"api_endpoint"`
				ApiToken            *string `json:"api_token"`
				LLMMode             string  `json:"llm_mode"`
				EnableTTS           *bool   `json:"enable_tts"`
				EnableMCP           *bool   `json:"enable_mcp"`
				EnableMemory        *bool   `json:"enable_memory"`
				StatefulTurnLimit   *int    `json:"stateful_turn_limit"`
				StatefulCharBudget  *int    `json:"stateful_char_budget"`
				StatefulTokenBudget *int    `json:"stateful_token_budget"`
			}
			if err := json.NewDecoder(r.Body).Decode(&newCfg); err == nil {
				// Check for authenticated user
				userID := r.Header.Get("X-User-ID")
				var user *User
				if userID != "" {
					authMgr.mu.Lock()
					user = authMgr.users[userID]
					authMgr.mu.Unlock()
				}

				if user != nil {
					// User-specific save
					updated := false
					if newCfg.ApiEndpoint != "" {
						cleanEndpoint := strings.TrimSuffix(strings.TrimSpace(newCfg.ApiEndpoint), "/")
						cleanEndpoint = strings.TrimSuffix(cleanEndpoint, "/v1")
						user.Settings.ApiEndpoint = &cleanEndpoint
						updated = true
					}
					if newCfg.ApiToken != nil {
						token := strings.TrimSpace(*newCfg.ApiToken)
						if strings.HasPrefix(strings.ToLower(token), "bearer ") {
							token = strings.TrimSpace(token[7:])
						}
						user.Settings.ApiToken = &token
						updated = true
					}
					if newCfg.LLMMode != "" {
						user.Settings.LLMMode = &newCfg.LLMMode
						updated = true
					}
					if newCfg.EnableTTS != nil {
						user.Settings.EnableTTS = newCfg.EnableTTS
						updated = true
					}
					if newCfg.EnableMCP != nil {
						user.Settings.EnableMCP = newCfg.EnableMCP
						updated = true
					}
					if newCfg.EnableMemory != nil {
						user.Settings.EnableMemory = newCfg.EnableMemory
						updated = true
						// Sync to MCP context
						// We need disallowed lists here too, but handleConfig is partial update.
						// Let's retrieve full user settings to be safe.
						mcp.SetContext(user.ID, *newCfg.EnableMemory, user.Settings.DisabledTools, "", user.Settings.DisallowedCommands, user.Settings.DisallowedDirectories)
					}
					if newCfg.StatefulTurnLimit != nil {
						value := *newCfg.StatefulTurnLimit
						if value < 1 {
							value = 1
						}
						user.Settings.StatefulTurnLimit = &value
						updated = true
					}
					if newCfg.StatefulCharBudget != nil {
						value := *newCfg.StatefulCharBudget
						if value < 1000 {
							value = 1000
						}
						user.Settings.StatefulCharBudget = &value
						updated = true
					}
					if newCfg.StatefulTokenBudget != nil {
						value := *newCfg.StatefulTokenBudget
						if value < 1000 {
							value = 1000
						}
						user.Settings.StatefulTokenBudget = &value
						updated = true
					}
					// Handle TTS Config partial updates if needed, for now simplistic
					if newCfg.TTSThreads > 0 {
						if user.Settings.TTSConfig == nil {
							user.Settings.TTSConfig = &ServerTTSConfig{}
						}
						user.Settings.TTSConfig.Threads = newCfg.TTSThreads
						updated = true
					}

					if updated {
						if err := authMgr.SaveUsers(); err != nil {
							log.Printf("[handleConfig] Failed to save user settings: %v", err)
						} else {
							log.Printf("[handleConfig] Saved settings for user %s", userID)
						}
					}
				} else {
					// Global config save (Admin or fallback) - Only if no user context or explicitly desired?
					// For now, keeping legacy behavior for unauthenticated or admin might be confusing.
					// Let's assume if X-User-ID is missing (local mode) we save global.
					// If X-User-ID is present, we ONLY save to user.
					if userID == "" {
						if newCfg.TTSThreads > 0 {
							app.SetTTSThreads(newCfg.TTSThreads)
						}
						if newCfg.ApiEndpoint != "" {
							cleanEndpoint := strings.TrimSuffix(strings.TrimSpace(newCfg.ApiEndpoint), "/")
							cleanEndpoint = strings.TrimSuffix(cleanEndpoint, "/v1")
							app.SetLLMEndpoint(cleanEndpoint)
						}
						if newCfg.ApiToken != nil {
							token := strings.TrimSpace(*newCfg.ApiToken)
							if strings.HasPrefix(strings.ToLower(token), "bearer ") {
								token = strings.TrimSpace(token[7:])
							}
							app.SetLLMApiToken(token)
						}
						if newCfg.LLMMode != "" {
							app.SetLLMMode(newCfg.LLMMode)
						}
						if newCfg.EnableTTS != nil {
							app.SetEnableTTS(*newCfg.EnableTTS)
						}
						if newCfg.EnableMCP != nil {
							app.SetEnableMCP(*newCfg.EnableMCP)
						}
						if newCfg.EnableMemory != nil {
							// Global memory toggle is removed, handled per-user
						}
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")

		// Prepare response (Merge global + user)
		resp := map[string]interface{}{
			"llm_endpoint":          app.llmEndpoint,
			"llm_mode":              app.llmMode,
			"enable_tts":            app.enableTTS,
			"enable_mcp":            app.enableMCP,
			"enable_memory":         false, // Global default is false, overridden by user settings below
			"stateful_turn_limit":   8,
			"stateful_char_budget":  12000,
			"stateful_token_budget": 10000,
			"tts_config":            ttsConfig,
			"has_token":             app.llmApiToken != "",
		}

		// Overlay User Settings
		userID := r.Header.Get("X-User-ID")
		if userID != "" {
			authMgr.mu.RLock()
			user := authMgr.users[userID]
			authMgr.mu.RUnlock()

			if user != nil {
				if user.Settings.ApiEndpoint != nil {
					resp["llm_endpoint"] = *user.Settings.ApiEndpoint
				}
				if user.Settings.LLMMode != nil {
					resp["llm_mode"] = *user.Settings.LLMMode
				}
				if user.Settings.EnableTTS != nil {
					resp["enable_tts"] = *user.Settings.EnableTTS
				}
				if user.Settings.EnableMCP != nil {
					resp["enable_mcp"] = *user.Settings.EnableMCP
				}
				if user.Settings.EnableMemory != nil {
					resp["enable_memory"] = *user.Settings.EnableMemory
				}
				if user.Settings.StatefulTurnLimit != nil {
					resp["stateful_turn_limit"] = *user.Settings.StatefulTurnLimit
				}
				if user.Settings.StatefulCharBudget != nil {
					resp["stateful_char_budget"] = *user.Settings.StatefulCharBudget
				}
				if user.Settings.StatefulTokenBudget != nil {
					resp["stateful_token_budget"] = *user.Settings.StatefulTokenBudget
				}
				if user.Settings.ApiToken != nil && *user.Settings.ApiToken != "" {
					resp["has_token"] = true
				}
				// Note: We don't return the actul token for security, just has_token status
				// If the user wants to clear it, they send empty string.
				// But we assume if they set it, they know it.
			}
		}

		json.NewEncoder(w).Encode(resp)
	}))
	mux.HandleFunc("/api/tts/styles", AuthMiddleware(authMgr, handleTTSStyles))
	mux.HandleFunc("/v1/chat/completions", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		// Pass authMgr to allow user settings lookup
		handleChat(w, r, app, authMgr)
	}))
	mux.HandleFunc("/api/v1/chat", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		handleChat(w, r, app, authMgr)
	}))
	mux.HandleFunc("/api/models", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		handleModels(w, r, app)
	}))
	mux.HandleFunc("/api/dictionary", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(app.GetTTSDictionary(lang))
	}))
	mux.HandleFunc("/api/prompts", AuthMiddleware(authMgr, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(app.GetSystemPrompts())
	}))

	// Admin-only endpoints
	mux.HandleFunc("/api/users", AdminMiddleware(authMgr, handleUsers(authMgr)))
	mux.HandleFunc("/api/users/add", AdminMiddleware(authMgr, handleAddUser(authMgr)))
	mux.HandleFunc("/api/users/delete", AdminMiddleware(authMgr, handleDeleteUser(authMgr)))

	// Static file server for frontend (embedded)
	frontendFS, err := fs.Sub(app.assets, "frontend")
	if err != nil {
		log.Fatal("Failed to get frontend FS:", err)
	}
	webFS := http.FileServer(http.FS(frontendFS))

	// Serve web.html at root (Chat UI for web)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			// Serve web.html from embedded FS
			f, err := frontendFS.Open("web.html")
			if err != nil {
				http.Error(w, "web.html not found", http.StatusInternalServerError)
				return
			}
			defer f.Close()

			stat, _ := f.Stat()
			http.ServeContent(w, r, "web.html", stat.ModTime(), f.(io.ReadSeeker))
			return
		}
		webFS.ServeHTTP(w, r)
	})

	return mux
}

// handleLogin processes login requests
func handleLogin(am *AuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ID         string `json:"id"`
			Password   string `json:"password"`
			RememberMe bool   `json:"remember_me"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		token, err := am.Authenticate(req.ID, req.Password, req.RememberMe, r.UserAgent(), getClientAddress(r))
		if err != nil || token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
			return
		}

		// Set session cookie
		maxAge := 0
		if req.RememberMe {
			maxAge = 86400 * 30 // 30 days
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   r.TLS != nil,
			MaxAge:   maxAge,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// handleLogout processes logout requests
func handleLogout(am *AuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err == nil {
			am.InvalidateSession(cookie.Value)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   r.TLS != nil,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// handleAuthCheck checks if user is authenticated
func handleAuthCheck(am *AuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
			return
		}

		user, valid := am.ValidateSession(cookie.Value)
		if !valid {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": true,
			"user_id":       user.ID,
			"role":          user.Role,
		})
	}
}

// handleUsers returns list of users
func handleUsers(am *AuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(am.GetUsers())
	}
}

// handleAddUser adds a new user
func handleAddUser(am *AuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ID       string `json:"id"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if req.Role == "" {
			req.Role = "user"
		}

		if err := am.AddUser(req.ID, req.Password, req.Role); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// handleDeleteUser removes a user
func handleDeleteUser(am *AuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if err := am.DeleteUser(req.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleCurrentChatSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
		if userID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		entry, err := mcp.GetCurrentChatSession(userID)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Printf("[handleCurrentChatSession] Failed to load chat session for %s: %v", userID, err)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"has_session": false})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"has_session": true,
			"item":        entry,
		})
	}
}

func handleChatSessionEvents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
		if userID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		session, err := mcp.GetCurrentChatSession(userID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"has_session": false,
					"items":       []mcp.ChatEventEntry{},
				})
				return
			}
			http.Error(w, "Failed to load chat session", http.StatusInternalServerError)
			return
		}

		afterSeq := 0
		if raw := strings.TrimSpace(r.URL.Query().Get("after_seq")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
				afterSeq = parsed
			}
		}
		limit := 200
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		events, err := mcp.ListChatEvents(userID, session.ID, afterSeq, limit)
		if err != nil {
			http.Error(w, "Failed to load chat events", http.StatusInternalServerError)
			return
		}
		totalCount, err := mcp.CountChatEvents(userID, session.ID)
		if err != nil {
			http.Error(w, "Failed to count chat events", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"has_session": true,
			"session":     session,
			"items":       events,
			"total_count": totalCount,
		})
	}
}

func handleStopCurrentChat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
		if userID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cancelled := cancelCurrentChat(userID)
		sessionEntry, err := mcp.GetCurrentChatSession(userID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			log.Printf("[handleStopCurrentChat] Failed to load current session for %s: %v", userID, err)
		}
		sessionEntry.UserID = userID
		sessionEntry.SessionKey = "default"
		sessionEntry.Status = "cancelled"
		entry, err := mcp.UpsertChatSession(sessionEntry)
		if err == nil && entry.ID != 0 {
			_, _ = mcp.AppendChatEvent(userID, entry.ID, "system", "request.cancelled", "", "", `{"source":"remote_stop"}`)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "ok",
			"cancelled": cancelled,
		})
	}
}

func handleClearCurrentChat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
		if userID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cancelCurrentChat(userID)

		sessionEntry, err := mcp.GetCurrentChatSession(userID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			log.Printf("[handleClearCurrentChat] Failed to load current session for %s: %v", userID, err)
		}

		now := time.Now()
		sessionEntry.UserID = userID
		sessionEntry.SessionKey = "default"
		sessionEntry.Status = "idle"
		sessionEntry.LastResponseID = ""
		sessionEntry.SummaryText = ""
		sessionEntry.TurnCount = 0
		sessionEntry.EstimatedChars = 0
		sessionEntry.LastInputTokens = 0
		sessionEntry.LastOutputTokens = 0
		sessionEntry.PeakInputTokens = 0
		sessionEntry.RiskScore = 0
		sessionEntry.RiskLevel = ""
		sessionEntry.LastResetReason = "manual_clear_chat"
		sessionEntry.UIStateJSON = "{}"
		sessionEntry.ClearedAt = sql.NullTime{Time: now, Valid: true}

		entry, err := mcp.UpsertChatSession(sessionEntry)
		if err != nil {
			log.Printf("[handleClearCurrentChat] Failed to upsert current session for %s: %v", userID, err)
			http.Error(w, "Failed to clear current chat session", http.StatusInternalServerError)
			return
		}
		if err := mcp.ClearChatSessionEvents(userID, entry.ID); err != nil {
			log.Printf("[handleClearCurrentChat] Failed to clear chat events for %s: %v", userID, err)
		}

		clearPayload := map[string]interface{}{
			"source":     "remote_clear",
			"cleared_at": now.Format(time.RFC3339Nano),
		}
		if clearJSON, marshalErr := json.Marshal(clearPayload); marshalErr != nil {
			log.Printf("[handleClearCurrentChat] Failed to encode clear event for %s: %v", userID, marshalErr)
		} else if _, err := mcp.AppendChatEvent(userID, entry.ID, "system", "session.cleared", "", "", string(clearJSON)); err != nil {
			log.Printf("[handleClearCurrentChat] Failed to append clear event for %s: %v", userID, err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
		})
	}
}

func handleLastSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
		if userID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			entry, err := mcp.GetLastSession(userID)
			if err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					log.Printf("[handleLastSession] Failed to load last session for user %s: %v", userID, err)
				}
				json.NewEncoder(w).Encode(map[string]interface{}{
					"has_session": false,
				})
				return
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"has_session":       true,
				"user_message":      entry.LastUserMessage,
				"assistant_message": entry.LastAssistantMessage,
				"mode":              entry.Mode,
				"updated_at":        entry.UpdatedAt,
			})
		case http.MethodPost:
			var req struct {
				UserMessage      string `json:"user_message"`
				AssistantMessage string `json:"assistant_message"`
				Mode             string `json:"mode"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request", http.StatusBadRequest)
				return
			}

			if strings.TrimSpace(req.UserMessage) == "" || strings.TrimSpace(req.AssistantMessage) == "" {
				http.Error(w, "Both user_message and assistant_message are required", http.StatusBadRequest)
				return
			}

			if err := mcp.UpsertLastSession(userID, req.UserMessage, req.AssistantMessage, req.Mode, time.Now()); err != nil {
				log.Printf("[handleLastSession] Failed to save last session for %s: %v", userID, err)
				http.Error(w, "Failed to save last session", http.StatusInternalServerError)
				return
			}

			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case http.MethodDelete:
			if err := mcp.DeleteLastSession(userID); err != nil {
				log.Printf("[handleLastSession] Failed to delete last session for %s: %v", userID, err)
				http.Error(w, "Failed to delete last session", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func buildSavedTurnLLMTitle(ctx context.Context, promptText, responseText string, opts savedTurnTitleOptions) string {
	request := fmt.Sprintf(`Create a short title for this saved chat turn.

Rules:
- Use the main language of the saved content.
- Aim for about 20 characters.
- Make it specific and readable.
- Respond as JSON only in this exact shape: {"title":"..."}

Saved user prompt:
%s

Saved assistant response:
%s`, cleanSavedTurnTitleContext(promptText, 320), cleanSavedTurnTitleContext(responseText, 1200))

	AddDebugTrace("saved-turn-title", "parse.start", "Starting saved turn title generation", map[string]interface{}{
		"model":           strings.TrimSpace(opts.ModelID),
		"temperature":     normalizeSavedTurnTemperature(opts.Temperature),
		"prompt_chars":    len([]rune(strings.TrimSpace(promptText))),
		"response_chars":  len([]rune(strings.TrimSpace(responseText))),
		"request_preview": compactText(request, 220),
	})

	rawResponse := callLLMInternal(ctx, request, opts)
	title := parseSavedTurnTitleFromJSON(rawResponse)
	title = strings.Trim(title, "\"'`")
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		AddDebugTrace("saved-turn-title", "parse.failed", "Failed to parse title JSON response", map[string]interface{}{
			"raw_response": compactText(rawResponse, 220),
			"__payload":    rawResponse,
		})
		return ""
	}
	runes := []rune(title)
	if len(runes) > 24 {
		title = strings.TrimSpace(string(runes[:24]))
	}
	AddDebugTrace("saved-turn-title", "parse.success", "Parsed saved turn title JSON", map[string]interface{}{
		"title":        title,
		"title_length": len([]rune(title)),
	})
	return title
}

func handleSavedTurns() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
		if userID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			queryStr := strings.TrimSpace(r.URL.Query().Get("q"))
			var (
				entries []mcp.SavedTurnEntry
				err     error
			)
			if queryStr == "" {
				entries, err = mcp.ListSavedTurns(userID, 200)
			} else {
				entries, err = mcp.SearchSavedTurns(userID, queryStr, 200)
			}
			if err != nil {
				log.Printf("[handleSavedTurns] Failed to load saved turns for %s: %v", userID, err)
				http.Error(w, "Failed to load saved turns", http.StatusInternalServerError)
				return
			}
			entries = markSavedTurnEntriesProcessing(userID, entries)
			json.NewEncoder(w).Encode(map[string]interface{}{"items": entries})
		case http.MethodPost:
			var req struct {
				PromptText   string   `json:"prompt_text"`
				ResponseText string   `json:"response_text"`
				ModelID      string   `json:"model_id"`
				APIToken     string   `json:"api_token"`
				LLMMode      string   `json:"llm_mode"`
				Temperature  *float64 `json:"temperature"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request", http.StatusBadRequest)
				return
			}
			entry, err := mcp.SaveSavedTurn(userID, req.PromptText, req.ResponseText)
			if err != nil {
				log.Printf("[handleSavedTurns] Failed to save turn for %s: %v", userID, err)
				http.Error(w, "Failed to save turn", http.StatusInternalServerError)
				return
			}

			titleOpts := savedTurnTitleOptions{
				ModelID:  strings.TrimSpace(req.ModelID),
				APIToken: strings.TrimSpace(req.APIToken),
				LLMMode:  strings.TrimSpace(req.LLMMode),
			}
			if req.Temperature != nil {
				titleOpts.Temperature = normalizeSavedTurnTemperature(*req.Temperature)
			}

			started := startSavedTurnTitleTask(userID, entry.ID, func(ctx context.Context) {
				title := buildSavedTurnLLMTitle(ctx, req.PromptText, req.ResponseText, titleOpts)
				if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
					AddDebugTrace("saved-turn-title", "llm.cancelled", "Saved turn title generation stopped before completion", map[string]interface{}{
						"user_id": userID,
						"turn_id": entry.ID,
						"error":   ctx.Err(),
					})
					return
				}
				if title == "" {
					AddDebugTrace("saved-turn-title", "db.skipped", "Skipped DB update because no title was generated", map[string]interface{}{
						"user_id": userID,
						"turn_id": entry.ID,
					})
					return
				}
				if latestEntry, err := mcp.GetSavedTurn(userID, entry.ID); err == nil && strings.TrimSpace(latestEntry.TitleSource) != "fallback" {
					AddDebugTrace("saved-turn-title", "db.skipped", "Skipped DB update because title source changed while generating", map[string]interface{}{
						"user_id":      userID,
						"turn_id":      entry.ID,
						"title_source": latestEntry.TitleSource,
					})
					return
				}
				if err := mcp.UpdateSavedTurnTitle(userID, entry.ID, title, "generated"); err != nil {
					log.Printf("[handleSavedTurns] Failed to generate async title for %s turn %d: %v", userID, entry.ID, err)
					AddDebugTrace("saved-turn-title", "db.error", "Failed to update saved turn title in DB", map[string]interface{}{
						"user_id": userID,
						"turn_id": entry.ID,
						"title":   title,
						"error":   err,
					})
					return
				}
				AddDebugTrace("saved-turn-title", "db.updated", "Updated saved turn title in DB", map[string]interface{}{
					"user_id":      userID,
					"turn_id":      entry.ID,
					"title":        title,
					"title_source": "generated",
				})
			})

			if started {
				entry.Processing = true
				AddDebugTrace("saved-turn-title", "llm.started", "Started saved turn title generation task", map[string]interface{}{
					"user_id": userID,
					"turn_id": entry.ID,
				})
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":     "ok",
				"processing": entry.Processing,
				"item":       entry,
			})
		case http.MethodPatch:
			var req struct {
				ID    int64  `json:"id"`
				Title string `json:"title"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request", http.StatusBadRequest)
				return
			}
			req.Title = strings.TrimSpace(req.Title)
			if req.ID <= 0 || req.Title == "" {
				http.Error(w, "Valid id and title are required", http.StatusBadRequest)
				return
			}
			cancelSavedTurnTitleTask(userID, req.ID)
			if err := mcp.UpdateSavedTurnTitle(userID, req.ID, req.Title, "manual"); err != nil {
				log.Printf("[handleSavedTurns] Failed to update manual title for %s turn %d: %v", userID, req.ID, err)
				http.Error(w, "Failed to update title", http.StatusInternalServerError)
				return
			}
			entry, err := mcp.GetSavedTurn(userID, req.ID)
			if err != nil {
				log.Printf("[handleSavedTurns] Failed to load updated turn %d for %s: %v", req.ID, userID, err)
				http.Error(w, "Failed to load updated turn", http.StatusInternalServerError)
				return
			}
			AddDebugTrace("saved-turn-title", "db.updated", "Updated saved turn title manually", map[string]interface{}{
				"user_id":      userID,
				"turn_id":      req.ID,
				"title":        req.Title,
				"title_source": "manual",
			})
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ok",
				"item":   entry,
			})
		case http.MethodDelete:
			idStr := strings.TrimSpace(r.URL.Query().Get("id"))
			turnID, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil || turnID <= 0 {
				http.Error(w, "Valid id is required", http.StatusBadRequest)
				return
			}
			if err := mcp.DeleteSavedTurn(userID, turnID); err != nil {
				log.Printf("[handleSavedTurns] Failed to delete turn %d for %s: %v", turnID, userID, err)
				http.Error(w, "Failed to delete turn", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleSavedTurnTitleRefresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
		if userID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		titleOpts, err := parseSavedTurnTitleOptionsFromBody(r)
		if err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		idStr := strings.TrimSpace(r.URL.Query().Get("id"))
		var entry mcp.SavedTurnEntry
		if idStr != "" {
			turnID, parseErr := strconv.ParseInt(idStr, 10, 64)
			if parseErr != nil || turnID <= 0 {
				http.Error(w, "Valid id is required", http.StatusBadRequest)
				return
			}
			entry, err = mcp.GetSavedTurn(userID, turnID)
			if err != nil {
				log.Printf("[handleSavedTurnTitleRefresh] Failed to load turn %d for %s: %v", turnID, userID, err)
				http.Error(w, "Failed to load saved turn", http.StatusInternalServerError)
				return
			}
			if strings.TrimSpace(entry.TitleSource) != "fallback" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":  "noop",
					"updated": false,
					"item":    entry,
				})
				return
			}
			if isSavedTurnTitleProcessing(userID, entry.ID) {
				entry.Processing = true
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":     "processing",
					"updated":    false,
					"processing": true,
					"item":       entry,
				})
				return
			}
		} else {
			entry, err = mcp.GetNextSavedTurnPendingTitle(userID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":  "idle",
						"updated": false,
					})
					return
				}
				log.Printf("[handleSavedTurnTitleRefresh] Failed to load pending title for %s: %v", userID, err)
				http.Error(w, "Failed to load pending title", http.StatusInternalServerError)
				return
			}
			if isSavedTurnTitleProcessing(userID, entry.ID) {
				entry.Processing = true
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":     "processing",
					"updated":    false,
					"processing": true,
					"item":       entry,
				})
				return
			}
		}

		started := startSavedTurnTitleTask(userID, entry.ID, func(ctx context.Context) {
			title := buildSavedTurnLLMTitle(ctx, entry.PromptText, entry.ResponseText, titleOpts)
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				AddDebugTrace("saved-turn-title", "llm.cancelled", "Manual title refresh task stopped before completion", map[string]interface{}{
					"user_id": userID,
					"turn_id": entry.ID,
					"error":   ctx.Err(),
				})
				return
			}
			if title == "" {
				AddDebugTrace("saved-turn-title", "db.skipped", "Manual title refresh produced no title", map[string]interface{}{
					"user_id": userID,
					"turn_id": entry.ID,
				})
				return
			}
			if latestEntry, err := mcp.GetSavedTurn(userID, entry.ID); err == nil && strings.TrimSpace(latestEntry.TitleSource) != "fallback" {
				AddDebugTrace("saved-turn-title", "db.skipped", "Skipped manual refresh DB update because title source changed", map[string]interface{}{
					"user_id":      userID,
					"turn_id":      entry.ID,
					"title_source": latestEntry.TitleSource,
				})
				return
			}
			if err := mcp.UpdateSavedTurnTitle(userID, entry.ID, title, "generated"); err != nil {
				log.Printf("[handleSavedTurnTitleRefresh] Failed to update title for %s turn %d: %v", userID, entry.ID, err)
				AddDebugTrace("saved-turn-title", "db.error", "Failed to update title during manual refresh", map[string]interface{}{
					"user_id": userID,
					"turn_id": entry.ID,
					"title":   title,
					"error":   err,
				})
				return
			}
			AddDebugTrace("saved-turn-title", "db.updated", "Updated saved turn title during manual refresh", map[string]interface{}{
				"user_id":      userID,
				"turn_id":      entry.ID,
				"title":        title,
				"title_source": "generated",
			})
		})

		if started {
			entry.Processing = true
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     savedTurnQueuedStatus(started),
			"updated":    false,
			"processing": started,
			"item":       entry,
		})
	}
}

// handleModels proxies model list requests to LLM server
func handleModels(w http.ResponseWriter, r *http.Request, app *App) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodPost {
		// Handle Model Load Request
		var req struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.Model == "" {
			http.Error(w, "Model ID required", http.StatusBadRequest)
			return
		}

		if err := app.LoadModel(req.Model); err != nil {
			log.Printf("[handleModels] Load failed: %v", err)
			http.Error(w, fmt.Sprintf("Failed to load model: %v", err), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Model loaded"})
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Try to fetch fresh models
	bodyBytes, err := app.FetchAndCacheModels()
	if err != nil {
		log.Printf("[handleModels] Fetch failed: %v", err)

		// Fallback to cache if available
		cached := app.GetCachedModels()
		if cached != nil {
			log.Printf("[handleModels] Returning cached models (fallback)")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Model-Source", "cache-fallback")
			w.Write(cached)
			return
		}

		// No cache and fetch failed
		http.Error(w, fmt.Sprintf("Failed to fetch models: %v", err), http.StatusBadGateway)
		return
	}

	// Success
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Model-Source", "live")
	w.Write(bodyBytes)
}

// handleChat proxies chat requests to LM Studio with SSE streaming
// handleChat proxies chat requests to LM Studio with SSE streaming
func handleChat(w http.ResponseWriter, r *http.Request, app *App, authMgr *AuthManager) {
	requestStart := time.Now()
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var llmURL string

	// Default configuration (Global)
	endpointRaw := app.llmEndpoint
	tokenRaw := app.llmApiToken
	llmMode := app.llmMode
	enableMCP := app.enableMCP
	enableMemory := false // Default to false (Secure by default for unauthenticated)

	var disabledTools []string
	var locationInfo string
	var disallowedCmds []string
	var disallowedDirs []string

	// Override with User Settings
	userID := r.Header.Get("X-User-ID")
	// Extract Client Location
	locationInfo = r.Header.Get("X-User-Location")
	clientTurnID := strings.TrimSpace(r.Header.Get("X-Client-Turn-Id"))
	statefulTurnCount := strings.TrimSpace(r.Header.Get("X-Stateful-Turn-Count"))
	statefulEstChars := strings.TrimSpace(r.Header.Get("X-Stateful-Est-Chars"))
	statefulSummaryChars := strings.TrimSpace(r.Header.Get("X-Stateful-Summary-Chars"))
	statefulResetCount := strings.TrimSpace(r.Header.Get("X-Stateful-Reset-Count"))
	statefulInputTokens := strings.TrimSpace(r.Header.Get("X-Stateful-Input-Tokens"))
	statefulPeakInputTokens := strings.TrimSpace(r.Header.Get("X-Stateful-Peak-Input-Tokens"))
	statefulTokenBudget := strings.TrimSpace(r.Header.Get("X-Stateful-Token-Budget"))
	statefulRiskScore := strings.TrimSpace(r.Header.Get("X-Stateful-Risk-Score"))
	statefulRiskLevel := strings.TrimSpace(r.Header.Get("X-Stateful-Risk-Level"))
	statefulResetReason := strings.TrimSpace(r.Header.Get("X-Stateful-Reset-Reason"))

	chatCtx, chatCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer chatCancel()
	if strings.TrimSpace(userID) != "" {
		registerCurrentChatCancel(userID, chatCancel)
		defer unregisterCurrentChatCancel(userID)
	}

	clientStreaming := true
	var emitStreamChunk func(string)

	if userID != "" {
		authMgr.mu.RLock()
		user := authMgr.users[userID]
		authMgr.mu.RUnlock()
		if user != nil {
			if user.Settings.ApiEndpoint != nil {
				endpointRaw = *user.Settings.ApiEndpoint
			}
			if user.Settings.ApiToken != nil {
				tokenRaw = *user.Settings.ApiToken
			}
			if user.Settings.LLMMode != nil {
				llmMode = *user.Settings.LLMMode
			}
			if user.Settings.EnableMCP != nil {
				enableMCP = *user.Settings.EnableMCP
			}
			if user.Settings.EnableMemory != nil {
				enableMemory = *user.Settings.EnableMemory
			} else {
				enableMemory = true // Default to true for authenticated users
			}
			if user.Settings.DisabledTools != nil {
				disabledTools = user.Settings.DisabledTools
			}
			if user.Settings.DisallowedCommands != nil {
				disallowedCmds = user.Settings.DisallowedCommands
			}
			if user.Settings.DisallowedDirectories != nil {
				disallowedDirs = user.Settings.DisallowedDirectories
			}
		}
	}

	// Set MCP Context for this user interaction
	// This ensures that when LM Studio calls back to MCP, it has the correct context
	mcp.SetContext(userID, enableMemory, disabledTools, locationInfo, disallowedCmds, disallowedDirs)
	log.Printf("[handleChat-DEBUG] userID=%s, enableMemory=%v, disabledTools=%v, Location=%s, DisallowedCmds=%v, DisallowedDirs=%v", userID, enableMemory, disabledTools, locationInfo, disallowedCmds, disallowedDirs)
	AddDebugTrace("chat", "request.context", "Resolved chat execution context", map[string]interface{}{
		"user":            userID,
		"memory":          enableMemory,
		"disabled_tools":  len(disabledTools),
		"disallowed_cmds": len(disallowedCmds),
		"disallowed_dirs": len(disallowedDirs),
		"location":        compactText(locationInfo, 80),
		"stateful_turns":  statefulTurnCount,
		"stateful_chars":  statefulEstChars,
		"summary_chars":   statefulSummaryChars,
		"input_tokens":    statefulInputTokens,
		"peak_tokens":     statefulPeakInputTokens,
		"token_budget":    statefulTokenBudget,
		"risk_score":      statefulRiskScore,
		"risk_level":      statefulRiskLevel,
	})
	if statefulResetReason != "" {
		AddDebugTrace("stateful", "reset", "Stateful conversation was compacted or reset", map[string]interface{}{
			"user":         userID,
			"reason":       statefulResetReason,
			"turns_before": statefulTurnCount,
			"chars_before": statefulEstChars,
			"reset_count":  statefulResetCount,
		})
	}

	// Sanitize endpoint: Remove trailing slash and optional /v1 suffix if user included it
	endpoint := strings.TrimRight(endpointRaw, "/")
	endpoint = strings.TrimSuffix(endpoint, "/v1")
	token := strings.TrimSpace(tokenRaw)

	// Sanitize token: Remove "Bearer " prefix if user pasted it
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}

	var reqMap map[string]interface{}
	// Always unmarshal body into reqMap to prevent nil panics later in the turn loop
	json.Unmarshal(body, &reqMap)
	initialUserInputText := extractChatInputText(reqMap)
	AddDebugTrace("chat", "request.received", "Incoming chat request", map[string]interface{}{
		"user":       userID,
		"body_bytes": len(body),
		"messages":   lenInterfaceSlice(reqMap["messages"]),
		"__payload":  prettyJSONForDebug(body),
	})

	var procCtx *RequestExecutionContext
	if ctx, err := buildRequestExecutionContext(userID, reqMap, requestStart); err != nil {
		log.Printf("[ProceduralMemory] failed to initialize request context: %v", err)
	} else {
		procCtx = ctx
	}

	// Inject MCP integration if enabled AND NOT IN STANDARD MODE
	// Standard Mode (OpenAI compliant) with 'integrations' field might trigger strict auth in LM Studio.
	if enableMCP && llmMode != "standard" {
		if reqMap != nil {
			// Optimization for LM Studio Stateful mode:
			// If we are in stateful mode and have a previous_response_id, we can skip redundant injections
			// because LM Studio remembers the context (system_prompt, integrations, etc.) in the chat thread.
			isStatefulTurn := false
			if llmMode == "stateful" {
				if pid, ok := reqMap["previous_response_id"].(string); ok && pid != "" {
					isStatefulTurn = true
					log.Println("[handleChat] Detected follow-up stateful turn, skipping redundant system prompt (maintaining tools)")
				}
			}

			if enableMCP {
				targetMCP := "mcp/dinkisstyle-gateway"
				var integrations []string
				if existing, ok := reqMap["integrations"].([]interface{}); ok {
					for _, v := range existing {
						if str, ok := v.(string); ok {
							integrations = append(integrations, str)
						}
					}
				}

				hasMCP := false
				for _, v := range integrations {
					if v == targetMCP {
						hasMCP = true
						break
					}
				}

				if !hasMCP {
					integrations = append(integrations, targetMCP)
					reqMap["integrations"] = integrations
					// Important: internal body update will happen after system prompt injection
				}
			}

			// EXTRA SAFEGUARD: Inject System Prompt instruction for cleaner Tool Calls
			// Qwen/VL models often mess up XML tags (nested or unclosed).
			if !isStatefulTurn {
				var envInfo string
				if cwd, err := os.Getwd(); err == nil {
					envInfo = fmt.Sprintf("- Current Working Directory: %s\n", cwd)
				}

				// We use a marker to detect if we already injected instructions
				instrMarker := "### TOOL CALL GUIDELINES ###"
				extraInstr := mcp.SystemPromptToolUsage(envInfo)
				if hint, recipeVersion, err := getProceduralHint(procCtx); err != nil {
					log.Printf("[ProceduralMemory] failed to load procedural hint: %v", err)
				} else if hint != "" {
					extraInstr += hint
					if procCtx != nil {
						procCtx.RecipeVersion = recipeVersion
					}
				}

				if enableMemory {
					// 1. Memory Snapshot: 10 most recent summaries
					snapshot := mcp.GetMemorySnapshot(userID)

					// 2. Auto-RAG: Proactively search for full context based on the current user request
					var autoContext string
					if messages, ok := reqMap["messages"].([]interface{}); ok && len(messages) > 0 {
						// Find the last user message
						for i := len(messages) - 1; i >= 0; i-- {
							if m, ok := messages[i].(map[string]interface{}); ok {
								if role, ok := m["role"].(string); ok && role == "user" {
									if content, ok := m["content"].(string); ok {
										autoContext = compactText(mcp.AutoSearchMemory(userID, content), 1200)
										break
									}
								}
							}
						}
					}

					extraInstr += mcp.SystemPromptMemoryTemplate("", snapshot, autoContext)
				}

				foundSystem := false
				// Case A: Standard mode (OpenAI style)
				if messages, ok := reqMap["messages"].([]interface{}); ok {
					// 🚀 AGGRESSIVE SLIDING WINDOW & TRUNCATION
					// 1. First, truncate any individual message that is too long
					maxIndividualLen := 10000
					for i, msg := range messages {
						if m, ok := msg.(map[string]interface{}); ok {
							if content, ok := m["content"].(string); ok && len(content) > maxIndividualLen {
								m["content"] = content[:maxIndividualLen] + "\n... (content truncated for context optimization)"
								messages[i] = m
							}
						}
					}

					// 2. Limit total character count and number of messages
					maxTotalChars := 15000
					maxCount := 10

					currentTotal := 0
					var truncated []interface{}

					// Preserve system message if it exists at index 0
					var systemMsg interface{}
					if len(messages) > 0 {
						if m, ok := messages[0].(map[string]interface{}); ok {
							if role, ok := m["role"].(string); ok && role == "system" {
								systemMsg = messages[0]
								if content, ok := m["content"].(string); ok {
									currentTotal += len(content)
								}
							}
						}
					}

					// Build history from most recent, preserving space for system message
					for i := len(messages) - 1; i >= 0; i-- {
						if messages[i] == systemMsg {
							continue
						}
						msg := messages[i]
						if m, ok := msg.(map[string]interface{}); ok {
							if content, ok := m["content"].(string); ok {
								if currentTotal+len(content) > maxTotalChars || len(truncated) >= maxCount {
									break
								}
								currentTotal += len(content)
								truncated = append([]interface{}{msg}, truncated...)
							}
						}
					}

					if systemMsg != nil {
						truncated = append([]interface{}{systemMsg}, truncated...)
					}

					messages = truncated
					reqMap["messages"] = messages

					for i, msg := range messages {
						if m, ok := msg.(map[string]interface{}); ok {
							if role, ok := m["role"].(string); ok && role == "system" {
								content, _ := m["content"].(string)
								// Prevent duplicate injection
								if !strings.Contains(content, instrMarker) {
									m["content"] = content + extraInstr
									messages[i] = m
								}
								foundSystem = true
								break
							}
						}
					}
					if !foundSystem {
						newMsg := map[string]interface{}{
							"role":    "system",
							"content": "You are a helpful assistant." + extraInstr,
						}
						reqMap["messages"] = append([]interface{}{newMsg}, messages...)
						foundSystem = true
					}
				}

				// Case B: Stateful mode (LM Studio style)
				// LM Studio might append system_prompt every turn if we keep sending it.
				if sp, ok := reqMap["system_prompt"].(string); ok {
					// Prevent duplicate injection
					if !strings.Contains(sp, instrMarker) {
						// Check if this turn has previous state.
						// If it does, we might want to ONLY send the extra instructions if something changed,
						// but for now, we'll just ensure we don't duplicate the marker.
						reqMap["system_prompt"] = sp + extraInstr
					}
					foundSystem = true
				}

				if foundSystem {
					log.Println("[handleChat] Injected or deduplicated System Prompt instructions")
				}
			}

			// Final Body update if we changed anything
			if newBody, err := json.Marshal(reqMap); err == nil {
				body = newBody
			}
		}
	} else {
		log.Printf("[handleChat] MCP injection skipped (EnableMCP=%v, Mode=%s)", enableMCP, llmMode)
	}

	// Set the LLM URL based on mode
	if llmMode == "stateful" {
		llmURL = endpoint + "/api/v1/chat"
	} else {
		llmURL = endpoint + "/v1/chat/completions"
	}
	log.Printf("[handleChat] User: %s, Mode: %s, Endpoint: %s, URL: %s", userID, llmMode, endpoint, llmURL)

	// Determine Model ID for Tool Pattern Lookup
	var modelID string
	var tmpModel struct {
		Model string `json:"model"`
	}
	json.Unmarshal(body, &tmpModel)
	modelID = tmpModel.Model
	AddDebugTrace("chat", "request.prepared", "Prepared upstream LLM request", map[string]interface{}{
		"user":           userID,
		"mode":           llmMode,
		"model":          modelID,
		"url":            llmURL,
		"body_bytes":     len(body),
		"stateful_turns": statefulTurnCount,
		"stateful_chars": statefulEstChars,
		"input_tokens":   statefulInputTokens,
		"peak_tokens":    statefulPeakInputTokens,
		"token_budget":   statefulTokenBudget,
		"risk_score":     statefulRiskScore,
		"risk_level":     statefulRiskLevel,
	})

	parseIntHeader := func(value string) int {
		parsed, _ := strconv.Atoi(strings.TrimSpace(value))
		return parsed
	}
	parseFloatHeader := func(value string) float64 {
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return parsed
	}

	statefulTurnCountValue := parseIntHeader(statefulTurnCount)
	statefulEstimatedCharsValue := parseIntHeader(statefulEstChars)
	statefulSummaryCharsValue := parseIntHeader(statefulSummaryChars)
	statefulResetCountValue := parseIntHeader(statefulResetCount)
	statefulLastInputTokensValue := parseIntHeader(statefulInputTokens)
	statefulPeakInputTokensValue := parseIntHeader(statefulPeakInputTokens)
	statefulTokenBudgetValue := parseIntHeader(statefulTokenBudget)
	if statefulTokenBudgetValue <= 0 {
		statefulTokenBudgetValue = 10000
	}
	statefulRiskScoreValue := parseFloatHeader(statefulRiskScore)
	statefulRiskLevelValue := strings.TrimSpace(statefulRiskLevel)
	if statefulRiskLevelValue == "" {
		statefulRiskLevelValue = "low"
	}
	statefulLastOutputTokensValue := 0
	statefulSummaryText := ""

	var (
		chatSession           mcp.ChatSessionEntry
		chatSessionOK         bool
		sessionStatus         = "failed"
		sessionLastResponseID string
		sessionUIStateJSON    = "{}"
		sessionUISnapshot     = chatSessionUISnapshot{ToolCards: map[string]chatSessionToolCardSnapshot{}}
	)
	if strings.TrimSpace(userID) != "" {
		if existingSession, existingErr := mcp.GetCurrentChatSession(userID); existingErr == nil {
			sessionUIStateJSON = existingSession.UIStateJSON
			sessionUISnapshot = parseChatSessionUISnapshot(existingSession.UIStateJSON)
			statefulSummaryText = existingSession.SummaryText
			if llmMode == "stateful" && statefulResetReason == "" {
				statefulTurnCountValue = existingSession.TurnCount
				statefulEstimatedCharsValue = existingSession.EstimatedChars
				statefulLastInputTokensValue = existingSession.LastInputTokens
				statefulLastOutputTokensValue = existingSession.LastOutputTokens
				statefulPeakInputTokensValue = existingSession.PeakInputTokens
				if existingSession.TokenBudget > 0 {
					statefulTokenBudgetValue = existingSession.TokenBudget
				}
				if existingSession.RiskScore > 0 {
					statefulRiskScoreValue = existingSession.RiskScore
				}
				if strings.TrimSpace(existingSession.RiskLevel) != "" {
					statefulRiskLevelValue = existingSession.RiskLevel
				}
				if strings.TrimSpace(existingSession.LastResponseID) != "" {
					sessionLastResponseID = existingSession.LastResponseID
					if reqMap != nil {
						reqMap["previous_response_id"] = existingSession.LastResponseID
					}
				}
			}
		}
		if llmMode == "stateful" && statefulResetReason != "" && reqMap != nil {
			delete(reqMap, "previous_response_id")
			sessionLastResponseID = ""
		}
		statefulTurnCount = strconv.Itoa(statefulTurnCountValue)
		statefulEstChars = strconv.Itoa(statefulEstimatedCharsValue)
		statefulSummaryChars = strconv.Itoa(statefulSummaryCharsValue)
		statefulResetCount = strconv.Itoa(statefulResetCountValue)
		statefulInputTokens = strconv.Itoa(statefulLastInputTokensValue)
		statefulPeakInputTokens = strconv.Itoa(statefulPeakInputTokensValue)
		statefulTokenBudget = strconv.Itoa(statefulTokenBudgetValue)
		statefulRiskScore = strconv.FormatFloat(statefulRiskScoreValue, 'f', -1, 64)
		statefulRiskLevel = statefulRiskLevelValue
		if newBody, marshalErr := json.Marshal(reqMap); marshalErr == nil {
			body = newBody
		}
		chatSession, err = mcp.UpsertChatSession(mcp.ChatSessionEntry{
			UserID:           userID,
			SessionKey:       "default",
			Status:           "running",
			LLMMode:          llmMode,
			ModelID:          modelID,
			LastResponseID:   sessionLastResponseID,
			SummaryText:      statefulSummaryText,
			TurnCount:        statefulTurnCountValue,
			EstimatedChars:   statefulEstimatedCharsValue,
			LastInputTokens:  statefulLastInputTokensValue,
			LastOutputTokens: statefulLastOutputTokensValue,
			PeakInputTokens:  statefulPeakInputTokensValue,
			TokenBudget:      statefulTokenBudgetValue,
			RiskScore:        statefulRiskScoreValue,
			RiskLevel:        statefulRiskLevelValue,
			LastResetReason:  statefulResetReason,
			UIStateJSON:      sessionUIStateJSON,
		})
		if err != nil {
			log.Printf("[chat-session] failed to initialize current session for %s: %v", userID, err)
		} else {
			chatSessionOK = true
		}
	}

	appendChatEvent := func(role, eventType string, payload interface{}) {
		if !chatSessionOK {
			return
		}
		jsonPayload := "{}"
		if payload != nil {
			if bytes, err := json.Marshal(payload); err == nil {
				jsonPayload = string(bytes)
			}
		}
		if _, err := mcp.AppendChatEvent(userID, chatSession.ID, role, eventType, "", clientTurnID, jsonPayload); err != nil {
			log.Printf("[chat-session] failed to append %s event for %s: %v", eventType, userID, err)
		}
		if strings.HasPrefix(eventType, "tool_call.") {
			updateChatSessionToolSnapshot(&sessionUISnapshot, clientTurnID, eventType, payload)
			sessionUIStateJSON = encodeChatSessionUISnapshot(sessionUISnapshot)
			chatSession.UIStateJSON = sessionUIStateJSON
			if _, err := mcp.UpsertChatSession(mcp.ChatSessionEntry{
				UserID:           userID,
				SessionKey:       "default",
				Status:           "running",
				LLMMode:          llmMode,
				ModelID:          modelID,
				LastResponseID:   sessionLastResponseID,
				SummaryText:      statefulSummaryText,
				TurnCount:        statefulTurnCountValue,
				EstimatedChars:   statefulEstimatedCharsValue,
				LastInputTokens:  statefulLastInputTokensValue,
				LastOutputTokens: statefulLastOutputTokensValue,
				PeakInputTokens:  statefulPeakInputTokensValue,
				TokenBudget:      statefulTokenBudgetValue,
				RiskScore:        statefulRiskScoreValue,
				RiskLevel:        statefulRiskLevelValue,
				LastResetReason:  statefulResetReason,
				UIStateJSON:      sessionUIStateJSON,
			}); err != nil {
				log.Printf("[chat-session] failed to persist ui state for %s: %v", userID, err)
			}
		}
	}

	defer func() {
		if !chatSessionOK {
			return
		}
		if chatCtx.Err() == context.Canceled && sessionStatus != "idle" {
			sessionStatus = "cancelled"
		}
		if _, err := mcp.UpsertChatSession(mcp.ChatSessionEntry{
			UserID:           userID,
			SessionKey:       "default",
			Status:           sessionStatus,
			LLMMode:          llmMode,
			ModelID:          modelID,
			LastResponseID:   sessionLastResponseID,
			SummaryText:      statefulSummaryText,
			TurnCount:        statefulTurnCountValue,
			EstimatedChars:   statefulEstimatedCharsValue,
			LastInputTokens:  statefulLastInputTokensValue,
			LastOutputTokens: statefulLastOutputTokensValue,
			PeakInputTokens:  statefulPeakInputTokensValue,
			TokenBudget:      statefulTokenBudgetValue,
			RiskScore:        statefulRiskScoreValue,
			RiskLevel:        statefulRiskLevelValue,
			LastResetReason:  statefulResetReason,
			UIStateJSON:      sessionUIStateJSON,
		}); err != nil {
			log.Printf("[chat-session] failed to finalize current session for %s: %v", userID, err)
		}
	}()

	appendChatEvent("system", "request.prepared", map[string]interface{}{
		"mode":       llmMode,
		"model":      modelID,
		"url":        llmURL,
		"body_bytes": len(body),
	})
	if statefulResetReason != "" {
		appendChatEvent("system", "stateful.reset", map[string]interface{}{
			"reason":         statefulResetReason,
			"turn_count":     parseIntHeader(statefulTurnCount),
			"estimatedChars": parseIntHeader(statefulEstChars),
		})
	}
	if messages, ok := reqMap["messages"].([]interface{}); ok {
		for i := len(messages) - 1; i >= 0; i-- {
			msg, ok := messages[i].(map[string]interface{})
			if !ok {
				continue
			}
			if role, _ := msg["role"].(string); role == "user" {
				appendChatEvent("user", "message.created", msg)
				break
			}
		}
	} else if inputStr, ok := reqMap["input"].(string); ok && strings.TrimSpace(inputStr) != "" {
		appendChatEvent("user", "message.created", map[string]interface{}{"content": inputStr})
	}

	// Set SSE headers ONCE before turn loop
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	emitStreamChunk = func(payload string) {
		if !clientStreaming {
			return
		}
		if _, writeErr := fmt.Fprintf(w, "%s\n\n", payload); writeErr != nil {
			clientStreaming = false
			log.Printf("[handleChat] Client stream detached for %s: %v", userID, writeErr)
			return
		}
		flusher.Flush()
	}

	// Shared turn-state variables
	fullResponse := ""
	var messagesForMemory []map[string]interface{}
	needsCorrection := false
	var badContentCapture string
	var lastResponseID string // Captured from chat.end for stateful chaining
	reasoningActive := false
	reasoningStartedAt := time.Time{}
	toolUsageCounts := make(map[string]int)
	toolSignatureCounts := make(map[string]int)
	previousResponseRetryUsed := false
	discardStatefulResponseIDForTurn := false

	// --- TURN LOOP START ---
	// We allow up to 10 turns (tool call cycles) per request
	for turn := 0; turn < 10; turn++ {
		turnStart := time.Now()
		toolExecutedThisTurn := false
		var lastToolName string
		var lastToolArgsStr string
		var lastSavedBufferForTurn string

		// Variables for tool execution (shared between Regex and JSON modes)
		var toolName string
		var toolArgsStr string

		// Use r.Context() to propagate cancellation from frontend
		// Note: 'body' must be updated if we loop
		req, err := http.NewRequestWithContext(chatCtx, "POST", llmURL, bytes.NewReader(body))
		if err != nil {
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}

		req.Header.Set("Content-Type", "application/json")

		// Check if token is effectively empty or just "bearer", OR IS A MASKED VALUE
		token = strings.TrimSpace(token)
		isMasked := strings.HasPrefix(token, "***") || strings.HasSuffix(token, "...")
		if token != "" && !isMasked {
			req.Header.Set("Authorization", "Bearer "+token)
		} else {
			// Default to lm-studio (standard, no hacks).
			// If this fails with 401, we will handle the error response to guide the user.
			log.Printf("[handleChat] Empty/Invalid/Masked Token detected ('%s'), using Default: lm-studio", token)
			req.Header.Set("Authorization", "Bearer lm-studio")
		}

		client := &http.Client{Timeout: 5 * time.Minute}
		AddDebugTrace("chat", "turn.start", "Dispatching upstream turn", map[string]interface{}{
			"turn":       turn,
			"model":      modelID,
			"body_bytes": len(body),
		})
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("LLM request failed: %v", err)
			AddDebugTrace("chat", "turn.error", "Upstream request failed", map[string]interface{}{
				"turn":       turn,
				"elapsed_ms": time.Since(turnStart).Milliseconds(),
				"error":      err.Error(),
			})
			if turn == 0 {
				http.Error(w, fmt.Sprintf("LLM connection failed: %v", err), http.StatusBadGateway)
			}
			return
		}
		// Close body at end of this turn
		// We will close it explicitly after the scanner instead of using defer in a loop

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			errorMsg := string(bodyBytes)
			log.Printf("LLM error response: %s", errorMsg)
			AddDebugTrace("chat", "turn.error", "Upstream returned an error status", map[string]interface{}{
				"turn":        turn,
				"status_code": resp.StatusCode,
				"elapsed_ms":  time.Since(turnStart).Milliseconds(),
				"error":       compactText(errorMsg, 180),
			})
			if llmMode == "stateful" &&
				!previousResponseRetryUsed &&
				strings.Contains(errorMsg, "Could not find stored response for previous_response_id") {
				previousResponseRetryUsed = true
				lastResponseID = ""
				sessionLastResponseID = ""
				discardStatefulResponseIDForTurn = false
				statefulResetReason = "invalid_previous_response_id"
				appendChatEvent("system", "stateful.reset", map[string]interface{}{
					"reason": "invalid_previous_response_id",
				})
				if reqMap != nil {
					delete(reqMap, "previous_response_id")
					if newBody, marshalErr := json.Marshal(reqMap); marshalErr == nil {
						body = newBody
					}
				}
				chatSession.LastResponseID = ""
				AddDebugTrace("stateful", "reset", "Retrying request without invalid previous_response_id", map[string]interface{}{
					"user": userID,
					"turn": turn,
				})
				resp.Body.Close()
				continue
			}

			// Check for specific LM Studio auth error
			// We return a text starting with "LM_STUDIO_AUTH_ERROR:" so frontend can localize it.
			if resp.StatusCode == http.StatusUnauthorized || strings.Contains(errorMsg, "invalid_api_key") || strings.Contains(errorMsg, "Malformed LM Studio API token") {
				// Frontend will detect this prefix and show translated message
				http.Error(w, "LM_STUDIO_AUTH_ERROR: "+errorMsg, resp.StatusCode)
				return
			}

			// Check for MCP Permission Error (403)
			if resp.StatusCode == http.StatusForbidden && strings.Contains(errorMsg, "Permission denied to use plugin") {
				http.Error(w, "LM_STUDIO_MCP_ERROR: "+errorMsg, resp.StatusCode)
				return
			}

			// Check for Context Overflow Error
			if strings.Contains(errorMsg, "Context size has been exceeded") ||
				strings.Contains(errorMsg, "context_length_exceeded") ||
				strings.Contains(errorMsg, "exceeds the available context size") ||
				strings.Contains(errorMsg, "too many tokens") {
				log.Printf("[handleChat] LM Studio Context Limit Reached. Informing user.")
				sendSSEError(w, flusher, "LM_STUDIO_CONTEXT_ERROR: Context limit reached. Please clear the chat or use a larger context model.")
				return
			}

			// Check for Non-Vision Model Error
			if strings.Contains(errorMsg, "does not support image inputs") {
				log.Printf("[handleChat] Non-Vision Model Error detected. Informing user.")
				sendSSEError(w, flusher, "LM_STUDIO_VISION_ERROR: Model does not support images.")
				return
			}

			sendSSEError(w, flusher, fmt.Sprintf("LLM error: %s", errorMsg))
			return
		}
		AddDebugTrace("chat", "turn.response", "Upstream stream opened", map[string]interface{}{
			"turn":        turn,
			"status_code": resp.StatusCode,
			"elapsed_ms":  time.Since(turnStart).Milliseconds(),
		})

		// Tool Pattern Logic
		toolPattern := app.GetToolPattern(modelID)
		var toolRegex = (*regexp.Regexp)(nil)

		// Buffer for handling split tags (e.g., "<", "tool", "_call>")
		var partialTagBuffer string

		// Flag if we are in "buffering mode" (waiting for complete tool call)
		isBuffering := false
		var buffer string             // Declare buffer here
		var bufferingThreshold = 8000 // Buffer size (increased for large JSON args)

		if toolPattern != nil {
			if regexStr, ok := toolPattern["regex"]; ok {
				var err error
				toolRegex, err = regexp.Compile(regexStr)
				if err != nil {
					log.Printf("[handleChat] Invalid regex for model %s: %v", modelID, err)
					toolPattern = nil // Disable if invalid
				} else {
					isBuffering = true
					log.Printf("[handleChat] Enabled Custom Tool Parsing for model: %s", modelID)
				}
			}
		}

		// Extract messages for Memory Analysis (if enabled)
		if enableMemory {
			log.Printf("[handleChat-DEBUG] Request Body Snippet: %s", string(body)[:min(len(body), 500)])

			var tmpBody struct {
				Messages []map[string]interface{} `json:"messages"`
				Input    interface{}              `json:"input"` // Flexible for any type
			}
			if err := json.Unmarshal(body, &tmpBody); err == nil {
				if len(tmpBody.Messages) > 0 {
					messagesForMemory = tmpBody.Messages
					log.Printf("[handleChat-DEBUG] Extracted %d messages for memory", len(messagesForMemory))
				} else if inputStr, ok := tmpBody.Input.(string); ok && inputStr != "" {
					// Construct a single user message from Input
					messagesForMemory = []map[string]interface{}{
						{"role": "user", "content": inputStr},
					}
					log.Printf("[handleChat-DEBUG] Extracted 'input' string as user message for memory")
				} else {
					log.Printf("[handleChat-DEBUG] No valid 'messages' or 'input' string found in body")
				}
			} else {
				log.Printf("[handleChat-DEBUG] Failed to extract messages for memory: %v", err)
			}
		}

		scanner := bufio.NewScanner(resp.Body)
		log.Println("[handleChat-DEBUG] Starting response scanner loop")
		for scanner.Scan() {
			line := scanner.Text()

			// Log first few lines to debug stream format
			if len(fullResponse) < 100 && len(fullResponse) > 0 {
				// Don't log every line forever, just start
			}

			// CUSTOM PARSING LOGIC & SSE Handling
			// We process every line to handle both standard OpenAI and the custom format seen in logs
			trimmedLine := strings.TrimSpace(line)

			// Skip empty lines or comment lines (unless we need event types, which we might for the custom format)
			if trimmedLine == "" {
				continue
			}

			// Check for data: prefix
			if strings.HasPrefix(trimmedLine, "data: ") {
				dataStr := strings.TrimPrefix(trimmedLine, "data: ")

				// 1. Check for Standard OpenAI [DONE]
				if dataStr == "[DONE]" {
					log.Println("[handleChat-DEBUG] [DONE] signal received (Standard)")
					// If buffering, flush any remaining buffer as content before sending DONE
					if isBuffering && len(buffer) > 0 {
						payload := map[string]interface{}{
							"choices": []interface{}{
								map[string]interface{}{
									"delta": map[string]string{
										"content": buffer,
									},
								},
							},
						}
						jsonBytes, _ := json.Marshal(payload)
						emitStreamChunk(fmt.Sprintf("data: %s", string(jsonBytes)))
						fullResponse += buffer
						buffer = ""
					}

					// Just forward the DONE
					emitStreamChunk(line)
					continue
				}

				// 2. Parse JSON
				var chunk map[string]interface{}
				if err := json.Unmarshal([]byte(dataStr), &chunk); err == nil {

					// --- A. Handle Custom Format (type: "message.delta", etc) ---
					if msgType, ok := chunk["type"].(string); ok {
						if msgType == "message.delta" {
							if content, ok := chunk["content"].(string); ok {
								fullResponse += content

								// 🔍 Self-Evolution for Custom Format
								if toolPattern == nil && len(content) > 5 {
									lc := strings.ToLower(content)
									if strings.Contains(lc, "<|") ||
										strings.Contains(lc, "function") ||
										strings.Contains(lc, "tool") ||
										strings.Contains(lc, "execute") ||
										(strings.Contains(lc, "{\"") && strings.Contains(lc, "args")) {
										log.Printf("[handleChat] Invalid tool pattern detected in content (Custom Format): %s", content)
										needsCorrection = true
										badContentCapture = content // Capture the snippet for the prompt
									}
								}

								eventPayload := map[string]interface{}{}
								for key, value := range chunk {
									eventPayload[key] = value
								}
								eventPayload["full_content"] = fullResponse

								// Forward to client identical to source
								appendChatEvent("assistant", "message.delta", eventPayload)
								emitStreamChunk(line)
								continue
							}
						} else if msgType == "chat.end" || msgType == "message.end" {
							log.Printf("[handleChat-DEBUG] Custom End Signal Received: %s", msgType)

							// Capture response_id for chaining in stateful mode
							// Line is like: event: chat.end\ndata: {"result": {"response_id": "..."}}
							// We need to decode "data" part.
							// The 'line' variable here is the raw SSE line, e.g. "data: {...}"
							if strings.HasPrefix(line, "data: ") {
								jsonPart := strings.TrimPrefix(line, "data: ")
								var endPayload map[string]interface{}
								if err := json.Unmarshal([]byte(jsonPart), &endPayload); err == nil {
									if res, ok := endPayload["result"].(map[string]interface{}); ok {
										if rid, ok := res["response_id"].(string); ok && !discardStatefulResponseIDForTurn {
											lastResponseID = rid
											sessionLastResponseID = rid
											log.Printf("[handleChat] Captured response_id for chaining: %s", lastResponseID)
										}
										if stats, ok := res["stats"].(map[string]interface{}); ok {
											if inputTokens, ok := stats["input_tokens"].(float64); ok && inputTokens > 0 {
												statefulLastInputTokensValue = int(inputTokens)
												if statefulPeakInputTokensValue < statefulLastInputTokensValue {
													statefulPeakInputTokensValue = statefulLastInputTokensValue
												}
											}
											if outputTokens, ok := stats["total_output_tokens"].(float64); ok && outputTokens > 0 {
												statefulLastOutputTokensValue = int(outputTokens)
											}
										}
									}
								}
							}

							appendChatEvent("assistant", msgType, chunk)
							// Forward
							emitStreamChunk(line)
							continue
						} else if msgType == "error" {
							appendChatEvent("system", msgType, chunk)
							if errPayload, ok := chunk["error"].(map[string]interface{}); ok {
								errType, _ := errPayload["type"].(string)
								errMessage, _ := errPayload["message"].(string)
								if errType == "tool_format_generation_error" {
									discardStatefulResponseIDForTurn = true
									lastResponseID = ""
									sessionLastResponseID = ""
									appendChatEvent("assistant", "tool_call.failure", map[string]interface{}{
										"type":   "tool_call.failure",
										"tool":   "Tool",
										"reason": errMessage,
									})
									AddDebugTrace("chat", "tool.error", "Tool format generation error invalidated current stateful response chain", map[string]interface{}{
										"turn":  turn,
										"error": compactText(errMessage, 180),
									})
								}
							}
							emitStreamChunk(line)
							continue
						} else {
							appendChatEvent("system", msgType, chunk)
							// Forward other events (start, progress, etc)
							emitStreamChunk(line)
							continue
						}
					}

					// --- B. Handle Tool Pattern Logic (if enabled and buffering) ---
					if toolPattern != nil && isBuffering {
						// Extract content for buffering
						var content string
						if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
							if choice, ok := choices[0].(map[string]interface{}); ok {
								if delta, ok := choice["delta"].(map[string]interface{}); ok {
									if c, ok := delta["content"].(string); ok {
										content = c
									} else if rc, ok := delta["reasoning_content"].(string); ok {
										content = rc
									} else if r, ok := delta["reasoning"].(string); ok {
										content = r
									}
								} else if message, ok := choice["message"].(map[string]interface{}); ok {
									if c, ok := message["content"].(string); ok {
										content = c
									}
								}
							}
						}
						buffer += content

						// Check Regex
						matches := toolRegex.FindStringSubmatch(buffer)
						if len(matches) > 2 {
							// Match Found! (Group 1: Tool Name, Group 2: Args)
							toolName = matches[1]
							toolArgsStr = matches[2]

							// 🛠️ Command-R / GPT-OSS Name Extraction
							// If the tool name (Group 1) looks like a Command-R prefix, try to extract real name from "to=..."
							if strings.Contains(toolName, "<|channel|>") {
								reName := regexp.MustCompile(`to=([a-zA-Z0-9_]+)`)
								nameMatch := reName.FindStringSubmatch(toolName)
								if len(nameMatch) > 1 {
									toolName = nameMatch[1]
									// If generic "functions", we expect the JSON to have the real name (handled by Smart Parsing below)
								}
							}

							// Smart Parsing: Check if G2 is actually a wrapper object with "name" and "arguments"
							var wrapper struct {
								Name      string      `json:"name"`
								Arguments interface{} `json:"arguments"`
							}

							isWrapper := false
							if err := json.Unmarshal([]byte(toolArgsStr), &wrapper); err == nil {
								if wrapper.Name != "" && wrapper.Arguments != nil {
									toolName = wrapper.Name
									isWrapper = true
									log.Printf("[handleChat] Detected Wrapper JSON format. Extracted Tool: %s", toolName)
								}
							}

							log.Printf("[handleChat] Custom Tool Pattern Matched! Tool: %s", toolName)

							// 🛠️ Mark for execution after stream ends
							toolExecutedThisTurn = true
							lastToolName = toolName
							lastToolArgsStr = toolArgsStr

							// 1. Emit start event
							startEvt := map[string]string{
								"type": "tool_call.start",
								"tool": toolName,
							}
							startBytes, _ := json.Marshal(startEvt)
							appendChatEvent("assistant", "tool_call.start", startEvt)
							emitStreamChunk(fmt.Sprintf("data: %s", string(startBytes)))

							// 2. Emit arguments event
							if isWrapper {
								argsEvt := map[string]interface{}{
									"type":      "tool_call.arguments",
									"tool":      toolName,
									"arguments": wrapper.Arguments,
								}
								argsBytes, _ := json.Marshal(argsEvt)
								appendChatEvent("assistant", "tool_call.arguments", argsEvt)
								emitStreamChunk(fmt.Sprintf("data: %s", string(argsBytes)))
							} else {
								appendChatEvent("assistant", "tool_call.arguments", map[string]interface{}{
									"type":      "tool_call.arguments",
									"tool":      toolName,
									"arguments": toolArgsStr,
								})
								emitStreamChunk(fmt.Sprintf("data: {\"type\": \"tool_call.arguments\", \"tool\": \"%s\", \"arguments\": %s}", toolName, toolArgsStr))
							}

							// 3. Clear Buffer & Stop Buffering
							buffer = ""
							isBuffering = false
							continue // Tool call handled, move to next line
						}

						// If buffer too long, assume no match and flush
						if len(buffer) > bufferingThreshold {
							// 🔍 Self-Evolution Check
							lowerBuf := strings.ToLower(buffer)
							if (strings.Contains(lowerBuf, "function") || strings.Contains(lowerBuf, "call") || strings.Contains(lowerBuf, "tool")) &&
								(strings.Contains(lowerBuf, "{") && strings.Contains(lowerBuf, "}")) {
								log.Printf("[handleChat] Invalid tool pattern detected in buffer: %s", buffer)
								needsCorrection = true
								badContentCapture = buffer
							}

							// Flush buffer as regular content
							payload := map[string]interface{}{
								"choices": []interface{}{
									map[string]interface{}{
										"delta": map[string]string{
											"content": buffer,
										},
									},
								},
							}
							jsonBytes, _ := json.Marshal(payload)
							emitStreamChunk(fmt.Sprintf("data: %s", string(jsonBytes)))
							fullResponse += buffer // Add to full response
							buffer = ""
						}
						continue // Buffering logic handled, move to next line
					}

					// --- C. Handle Standard OpenAI Format (if not custom and not tool-buffered) ---
					if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]interface{}); ok {
							if delta, ok := choice["delta"].(map[string]interface{}); ok {
								reasoningText := ""
								if rc, ok := delta["reasoning_content"].(string); ok {
									reasoningText = rc
								} else if r, ok := delta["reasoning"].(string); ok {
									reasoningText = r
								}
								if reasoningText != "" {
									if !reasoningActive {
										reasoningActive = true
										reasoningStartedAt = time.Now()
										appendChatEvent("assistant", "reasoning.start", map[string]interface{}{
											"type":       "reasoning.start",
											"started_at": reasoningStartedAt.Format(time.RFC3339Nano),
										})
									}
									appendChatEvent("assistant", "reasoning.delta", map[string]interface{}{
										"type":       "reasoning.delta",
										"content":    reasoningText,
										"elapsed_ms": time.Since(reasoningStartedAt).Milliseconds(),
									})
								}
								if c, ok := delta["content"].(string); ok {
									if reasoningActive {
										appendChatEvent("assistant", "reasoning.end", map[string]interface{}{
											"type":       "reasoning.end",
											"elapsed_ms": time.Since(reasoningStartedAt).Milliseconds(),
										})
										reasoningActive = false
										reasoningStartedAt = time.Time{}
									}
									fullResponse += c
									appendChatEvent("assistant", "message.delta", map[string]interface{}{
										"type":         "message.delta",
										"content":      c,
										"full_content": fullResponse,
									})
								}
							}
						}
					}

					// --- D. Self-Evolution for non-buffered models ---
					// This block was previously in an `else if strings.HasPrefix(strings.TrimSpace(line), "data: ")`
					// It should now be integrated here, after content extraction but before forwarding.
					if toolPattern == nil { // Only if no tool pattern is active
						var content string
						if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
							if choice, ok := choices[0].(map[string]interface{}); ok {
								if delta, ok := choice["delta"].(map[string]interface{}); ok {
									if c, ok := delta["content"].(string); ok {
										content = c
									} else if rc, ok := delta["reasoning_content"].(string); ok {
										content = rc
									} else if r, ok := delta["reasoning"].(string); ok {
										content = r
									}
								}
							}
						}

						// 🛠️ Structured Output Support: Force buffering if start of JSON object is detected
						if !isBuffering && len(fullResponse) < 50 && strings.HasPrefix(strings.TrimSpace(content), "{") {
							log.Printf("[handleChat] Detected potential JSON start. Switching to buffering mode.")
							isBuffering = true
							buffer = content
							flusher.Flush()
							continue
						}

						// 🧪 Special Handling for Command-R / GPT-OSS Format (<|channel|>)
						if !isBuffering && (strings.Contains(content, "<|channel|>") || strings.Contains(content, "<|tool_code|>") || strings.Contains(content, "<tool_call>")) {
							log.Printf("[handleChat] Detected Command-R/GPT-OSS/Qwen Tool Call Pattern. Switching to buffering mode.")
							isBuffering = true
							buffer = content

							if strings.Contains(content, "<tool_call>") {
								// Qwen Style: <tool_call>{JSON}</tool_call>
								toolPattern = map[string]string{"format": "qwen"}
								// Regex: Group 1 (Dummy/Name) + Group 2 (JSON)
								toolRegex = regexp.MustCompile(`(?s)(<tool_call>)\s*(\{[\s\S]*?\})\s*</tool_call>`)
							} else {
								// Command-R / GPT-OSS Style
								// Define a regex that captures the prefix as Group 1 (ignored) and the JSON as Group 2
								// Pattern: <|channel|>...<|message|> { JSON }
								toolPattern = map[string]string{"format": "command-r"}
								toolRegex = regexp.MustCompile(`(?s)(<\|channel\|>.*?<\|message\|>)\s*(\{[\s\S]*\})`)
							}
							flusher.Flush()
							continue
						}

						// 🔍 Self-Evolution for non-buffered models
						if len(content) > 5 {
							lc := strings.ToLower(content)
							// Broaden trigger keywords: <|, function, tool, execute, {"name":, and tool names
							hasToolName := strings.Contains(lc, "search_web") ||
								strings.Contains(lc, "personal_memory") ||
								strings.Contains(lc, "read_user_document") ||
								strings.Contains(lc, "read_web_page") ||
								strings.Contains(lc, "read_buffered_source")

							if strings.Contains(lc, "<|") ||
								strings.Contains(lc, "function") ||
								strings.Contains(lc, "tool") ||
								strings.Contains(lc, "execute") ||
								hasToolName ||
								(strings.Contains(lc, "{\"") && strings.Contains(lc, "args")) {

								// 🧬 Anti-Recursion & Meta-Content Filter
								// We want to skip learning if the content is:
								// 1. A regex definition (has "(?s)", "regex")
								// 2. A code block explanation of a tool call (has "tool_call[")
								// 3. A JSON definition inside a markdown block (often starts with ```json) - though hard to detect in fragments
								// 4. Broken/Partial JSON that is clearly just text discussion
								if strings.Contains(lc, "(?s)") ||
									strings.Contains(lc, "regex") ||
									strings.Contains(lc, "tool_call[") || // Catch array/map access style which is code, not natural language
									strings.Contains(lc, "tool_call [") ||
									strings.Contains(lc, "```") {
									log.Printf("[handleChat] Self-Evolution trigger skipped: detected meta-content (regex/code)")
								} else {
									// Double check: if it's just a tool name mention without execution context, skip
									// Real execution usually involves JSON-like structure "{" or special tokens "<|"
									isRealExecution := strings.Contains(lc, "<|") || (strings.Contains(lc, "{") && strings.Contains(lc, ":"))

									if isRealExecution {
										log.Printf("[handleChat] Invalid tool pattern detected in content: %s", content)
										// 🛡️ REPLACEMENT: Instead of Self-Evolution (which loops), mark for Self-Correction
										needsCorrection = true
										badContentCapture = content // Capture the snippet for the prompt
									} else {
										// If it's just a word "tool" or "function" without structure, ignore
									}
								}
							}
						}
					}
				}

				// Forward the line (if not already continued by custom format or tool logic)
				emitStreamChunk(line)

			} else {
				// Check for raw error JSON (not prefixed with data:)
				// e.g. {"error":{"message":"Context size has been exceeded.",...}}
				if strings.HasPrefix(line, "{") && strings.Contains(line, "\"error\"") {
					log.Printf("[handleChat] Detected Raw JSON Error in stream: %s", line)
					if strings.Contains(line, "Context size has been exceeded") || strings.Contains(line, "context_length_exceeded") {
						// Send explicit known error event
						// We use a custom event type or just an error field that app.js will pick up
						emitStreamChunk("data: {\"error\": \"LM_STUDIO_CONTEXT_ERROR: Context size exceeded.\"}")
						return // Stop processing
					}
				}

				// Forward non-data lines (e.g. event: ...)
				emitStreamChunk(line)
			}
		}

		resp.Body.Close() // Explicit close after scanner is done with this turn

		if err := scanner.Err(); err != nil {
			log.Printf("[handleChat] Stream scanner error: %v", err)
		}

		// 🛠️ Structured Output Support (JSON)
		// Check if buffer looks like a complete JSON object from a Structured Output model
		// Pattern: {"thought": "...", "tool_name": "...", "tool_arguments": ...}
		if isBuffering || (strings.HasPrefix(strings.TrimSpace(buffer), "{") && strings.Contains(buffer, "\"tool_name\"")) {
			var structTool StructuredToolCall
			if err := json.Unmarshal([]byte(buffer), &structTool); err == nil {
				if structTool.ToolName != "" {
					log.Printf("[handleChat] Detected Structured JSON Tool Call: %s", structTool.ToolName)
					toolName = structTool.ToolName

					// Convert arguments back to JSON string for consistent handling
					if argsBytes, err := json.Marshal(structTool.ToolArguments); err == nil {
						toolArgsStr = string(argsBytes)
					} else {
						toolArgsStr = "{}"
					}

					toolExecutedThisTurn = true
					lastToolName = toolName
					lastToolArgsStr = toolArgsStr

					// Emit events to frontend
					startEvt := map[string]string{
						"type": "tool_call.start",
						"tool": toolName,
					}
					startBytes, _ := json.Marshal(startEvt)
					emitStreamChunk(fmt.Sprintf("data: %s", string(startBytes)))

					argsEvt := map[string]interface{}{
						"type":      "tool_call.arguments",
						"tool":      toolName,
						"arguments": structTool.ToolArguments,
					}
					argsBytes, _ := json.Marshal(argsEvt)
					emitStreamChunk(fmt.Sprintf("data: %s", string(argsBytes)))

					// Clear buffer and stop any further buffering
					buffer = ""
					isBuffering = false
					continue
				}
			}
		}

		// 🛠️ FINAL BUFFER FLUSH: If we were buffering and the stream ended, flush what's left.
		if isBuffering && len(buffer) > 0 {
			log.Printf("[handleChat] Final buffer flush triggered (Stream End)")
			payload := map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"delta": map[string]string{
							"content": buffer,
						},
					},
				},
			}
			jsonBytes, _ := json.Marshal(payload)
			emitStreamChunk(fmt.Sprintf("data: %s", string(jsonBytes)))
			fullResponse += buffer
			lastSavedBufferForTurn = buffer // Save for history before clearing
			buffer = ""
		} else if len(partialTagBuffer) > 0 {
			// Flush partial tag buffer if stream ends
			payload := map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"delta": map[string]string{
							"content": partialTagBuffer,
						},
					},
				},
			}
			jsonBytes, _ := json.Marshal(payload)
			emitStreamChunk(fmt.Sprintf("data: %s", string(jsonBytes)))
			fullResponse += partialTagBuffer
			partialTagBuffer = ""
		}

		log.Printf("[handleChat-DEBUG] turn %d processing complete. Total response len: %d", turn, len(fullResponse))
		if reasoningActive {
			appendChatEvent("assistant", "reasoning.end", map[string]interface{}{
				"type":       "reasoning.end",
				"elapsed_ms": time.Since(reasoningStartedAt).Milliseconds(),
			})
			reasoningActive = false
			reasoningStartedAt = time.Time{}
		}

		// 🛡️ TOOL EXECUTION & LOOP LOGIC
		if toolExecutedThisTurn {
			log.Printf("[handleChat] Turn %d detected Tool Call: %s. Executing...", turn, lastToolName)
			AddDebugTrace("chat", "tool.detected", "Tool call detected in assistant output", map[string]interface{}{
				"turn": turn,
				"tool": lastToolName,
				"args": compactText(lastToolArgsStr, 200),
			})

			// 1. Execute Tool
			toolStart := time.Now()
			toolUsageCounts[lastToolName]++
			toolSig := lastToolName + ":" + compactText(strings.TrimSpace(lastToolArgsStr), 240)
			toolSignatureCounts[toolSig]++

			var result string
			var err error
			toolSkipped := false
			if (lastToolName == "search_web" || lastToolName == "naver_search") && toolUsageCounts[lastToolName] > 3 {
				result = fmt.Sprintf("Tool budget reached for %s. Do not search again in this answer. Use the evidence already buffered and answer the user directly.", lastToolName)
				toolSkipped = true
				if procCtx != nil {
					procCtx.FallbackUsed = true
					procCtx.RepeatedBlocked = true
				}
				AddDebugTrace("chat", "tool.skipped", "Skipped repeated web search due to per-request budget", map[string]interface{}{
					"turn":  turn,
					"tool":  lastToolName,
					"count": toolUsageCounts[lastToolName],
				})
			} else if lastToolName == "read_web_page" && toolUsageCounts[lastToolName] > 2 {
				result = "read_web_page already ran multiple times in this answer. Avoid more page reads unless the user explicitly asks to retry. Answer from buffered search evidence or use read_buffered_source."
				toolSkipped = true
				if procCtx != nil {
					procCtx.FallbackUsed = true
					procCtx.RepeatedBlocked = true
				}
				AddDebugTrace("chat", "tool.skipped", "Skipped repeated page read due to per-request budget", map[string]interface{}{
					"turn":  turn,
					"tool":  lastToolName,
					"count": toolUsageCounts[lastToolName],
				})
			} else if toolSignatureCounts[toolSig] > 1 {
				result = fmt.Sprintf("Duplicate tool call prevented for %s with near-identical arguments. Use existing buffered evidence and continue answering.", lastToolName)
				toolSkipped = true
				if procCtx != nil {
					procCtx.FallbackUsed = true
					procCtx.RepeatedBlocked = true
				}
				AddDebugTrace("chat", "tool.skipped", "Skipped duplicate tool call with same arguments", map[string]interface{}{
					"turn":  turn,
					"tool":  lastToolName,
					"count": toolSignatureCounts[toolSig],
				})
			} else {
				result, err = mcp.ExecuteToolByName(lastToolName, []byte(lastToolArgsStr), userID, enableMemory, disabledTools)
			}
			var toolResultEvt map[string]interface{}
			if err != nil {
				log.Printf("[handleChat] Tool Execution Failed: %v", err)
				AddDebugTrace("chat", "tool.error", "Tool execution failed", map[string]interface{}{
					"turn":       turn,
					"tool":       lastToolName,
					"elapsed_ms": time.Since(toolStart).Milliseconds(),
					"error":      err.Error(),
				})
				toolResultEvt = map[string]interface{}{
					"type":   "tool_call.failure",
					"tool":   lastToolName,
					"reason": err.Error(),
				}
				result = fmt.Sprintf("Error executing tool %s: %v", lastToolName, err)
			} else {
				log.Printf("[handleChat] Tool Execution Success.")
				AddDebugTrace("chat", "tool.success", "Tool execution completed", map[string]interface{}{
					"turn":         turn,
					"tool":         lastToolName,
					"elapsed_ms":   time.Since(toolStart).Milliseconds(),
					"result_chars": len(result),
				})
				toolResultEvt = map[string]interface{}{
					"type": "tool_call.success",
					"tool": lastToolName,
				}
			}
			if procCtx != nil {
				procCtx.AddToolEvent(lastToolName, lastToolArgsStr, time.Since(toolStart), err == nil, toolSkipped)
				if err != nil {
					procCtx.FallbackUsed = true
				}
			}

			// Emit Result Event to Frontend
			resBytes, _ := json.Marshal(toolResultEvt)
			appendChatEvent("assistant", fmt.Sprintf("%v", toolResultEvt["type"]), toolResultEvt)
			emitStreamChunk(fmt.Sprintf("data: %s", string(resBytes)))

			// 2. Prepare Follow-up Request
			// We feed the result back as a hidden user message or a tool response if the model supports it.
			// For consistency across modes, we'll use a simulated message.

			if llmMode == "stateful" {
				// Stateful: use previous_response_id of the JUST FINISHED assistant turn
				if lastResponseID == "" {
					log.Printf("[handleChat] WARNING: No lastResponseID captured for turn %d. Multi-turn might break.", turn)
				}
				reqMap = map[string]interface{}{
					"model":                modelID,
					"input":                compactToolResult(lastToolName, result),
					"previous_response_id": lastResponseID,
					"stream":               true,
				}
			} else {
				// Standard: Append Assistant turn and Tool Result turn
				msgs, _ := reqMap["messages"].([]interface{})
				// Add what the assistant just said (the tool call) - use the saved buffer
				msgs = append(msgs, map[string]interface{}{
					"role":    "assistant",
					"content": compactText(lastSavedBufferForTurn, 400),
				})
				// Add the result
				msgs = append(msgs, map[string]interface{}{
					"role":    "user",
					"content": compactToolResult(lastToolName, result),
				})
				reqMap["messages"] = msgs
			}

			// Reinject integrations for the next turn
			if enableMCP {
				reqMap["integrations"] = []string{"mcp/dinkisstyle-gateway"}
			}

			// Update body for next turn
			body, _ = json.Marshal(reqMap)
			AddDebugTrace("chat", "turn.followup", "Prepared follow-up turn with tool result", map[string]interface{}{
				"turn":       turn,
				"tool":       lastToolName,
				"body_bytes": len(body),
			})
			continue // Start next turn loop
		}

		// If no tool executed, we are done with all turns
		AddDebugTrace("chat", "turn.complete", "Turn completed without additional tool recursion", map[string]interface{}{
			"turn":           turn,
			"elapsed_ms":     time.Since(turnStart).Milliseconds(),
			"response_chars": len(fullResponse),
		})
		break
	} // --- TURN LOOP END ---

	// 🛡️ SELF-CORRECTION TRIGGER (Only if we didn't loop or at the very end)
	if needsCorrection && badContentCapture != "" {
		log.Printf("[handleChat] Triggering Self-Correction for invalid tool format...")
		if procCtx != nil {
			procCtx.SelfCorrectionUsed = true
			procCtx.FallbackUsed = true
		}
		AddDebugTrace("chat", "self_correction.start", "Triggering tool-call self-correction", map[string]interface{}{
			"snippet": compactText(badContentCapture, 180),
		})

		// Prepare Correction Request
		correctionPrompt := mcp.SelfCorrectionPromptTemplate(badContentCapture)
		var correctionReq map[string]interface{}

		// Determine if we are in stateful mode or standard mode
		// We need to re-parse body or re-use reqMap if available.
		// Since reqMap was local to an if block earlier, we might not have it here.
		// We will reconstruct a minimal valid request based on llmMode.

		if llmMode == "stateful" {
			// Stateful: just send input and previous_response_id
			// We need the response ID from the JUST FINISHED stream.
			// It was in the 'chat.end' event: "response_id": "resp_..."
			// However, capturing it from the stream is hard without parsing every chunk JSON.
			// Fallback: Just send a new message with the SAME previous_response_id as the original request,
			// effectively branching or continuing.
			// But original 'reqMap' is not in scope here. We need to parse 'body' again or lift 'reqMap' scope.
			// Since parsing is cheap, let's re-parse 'body' to get previous_id.
			var tempMap map[string]interface{}
			if err := json.Unmarshal(body, &tempMap); err == nil {
				// Use the lastResponseID from the just-completed turn if available
				// This chains the correction AFTER the bad response.
				correctionReq = map[string]interface{}{
					"model":       modelID,
					"input":       correctionPrompt, // Just the prompt
					"stream":      true,
					"temperature": 0.1,
				}

				if enableMCP {
					correctionReq["integrations"] = []string{"mcp/dinkisstyle-gateway"}
				}

				if lastResponseID != "" {
					correctionReq["previous_response_id"] = lastResponseID
				} else {
					// Fallback: fork from original parent
					if pid, ok := tempMap["previous_response_id"].(string); ok && pid != "" {
						correctionReq["previous_response_id"] = pid
					}
				}
			}
		} else {
			// Standard/Stateless: Use a minimal correction request instead of replaying the full conversation.
			correctionReq = map[string]interface{}{
				"model": modelID,
				"messages": []map[string]string{
					{"role": "system", "content": "Return only the corrected tool call or plain answer."},
					{"role": "user", "content": correctionPrompt},
				},
				"stream":      true,
				"temperature": 0.1,
			}
			if enableMCP {
				correctionReq["integrations"] = []string{"mcp/dinkisstyle-gateway"}
			}
		}

		if correctionReq != nil {
			jsonPayload, _ := json.Marshal(correctionReq)

			// Use 'url' which is defined in handleChat scope (we need to verify this variable name)
			// Looking at code, 'url' variable holds the endpoint.
			// If 'url' is not available, we reconstruct it:
			targetURL := app.llmEndpoint
			if !strings.HasSuffix(targetURL, "/v1/chat/completions") && !strings.HasSuffix(targetURL, "/api/v1/chat") {
				// Basic fix, though precise path depends on mode.
				// Ideally we use a variable that holds the valid endpoint used earlier.
				// Let's assume 'app.llmEndpoint' + appropriate suffix if needed, or better:
				// The variable 'url' IS usually defined in handleChat. Let's check previous context.
				// In `handleChat`:
				// url := app.llmEndpoint
				// So we can use 'url'.
			}
			// Force valid URL for safety if 'url' is not in scope of this block (it should be)
			// Actually, to be safe against scope issues, we use 'app.llmEndpoint' and fix path.
			reqUrl := app.llmEndpoint
			if llmMode == "stateful" && !strings.Contains(reqUrl, "chat") {
				reqUrl = strings.TrimSuffix(reqUrl, "/") + "/api/v1/chat"
			} else if !strings.Contains(reqUrl, "chat") {
				reqUrl = strings.TrimSuffix(reqUrl, "/") + "/v1/chat/completions"
			}

			req, _ := http.NewRequest("POST", reqUrl, bytes.NewBuffer(jsonPayload))
			req.Header.Set("Content-Type", "application/json")
			if app.llmApiToken != "" {
				req.Header.Set("Authorization", "Bearer "+app.llmApiToken)
			}

			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				correctionScanner := bufio.NewScanner(resp.Body)
				for correctionScanner.Scan() {
					line := correctionScanner.Text()
					if strings.HasPrefix(line, "data: ") {
						dataStr := strings.TrimPrefix(line, "data: ")
						if dataStr != "[DONE]" {
							var chunk map[string]interface{}
							if err := json.Unmarshal([]byte(dataStr), &chunk); err == nil {
								if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
									if choice, ok := choices[0].(map[string]interface{}); ok {
										if delta, ok := choice["delta"].(map[string]interface{}); ok {
											if c, ok := delta["content"].(string); ok {
												fullResponse += c
											}
										}
									}
								}
							}
						}
						fmt.Fprintf(w, "%s\n\n", line)
						flusher.Flush()
					}
				}
			} else {
				log.Printf("[handleChat] Self-Correction Request Failed: %v", err)
				AddDebugTrace("chat", "self_correction.error", "Self-correction request failed", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
	}

	// 🔍 FINAL Memory Logging: Catch everything after all turns and corrections
	if enableMemory && len(messagesForMemory) > 0 && fullResponse != "" {
		log.Printf("[handleChat] Final Assistant Response Captured (Len: %d). Logging to DB...", len(fullResponse))
		go logChatToHistory(userID, messagesForMemory, fullResponse, modelID)
	}
	if procCtx != nil {
		procCtx.Success = strings.TrimSpace(fullResponse) != ""
		persistRequestExecution(procCtx)
	}
	if llmMode == "stateful" {
		if strings.TrimSpace(fullResponse) != "" || strings.TrimSpace(sessionLastResponseID) != "" {
			statefulTurnCountValue += 1
			statefulEstimatedCharsValue += len([]rune(strings.TrimSpace(initialUserInputText))) + len([]rune(strings.TrimSpace(fullResponse)))
		}
		if statefulLastInputTokensValue <= 0 {
			statefulLastInputTokensValue = estimateStatefulTokens(initialUserInputText) + estimateStatefulTokens(statefulSummaryText)
		}
		if statefulLastOutputTokensValue <= 0 {
			statefulLastOutputTokensValue = estimateStatefulTokens(fullResponse)
		}
		if statefulPeakInputTokensValue < statefulLastInputTokensValue {
			statefulPeakInputTokensValue = statefulLastInputTokensValue
		}
		statefulTurnCount = strconv.Itoa(statefulTurnCountValue)
		statefulEstChars = strconv.Itoa(statefulEstimatedCharsValue)
		statefulInputTokens = strconv.Itoa(statefulLastInputTokensValue)
		statefulPeakInputTokens = strconv.Itoa(statefulPeakInputTokensValue)
		statefulTokenBudget = strconv.Itoa(statefulTokenBudgetValue)
		statefulRiskScore = strconv.FormatFloat(statefulRiskScoreValue, 'f', -1, 64)
		statefulRiskLevel = statefulRiskLevelValue
	}
	sessionStatus = "idle"
	appendChatEvent("assistant", "request.complete", map[string]interface{}{
		"response_chars": len(fullResponse),
		"response_id":    sessionLastResponseID,
		"mode":           llmMode,
		"elapsed_ms":     time.Since(requestStart).Milliseconds(),
		"turn_id":        clientTurnID,
	})
	AddDebugTrace("chat", "request.complete", "Chat request finished", map[string]interface{}{
		"user":           userID,
		"elapsed_ms":     time.Since(requestStart).Milliseconds(),
		"response_chars": len(fullResponse),
		"memory_logged":  enableMemory && len(messagesForMemory) > 0 && fullResponse != "",
		"stateful_turns": statefulTurnCount,
		"stateful_chars": statefulEstChars,
		"risk_score":     statefulRiskScore,
		"risk_level":     statefulRiskLevel,
		"__payload":      fullResponse,
	})
}

// handleTTS converts text to speech using Supertonic
func handleTTS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Text       string  `json:"text"`
		Lang       string  `json:"lang"`
		ChunkSize  int     `json:"chunkSize"`
		VoiceStyle string  `json:"voiceStyle"`
		Speed      float32 `json:"speed"`
		Format     string  `json:"format"` // "wav" or "mp3"
		Steps      int     `json:"steps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Lang == "" {
		req.Lang = "ko"
	}
	if req.Format == "" {
		req.Format = "wav" // Default to WAV for backward compatibility
	}

	// Check if TTS is initialized
	globalTTSMutex.RLock()
	ttsInstance := globalTTS
	globalTTSMutex.RUnlock()

	if ttsInstance == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "not_available",
			"message": "TTS not initialized. Check assets folder.",
		})
		return
	}

	// Load voice style from config or request
	styleName := ttsConfig.VoiceStyle
	if req.VoiceStyle != "" {
		styleName = req.VoiceStyle
	}
	if !strings.HasSuffix(styleName, ".json") {
		styleName += ".json"
	}
	voiceStylePath := GetResourcePath(filepath.Join("assets", "voice_styles", styleName))

	// Check Cache
	styleMutex.Lock()
	style, found := styleCache[styleName]
	styleMutex.Unlock()

	if !found {
		// Load if not in cache
		loadedStyle, err := LoadVoiceStyle(voiceStylePath)
		if err != nil {
			log.Printf("Failed to load voice style %s: %v", styleName, err)
			http.Error(w, "Failed to load voice style", http.StatusInternalServerError)
			return
		}

		styleMutex.Lock()
		// Double check locking (standard double-checked locking pattern not strictly needed for this scale but safe)
		if cached, ok := styleCache[styleName]; ok {
			loadedStyle.Destroy() // discard duplicate
			style = cached
		} else {
			styleCache[styleName] = loadedStyle
			style = loadedStyle
		}
		styleMutex.Unlock()
		log.Printf("Loaded and cached voice style: %s", styleName)
	}

	// Do NOT destroy style here, it is cached for lifetime of app (or until explicit clear)
	// defer style.Destroy() <--- REMOVED

	// Generate speech using configured speed
	speed := ttsConfig.Speed
	if req.Speed > 0 {
		speed = req.Speed
	}
	steps := 5
	if req.Steps > 0 {
		steps = req.Steps
		if steps > 50 {
			steps = 50
		}
	}
	globalTTSMutex.RLock()
	if globalTTS == nil {
		globalTTSMutex.RUnlock()
		http.Error(w, "TTS not initialized", http.StatusInternalServerError)
		return
	}
	// Use globalTTS directly while holding the lock to prevent destruction
	wavData, _, err := globalTTS.Call(r.Context(), req.Text, req.Lang, style, steps, speed, req.ChunkSize)
	sampleRate := globalTTS.SampleRate
	globalTTSMutex.RUnlock()

	if err != nil {
		log.Printf("TTS failed: %v", err)
		http.Error(w, "TTS generation failed", http.StatusInternalServerError)
		return
	}

	// Generate audio bytes in requested format
	audioBytes, contentType, err := GenerateAudio(wavData, sampleRate, req.Format)
	if err != nil {
		log.Printf("Audio generation failed: %v", err)
		http.Error(w, "Audio generation failed", http.StatusInternalServerError)
		return
	}

	// Return audio
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(audioBytes)))

	startTransfer := time.Now()
	n, err := w.Write(audioBytes)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	elapsedTransfer := time.Since(startTransfer)

	if err != nil {
		log.Printf("[TTS] Network transfer failed after %d bytes: %v", n, err)
	} else {
		log.Printf("[TTS] Network transfer complete: %d bytes sent in %v", n, elapsedTransfer)
	}
}

// handleTTSStyles returns list of available voice styles
func handleTTSStyles(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(GetResourcePath(filepath.Join("assets", "voice_styles")))
	if err != nil {
		http.Error(w, "Failed to read styles directory", http.StatusInternalServerError)
		return
	}

	var styles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
			styles = append(styles, strings.TrimSuffix(f.Name(), ".json"))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(styles)
}

// Global TTS instance

// InitTTS initializes the TTS engine
func InitTTS(assetsDir string, threads int) error {
	onnxDir := assetsDir + "/onnx"

	// Check if TTS files exist
	if _, err := os.Stat(onnxDir + "/vocoder.onnx"); os.IsNotExist(err) {
		log.Println("TTS assets not found, TTS disabled")
		return nil
	}

	// Initialize ONNX Runtime (idempotent, safe to call multiple times or check internal flag)
	if err := InitializeONNXRuntime(); err != nil {
		return fmt.Errorf("failed to initialize ONNX Runtime: %w", err)
	}

	// Load TTS config
	cfg, err := LoadTTSConfig(onnxDir)
	if err != nil {
		return fmt.Errorf("failed to load TTS config: %w", err)
	}

	// Load TTS models
	// Note: Loading takes time, do it before acquiring lock
	tts, err := LoadTextToSpeech(onnxDir, cfg, threads)
	if err != nil {
		return fmt.Errorf("failed to load TTS: %w", err)
	}

	// Swap instances
	globalTTSMutex.Lock()
	defer globalTTSMutex.Unlock()

	if globalTTS != nil {
		globalTTS.Destroy()
	}

	globalTTS = tts
	log.Printf("TTS initialized successfully (Threads: %d)", threads)
	return nil
}

// logChatToHistory appends the latest chat turn to a log file for async processing.
func logChatToHistory(userID string, messages []map[string]interface{}, assistantResponse string, modelID string) {
	log.Printf("[AsyncMemory] logChatToHistory called for user: %s, model: %s, msgs: %d", userID, modelID, len(messages))

	// 1. Find the last user message
	var lastUserMsg string
	for i := len(messages) - 1; i >= 0; i-- {
		if role, ok := messages[i]["role"].(string); ok && role == "user" {
			if content, ok := messages[i]["content"].(string); ok {
				lastUserMsg = content
				break
			}
		}
	}

	if lastUserMsg == "" {
		return
	}

	// 2. Real-time Raw Memory Storage: Insert directly into SQLite
	fullContext := fmt.Sprintf("User: %s\nAssistant: %s", lastUserMsg, assistantResponse)

	id, err := mcp.InsertMemory(userID, fullContext)
	if err != nil {
		log.Printf("[AsyncMemory] ❌ Failed to insert raw memory to DB: %v", err)
	} else {
		log.Printf("[AsyncMemory] ✅ Saved raw interaction to DB (ID: %d) for user %s", id, userID)
	}
}

// sendSSEError sends a properly formatted SSE error event to the client
func sendSSEError(w http.ResponseWriter, flusher http.Flusher, msg string) {
	payload := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"delta": map[string]string{
					"content": "\n\n❌ **Error:** " + msg + "\n",
				},
			},
		},
	}
	data, _ := json.Marshal(payload)
	fmt.Fprintf(w, "data: %s\n\n", string(data))
	flusher.Flush()

	// Also send a specialized error event if the frontend supports it
	fmt.Fprintf(w, "event: error\ndata: %s\n\n", msg)
	flusher.Flush()
}

func lenInterfaceSlice(v interface{}) int {
	items, ok := v.([]interface{})
	if !ok {
		return 0
	}
	return len(items)
}

func prettyJSONForDebug(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}

	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw)
	}

	pretty, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(pretty)
}
