package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	legacyMacMemoryRootName = "DKST LLM Chat"
	macMemoryRootName       = "DKST LLM Chat Server"
)

// ManageMemory is deprecated. All memory is handled via SQLite (SearchMemoryDB / ReadMemoryDB).

// SearchMemoryDB calls the SQLite db to search memory by keyword
func SearchMemoryDB(userID, query string) (string, error) {
	log.Printf("[MCP] SearchMemoryDB: User=%s, Query=%s", userID, query)
	rewrittenQuery := rewriteMemoryQuery(query)
	searchQueries := buildSearchQueries(query, rewrittenQuery)
	chunkResults, err := SearchMemoryChunkMatchesMultiQuery(userID, searchQueries, 10)
	if err != nil {
		return "", fmt.Errorf("chunk search failed: %v", err)
	}
	results, err := SearchMemoriesMultiQuery(userID, searchQueries)
	if err != nil {
		return "", fmt.Errorf("db search failed: %v", err)
	}
	savedTurns, err := searchSavedTurnsMultiQuery(userID, searchQueries)
	if err != nil {
		return "", fmt.Errorf("saved turn search failed: %v", err)
	}

	if len(chunkResults) == 0 && len(results) == 0 && len(savedTurns) == 0 {
		return "No relevant memories found.", nil
	}

	var sb strings.Builder
	sb.WriteString("Found records:\n")
	if rewrittenQuery != query {
		sb.WriteString(fmt.Sprintf("(query rewritten from %q to %q)\n", query, rewrittenQuery))
	}
	if len(searchQueries) > 0 {
		sb.WriteString(fmt.Sprintf("(search terms: %s)\n", strings.Join(searchQueries, ", ")))
	}
	if len(chunkResults) > 0 {
		for _, r := range chunkResults {
			memoryType := strings.TrimSpace(r.MemoryType)
			if memoryType == "" {
				memoryType = "raw_interaction"
			}
			sb.WriteString(fmt.Sprintf("\n--- MEMORY ID: %d | DATE: %s | TYPE: %s | CHUNK: %d | SCORES: %s ---\n", r.ID, r.CreatedAt.Format("2006-01-02"), memoryType, r.ChunkIndex+1, formatRetrievalScoreLine(r.FTSScore, r.VectorScore, r.HybridScore)))
			sb.WriteString(fmt.Sprintf("RELEVANT EXCERPT:\n%s\n", r.ChunkText))
			_ = IncrementMemoryChunkHitCount(r.ChunkID)
			_ = IncrementHitCount(r.ID)
		}
	}
	if len(results) > 0 {
		sb.WriteString("\n(Full memory matches)\n")
	}
	for _, r := range results {
		memoryType := strings.TrimSpace(r.MemoryType)
		if memoryType == "" {
			memoryType = "raw_interaction"
		}
		sb.WriteString(fmt.Sprintf("\n--- MEMORY ID: %d | DATE: %s | TYPE: %s ---\n", r.ID, r.CreatedAt.Format("2006-01-02"), memoryType))
		sb.WriteString(fmt.Sprintf("FULL TEXT:\n%s\n", compactMemoryText(r.FullText, 500)))
	}
	for _, turn := range savedTurns {
		sb.WriteString(fmt.Sprintf("\n--- SAVED TURN ID: %d | DATE: %s | TITLE: %s ---\n", turn.ID, turn.CreatedAt.Format("2006-01-02"), turn.Title))
		sb.WriteString(fmt.Sprintf("USER PROMPT:\n%s\n", turn.PromptText))
		sb.WriteString(fmt.Sprintf("ASSISTANT RESPONSE:\n%s\n", turn.ResponseText))
	}
	return sb.String(), nil
}

func searchSavedTurnsMultiQuery(userID string, queryStrs []string) ([]SavedTurnEntry, error) {
	seen := make(map[int64]bool)
	var merged []SavedTurnEntry
	for _, queryStr := range queryStrs {
		trimmed := strings.TrimSpace(queryStr)
		if trimmed == "" {
			continue
		}
		results, err := SearchSavedTurns(userID, trimmed, 10)
		if err != nil {
			return nil, err
		}
		for _, result := range results {
			if seen[result.ID] {
				continue
			}
			seen[result.ID] = true
			merged = append(merged, result)
			if len(merged) >= 10 {
				return merged, nil
			}
		}
	}
	return merged, nil
}

