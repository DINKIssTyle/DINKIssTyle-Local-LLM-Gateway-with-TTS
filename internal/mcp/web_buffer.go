package mcp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	webBufferChunkSize    = 1000
	webBufferChunkOverlap = 150
	webBufferMaxPerUser   = 24
	webEmbeddingDims      = 128
	webEmbeddingModel     = "token-hash-v1"
)

type BufferedWebChunk struct {
	Index       int
	Text        string
	FTSScore    float64
	VectorScore float64
	HybridScore float64
}

type bufferedChunkCandidate struct {
	ID int64
	BufferedWebChunk
	FTSScore    float64
	VectorScore float64
	HybridScore float64
}

type BufferedEmbeddingProvider struct {
	ModelName      string
	Build          func(text string) ([]float64, string, error)
	BuildWithUsage func(text string, usage BufferedEmbeddingUsage) ([]float64, string, error)
}

type BufferedEmbeddingUsage string

const (
	BufferedEmbeddingUsageQuery    BufferedEmbeddingUsage = "query"
	BufferedEmbeddingUsageDocument BufferedEmbeddingUsage = "document"
)

type BufferedWebSource struct {
	SourceID   string
	UserID     string
	ToolName   string
	Query      string
	URL        string
	Title      string
	Summary    string
	Content    string
	Chunks     []BufferedWebChunk
	FetchedAt  time.Time
	LastUsedAt time.Time
}

type userWebBuffer struct {
	Sources map[string]*BufferedWebSource
	Order   []string
}

var (
	webBufferMu                sync.RWMutex
	webBuffers                 = make(map[string]*userWebBuffer)
	bufferedEmbeddingProvider  BufferedEmbeddingProvider
	bufferedEmbeddingProviderM sync.RWMutex
)

func SetBufferedEmbeddingProvider(provider BufferedEmbeddingProvider) {
	bufferedEmbeddingProviderM.Lock()
	defer bufferedEmbeddingProviderM.Unlock()
	bufferedEmbeddingProvider = provider
}

func getBufferedEmbeddingProvider() BufferedEmbeddingProvider {
	bufferedEmbeddingProviderM.RLock()
	defer bufferedEmbeddingProviderM.RUnlock()
	return bufferedEmbeddingProvider
}

func normalizeBufferedUserID(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "default"
	}
	return userID
}

func saveBufferedWebSource(userID, toolName, query, pageURL, title, content string) *BufferedWebSource {
	userID = normalizeBufferedUserID(userID)
	content = strings.TrimSpace(content)
	now := time.Now()
	if title == "" {
		title = inferBufferedTitle(pageURL, query, content)
	}

	source := &BufferedWebSource{
		SourceID:   fmt.Sprintf("src_%x", now.UnixNano()),
		UserID:     userID,
		ToolName:   toolName,
		Query:      strings.TrimSpace(query),
		URL:        strings.TrimSpace(pageURL),
		Title:      title,
		Summary:    summarizeBufferedContent(content),
		Content:    content,
		Chunks:     chunkBufferedContent(content, webBufferChunkSize, webBufferChunkOverlap),
		FetchedAt:  now,
		LastUsedAt: now,
	}

	if db != nil {
		if err := saveBufferedWebSourceDB(source); err == nil {
			EmitTrace("mcp", "buffer.saved", "Buffered web source saved", traceDetails(
				"tool", toolName,
				"user", userID,
				"source_id", source.SourceID,
				"title", source.Title,
				"url", source.URL,
				"query", source.Query,
				"chars", len(source.Content),
				"chunks", len(source.Chunks),
				"storage", "sqlite_fts5",
				"__payload", map[string]interface{}{
					"kind":      "buffered_web_source",
					"tool":      source.ToolName,
					"user":      source.UserID,
					"source_id": source.SourceID,
					"title":     source.Title,
					"query":     source.Query,
					"url":       source.URL,
					"summary":   source.Summary,
					"chars":     len(source.Content),
					"chunks":    len(source.Chunks),
					"storage":   "sqlite_fts5",
					"content":   compactMemoryText(source.Content, 24000),
				},
			))
			return source
		}
	}

	webBufferMu.Lock()
	defer webBufferMu.Unlock()

	buf := webBuffers[userID]
	if buf == nil {
		buf = &userWebBuffer{Sources: make(map[string]*BufferedWebSource)}
		webBuffers[userID] = buf
	}

	buf.Sources[source.SourceID] = source
	buf.Order = append(buf.Order, source.SourceID)

	for len(buf.Order) > webBufferMaxPerUser {
		oldest := buf.Order[0]
		buf.Order = buf.Order[1:]
		delete(buf.Sources, oldest)
	}

	EmitTrace("mcp", "buffer.saved", "Buffered web source saved", traceDetails(
		"tool", toolName,
		"user", userID,
		"source_id", source.SourceID,
		"title", source.Title,
		"url", source.URL,
		"query", source.Query,
		"chars", len(source.Content),
		"chunks", len(source.Chunks),
		"storage", "memory",
		"__payload", map[string]interface{}{
			"kind":      "buffered_web_source",
			"tool":      source.ToolName,
			"user":      source.UserID,
			"source_id": source.SourceID,
			"title":     source.Title,
			"query":     source.Query,
			"url":       source.URL,
			"summary":   source.Summary,
			"chars":     len(source.Content),
			"chunks":    len(source.Chunks),
			"storage":   "memory",
			"content":   compactMemoryText(source.Content, 24000),
		},
	))

	return source
}

