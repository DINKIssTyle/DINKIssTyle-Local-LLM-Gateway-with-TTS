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
	"sort"
	"strings"
	"time"
)

const (
	legacyMacMemoryRootName = "DKST LLM Chat"
	macMemoryRootName       = "DKST LLM Chat Server"
	savedTurnMemoryOffset   = int64(1_000_000_000)
)

type MemorySnapshotDebug struct {
	Text           string  `json:"text"`
	MemoryCount    int     `json:"memory_count"`
	SavedTurnCount int     `json:"saved_turn_count"`
	MemoryIDs      []int64 `json:"memory_ids,omitempty"`
	SavedTurnIDs   []int64 `json:"saved_turn_ids,omitempty"`
}

type AutoSearchMemoryDebug struct {
	Context                string   `json:"context"`
	Keywords               []string `json:"keywords,omitempty"`
	ChunkMatchCount        int      `json:"chunk_match_count"`
	RetrievedMemoriesCount int      `json:"retrieved_memories_count"`
	SavedTurnHits          int      `json:"saved_turn_hits"`
	UsedSynthesis          bool     `json:"used_synthesis"`
	RawContext             string   `json:"raw_context,omitempty"`
	SynthesizedContext     string   `json:"synthesized_context,omitempty"`
}

type memoryChunkContext struct {
	Index int
	Text  string
}

type memoryCandidate struct {
	MemoryID    int64
	BaseID      int64
	SourceType  string
	MemoryType  string
	Title       string
	Date        time.Time
	Snippet     string
	MatchReason string
	ChunkIndex  int
	FTSScore    float64
	VectorScore float64
	HybridScore float64
}

func isSavedTurnMemoryID(memoryID int64) bool {
	return memoryID >= savedTurnMemoryOffset
}

func makeSavedTurnMemoryID(turnID int64) int64 {
	return savedTurnMemoryOffset + turnID
}

func originalSavedTurnID(memoryID int64) int64 {
	return memoryID - savedTurnMemoryOffset
}

func formatMemoryCandidateSource(candidate memoryCandidate) string {
	switch {
	case candidate.SourceType == "saved_turn" && candidate.MatchReason == "saved turn chunk match":
		return "saved_turn_chunk"
	case candidate.SourceType == "saved_turn":
		return "saved_turn"
	case candidate.SourceType == "memory" && candidate.MatchReason == "chunk match":
		return "memory_chunk"
	case candidate.SourceType == "memory":
		return "memory_fulltext"
	default:
		return candidate.SourceType
	}
}

// ManageMemory is deprecated. All memory is handled via SQLite (SearchMemoryDB / ReadMemoryDB).

// SearchMemoryDB calls the SQLite db to search memory by keyword
func SearchMemoryDB(userID, query string) (string, error) {
	log.Printf("[MCP] SearchMemoryDB: User=%s, Query=%s", userID, query)
	candidates, err := buildMemoryCandidates(userID, query, 8)
	if err != nil {
		return "", fmt.Errorf("memory candidate search failed: %v", err)
	}
	if len(candidates) == 0 {
		return "No relevant memories found.", nil
	}

	var sb strings.Builder
	sb.WriteString("Memory candidates:\n")
	for idx, candidate := range candidates {
		title := strings.TrimSpace(candidate.Title)
		if title == "" {
			title = candidate.MemoryType
		}
		sb.WriteString(fmt.Sprintf(
			"\n%d. MEMORY ID: %d | SOURCE: %s | DATE: %s | TITLE: %s\n",
			idx+1,
			candidate.MemoryID,
			formatMemoryCandidateSource(candidate),
			candidate.Date.Format("2006-01-02"),
			title,
		))
		sb.WriteString(fmt.Sprintf("   MATCH: %s\n", candidate.MatchReason))
		if candidate.ChunkIndex >= 0 {
			sb.WriteString(fmt.Sprintf("   CHUNK INDEX: %d\n", candidate.ChunkIndex))
		}
		if scoreLine := formatRetrievalScoreLine(candidate.FTSScore, candidate.VectorScore, candidate.HybridScore); scoreLine != "" {
			sb.WriteString(fmt.Sprintf("   SCORES: %s\n", scoreLine))
		}
		sb.WriteString(fmt.Sprintf("   SNIPPET: %s\n", compactMemoryText(candidate.Snippet, 280)))
	}
	return sb.String(), nil
}