func rewriteMemoryQuery(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	lower := strings.ToLower(trimmed)
	if strings.Contains(trimmed, "내 이름") || strings.Contains(trimmed, "제 이름") || strings.Contains(lower, "my name") || strings.Contains(lower, "who am i") {
		return "내 이름은 제 이름은 사용자 이름 이름은 user name my name call me"
	}

	if len([]rune(trimmed)) < 12 {
		return trimmed
	}

	prompt := fmt.Sprintf(`Rewrite the user's message into a short memory search query.

Rules:
- Keep only stable entities, names, preferences, dates, or technical topics.
- Resolve vague references like "that project" into a searchable phrase if possible.
- Output a single plain text query only.
- If rewriting would not help, return the original message unchanged.

User message:
%s`, trimmed)

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	payload := map[string]interface{}{
		"model": "local-model",
		"messages": []message{
			{Role: "system", Content: "You rewrite user messages into concise memory retrieval queries. Output plain text only."},
			{Role: "user", Content: prompt},
		},
		"temperature": 0.0,
	}

	reqBody, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "http://127.0.0.1:1234/v1/chat/completions", strings.NewReader(string(reqBody)))
	if err != nil {
		return trimmed
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return trimmed
	}
	defer resp.Body.Close()

	var resData struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&resData); err != nil {
		return trimmed
	}
	if len(resData.Choices) == 0 {
		return trimmed
	}

	rewritten := strings.TrimSpace(resData.Choices[0].Message.Content)
	rewritten = strings.Trim(rewritten, "\"'`")
	if rewritten == "" {
		return trimmed
	}
	return compactMemoryText(rewritten, 120)
}

func buildSearchQueries(originalQuery, rewrittenQuery string) []string {
	var queries []string
	seen := map[string]bool{}

	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if seen[key] {
			return
		}
		seen[key] = true
		queries = append(queries, value)
	}

	add(originalQuery)
	add(rewrittenQuery)
	for _, keyword := range ExtractKeywords(rewrittenQuery) {
		add(keyword)
	}
	for _, keyword := range ExtractKeywords(originalQuery) {
		add(keyword)
	}
	if strings.Contains(originalQuery, "이름") || strings.Contains(rewrittenQuery, "이름") {
		add("내 이름은")
		add("제 이름은")
		add("사용자 이름")
		add("이름은")
		add("user name")
		add("my name")
	}

	return queries
}

// GetMemorySnapshot returns a formatted string of the most recent memories for system prompt injection.
func GetMemorySnapshot(userID string) string {
	results, err := SearchMemoriesByRecent(userID, 5)
	if err != nil {
		log.Printf("[MCP] Failed to get memory snapshot: %v", err)
		return "No recent memories found."
	}
	savedTurns, savedErr := ListSavedTurns(userID, 5)
	if savedErr != nil {
		log.Printf("[MCP] Failed to get saved turn snapshot: %v", savedErr)
	}
	if len(results) == 0 && len(savedTurns) == 0 {
		return "No recent memories found."
	}

	var sb strings.Builder
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", r.CreatedAt.Format("2006-01-02"), compactMemoryText(r.FullText, 120)))
	}
	for _, turn := range savedTurns {
		sb.WriteString(fmt.Sprintf("- [%s] Saved turn: %s\n", turn.CreatedAt.Format("2006-01-02"), compactMemoryText(turn.Title, 120)))
	}
	return sb.String()
}

