/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// MemoryIndex represents the compiled index of user memory
type MemoryIndex struct {
	Version   int               `json:"version"`
	UpdatedAt string            `json:"updated_at"`
	Facts     map[string]string `json:"facts"`
	Summary   string            `json:"summary"`
}

// MemoryLogEntry represents a single log entry
type MemoryLogEntry struct {
	Timestamp time.Time
	Action    string // SET, DELETE
	Key       string
	Value     string
}

var indexCache = make(map[string]*MemoryIndex)
var indexCacheMu sync.RWMutex

// GetMemoryIndexPath returns the path to the index.json file for a user
func GetMemoryIndexPath(userID string) (string, error) {
	return GetUserMemoryFilePath(userID, "index.json")
}

// ParseMemoryLog reads the log file and parses all entries
func ParseMemoryLog(logPath string) ([]MemoryLogEntry, error) {
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []MemoryLogEntry{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var entries []MemoryLogEntry
	scanner := bufio.NewScanner(file)

	// Pattern: [2026-02-06 10:00:00] SET key: value
	// or: [2026-02-06 10:00:00] key: value (legacy format, treat as SET)
	timestampRe := regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]\s*(.+)$`)
	actionRe := regexp.MustCompile(`^(SET|DELETE)\s+(.+)$`)
	kvRe := regexp.MustCompile(`^([^:]+):\s*(.*)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		matches := timestampRe.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}

		ts, err := time.Parse("2006-01-02 15:04:05", matches[1])
		if err != nil {
			continue
		}

		content := matches[2]
		entry := MemoryLogEntry{Timestamp: ts}

		// Check for explicit action prefix
		actionMatches := actionRe.FindStringSubmatch(content)
		if len(actionMatches) >= 3 {
			entry.Action = actionMatches[1]
			content = actionMatches[2]
		} else {
			entry.Action = "SET" // Default action
		}

		// Parse key: value
		kvMatches := kvRe.FindStringSubmatch(content)
		if len(kvMatches) >= 3 {
			entry.Key = strings.TrimSpace(strings.ToLower(kvMatches[1]))
			entry.Value = strings.TrimSpace(kvMatches[2])
			entries = append(entries, entry)
		}
	}

	return entries, scanner.Err()
}

// BuildIndex creates an index from log entries (latest value wins)
func BuildIndex(entries []MemoryLogEntry) *MemoryIndex {
	facts := make(map[string]string)

	for _, entry := range entries {
		switch entry.Action {
		case "SET":
			facts[entry.Key] = entry.Value
		case "DELETE":
			delete(facts, entry.Key)
		}
	}

	return &MemoryIndex{
		Version:   1,
		UpdatedAt: time.Now().Format(time.RFC3339),
		Facts:     facts,
		Summary:   generateSummary(facts),
	}
}

// generateSummary creates a compact summary from facts
func generateSummary(facts map[string]string) string {
	if len(facts) == 0 {
		return ""
	}

	var parts []string
	priorityKeys := []string{"name", "이름", "nickname", "별명", "language", "언어", "preference"}

	// Add priority facts first
	for _, key := range priorityKeys {
		if val, ok := facts[key]; ok {
			parts = append(parts, fmt.Sprintf("%s: %s", key, val))
		}
	}

	// Add remaining facts (up to 10 total)
	for k, v := range facts {
		if len(parts) >= 10 {
			break
		}
		// Skip if already added
		found := false
		for _, pk := range priorityKeys {
			if k == pk {
				found = true
				break
			}
		}
		if !found {
			parts = append(parts, fmt.Sprintf("%s: %s", k, v))
		}
	}

	return strings.Join(parts, "; ")
}

// SaveIndex writes the index to disk
func SaveIndex(indexPath string, index *MemoryIndex) error {
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath, data, 0644)
}