func buildMemoryCandidates(userID, query string, limit int) ([]memoryCandidate, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 8
	}

	chunkResults, err := SearchMemoryChunkMatches(userID, trimmed, limit*2)
	if err != nil {
		return nil, err
	}
	savedTurnChunkResults, err := SearchSavedTurnChunkMatches(userID, trimmed, limit*2)
	if err != nil {
		return nil, err
	}
	fullResults, err := SearchMemories(userID, trimmed)
	if err != nil {
		return nil, err
	}
	savedTurns, err := SearchSavedTurns(userID, trimmed, limit)
	if err != nil {
		return nil, err
	}

	candidateMap := make(map[int64]memoryCandidate)
	for _, match := range chunkResults {
		memoryType := strings.TrimSpace(match.MemoryType)
		if memoryType == "" {
			memoryType = "raw_interaction"
		}
		candidate := memoryCandidate{
			MemoryID:    match.ID,
			BaseID:      match.ID,
			SourceType:  "memory",
			MemoryType:  memoryType,
			Date:        match.CreatedAt,
			Snippet:     match.ChunkText,
			MatchReason: "chunk match",
			ChunkIndex:  match.ChunkIndex,
			FTSScore:    match.FTSScore,
			VectorScore: match.VectorScore,
			HybridScore: match.HybridScore,
		}
		existing, ok := candidateMap[candidate.MemoryID]
		if !ok || candidate.HybridScore > existing.HybridScore {
			candidateMap[candidate.MemoryID] = candidate
		}
	}
	for _, memory := range fullResults {
		memoryType := strings.TrimSpace(memory.MemoryType)
		if memoryType == "" {
			memoryType = "raw_interaction"
		}
		candidate := memoryCandidate{
			MemoryID:    memory.ID,
			BaseID:      memory.ID,
			SourceType:  "memory",
			MemoryType:  memoryType,
			Date:        memory.CreatedAt,
			Snippet:     memory.FullText,
			MatchReason: "full text match",
			ChunkIndex:  -1,
			HybridScore: float64(memory.HitCount) * 0.01,
		}
		if existing, ok := candidateMap[candidate.MemoryID]; ok {
			if existing.Snippet == "" {
				existing.Snippet = candidate.Snippet
			}
			candidateMap[candidate.MemoryID] = existing
			continue
		}
		candidateMap[candidate.MemoryID] = candidate
	}
	for _, match := range savedTurnChunkResults {
		memoryID := makeSavedTurnMemoryID(match.ID)
		candidate := memoryCandidate{
			MemoryID:    memoryID,
			BaseID:      match.ID,
			SourceType:  "saved_turn",
			MemoryType:  "saved_turn",
			Title:       strings.TrimSpace(match.Title),
			Date:        match.CreatedAt,
			Snippet:     match.ChunkText,
			MatchReason: "saved turn chunk match",
			ChunkIndex:  match.ChunkIndex,
			FTSScore:    match.FTSScore,
			VectorScore: match.VectorScore,
			HybridScore: match.HybridScore,
		}
		existing, ok := candidateMap[memoryID]
		if !ok || candidate.HybridScore > existing.HybridScore {
			candidateMap[memoryID] = candidate
		}
	}
	for _, turn := range savedTurns {
		memoryID := makeSavedTurnMemoryID(turn.ID)
		if existing, ok := candidateMap[memoryID]; ok && existing.MatchReason == "saved turn chunk match" {
			continue
		}
		candidateMap[memoryID] = memoryCandidate{
			MemoryID:    memoryID,
			BaseID:      turn.ID,
			SourceType:  "saved_turn",
			MemoryType:  "saved_turn",
			Title:       strings.TrimSpace(turn.Title),
			Date:        turn.CreatedAt,
			Snippet:     strings.TrimSpace(turn.PromptText + "\n" + turn.ResponseText),
			MatchReason: "saved turn match",
			ChunkIndex:  -1,
		}
	}

	candidates := make([]memoryCandidate, 0, len(candidateMap))
	for _, candidate := range candidateMap {
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].SourceType != candidates[j].SourceType && candidates[i].HybridScore == candidates[j].HybridScore {
			return candidates[i].SourceType == "memory"
		}
		if candidates[i].HybridScore == candidates[j].HybridScore {
			return candidates[i].Date.After(candidates[j].Date)
		}
		return candidates[i].HybridScore > candidates[j].HybridScore
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

