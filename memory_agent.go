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
	log.Printf("[MemoryWorker] [%s] Structured summary/keyword extraction has been removed. Skipping legacy analysis for %d chars.", userID, len(conversationText))
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