// ExtractKeywords provides keyword extraction from user message
// by stripping common Korean particles (조사) and stopwords.
func ExtractKeywords(input string) []string {
	inputLower := strings.ToLower(input)

	// Remove common punctuation
	replacer := strings.NewReplacer(",", " ", ".", " ", "?", " ", "!", " ", "\"", " ", "'", " ", "(", " ", ")", " ", "-", " ")
	clean := replacer.Replace(inputLower)
	words := strings.Fields(clean)

	var keywords []string

	// Words to completely ignore
	stopwords := map[string]bool{
		"그리고": true, "그래서": true, "하지만": true, "알려줘": true, "해줘": true,
		"뭐야": true, "어때": true, "어디": true, "누구": true, "어떻게": true, "왜": true,
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "about": true, "like": true, "this": true, "that": true,
		"tell": true, "me": true, "what": true, "who": true, "when": true,
		"where": true, "why": true, "how": true,
	}

	// Suffixes (particles/조사) to strip from the end of words
	particles := []string{
		"이라고", "이라는", "에서는", "로부터", "까지도", "마저도", "조차도",
		"에서", "부터", "까지", "으로", "보다", "처럼", "만큼", "마다", "이랑", "하고",
		"은", "는", "이", "가", "을", "를", "에", "도", "로", "와", "과", "의", "만", "요", "다",
	}

	for _, w := range words {
		if stopwords[w] {
			continue
		}

		// Strip particles
		cleanedWord := w
		for _, p := range particles {
			if strings.HasSuffix(cleanedWord, p) {
				potential := strings.TrimSuffix(cleanedWord, p)
				if len([]rune(potential)) >= 1 {
					cleanedWord = potential
					break
				}
			}
		}

		// Priority keywords (relations, etc) - if they are part of a word, keep them
		priorities := []string{"아내", "배우자", "아들", "딸", "부모", "아버지", "어머니", "생일", "전화번호", "주소", "이름"}
		for _, p := range priorities {
			if strings.Contains(w, p) {
				keywords = append(keywords, p)
			}
		}

		// Only add if it's meaningful length
		if len([]rune(cleanedWord)) >= 1 {
			if len(cleanedWord) == 1 && stopwords[cleanedWord] {
				continue
			}
			keywords = append(keywords, cleanedWord)
		}
	}

	// Dedup keywords
	uniqueMap := make(map[string]bool)
	var finalKeywords []string
	for _, k := range keywords {
		if !uniqueMap[k] {
			finalKeywords = append(finalKeywords, k)
			uniqueMap[k] = true
		}
	}

	return finalKeywords
}