func getBufferedWebSource(userID, sourceID string) (*BufferedWebSource, error) {
	userID = normalizeBufferedUserID(userID)

	if db != nil {
		source, err := getBufferedWebSourceDB(userID, sourceID)
		if err == nil {
			return source, nil
		}
	}

	webBufferMu.Lock()
	defer webBufferMu.Unlock()

	buf := webBuffers[userID]
	if buf == nil {
		return nil, fmt.Errorf("no buffered web sources for user")
	}

	if strings.TrimSpace(sourceID) != "" {
		source := buf.Sources[sourceID]
		if source == nil {
			return nil, fmt.Errorf("buffered source not found: %s", sourceID)
		}
		source.LastUsedAt = time.Now()
		return source, nil
	}

	if len(buf.Order) == 0 {
		return nil, fmt.Errorf("no buffered web sources available")
	}

	source := buf.Sources[buf.Order[len(buf.Order)-1]]
	if source == nil {
		return nil, fmt.Errorf("latest buffered source is missing")
	}
	source.LastUsedAt = time.Now()
	return source, nil
}

func formatBufferedFallbackAfterToolError(userID, failedTool, target string, err error) string {
	source, sourceErr := getBufferedWebSource(userID, "")
	if sourceErr != nil || source == nil {
		return fmt.Sprintf(
			"%s could not be completed for target %q. Error: %v. Do not retry the same page read immediately. Answer using the evidence already gathered, and clearly note that direct page fetch failed.",
			failedTool,
			strings.TrimSpace(target),
			err,
		)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s could not be completed for target %q.\n", failedTool, strings.TrimSpace(target))
	fmt.Fprintf(&b, "Error: %v\n", err)
	fmt.Fprintf(&b, "Use the existing buffered source instead of retrying the same page immediately.\n")
	fmt.Fprintf(&b, "Buffered Source ID: %s\n", source.SourceID)
	if source.Title != "" {
		fmt.Fprintf(&b, "Buffered Title: %s\n", source.Title)
	}
	if source.Query != "" {
		fmt.Fprintf(&b, "Buffered Query: %s\n", source.Query)
	}
	if source.URL != "" {
		fmt.Fprintf(&b, "Buffered URL: %s\n", source.URL)
	}
	fmt.Fprintf(&b, "Buffered Summary: %s\n", source.Summary)
	fmt.Fprintf(&b, "If more detail is needed, call read_buffered_source with this source_id and the user's question. Otherwise answer from the buffered evidence now.")
	return b.String()
}

func summarizeBufferedContent(content string) string {
	lines := strings.FieldsFunc(content, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	var picked []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		picked = append(picked, line)
		if len(strings.Join(picked, " ")) > 320 || len(picked) >= 3 {
			break
		}
	}
	if len(picked) == 0 {
		return compactMemoryText(content, 280)
	}
	return compactMemoryText(strings.Join(picked, " "), 280)
}

func inferBufferedTitle(pageURL, query, content string) string {
	lines := strings.FieldsFunc(content, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len([]rune(line)) >= 8 {
			return compactMemoryText(line, 120)
		}
	}
	if query != "" {
		return fmt.Sprintf("Search: %s", compactMemoryText(query, 80))
	}
	if pageURL != "" {
		if parsed, err := url.Parse(pageURL); err == nil {
			host := parsed.Hostname()
			if host != "" {
				return host
			}
		}
		return compactMemoryText(pageURL, 120)
	}
	return "Buffered Web Source"
}

func chunkBufferedContent(content string, chunkSize, overlap int) []BufferedWebChunk {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	runes := []rune(content)
	if chunkSize <= 0 {
		chunkSize = webBufferChunkSize
	}
	if overlap < 0 || overlap >= chunkSize {
		overlap = 0
	}

	step := chunkSize - overlap
	var chunks []BufferedWebChunk
	for start := 0; start < len(runes); start += step {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		text := strings.TrimSpace(string(runes[start:end]))
		if text != "" {
			chunks = append(chunks, BufferedWebChunk{
				Index: len(chunks),
				Text:  text,
			})
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}

func formatBufferedSourceHandle(source *BufferedWebSource) string {
	if source == nil {
		return "Buffered web source unavailable."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Buffered Web Source Saved\n")
	fmt.Fprintf(&b, "Source ID: %s\n", source.SourceID)
	if source.Title != "" {
		fmt.Fprintf(&b, "Title: %s\n", source.Title)
	}
	if source.Query != "" {
		fmt.Fprintf(&b, "Query: %s\n", source.Query)
	}
	if source.URL != "" {
		fmt.Fprintf(&b, "URL: %s\n", source.URL)
	}
	fmt.Fprintf(&b, "Summary: %s\n", source.Summary)
	fmt.Fprintf(&b, "Full content is buffered server-side for this user. Use read_buffered_source with source_id and a focused query before answering in detail.")
	return b.String()
}

func readBufferedSource(userID, sourceID, query string, maxChunks int) (string, error) {
	if db != nil {
		result, err := readBufferedSourceDB(userID, sourceID, query, maxChunks)
		if err == nil {
			return result, nil
		}
	}

	source, err := getBufferedWebSource(userID, sourceID)
	if err != nil {
		return "", err
	}
	if maxChunks <= 0 {
		maxChunks = 3
	}
	if maxChunks > 6 {
		maxChunks = 6
	}

	selected := selectRelevantBufferedChunks(source, query, maxChunks)
	if len(selected) == 0 {
		return "", fmt.Errorf("no buffered passages available")
	}

	selectedPayload := make([]map[string]interface{}, 0, len(selected))
	for _, chunk := range selected {
		selectedPayload = append(selectedPayload, map[string]interface{}{
			"index":        chunk.Index + 1,
			"text":         chunk.Text,
			"fts_score":    chunk.FTSScore,
			"vector_score": chunk.VectorScore,
			"hybrid_score": chunk.HybridScore,
		})
	}

	EmitTrace("mcp", "buffer.read", "Buffered web source excerpts selected", traceDetails(
		"tool", "read_buffered_source",
		"user", normalizeBufferedUserID(userID),
		"source_id", source.SourceID,
		"title", source.Title,
		"query", query,
		"selected_chunks", len(selected),
		"storage", "memory",
		"__payload", map[string]interface{}{
			"kind":            "buffered_web_excerpt",
			"tool":            "read_buffered_source",
			"user":            normalizeBufferedUserID(userID),
			"source_id":       source.SourceID,
			"title":           source.Title,
			"url":             source.URL,
			"query":           query,
			"summary":         source.Summary,
			"selected_chunks": selectedPayload,
			"storage":         "memory",
		},
	))

	var b strings.Builder
	fmt.Fprintf(&b, "Buffered Web Source\n")
	fmt.Fprintf(&b, "Source ID: %s\n", source.SourceID)
	fmt.Fprintf(&b, "Title: %s\n", source.Title)
	if source.URL != "" {
		fmt.Fprintf(&b, "URL: %s\n", source.URL)
	}
	if query != "" {
		fmt.Fprintf(&b, "Focus Query: %s\n", query)
	}
	fmt.Fprintf(&b, "Summary: %s\n", source.Summary)
	fmt.Fprintf(&b, "\nRelevant Excerpts:\n")
	for _, chunk := range selected {
		fmt.Fprintf(&b, "\n[Chunk %d]\n%s\n", chunk.Index+1, chunk.Text)
	}
	return strings.TrimSpace(b.String()), nil
}

func selectRelevantBufferedChunks(source *BufferedWebSource, query string, maxChunks int) []BufferedWebChunk {
	if source == nil || len(source.Chunks) == 0 {
		return nil
	}
	if strings.TrimSpace(query) == "" {
		if len(source.Chunks) < maxChunks {
			maxChunks = len(source.Chunks)
		}
		return append([]BufferedWebChunk(nil), source.Chunks[:maxChunks]...)
	}

	terms := tokenizeQuery(query)
	type scoredChunk struct {
		BufferedWebChunk
		Score int
	}
	var scored []scoredChunk
	for _, chunk := range source.Chunks {
		score := scoreBufferedChunk(chunk.Text, terms)
		if score > 0 {
			scored = append(scored, scoredChunk{BufferedWebChunk: chunk, Score: score})
		}
	}
	if len(scored) == 0 {
		if len(source.Chunks) < maxChunks {
			maxChunks = len(source.Chunks)
		}
		return append([]BufferedWebChunk(nil), source.Chunks[:maxChunks]...)
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Index < scored[j].Index
		}
		return scored[i].Score > scored[j].Score
	})

	if len(scored) > maxChunks {
		scored = scored[:maxChunks]
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Index < scored[j].Index
	})

	selected := make([]BufferedWebChunk, 0, len(scored))
	for _, item := range scored {
		selected = append(selected, item.BufferedWebChunk)
	}
	return selected
}

func tokenizeQuery(query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\r' || r == '\t' || r == ',' || r == '.' || r == ':' || r == ';' || r == '(' || r == ')' || r == '"' || r == '\'' || r == '?' || r == '!'
	})
	var terms []string
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len([]rune(field)) >= 2 {
			terms = append(terms, field)
		}
	}
	return terms
}