func loadMemoryChunkContext(memoryID int64, centerIndex int) []memoryChunkContext {
	if db == nil {
		return nil
	}
	rows, err := db.Query(`
		SELECT chunk_index, chunk_text
		FROM memory_chunks
		WHERE memory_id = ? AND chunk_index BETWEEN ? AND ?
		ORDER BY chunk_index ASC
	`, memoryID, max(centerIndex-1, 0), centerIndex+1)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []memoryChunkContext
	for rows.Next() {
		var item memoryChunkContext
		if err := rows.Scan(&item.Index, &item.Text); err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
}

func formatMemoryChunkContext(match MemoryChunkMatch) string {
	contextChunks := loadMemoryChunkContext(match.ID, match.ChunkIndex)
	if len(contextChunks) == 0 {
		return compactMemoryText(match.ChunkText, 400)
	}

	parts := make([]string, 0, len(contextChunks))
	for _, chunk := range contextChunks {
		label := fmt.Sprintf("Chunk %d", chunk.Index+1)
		if chunk.Index == match.ChunkIndex {
			label += " [match]"
		}
		parts = append(parts, fmt.Sprintf("%s: %s", label, compactMemoryText(chunk.Text, 260)))
	}
	return strings.Join(parts, "\n")
}

// GetMemorySnapshot returns a formatted string of the most recent memories for system prompt injection.
func GetMemorySnapshot(userID string) string {
	debug := GetMemorySnapshotDebug(userID)
	return debug.Text
}

func GetMemorySnapshotDebug(userID string) MemorySnapshotDebug {
	results, err := SearchMemoriesByRecent(userID, 5)
	if err != nil {
		log.Printf("[MCP] Failed to get memory snapshot: %v", err)
		return MemorySnapshotDebug{Text: "No recent memories found."}
	}
	savedTurns, savedErr := ListSavedTurns(userID, 5)
	if savedErr != nil {
		log.Printf("[MCP] Failed to get saved turn snapshot: %v", savedErr)
	}
	if len(results) == 0 && len(savedTurns) == 0 {
		return MemorySnapshotDebug{Text: "No recent memories found."}
	}

	var sb strings.Builder
	debug := MemorySnapshotDebug{
		MemoryCount:    len(results),
		SavedTurnCount: len(savedTurns),
		MemoryIDs:      make([]int64, 0, len(results)),
		SavedTurnIDs:   make([]int64, 0, len(savedTurns)),
	}
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", r.CreatedAt.Format("2006-01-02"), compactMemoryText(r.FullText, 120)))
		debug.MemoryIDs = append(debug.MemoryIDs, r.ID)
	}
	for _, turn := range savedTurns {
		sb.WriteString(fmt.Sprintf("- [%s] Saved turn: %s\n", turn.CreatedAt.Format("2006-01-02"), compactMemoryText(turn.Title, 120)))
		debug.SavedTurnIDs = append(debug.SavedTurnIDs, turn.ID)
	}
	debug.Text = sb.String()
	return debug
}

// AutoSearchMemory searches for the most relevant memories using extracted keywords
// and returns their full text to be injected proactively into the system prompt.
func AutoSearchMemory(userID, input string) string {
	debug := AutoSearchMemoryDebugQuery(userID, input)
	return debug.Context
}