// LoadIndex reads the index from disk or cache
func LoadIndex(indexPath string) (*MemoryIndex, error) {
	indexCacheMu.RLock()
	if cached, ok := indexCache[indexPath]; ok {
		indexCacheMu.RUnlock()
		return cached, nil
	}
	indexCacheMu.RUnlock()

	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &MemoryIndex{Facts: make(map[string]string)}, nil
		}
		return nil, err
	}

	var index MemoryIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	indexCacheMu.Lock()
	indexCache[indexPath] = &index
	indexCacheMu.Unlock()

	return &index, nil
}

// RebuildAndSaveIndex rebuilds the index from the log file and updates MD files
func RebuildAndSaveIndex(userID string) (*MemoryIndex, error) {
	logPath, err := GetUserMemoryPath(userID)
	if err != nil {
		return nil, err
	}

	indexPath, err := GetMemoryIndexPath(userID)
	if err != nil {
		return nil, err
	}

	entries, err := ParseMemoryLog(logPath)
	if err != nil {
		return nil, err
	}

	index := BuildIndex(entries)

	// Update individual category files (personal.md, work.md)
	personalPath, _ := GetUserMemoryFilePath(userID, "personal.md")
	workPath, _ := GetUserMemoryFilePath(userID, "work.md")

	var personalLines []string
	var workLines []string

	for k, v := range index.Facts {
		// Use the context/key/value to determine category
		category := DetermineCategory(k + " " + v)
		line := fmt.Sprintf("- **%s**: %s", k, v)
		if category == "work" {
			workLines = append(workLines, line)
		} else {
			personalLines = append(personalLines, line)
		}
	}

	// Write category files
	if len(personalLines) > 0 {
		content := "# Personal Memory\n\n" + strings.Join(personalLines, "\n") + "\n"
		os.WriteFile(personalPath, []byte(content), 0644)
	}
	if len(workLines) > 0 {
		content := "# Work Memory\n\n" + strings.Join(workLines, "\n") + "\n"
		os.WriteFile(workPath, []byte(content), 0644)
	}

	// Update index.json (system use)
	if err := SaveIndex(indexPath, index); err != nil {
		log.Printf("[Memory] Failed to save index: %v", err)
	}

	// Update cache
	indexCacheMu.Lock()
	indexCache[indexPath] = index
	indexCacheMu.Unlock()

	// Update index.md (human readable / MCP guide)
	if err := GenerateIndexMD(userID); err != nil {
		log.Printf("[Memory] Failed to generate index.md: %v", err)
	}

	return index, nil
}

// AppendToLog writes a new entry to the log file (Append-Only)
func AppendToLog(logPath string, action string, key string, value string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	var entry string
	if action == "DELETE" {
		entry = fmt.Sprintf("[%s] DELETE %s:\n", timestamp, key)
	} else {
		entry = fmt.Sprintf("[%s] SET %s: %s\n", timestamp, key, value)
	}

	_, err = f.WriteString(entry)
	return err
}