func scoreBufferedChunk(text string, terms []string) int {
	if len(terms) == 0 {
		return 0
	}
	text = strings.ToLower(text)
	score := 0
	for _, term := range terms {
		if strings.Contains(text, term) {
			score += 2
		}
	}
	return score
}

func saveBufferedWebSourceDB(source *BufferedWebSource) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	if source == nil {
		return fmt.Errorf("buffered source is nil")
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin buffered source tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO web_sources (
			source_id, user_id, tool_name, query_text, url, title, summary, content, fetched_at, last_used_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		source.SourceID,
		source.UserID,
		source.ToolName,
		source.Query,
		source.URL,
		source.Title,
		source.Summary,
		source.Content,
		source.FetchedAt.UTC(),
		source.LastUsedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert web source: %w", err)
	}

	for _, chunk := range source.Chunks {
		result, err := tx.Exec(`
			INSERT INTO web_source_chunks (
				source_id, user_id, chunk_index, chunk_text, token_count, created_at
			) VALUES (?, ?, ?, ?, ?, ?)
		`,
			source.SourceID,
			source.UserID,
			chunk.Index,
			chunk.Text,
			len(strings.Fields(chunk.Text)),
			source.FetchedAt.UTC(),
		)
		if err != nil {
			return fmt.Errorf("failed to insert web source chunk: %w", err)
		}
		chunkID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to read web source chunk id: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT INTO web_source_chunks_fts(rowid, chunk_text, source_id, user_id, chunk_index)
			VALUES (?, ?, ?, ?, ?)
		`, chunkID, chunk.Text, source.SourceID, source.UserID, chunk.Index); err != nil {
			return fmt.Errorf("failed to index web source chunk: %w", err)
		}
		if err := upsertBufferedChunkEmbeddingTx(tx, chunkID, chunk.Text); err != nil {
			return err
		}
	}

	if err := pruneBufferedWebSourcesTx(tx, source.UserID, webBufferMaxPerUser); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit buffered source tx: %w", err)
	}
	return nil
}

func getBufferedWebSourceDB(userID, sourceID string) (*BufferedWebSource, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	userID = normalizeBufferedUserID(userID)
	query := `
		SELECT source_id, user_id, tool_name, query_text, url, title, summary, content, fetched_at, last_used_at
		FROM web_sources
		WHERE user_id = ?
	`
	args := []interface{}{userID}
	if strings.TrimSpace(sourceID) != "" {
		query += ` AND source_id = ?`
		args = append(args, strings.TrimSpace(sourceID))
	}
	query += ` ORDER BY last_used_at DESC, fetched_at DESC, id DESC LIMIT 1`

	source := &BufferedWebSource{}
	err := db.QueryRow(query, args...).Scan(
		&source.SourceID,
		&source.UserID,
		&source.ToolName,
		&source.Query,
		&source.URL,
		&source.Title,
		&source.Summary,
		&source.Content,
		&source.FetchedAt,
		&source.LastUsedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("buffered source not found")
		}
		return nil, fmt.Errorf("failed to load buffered source: %w", err)
	}

	now := time.Now().UTC()
	source.LastUsedAt = now
	if _, err := db.Exec(`UPDATE web_sources SET last_used_at = ? WHERE source_id = ?`, now, source.SourceID); err != nil {
		return nil, fmt.Errorf("failed to update buffered source usage: %w", err)
	}
	return source, nil
}

func readBufferedSourceDB(userID, sourceID, query string, maxChunks int) (string, error) {
	source, err := getBufferedWebSourceDB(userID, sourceID)
	if err != nil {
		return "", err
	}
	if maxChunks <= 0 {
		maxChunks = 3
	}
	if maxChunks > 6 {
		maxChunks = 6
	}

	selected, retrievalMode, err := selectRelevantBufferedChunksDB(source.UserID, source.SourceID, query, maxChunks)
	if err != nil {
		return "", err
	}
	if len(selected) == 0 {
		return "", fmt.Errorf("no buffered passages available")
	}

	selectedPayload := make([]map[string]interface{}, 0, len(selected))
	for _, chunk := range selected {
		selectedPayload = append(selectedPayload, map[string]interface{}{
			"index": chunk.Index + 1,
			"text":  chunk.Text,
		})
	}

	EmitTrace("mcp", "buffer.read", "Buffered web source excerpts selected", traceDetails(
		"tool", "read_buffered_source",
		"user", normalizeBufferedUserID(userID),
		"source_id", source.SourceID,
		"title", source.Title,
		"query", query,
		"selected_chunks", len(selected),
		"storage", "sqlite_fts5",
		"retrieval_mode", retrievalMode,
		"__payload", map[string]interface{}{
			"kind":            "buffered_web_excerpt",
			"tool":            "read_buffered_source",
			"user":            normalizeBufferedUserID(userID),
			"source_id":       source.SourceID,
			"title":           source.Title,
			"url":             source.URL,
			"query":           query,
			"summary":         source.Summary,
			"selected_chunks": selectedPayload,
			"storage":         "sqlite_fts5",
			"retrieval_mode":  retrievalMode,
		},
	))

	var b strings.Builder
	fmt.Fprintf(&b, "Buffered Web Source\n")
	fmt.Fprintf(&b, "Source ID: %s\n", source.SourceID)
	fmt.Fprintf(&b, "Title: %s\n", source.Title)
	if source.URL != "" {
		fmt.Fprintf(&b, "URL: %s\n", source.URL)
	}
	if query != "" {
		fmt.Fprintf(&b, "Focus Query: %s\n", query)
	}
	fmt.Fprintf(&b, "Summary: %s\n", source.Summary)
	fmt.Fprintf(&b, "Retrieval: %s\n", retrievalMode)
	fmt.Fprintf(&b, "\nRelevant Excerpts:\n")
	for _, chunk := range selected {
		fmt.Fprintf(&b, "\n[Chunk %d | scores: %s]\n%s\n", chunk.Index+1, formatRetrievalScoreLine(chunk.FTSScore, chunk.VectorScore, chunk.HybridScore), chunk.Text)
	}
	return strings.TrimSpace(b.String()), nil
}

func formatRetrievalScoreLine(ftsScore, vectorScore, hybridScore float64) string {
	parts := make([]string, 0, 3)
	if ftsScore > 0 {
		parts = append(parts, fmt.Sprintf("fts=%.3f", ftsScore))
	}
	if vectorScore > 0 {
		parts = append(parts, fmt.Sprintf("vector=%.3f", vectorScore))
	}
	if hybridScore > 0 {
		parts = append(parts, fmt.Sprintf("hybrid=%.3f", hybridScore))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func selectRelevantBufferedChunksDB(userID, sourceID, query string, maxChunks int) ([]BufferedWebChunk, string, error) {
	if strings.TrimSpace(query) == "" {
		chunks, err := loadBufferedChunksSequentialDB(sourceID, maxChunks)
		return chunks, "sequential", err
	}

	candidates, mode, err := hybridSearchBufferedChunksDB(userID, sourceID, query, maxChunks)
	if err == nil && len(candidates) > 0 {
		return bufferedCandidatesToChunks(candidates), mode, nil
	}

	chunks, err := loadBufferedChunksSequentialDB(sourceID, maxChunks)
	if err != nil {
		return nil, "", err
	}
	return chunks, "fallback_sequential", nil
}

func loadBufferedChunksSequentialDB(sourceID string, maxChunks int) ([]BufferedWebChunk, error) {
	rows, err := db.Query(`
		SELECT chunk_index, chunk_text
		FROM web_source_chunks
		WHERE source_id = ?
		ORDER BY chunk_index ASC
		LIMIT ?
	`, sourceID, maxChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to load buffered chunks: %w", err)
	}
	defer rows.Close()

	var chunks []BufferedWebChunk
	for rows.Next() {
		var chunk BufferedWebChunk
		if err := rows.Scan(&chunk.Index, &chunk.Text); err != nil {
			return nil, fmt.Errorf("failed to scan buffered chunk: %w", err)
		}
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate buffered chunks: %w", err)
	}
	return chunks, nil
}

func searchBufferedChunksFTSDB(userID, sourceID, ftsQuery string, maxChunks int) ([]bufferedChunkCandidate, error) {
	rows, err := db.Query(`
		SELECT c.id, c.chunk_index, c.chunk_text, bm25(web_source_chunks_fts)
		FROM web_source_chunks_fts
		JOIN web_source_chunks c ON c.id = web_source_chunks_fts.rowid
		WHERE web_source_chunks_fts MATCH ?
		  AND c.user_id = ?
		  AND c.source_id = ?
		ORDER BY bm25(web_source_chunks_fts), c.chunk_index ASC
		LIMIT ?
	`, ftsQuery, userID, sourceID, maxChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to search buffered chunks via fts5: %w", err)
	}
	defer rows.Close()

	var chunks []bufferedChunkCandidate
	for rows.Next() {
		var chunk bufferedChunkCandidate
		var bm25 float64
		if err := rows.Scan(&chunk.ID, &chunk.Index, &chunk.Text, &bm25); err != nil {
			return nil, fmt.Errorf("failed to scan buffered fts chunk: %w", err)
		}
		chunk.FTSScore = normalizeFTSScore(bm25)
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate buffered fts chunks: %w", err)
	}
	return chunks, nil
}

func hybridSearchBufferedChunksDB(userID, sourceID, query string, maxChunks int) ([]bufferedChunkCandidate, string, error) {
	ftsQuery := buildBufferedFTSQuery(query)
	ftsLimit := max(maxChunks*3, maxChunks)

	ftsCandidates := make(map[int64]bufferedChunkCandidate)
	if ftsQuery != "" {
		matches, err := searchBufferedChunksFTSDB(userID, sourceID, ftsQuery, ftsLimit)
		if err == nil {
			for _, match := range matches {
				ftsCandidates[match.ID] = match
			}
		}
	}

	vectorCandidates, err := searchBufferedChunksVectorDB(userID, sourceID, query, ftsLimit)
	if err != nil {
		vectorCandidates = nil
	}

	if len(ftsCandidates) == 0 && len(vectorCandidates) == 0 {
		return nil, "", fmt.Errorf("no buffered passages available")
	}

	merged := make(map[int64]bufferedChunkCandidate, len(ftsCandidates)+len(vectorCandidates))
	for id, candidate := range ftsCandidates {
		candidate.HybridScore = candidate.FTSScore
		merged[id] = candidate
	}
	for _, candidate := range vectorCandidates {
		existing, ok := merged[candidate.ID]
		if ok {
			existing.VectorScore = candidate.VectorScore
			existing.HybridScore = (existing.FTSScore * 0.65) + (candidate.VectorScore * 0.35)
			merged[candidate.ID] = existing
			continue
		}
		candidate.HybridScore = candidate.VectorScore * 0.35
		merged[candidate.ID] = candidate
	}

	ranked := make([]bufferedChunkCandidate, 0, len(merged))
	for _, candidate := range merged {
		ranked = append(ranked, candidate)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if math.Abs(ranked[i].HybridScore-ranked[j].HybridScore) < 1e-9 {
			return ranked[i].Index < ranked[j].Index
		}
		return ranked[i].HybridScore > ranked[j].HybridScore
	})
	if len(ranked) > maxChunks {
		ranked = ranked[:maxChunks]
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].Index < ranked[j].Index
	})

	mode := "hybrid_fts5_vector"
	if len(ftsCandidates) == 0 {
		mode = "vector_only"
	} else if len(vectorCandidates) == 0 {
		mode = "fts5"
	}
	return ranked, mode, nil
}

func searchBufferedChunksVectorDB(userID, sourceID, query string, maxChunks int) ([]bufferedChunkCandidate, error) {
	queryVector, queryModel := buildBufferedEmbedding(query, BufferedEmbeddingUsageQuery)
	if len(queryVector) == 0 {
		return nil, nil
	}

	rows, err := db.Query(`
		SELECT c.id, c.chunk_index, c.chunk_text, e.embedding_json, e.embedding_model
		FROM web_source_chunks c
		JOIN web_chunk_embeddings e ON e.chunk_id = c.id
		WHERE c.user_id = ?
		  AND c.source_id = ?
	`, userID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load buffered vector candidates: %w", err)
	}
	defer rows.Close()

	var ranked []bufferedChunkCandidate
	for rows.Next() {
		var candidate bufferedChunkCandidate
		var embeddingJSON string
		var embeddingModel string
		if err := rows.Scan(&candidate.ID, &candidate.Index, &candidate.Text, &embeddingJSON, &embeddingModel); err != nil {
			return nil, fmt.Errorf("failed to scan buffered vector candidate: %w", err)
		}
		if strings.TrimSpace(queryModel) != "" && strings.TrimSpace(embeddingModel) != "" && embeddingModel != queryModel {
			continue
		}
		vector, err := parseBufferedEmbeddingJSON(embeddingJSON)
		if err != nil {
			continue
		}
		score := cosineSimilarity(queryVector, vector)
		if score <= 0 {
			continue
		}
		candidate.VectorScore = score
		ranked = append(ranked, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate buffered vector candidates: %w", err)
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if math.Abs(ranked[i].VectorScore-ranked[j].VectorScore) < 1e-9 {
			return ranked[i].Index < ranked[j].Index
		}
		return ranked[i].VectorScore > ranked[j].VectorScore
	})
	if len(ranked) > maxChunks {
		ranked = ranked[:maxChunks]
	}
	return ranked, nil
}

func buildBufferedFTSQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	rawTerms := tokenizeQuery(query)
	seen := make(map[string]struct{}, len(rawTerms))
	terms := make([]string, 0, len(rawTerms))
	for _, term := range rawTerms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, quoteFTSPhrase(term))
	}
	if len(terms) == 0 {
		return quoteFTSPhrase(query)
	}
	return strings.Join(terms, " OR ")
}

func quoteFTSPhrase(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), `"`, `""`)
	if value == "" {
		return `""`
	}
	return `"` + value + `"`
}

func bufferedCandidatesToChunks(candidates []bufferedChunkCandidate) []BufferedWebChunk {
	chunks := make([]BufferedWebChunk, 0, len(candidates))
	for _, candidate := range candidates {
		chunks = append(chunks, candidate.BufferedWebChunk)
	}
	return chunks
}

func normalizeFTSScore(bm25 float64) float64 {
	return 1.0 / (1.0 + math.Abs(bm25))
}

func upsertBufferedChunkEmbeddingTx(tx *sql.Tx, chunkID int64, text string) error {
	vector, modelName := buildBufferedEmbedding(text, BufferedEmbeddingUsageDocument)
	if len(vector) == 0 {
		return nil
	}
	if strings.TrimSpace(modelName) == "" {
		modelName = webEmbeddingModel
	}
	embeddingJSON, err := json.Marshal(vector)
	if err != nil {
		return fmt.Errorf("failed to marshal buffered embedding: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO web_chunk_embeddings (
			chunk_id, embedding_model, embedding_dim, embedding_json, updated_at
		) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(chunk_id) DO UPDATE SET
			embedding_model = excluded.embedding_model,
			embedding_dim = excluded.embedding_dim,
			embedding_json = excluded.embedding_json,
			updated_at = excluded.updated_at
	`, chunkID, modelName, len(vector), string(embeddingJSON), time.Now().UTC()); err != nil {
		return fmt.Errorf("failed to store buffered chunk embedding: %w", err)
	}
	return nil
}

func buildBufferedEmbedding(text string, usage BufferedEmbeddingUsage) ([]float64, string) {
	provider := getBufferedEmbeddingProvider()
	if provider.BuildWithUsage != nil {
		vector, modelName, err := provider.BuildWithUsage(text, usage)
		if err == nil && len(vector) > 0 {
			if strings.TrimSpace(modelName) == "" {
				modelName = provider.ModelName
			}
			if strings.TrimSpace(modelName) == "" {
				modelName = webEmbeddingModel
			}
			return vector, modelName
		}
	}
	if provider.Build != nil {
		vector, modelName, err := provider.Build(text)
		if err == nil && len(vector) > 0 {
			if strings.TrimSpace(modelName) == "" {
				modelName = provider.ModelName
			}
			if strings.TrimSpace(modelName) == "" {
				modelName = webEmbeddingModel
			}
			return vector, modelName
		}
	}

	terms := tokenizeQuery(text)
	if len(terms) == 0 {
		return nil, webEmbeddingModel
	}

	vector := make([]float64, webEmbeddingDims)
	for _, term := range terms {
		hash := bufferedTokenHash(term)
		vector[hash%webEmbeddingDims] += 1.0
	}

	var magnitude float64
	for _, value := range vector {
		magnitude += value * value
	}
	if magnitude == 0 {
		return nil, webEmbeddingModel
	}
	magnitude = math.Sqrt(magnitude)
	for i := range vector {
		vector[i] = vector[i] / magnitude
	}
	return vector, webEmbeddingModel
}

func bufferedTokenHash(term string) uint32 {
	var hash uint32 = 2166136261
	for _, r := range term {
		hash ^= uint32(r)
		hash *= 16777619
	}
	return hash
}

func parseBufferedEmbeddingJSON(raw string) ([]float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty embedding")
	}
	var vector []float64
	if err := json.Unmarshal([]byte(raw), &vector); err != nil {
		return nil, err
	}
	return vector, nil
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot float64
	for i := range a {
		dot += a[i] * b[i]
	}
	return dot
}

func pruneBufferedWebSourcesTx(tx *sql.Tx, userID string, keep int) error {
	if keep <= 0 {
		return nil
	}

	rows, err := tx.Query(`
		SELECT source_id
		FROM web_sources
		WHERE user_id = ?
		ORDER BY last_used_at DESC, fetched_at DESC, id DESC
		LIMIT -1 OFFSET ?
	`, userID, keep)
	if err != nil {
		return fmt.Errorf("failed to query buffered source prune list: %w", err)
	}
	defer rows.Close()

	var stale []string
	for rows.Next() {
		var sourceID string
		if err := rows.Scan(&sourceID); err != nil {
			return fmt.Errorf("failed to scan buffered prune row: %w", err)
		}
		stale = append(stale, sourceID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate buffered prune rows: %w", err)
	}

	for _, sourceID := range stale {
		if err := deleteBufferedWebSourceTx(tx, sourceID); err != nil {
			return err
		}
	}
	return nil
}

func deleteBufferedWebSourceTx(tx *sql.Tx, sourceID string) error {
	chunkRows, err := tx.Query(`SELECT id FROM web_source_chunks WHERE source_id = ?`, sourceID)
	if err != nil {
		return fmt.Errorf("failed to query buffered source chunks for delete: %w", err)
	}

	var chunkIDs []int64
	for chunkRows.Next() {
		var chunkID int64
		if err := chunkRows.Scan(&chunkID); err != nil {
			chunkRows.Close()
			return fmt.Errorf("failed to scan buffered chunk id for delete: %w", err)
		}
		chunkIDs = append(chunkIDs, chunkID)
	}
	if err := chunkRows.Err(); err != nil {
		chunkRows.Close()
		return fmt.Errorf("failed to iterate buffered chunk ids for delete: %w", err)
	}
	chunkRows.Close()

	for _, chunkID := range chunkIDs {
		if _, err := tx.Exec(`DELETE FROM web_chunk_embeddings WHERE chunk_id = ?`, chunkID); err != nil {
			return fmt.Errorf("failed to delete buffered chunk embedding: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM web_source_chunks_fts WHERE rowid = ?`, chunkID); err != nil {
			return fmt.Errorf("failed to delete buffered chunk fts row: %w", err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM web_source_chunks WHERE source_id = ?`, sourceID); err != nil {
		return fmt.Errorf("failed to delete buffered chunks: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM web_sources WHERE source_id = ?`, sourceID); err != nil {
		return fmt.Errorf("failed to delete buffered source: %w", err)
	}
	return nil
}