// AutoSearchMemory searches for the most relevant memories using extracted keywords
// and returns their full text to be injected proactively into the system prompt.
func AutoSearchMemory(userID, input string) string {
	keywords := ExtractKeywords(input)
	log.Printf("[MCP] AutoSearchMemory: Input=%q, Keywords=%v", input, keywords)
	if len(keywords) == 0 {
		return ""
	}

	var chunkResults []MemoryChunkMatch
	seenChunkKeys := make(map[string]bool)
	var savedTurnResults []MemoryEntry
	seenSavedTurnIDs := make(map[int64]bool)

	// Step 1: Search with top 3 keywords (Priority)
	searchWords := keywords
	if len(searchWords) > 3 {
		searchWords = searchWords[:3]
	}

	runSearch := func(words []string) {
		for _, kw := range words {
			results, err := SearchMemoryChunkMatches(userID, kw, 5)
			if err == nil {
				if len(results) > 0 {
					log.Printf("[MCP] AutoSearchMemory: Keyword %q found %d chunk results", kw, len(results))
				}
				for _, r := range results {
					key := fmt.Sprintf("%d:%d", r.ID, r.ChunkIndex)
					if !seenChunkKeys[key] {
						chunkResults = append(chunkResults, r)
						seenChunkKeys[key] = true
					}
				}
			}

			savedTurns, err := SearchSavedTurns(userID, kw, 5)
			if err == nil {
				for _, turn := range savedTurns {
					savedID := turn.ID + 1_000_000_000
					if seenSavedTurnIDs[savedID] {
						continue
					}
					savedTurnResults = append(savedTurnResults, MemoryEntry{
						ID:         savedID,
						UserID:     turn.UserID,
						FullText:   fmt.Sprintf("User Prompt:\n%s\n\nAssistant Response:\n%s", turn.PromptText, turn.ResponseText),
						HitCount:   0,
						CreatedAt:  turn.CreatedAt,
						MemoryType: "saved_turn",
					})
					seenSavedTurnIDs[savedID] = true
				}
			}
		}
	}

	runSearch(searchWords)

	// Step 2: Retry with remaining keywords if no results found
	if len(chunkResults) == 0 && len(savedTurnResults) == 0 && len(keywords) > 3 {
		log.Printf("[MCP] AutoSearchMemory: No results in Step 1. Retrying with next keywords.")
		nextWords := keywords[3:]
		if len(nextWords) > 5 {
			nextWords = nextWords[:5]
		}
		runSearch(nextWords)
	}

	if len(chunkResults) == 0 && len(savedTurnResults) == 0 {
		return ""
	}

	var rawContextSb strings.Builder
	chunkLimit := 3
	if len(chunkResults) < chunkLimit {
		chunkLimit = len(chunkResults)
	}
	for i := 0; i < chunkLimit; i++ {
		r := chunkResults[i]
		memoryType := strings.TrimSpace(r.MemoryType)
		if memoryType == "" {
			memoryType = "raw_interaction"
		}
		rawContextSb.WriteString(fmt.Sprintf("\n--- MEMORY ID: %d | DATE: %s | TYPE: %s | CHUNK: %d ---\n", r.ID, r.CreatedAt.Format("2006-01-02"), memoryType, r.ChunkIndex+1))
		rawContextSb.WriteString(fmt.Sprintf("Relevant excerpt: %s\n", compactMemoryText(r.ChunkText, 400)))
		_ = IncrementMemoryChunkHitCount(r.ChunkID)
		_ = IncrementHitCount(r.ID)
	}

	savedLimit := 2
	if len(savedTurnResults) < savedLimit {
		savedLimit = len(savedTurnResults)
	}
	for i := 0; i < savedLimit; i++ {
		r := savedTurnResults[i]
		rawContextSb.WriteString(fmt.Sprintf("\n--- SAVED TURN ID: %d | DATE: %s | TYPE: %s ---\n", r.ID-1_000_000_000, r.CreatedAt.Format("2006-01-02"), r.MemoryType))
		rawContextSb.WriteString(fmt.Sprintf("Content: %s\n", compactMemoryText(r.FullText, 400)))
	}

	rawContext := rawContextSb.String()

	// Perform server-side memory synthesis
	syn, err := SynthesizeMemoryContext(userID, input, rawContext)
	if err != nil {
		log.Printf("[MCP] Synthesize failed, falling back to compact context: %v", err)
		return "\n[PROACTIVE MEMORY RETRIEVAL]\n" + rawContext
	}

	if strings.TrimSpace(syn) == "" || strings.TrimSpace(syn) == "NO_RELEVANT_INFO" {
		return ""
	}

	return "\n[PROACTIVE MEMORY RETRIEVAL (Synthesized)]\n" + syn
}

// SynthesizeMemoryContext makes a quick LLM call to extract only the facts relevant to the query
// from the raw database records, filtering out noise.
func SynthesizeMemoryContext(userID, query, rawMemories string) (string, error) {
	prompt := fmt.Sprintf(`You are a background memory filtering agent.
Below are raw logs of past conversations between the user and the assistant.
The user is currently asking or saying: "%s"

Your task is to extract ONLY the exact facts, quotes, or statements from the raw logs that are relevant to the user's current message.
DO NOT answer the user's message. 
DO NOT converse.
DO NOT add any conversational filler.
If nothing in the logs is relevant, output "NO_RELEVANT_INFO".

Raw Logs:
%s`, query, rawMemories)

	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	payload := map[string]interface{}{
		// Using a standard identifier, the local server should route it to the active model
		"model": "local-model",
		"messages": []Message{
			{Role: "system", Content: "Extract facts concisely. No chat. No markdown unless necessary."},
			{Role: "user", Content: prompt},
		},
		"temperature": 0.1,
	}

	reqBody, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "http://127.0.0.1:1234/v1/chat/completions", strings.NewReader(string(reqBody)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var resData struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&resData); err != nil {
		return "", err
	}

	if len(resData.Choices) > 0 {
		content := strings.TrimSpace(resData.Choices[0].Message.Content)
		if content == "NO_RELEVANT_INFO" || content == "" {
			return "", nil
		}
		return content, nil
	}

	return "", fmt.Errorf("empty response from LLM")
}

