package mcp

import (
	"fmt"
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
)

type BufferedWebChunk struct {
	Index int
	Text  string
}

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
	webBufferMu sync.RWMutex
	webBuffers  = make(map[string]*userWebBuffer)
)

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
			"content":   compactMemoryText(source.Content, 24000),
		},
	))

	return source
}

func getBufferedWebSource(userID, sourceID string) (*BufferedWebSource, error) {
	userID = normalizeBufferedUserID(userID)

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
