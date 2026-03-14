package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dinkisstyle-chat/mcp"
)

// MemoryAnalysisRequest represents the payload for memory extraction
type MemoryAnalysisRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Store       bool      `json:"store"`
}

// Checkpoint stores the last processed offset of the chat log
type Checkpoint struct {
	Offset int64 `json:"offset"`
}

// StartMemoryWorker initializes the background memory processing worker
func (a *App) StartMemoryWorker() {
	log.Println("[MemoryWorker] 🧠 Async Memory Worker initialized.")

	log.Println("[MemoryWorker] 🧠 Async Memory Worker started.")

	// Run immediately once (to catch up), then periodically
	go func() {
		log.Println("[MemoryWorker] ⏳ Worker goroutine started, waiting 10s for initial run...")
		// Initial delay to let app startup finish
		time.Sleep(10 * time.Second)
		log.Println("[MemoryWorker] 🚀 Performing initial memory scan...")
		a.scanAndProcessMemories()

		ticker := time.NewTicker(3 * time.Minute) // Check every 3 minutes
		defer ticker.Stop()

		for range ticker.C {
			log.Println("[MemoryWorker] ⏰ Ticker fired: starting scheduled memory scan...")
			a.scanAndProcessMemories()
		}
	}()
}

// scanAndProcessMemories iterates over all user memory directories and processes logs
func (a *App) scanAndProcessMemories() {
	log.Println("[MemoryWorker] 🚀 Starting memory scan...")
	// Get base memory directory using a dummy/default user to find the root
	sampleDir, err := mcp.GetUserMemoryDir("default")
	if err != nil {
		log.Printf("[MemoryWorker] ❌ Failed to get memory base dir: %v", err)
		return
	}
	baseDir := filepath.Dir(sampleDir)

	log.Printf("[MemoryWorker] 📁 Scanning base directory: %s", baseDir)

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		log.Printf("[MemoryWorker] ❌ Failed to read base dir %s: %v", baseDir, err)
		return
	}

	log.Printf("[MemoryWorker] 👥 Found %d entries in memory directory", len(entries))

	var wg sync.WaitGroup
	for _, entry := range entries {
		if entry.IsDir() {
			userID := entry.Name()
			log.Printf("[MemoryWorker] ⚡ Queuing log processing for user: %s", userID)
			wg.Add(1)
			go func(uid string) {
				defer wg.Done()
				a.processChatLog(uid)
			}(userID)
		}
	}
	wg.Wait()
	log.Println("[MemoryWorker] ✅ Scan and process complete.")
}