// ReadMemoryDB calls the SQLite db to read full text of a specific memory ID
func ReadMemoryDB(userID string, memoryID int64) (string, error) {
	log.Printf("[MCP] ReadMemoryDB: User=%s, ID=%d", userID, memoryID)
	mem, err := ReadMemory(userID, memoryID)
	if err != nil {
		return "", fmt.Errorf("db read failed: %v", err)
	}

	return fmt.Sprintf("Memory ID: %d\nDate: %s\nType: %s\n\n--- Full Context ---\n%s",
		mem.ID, mem.CreatedAt.Format("2006-01-02 15:04"), mem.MemoryType, mem.FullText), nil
}

// DeleteMemoryDB removes a specific memory entry.
func DeleteMemoryDB(userID string, memoryID int64) (string, error) {
	log.Printf("[MCP] DeleteMemoryDB: User=%s, ID=%d", userID, memoryID)
	err := DeleteMemory(userID, memoryID)
	if err != nil {
		return "", fmt.Errorf("db delete failed: %v", err)
	}
	return fmt.Sprintf("Successfully deleted Memory ID: %d", memoryID), nil
}

// GetUserMemoryDir returns the memory directory path for a user based on OS.
// macOS: ~/Documents/DKST LLM Chat Server/memory/{userID}/
// Windows/Linux: {executable_dir}/memory/{userID}/
func GetUserMemoryDir(userID string) (string, error) {
	if userID == "" {
		userID = "default"
	}

	var baseDir string
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		newRoot := filepath.Join(home, "Documents", macMemoryRootName)
		legacyRoot := filepath.Join(home, "Documents", legacyMacMemoryRootName)
		if err := migrateLegacyMacMemoryRoot(legacyRoot, newRoot); err != nil {
			return "", err
		}
		baseDir = filepath.Join(newRoot, "memory")
	} else {
		// Windows/Linux: Executable directory
		ex, err := os.Executable()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(filepath.Dir(ex), "memory")
	}

	return filepath.Join(baseDir, userID), nil
}

func migrateLegacyMacMemoryRoot(oldRoot, newRoot string) error {
	if oldRoot == newRoot {
		return nil
	}
	if _, err := os.Stat(newRoot); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(oldRoot); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.Rename(oldRoot, newRoot)
}

// GetUserMemoryFilePath returns the path to a specific memory file for a user.
func GetUserMemoryFilePath(userID, filename string) (string, error) {
	dir, err := GetUserMemoryDir(userID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filename), nil
}

// ListUserMemoryFiles returns all .md files in the user's memory directory
func ListUserMemoryFiles(userID string) ([]string, error) {
	dir, err := GetUserMemoryDir(userID)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

// ReadUserDocument reads a specific document from user's memory folder
func ReadUserDocument(userID, filename string) (string, error) {
	// Validate filename to prevent directory traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return "", fmt.Errorf("invalid filename: %s", filename)
	}

	filePath, err := GetUserMemoryFilePath(userID, filename)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("document '%s' not found", filename)
		}
		return "", err
	}

	return string(data), nil
}

// WriteUserDocument writes content to a specific document in user's memory folder
func WriteUserDocument(userID, filename, content string) error {
	// Validate filename
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("invalid filename: %s", filename)
	}

	filePath, err := GetUserMemoryFilePath(userID, filename)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(filePath, []byte(content), 0644)
}

// DetermineCategory analyzes content and determines if it's personal or work-related
func DetermineCategory(content string) string {
	contentLower := strings.ToLower(content)

	workKeywords := []string{
		"project", "프로젝트", "work", "업무", "회사", "company", "job", "직장",
		"task", "deadline", "마감", "meeting", "회의", "client", "고객",
		"code", "코드", "programming", "프로그래밍", "development", "개발",
		"report", "보고서", "presentation", "발표", "team", "팀",
	}

	for _, kw := range workKeywords {
		if strings.Contains(contentLower, kw) {
			return "work"
		}
	}

	return "personal"
}
