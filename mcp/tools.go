package mcp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// GetCurrentTime returns the current local time in a readable format.
func GetCurrentTime() (string, error) {
	now := time.Now()
	// Format: 2026-02-06 09:02:06 Friday (KST/Local)
	return fmt.Sprintf("Current Local Time: %s", now.Format("2006-01-02 15:04:05 Monday")), nil
}

// SearchWeb performs a search using DuckDuckGo Lite and returns a summary.
func SearchWeb(query string) (string, error) {
	log.Printf("[MCP] Searching Web for: %s", query)

	// Use DuckDuckGo Lite for easier parsing
	searchURL := fmt.Sprintf("https://lite.duckduckgo.com/lite/?q=%s", url.QueryEscape(query))

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	htmlContent := string(body)

	// Debug log to see what we got
	preview := htmlContent
	if len(preview) > 500 {
		preview = preview[:500]
	}
	log.Printf("[MCP-DEBUG] Search HTML Preview: %s", preview)

	// Simple regex parsing for DDG Lite results
	// Pattern to find links and snippets
	// <a rel="nofollow" href="http://...">Title</a><br><span class="result-snippet">Snippet</span>
	// This is approximate and might need adjustment if DDG changes HTML

	// Strategy: Extract the table rows that contain results
	// DDG Lite uses tables. We look for class="result-link" and result-snippet
	// Use (?s) to allow . to match newlines

	var results []string

	// Extract titles and links
	// HTML: <a rel="nofollow" href="..." class='result-link'>Title</a>
	// HTML: <td class='result-snippet'>Snippet</td>
	linkRegex := regexp.MustCompile(`(?s)href="(.*?)" class='result-link'>(.*?)</a>`)
	snippetRegex := regexp.MustCompile(`(?s)class='result-snippet'>(.*?)</td>`)

	matches := linkRegex.FindAllStringSubmatch(htmlContent, 5) // Limit to top 5
	snippets := snippetRegex.FindAllStringSubmatch(htmlContent, 5)

	count := len(matches)
	if len(snippets) < count {
		count = len(snippets)
	}

	for i := 0; i < count; i++ {
		link := matches[i][1]
		title := matches[i][2]
		snippet := snippets[i][1]

		// Clean up HTML entities if needed (basic ones)
		title = strings.ReplaceAll(title, "<b>", "")
		title = strings.ReplaceAll(title, "</b>", "")
		title = strings.ReplaceAll(title, "&quot;", "\"")
		title = strings.ReplaceAll(title, "&amp;", "&")

		snippet = strings.ReplaceAll(snippet, "&quot;", "\"")
		snippet = strings.ReplaceAll(snippet, "&amp;", "&")

		results = append(results, fmt.Sprintf("Title: %s\nLink: %s\nSnippet: %s\n", title, link, snippet))
	}

	if len(results) == 0 {
		return "No results found or parsing failed.", nil
	}

	return strings.Join(results, "\n---\n"), nil
}

// ReadPage fetches the text content of a URL using a headless browser.
func ReadPage(pageURL string) (string, error) {
	log.Printf("[MCP] Reading Page: %s", pageURL)

	// Create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Set timeout
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var res string
	err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.Sleep(2*time.Second), // Wait for dynamic content
		chromedp.Evaluate(`document.body.innerText`, &res),
	)

	if err != nil {
		return "", fmt.Errorf("failed to read page: %v", err)
	}

	// truncate if too long (simple protection)
	if len(res) > 20000 {
		res = res[:20000] + "... (truncated)"
	}

	return res, nil
}