// processChatLog reads new entries from chat_history.log and extracts facts
func (a *App) processChatLog(userID string) {
	// Check user-specific memory setting
	a.authMgr.mu.RLock()
	user, exists := a.authMgr.users[userID]
	a.authMgr.mu.RUnlock()

	if !exists {
		return // Skip if not a valid user directory
	}

	enabled := true // Default to true if not specified? Or false.
	if user.Settings.EnableMemory != nil {
		enabled = *user.Settings.EnableMemory
	}

	if !enabled {
		log.Printf("[MemoryWorker] [%s] ℹ️ Memory disabled for this user, skipping.", userID)
		return
	}

	log.Printf("[MemoryWorker] 📄 processChatLog for user: %s", userID)
	logPath, err := mcp.GetUserMemoryFilePath(userID, "chat_history.log")
	if err != nil {
		log.Printf("[MemoryWorker] [%s] ❌ Failed to get log path: %v", userID, err)
		return
	}
	processingPath := logPath + ".processing"

	// 1. Check if .processing file already exists (from crash/unfinished run)
	// If not, try to rename current log to processing
	if _, err := os.Stat(processingPath); os.IsNotExist(err) {
		// No processing file. Check if main log exists and has data.
		info, err := os.Stat(logPath)
		if os.IsNotExist(err) {
			log.Printf("[MemoryWorker] [%s] ℹ️ chat_history.log not found.", userID)
			return
		}
		if info.Size() == 0 {
			log.Printf("[MemoryWorker] [%s] ℹ️ chat_history.log is empty.", userID)
			return
		}

		log.Printf("[MemoryWorker] [%s] 📦 Renaming %s for processing (Size: %d bytes)...", userID, filepath.Base(logPath), info.Size())
		err = os.Rename(logPath, processingPath)
		if err != nil {
			log.Printf("[MemoryWorker] [%s] ❌ Failed to rename log for processing: %v", userID, err)
			return
		}
	} else {
		log.Printf("[MemoryWorker] [%s] ♻️ Found existing .processing file, using it.", userID)
	}

	// 2. Open .processing file
	file, err := os.Open(processingPath)
	if err != nil {
		log.Printf("[MemoryWorker] [%s] ❌ Failed to open processing log: %v", userID, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var sb strings.Builder
	count := 0

	log.Printf("[MemoryWorker] [%s] 🔍 Scanning lines in %s...", userID, filepath.Base(processingPath))

	var lastModel string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse JSON line
		var entry map[string]string
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			timestamp := entry["timestamp"]
			if timestamp == "" {
				timestamp = "Unknown Time"
			}
			sb.WriteString(fmt.Sprintf("[%s]\nUser: %s\nAssistant: %s\n", timestamp, entry["user"], entry["assistant"]))
			if m, ok := entry["model"]; ok && m != "" {
				lastModel = m
			}
			count++
		} else {
			log.Printf("[MemoryWorker] [%s] ⚠️ JSON Unmarshal error on line: %v", userID, err)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[MemoryWorker] [%s] ❌ Scanner error: %v", userID, err)
	}

	file.Close() // Close before removal

	if count > 0 {
		log.Printf("[MemoryWorker] [%s] 🧠 Analyzing %d interactions (Model: %s)...", userID, count, lastModel)
		conversationText := sb.String()
		err := a.analyzeAndSaveFacts(userID, conversationText, lastModel)
		if err != nil {
			log.Printf("[MemoryWorker] [%s] ⚠️ Memory analysis failed (will retry later): %v", userID, err)
			return // DO NOT DELETE the processing file
		}
	} else {
		log.Printf("[MemoryWorker] [%s] ℹ️ No valid interactions found.", userID)
	}

	// 4. Delete the processing file only on success or empty
	if err := os.Remove(processingPath); err != nil {
		log.Printf("[MemoryWorker] [%s] ❌ Failed to delete processed log: %v", userID, err)
	} else {
		log.Printf("[MemoryWorker] [%s] ✅ Flushed chat history.", userID)
	}

	// Optional: Limit recursion or periodic cleanup if needed, but this is fine.
}

func (a *App) analyzeAndSaveFacts(userID, conversationText, modelID string) error {
	// Request structured JSON format from the LLM
	// Request structured JSON format from the LLM
	prompt := fmt.Sprintf(`Extract ALL useful information from this conversation.

- ANY facts, persistent user preferences, or long-term events shared by the user → MUST SAVE
- Significant technical decisions or architectural patterns discussed → MUST SAVE
- If the user explicitly says "remember this" or "save this" → ABSOLUTELY MUST SAVE

SKIP RULES (Output { "summary": "NO_IMPORTANT_CONTENT", "keywords": "" } ONLY):
- Greetings, small talk, or empty/trivial responses ("hi", "ok", "thanks").
- User pastes a URL or file just to have it summarized, translated, or analyzed.
- One-off tool commands, code snippet fixes, or transient troubleshooting.
- Questions from the user that are clearly just seeking immediate help (e.g. "how do I run this?").
- Any information that does NOT have long-term value for future conversations.

OUTPUT FORMAT: A single JSON object, no other text.
{ "summary": "...", "keywords": "..." }

- "summary": Concise list of permanent facts.
- "keywords": Comma-separated tags.
- IF SKIP RULES APPLY, YOU MUST OUTPUT EXACTLY:
{ "summary": "NO_IMPORTANT_CONTENT", "keywords": "" }



Conversation:
%s`, conversationText)

	// Call LLM
	msgs := []Message{
		{Role: "system", Content: "You are a memory extraction agent. Output ONLY a raw JSON object. Do NOT output any thinking, reasoning, or explanation. DO NOT use markdown. Start your response directly with '{'. Direct response only."},
		{Role: "user", Content: prompt},
	}

	targetModel := modelID
	if targetModel == "" {
		targetModel = "qwen/qwen3-vl-30b"
	}

	payload := MemoryAnalysisRequest{
		Model:       targetModel,
		Messages:    msgs,
		Temperature: 0.0,
		// MaxTokens removed to allow the model to provide full output without truncation.
	}

	log.Printf("[MemoryWorker] [%s] 📡 Calling LLM at %s with model %s (No Token Limit)...", userID, a.llmEndpoint, targetModel)
	respBody, err := a.callLLM(payload)
	if err != nil {
		log.Printf("[MemoryWorker] [%s] ❌ LLM call failed: %v", userID, err)
		return fmt.Errorf("LLM request failed: %w", err)
	}
	log.Printf("[MemoryWorker] [%s] ✅ LLM responded (%d bytes)", userID, len(respBody))

	rawRes := strings.TrimSpace(respBody)

	// Strip possible markdown block wrappers like ```json and ```
	rawRes = strings.TrimPrefix(rawRes, "```json")
	rawRes = strings.TrimPrefix(rawRes, "```")
	rawRes = strings.TrimSuffix(rawRes, "```")
	rawRes = strings.TrimSpace(rawRes)

	// Extract ALL JSON objects from the response using brace-counting, then pick
	// the best candidate. This handles models like Qwen3.5 that output thinking
	// text containing template JSON examples before the actual response JSON.
	type jsonCandidate struct {
		raw      string
		startPos int
	}
	var candidates []jsonCandidate

	searchFrom := 0
	for searchFrom < len(rawRes) {
		firstBrace := strings.Index(rawRes[searchFrom:], "{")
		if firstBrace < 0 {
			break
		}
		firstBrace += searchFrom // Adjust to absolute position

		depth := 0
		inString := false
		escape := false
		endPos := -1
		for i := firstBrace; i < len(rawRes); i++ {
			ch := rawRes[i]
			if escape {
				escape = false
				continue
			}
			if ch == '\\' && inString {
				escape = true
				continue
			}
			if ch == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					endPos = i
					break
				}
			}
		}
		if endPos > firstBrace {
			extracted := rawRes[firstBrace : endPos+1]
			candidates = append(candidates, jsonCandidate{raw: extracted, startPos: firstBrace})
			searchFrom = endPos + 1
		} else {
			break
		}
	}

	log.Printf("[MemoryWorker] [%s] ⚙️ Found %d JSON candidate(s) in LLM output", userID, len(candidates))

	// Parse JSON - try each candidate and pick the first one with a real summary
	var memData struct {
		Summary  string `json:"summary"`
		Keywords string `json:"keywords"`
	}

	parsed := false
	for i, c := range candidates {
		var tmp struct {
			Summary  string `json:"summary"`
			Keywords string `json:"keywords"`
		}
		if err := json.Unmarshal([]byte(c.raw), &tmp); err != nil {
			log.Printf("[MemoryWorker] [%s]   candidate %d (pos %d, %d bytes): parse error: %v", userID, i, c.startPos, len(c.raw), err)
			continue
		}
		cleanSummary := strings.TrimSpace(strings.Trim(tmp.Summary, "."))
		if len(cleanSummary) < 10 || strings.Contains(strings.ToUpper(tmp.Summary), "NO_IMPORTANT_CONTENT") {
			log.Printf("[MemoryWorker] [%s]   candidate %d (pos %d): placeholder/skip (%q)", userID, i, c.startPos, tmp.Summary)
			continue
		}
		// Found a good candidate
		memData = tmp
		parsed = true
		log.Printf("[MemoryWorker] [%s]   ✅ Using candidate %d (pos %d, %d bytes)", userID, i, c.startPos, len(c.raw))
		break
	}

	if !parsed {
		if len(candidates) > 0 {
			// All candidates were placeholders — try parsing the last (largest) one as fallback
			last := candidates[len(candidates)-1]
			if err := json.Unmarshal([]byte(last.raw), &memData); err != nil {
				log.Printf("[MemoryWorker] [%s] ❌ All %d JSON candidates failed to parse. Last raw: %s", userID, len(candidates), last.raw)
				return nil
			}
			// Check if this last one is also a placeholder
			cleanSummary := strings.TrimSpace(strings.Trim(memData.Summary, "."))
			if len(cleanSummary) < 10 {
				log.Printf("[MemoryWorker] [%s] ⚠️ All candidates are placeholders, will retry.", userID)
				return fmt.Errorf("all JSON candidates are placeholders")
			}
		} else {
			log.Printf("[MemoryWorker] [%s] ❌ No JSON objects found in LLM output (%d bytes)", userID, len(rawRes))
			return nil
		}
	}

	log.Printf("[MemoryWorker] [%s] 📋 Final: Summary=%q, Keywords=%q", userID, memData.Summary, memData.Keywords)

	if memData.Summary == "" || 
		strings.Contains(strings.ToUpper(memData.Summary), "NO_IMPORTANT_CONTENT") {
		log.Printf("[MemoryWorker] [%s] No important content to remember.", userID)
		return nil
	}

	// Save to SQLite
	id, err := mcp.InsertMemory(userID, memData.Summary, memData.Keywords, conversationText)
	if err != nil {
		log.Printf("[MemoryWorker] ❌ Failed to insert memory to DB: %v", err)
		return err // Retry later
	} else {
		log.Printf("[MemoryWorker] ✅ Saved memory to DB for %s (ID: %d)", userID, id)
	}

	return nil
}