// ExtractKeyFromContent attempts to extract a key from natural language content
func ExtractKeyFromContent(content string) (string, string) {
	// Pattern 1: "Key: Value" or "Key = Value"
	kvRe := regexp.MustCompile(`^([^:=]+)[:\=]\s*(.+)$`)
	if matches := kvRe.FindStringSubmatch(content); len(matches) >= 3 {
		key := strings.TrimSpace(strings.ToLower(matches[1]))
		value := strings.TrimSpace(matches[2])
		return key, value
	}

	// Pattern 2: "my name is X" -> key: name, value: X
	patterns := []struct {
		re  *regexp.Regexp
		key string
	}{
		{regexp.MustCompile(`(?i)(?:my |나의? |내 )?(?:name|이름)(?:은|는|이|가)?\s*(?:is |:)?\s*(.+)`), "name"},
		{regexp.MustCompile(`(?i)(?:i am|i'm|저는|나는)\s+(.+)`), "name"},
		{regexp.MustCompile(`(?i)(?:my |나의? )?(?:birthday|생일)(?:은|는|이|가)?\s*(?:is |:)?\s*(.+)`), "birthday"},
		{regexp.MustCompile(`(?i)(?:i |나는? |저는? )?(?:prefer|like|좋아하|선호)\s*(.+)`), "preference"},
	}

	for _, p := range patterns {
		if matches := p.re.FindStringSubmatch(content); len(matches) >= 2 {
			return p.key, strings.TrimSpace(matches[1])
		}
	}

	// Fallback: Use first few words as key
	words := strings.Fields(content)
	if len(words) >= 2 {
		keyPart := strings.Join(words[:min(3, len(words))], "_")
		keyPart = strings.ToLower(regexp.MustCompile(`[^a-z0-9가-힣_]`).ReplaceAllString(keyPart, ""))
		return keyPart, content
	}

	return "note", content
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetIndexSummaryForPrompt returns a compact string for injection into system prompt
func GetIndexSummaryForPrompt(userID string) string {
	indexPath, err := GetMemoryIndexPath(userID)
	if err != nil {
		return ""
	}

	index, err := LoadIndex(indexPath)
	if err != nil || len(index.Facts) == 0 {
		// Try to rebuild from log
		index, err = RebuildAndSaveIndex(userID)
		if err != nil || len(index.Facts) == 0 {
			return ""
		}
	}

	// Build compact representation
	var lines []string

	// Explicitly state names to prevent confusion between ID and Display Name
	lines = append(lines, fmt.Sprintf("- ACCOUNT_ID: %s", userID))
	if name, ok := index.Facts["name"]; ok {
		lines = append(lines, fmt.Sprintf("- USER_NAME: %s", name))
	} else if name, ok := index.Facts["이름"]; ok {
		lines = append(lines, fmt.Sprintf("- USER_NAME: %s", name))
	}

	for k, v := range index.Facts {
		if k == "name" || k == "이름" {
			continue // Already added above
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", k, v))
	}

	if len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n")
}

// GenerateIndexMD creates or updates the index.md file that summarizes all user memory
// This file lists available documents and their summaries for quick lookup
func GenerateIndexMD(userID string) error {
	dir, err := GetUserMemoryDir(userID)
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	indexPath := filepath.Join(dir, "index.md")

	// Build markdown content
	var sb strings.Builder
	sb.WriteString("# User Memory Index\n\n")
	sb.WriteString(fmt.Sprintf("*Updated: %s*\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// List available files
	files, err := ListUserMemoryFiles(userID)
	if err != nil {
		log.Printf("[Memory] Failed to list files: %v", err)
	}

	sb.WriteString("## Available Documents\n\n")
	if len(files) == 0 {
		sb.WriteString("- No documents yet\n")
	} else {
		for _, f := range files {
			if f == "index.md" {
				continue // Skip self
			}
			sb.WriteString(fmt.Sprintf("- **%s**\n", f))
		}
	}

	// Add facts summary from index.json
	jsonIndex, _ := LoadIndex(filepath.Join(dir, "index.json"))
	if len(jsonIndex.Facts) > 0 {
		sb.WriteString("\n## Quick Facts\n\n")
		count := 0
		for k, v := range jsonIndex.Facts {
			if count >= 15 {
				sb.WriteString("- *(more facts available in log.md)*\n")
				break
			}
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", k, v))
			count++
		}
	}

	// Add file summaries
	personalPath := filepath.Join(dir, "personal.md")
	if data, err := os.ReadFile(personalPath); err == nil && len(data) > 0 {
		lines := strings.Split(string(data), "\n")
		preview := strings.Join(lines[:min(5, len(lines))], "\n")
		sb.WriteString("\n## Personal (Preview)\n\n")
		sb.WriteString(preview)
		if len(lines) > 5 {
			sb.WriteString("\n*...more in personal.md*\n")
		}
	}

	workPath := filepath.Join(dir, "work.md")
	if data, err := os.ReadFile(workPath); err == nil && len(data) > 0 {
		lines := strings.Split(string(data), "\n")
		preview := strings.Join(lines[:min(5, len(lines))], "\n")
		sb.WriteString("\n## Work (Preview)\n\n")
		sb.WriteString(preview)
		if len(lines) > 5 {
			sb.WriteString("\n*...more in work.md*\n")
		}
	}

	return os.WriteFile(indexPath, []byte(sb.String()), 0644)
}