// ManageMemory handles reading and writing to the user's memory file.
// Enhanced with server-side processing to protect data.
// Supported actions: read, remember, forget, query, search (legacy), append (legacy), upsert (legacy)
func ManageMemory(filePath string, action string, content string) (string, error) {
	log.Printf("[MCP] ManageMemory Action: %s, Path: %s", action, filePath)

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}

	// Extract userID from filePath for index operations
	// New structure: /path/to/memory/{userID}/log.md -> extract {userID} from parent dir
	userID := filepath.Base(filepath.Dir(filePath))

	switch action {
	case "read":
		// Read index summary instead of raw file for efficiency
		summary := GetIndexSummaryForPrompt(userID)
		if summary == "" {
			// Fallback to raw file for backwards compatibility
			data, err := os.ReadFile(filePath)
			if err != nil {
				if os.IsNotExist(err) {
					return "Memory is empty.", nil
				}
				return "", fmt.Errorf("failed to read memory: %v", err)
			}
			if len(data) == 0 {
				return "Memory is empty.", nil
			}
			return string(data), nil
		}
		return fmt.Sprintf("=== User Memory Index ===\n%s", summary), nil

	case "remember":
		// Server extracts key automatically (Append-Only, safe)
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("content cannot be empty for remember")
		}

		key, value := ExtractKeyFromContent(content)
		log.Printf("[MCP] Remember: Extracted key='%s' from content", key)

		// Append to log (never overwrites)
		if err := AppendToLog(filePath, "SET", key, value); err != nil {
			return "", fmt.Errorf("failed to save memory: %v", err)
		}

		// Rebuild index
		if _, err := RebuildAndSaveIndex(userID); err != nil {
			log.Printf("[MCP] Warning: Failed to rebuild index: %v", err)
		}

		return fmt.Sprintf("Remembered: %s = %s", key, value), nil

	case "forget":
		// Mark as deleted in log (data preserved in log history)
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("key to forget cannot be empty")
		}

		key := strings.ToLower(strings.TrimSpace(content))

		// Check if key exists in index
		indexPath, _ := GetMemoryIndexPath(userID)
		index, _ := LoadIndex(indexPath)
		if _, exists := index.Facts[key]; !exists {
			return fmt.Sprintf("Key '%s' not found in memory.", key), nil
		}

		// Append DELETE entry
		if err := AppendToLog(filePath, "DELETE", key, ""); err != nil {
			return "", fmt.Errorf("failed to update memory: %v", err)
		}

		// Rebuild index
		if _, err := RebuildAndSaveIndex(userID); err != nil {
			log.Printf("[MCP] Warning: Failed to rebuild index: %v", err)
		}

		return fmt.Sprintf("Forgot: %s (log preserved)", key), nil

	case "query":
		// Fast lookup from index
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("query key cannot be empty")
		}

		key := strings.ToLower(strings.TrimSpace(content))
		indexPath, _ := GetMemoryIndexPath(userID)
		index, err := LoadIndex(indexPath)
		if err != nil {
			return "", fmt.Errorf("failed to load index: %v", err)
		}

		if val, ok := index.Facts[key]; ok {
			return fmt.Sprintf("%s: %s", key, val), nil
		}

		// Fuzzy search in keys
		var matches []string
		for k, v := range index.Facts {
			if strings.Contains(k, key) || strings.Contains(strings.ToLower(v), key) {
				matches = append(matches, fmt.Sprintf("%s: %s", k, v))
			}
		}

		if len(matches) > 0 {
			return fmt.Sprintf("Partial matches:\n%s", strings.Join(matches, "\n")), nil
		}

		return fmt.Sprintf("No memory found for '%s'.", key), nil

	case "append":
		// Legacy: Redirect to remember for safety
		return ManageMemory(filePath, "remember", content)

	case "rewrite":
		// DEPRECATED: Blocked for safety
		return "", fmt.Errorf("'rewrite' action is disabled for data protection. Use 'remember' to add new facts or 'forget' to remove")

	case "search":
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("search query (content) cannot be empty")
		}

		// Try index first
		indexPath, _ := GetMemoryIndexPath(userID)
		index, _ := LoadIndex(indexPath)
		if len(index.Facts) > 0 {
			query := strings.ToLower(content)
			var matches []string
			for k, v := range index.Facts {
				if strings.Contains(k, query) || strings.Contains(strings.ToLower(v), query) {
					matches = append(matches, fmt.Sprintf("%s: %s", k, v))
				}
			}
			if len(matches) > 0 {
				return strings.Join(matches, "\n"), nil
			}
		}

		// Fallback to raw log search
		data, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return "Memory is empty.", nil
			}
			return "", fmt.Errorf("failed to read memory for search: %v", err)
		}

		fileContent := string(data)
		fileSize := len(fileContent)
		lines := strings.Split(fileContent, "\n")
		var results []string

		// 1. Precise check: Substring match
		query := strings.ToLower(content)
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), query) {
				results = append(results, line)
			}
		}

		// 2. Keyword check: If no precise match, split query into words
		if len(results) == 0 {
			keywords := strings.Fields(query)
			if len(keywords) > 1 {
				for _, line := range lines {
					lineLower := strings.ToLower(line)
					matchCount := 0
					for _, k := range keywords {
						if len(k) < 2 {
							continue
						} // Skip very short keywords
						if strings.Contains(lineLower, k) {
							matchCount++
						}
					}
					// If at least 50% of keywords match, include the line
					if matchCount > 0 && float64(matchCount)/float64(len(keywords)) >= 0.5 {
						results = append(results, line)
					}
				}
			}
		}

		// 3. Fallback: If still no results and file is small (< 5KB), return everything
		if len(results) == 0 && fileSize < 5120 {
			return fmt.Sprintf("[AUTOMATIC READ FALLBACK - No direct matches for '%s']\n%s", content, fileContent), nil
		}

		if len(results) == 0 {
			return fmt.Sprintf("No matches found for '%s' in a large memory file. Try different keywords.", content), nil
		}
		return strings.Join(results, "\n"), nil

	case "replace":
		// DEPRECATED: Too dangerous, redirect to remember
		return "", fmt.Errorf("'replace' action is disabled for data protection. Use 'remember' to add new facts")

	case "upsert":
		// Legacy: Redirect to remember for Append-Only safety
		// Extract key:value format and pass to remember
		return ManageMemory(filePath, "remember", content)

	default:
		return "", fmt.Errorf("unknown action: %s. Supported: read, remember, forget, query, search", action)
	}
}

// GetUserMemoryDir returns the memory directory path for a user based on OS.
// macOS: ~/Documents/DKST LLM Chat/memory/{userID}/
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
		baseDir = filepath.Join(home, "Documents", "DKST LLM Chat", "memory")
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
// filename can be: "personal.md", "work.md", "index.md", "log.md"
func GetUserMemoryFilePath(userID, filename string) (string, error) {
	dir, err := GetUserMemoryDir(userID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filename), nil
}

// GetUserMemoryPath is kept for backward compatibility - returns log.md path
func GetUserMemoryPath(userID string) (string, error) {
	return GetUserMemoryFilePath(userID, "log.md")
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