func AutoSearchMemoryDebugQuery(userID, input string) AutoSearchMemoryDebug {
	trimmed := strings.TrimSpace(input)
	log.Printf("[MCP] AutoSearchMemory: Input=%q", trimmed)
	if trimmed == "" {
		return AutoSearchMemoryDebug{}
	}
	candidates, err := buildMemoryCandidates(userID, trimmed, 5)
	if err != nil || len(candidates) == 0 {
		return AutoSearchMemoryDebug{}
	}

	var rawContextSb strings.Builder
	debug := AutoSearchMemoryDebug{
		Keywords:               []string{trimmed},
		RetrievedMemoriesCount: 0,
		SavedTurnHits:          0,
	}
	for i, candidate := range candidates {
		if i >= 4 {
			break
		}
		if candidate.SourceType == "saved_turn" {
			debug.SavedTurnHits++
			continue
		}
		debug.RetrievedMemoriesCount++
		if candidate.ChunkIndex >= 0 {
			debug.ChunkMatchCount++
		}
		ctx, err := ReadMemoryContextDB(userID, candidate.MemoryID, candidate.ChunkIndex)
		if err == nil {
			rawContextSb.WriteString("\n" + ctx + "\n")
		}
	}

	rawContext := rawContextSb.String()
	debug.RawContext = rawContext

	// Perform server-side memory synthesis
	syn, err := SynthesizeMemoryContext(userID, trimmed, rawContext)
	if err != nil {
		log.Printf("[MCP] Synthesize failed, falling back to compact context: %v", err)
		debug.Context = "\n[PROACTIVE MEMORY RETRIEVAL]\n" + rawContext
		return debug
	}

	if strings.TrimSpace(syn) == "" || strings.TrimSpace(syn) == "NO_RELEVANT_INFO" {
		return debug
	}

	debug.UsedSynthesis = true
	debug.SynthesizedContext = syn
	debug.Context = "\n[PROACTIVE MEMORY RETRIEVAL (Synthesized)]\n" + syn
	return debug
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
DO NOT output XML tags, HTML tags, markdown code fences, or any tool-call format.
DO NOT mention tools, commands, functions, JSON schemas, or how to search.
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
			{Role: "system", Content: "Extract facts concisely. No chat. No markdown unless necessary. Never emit XML tags, tool calls, commands, or JSON."},
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
	if isSavedTurnMemoryID(memoryID) {
		turn, err := GetSavedTurn(userID, originalSavedTurnID(memoryID))
		if err != nil {
			return "", fmt.Errorf("saved turn read failed: %v", err)
		}
		return fmt.Sprintf("Memory ID: %d\nSource: saved_turn\nDate: %s\nTitle: %s\n\n--- User Prompt ---\n%s\n\n--- Assistant Response ---\n%s",
			memoryID, turn.CreatedAt.Format("2006-01-02 15:04"), turn.Title, turn.PromptText, turn.ResponseText), nil
	}
	mem, err := ReadMemory(userID, memoryID)
	if err != nil {
		return "", fmt.Errorf("db read failed: %v", err)
	}

	return fmt.Sprintf("Memory ID: %d\nDate: %s\nType: %s\n\n--- Full Context ---\n%s",
		mem.ID, mem.CreatedAt.Format("2006-01-02 15:04"), mem.MemoryType, mem.FullText), nil
}

func ReadMemoryContextDB(userID string, memoryID int64, chunkIndex int) (string, error) {
	log.Printf("[MCP] ReadMemoryContextDB: User=%s, ID=%d, Chunk=%d", userID, memoryID, chunkIndex)
	if isSavedTurnMemoryID(memoryID) {
		turn, err := GetSavedTurn(userID, originalSavedTurnID(memoryID))
		if err != nil {
			return "", fmt.Errorf("saved turn context read failed: %v", err)
		}
		return fmt.Sprintf("Memory ID: %d\nSource: saved_turn\nDate: %s\nTitle: %s\n\n--- Prompt ---\n%s\n\n--- Response ---\n%s",
			memoryID, turn.CreatedAt.Format("2006-01-02 15:04"), turn.Title, compactMemoryText(turn.PromptText, 900), compactMemoryText(turn.ResponseText, 1400)), nil
	}

	mem, err := ReadMemory(userID, memoryID)
	if err != nil {
		return "", fmt.Errorf("memory context read failed: %v", err)
	}
	if chunkIndex < 0 {
		chunkIndex = 0
	}
	contextChunks := loadMemoryChunkContext(memoryID, chunkIndex)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Memory ID: %d\nDate: %s\nType: %s\n", mem.ID, mem.CreatedAt.Format("2006-01-02 15:04"), mem.MemoryType))
	sb.WriteString("\n--- Nearby Context ---\n")
	if len(contextChunks) == 0 {
		sb.WriteString(compactMemoryText(mem.FullText, 1800))
		return sb.String(), nil
	}
	for _, chunk := range contextChunks {
		label := fmt.Sprintf("Chunk %d", chunk.Index+1)
		if chunk.Index == chunkIndex {
			label += " [focus]"
		}
		sb.WriteString(fmt.Sprintf("%s:\n%s\n\n", label, compactMemoryText(chunk.Text, 900)))
	}
	sb.WriteString("--- Full Memory Summary ---\n")
	sb.WriteString(compactMemoryText(mem.FullText, 1200))
	return strings.TrimSpace(sb.String()), nil
}

// DeleteMemoryDB removes a specific memory entry.
func DeleteMemoryDB(userID string, memoryID int64) (string, error) {
	log.Printf("[MCP] DeleteMemoryDB: User=%s, ID=%d", userID, memoryID)
	if isSavedTurnMemoryID(memoryID) {
		err := DeleteSavedTurn(userID, originalSavedTurnID(memoryID))
		if err != nil {
			return "", fmt.Errorf("saved turn delete failed: %v", err)
		}
		return fmt.Sprintf("Successfully deleted saved turn Memory ID: %d", memoryID), nil
	}
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
		baseDir = filepath.Join(home, "Documents", macMemoryRootName, "memory")
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