// consolidateMemory is deprecated as SQLite handles memory consolidation automatically via search.

// Internal helper to call LLM (Kept from previous version)
func (a *App) callLLM(payload interface{}) (string, error) {
	jsonPayload, _ := json.Marshal(payload)
	// Ensure URL has /v1/chat/completions suffix if not present (simple heuristic)
	// But App.llmEndpoint usually is base. server.go handles proxying.
	// Here we call directly.
	url := a.llmEndpoint
	if !strings.HasSuffix(url, "/v1/chat/completions") && !strings.HasSuffix(url, "/v1/chat") {
		url = strings.TrimSuffix(url, "/") + "/v1/chat/completions"
	}

	log.Printf("[MemoryWorker-LLM] 📡 POST %s (payload: %d bytes, token: %v)", url, len(jsonPayload), a.llmApiToken != "")

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if a.llmApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.llmApiToken)
	}

	client := &http.Client{Timeout: 300 * time.Second} // Increased to 300s to handle heavy models or long contexts
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[MemoryWorker-LLM] ❌ HTTP request failed: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	log.Printf("[MemoryWorker-LLM] 📥 Response status: %s", resp.Status)
	body, _ := io.ReadAll(resp.Body)
	log.Printf("[MemoryWorker-LLM] 📥 Response body length: %d bytes", len(body))

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
