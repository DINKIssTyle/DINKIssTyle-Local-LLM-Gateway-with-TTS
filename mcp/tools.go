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
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

var (
	memoryMu sync.Mutex
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

// ManageMemory handles reading and writing to the user's memory file (personal.md).
// Simplified to use direct markdown file manipulation.
// Supported actions: read, remember, forget, query
func ManageMemory(filePath string, action string, content string) (string, error) {
	log.Printf("[MCP] ManageMemory Action: %s, Path: %s", action, filePath)

	memoryMu.Lock()
	defer memoryMu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}

	// We primarily operate on 'personal.md' in the same directory as filePath (which might be passed as anything)
	// But let's assume filePath IS the target file (usually personal.md for 'remember').
	// If action is 'read', we might want to read a specific file or the default 'personal.md'.
	// To be safe and consistent with new architecture:
	// If filePath ends in .md, use it. If not, default to personal.md?
	// The agent usually passes the full path to personal.md.

	switch action {
	case "read":
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

	case "remember":
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("content cannot be empty for remember")
		}

		// Append to file
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to open memory file: %v", err)
		}
		defer f.Close()

		// Simple format: - content
		entry := fmt.Sprintf("- %s\n", strings.TrimSpace(content))
		if _, err := f.WriteString(entry); err != nil {
			return "", fmt.Errorf("failed to write to memory: %v", err)
		}

		return fmt.Sprintf("Remembered: %s", content), nil

	case "forget":
		// For 'forget', we need to read the file, remove the line, and rewrite.
		// This is a bit expensive but fine for text files.
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("content to forget cannot be empty")
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read memory for forget: %v", err)
		}

		lines := strings.Split(string(data), "\n")
		var newLines []string
		deleted := false
		target := strings.ToLower(strings.TrimSpace(content))

		for _, line := range lines {
			// fuzzy match or exact match? Let's do contains for now to be easier.
			// But 'content' usually comes from the LLM wishing to delete a specific fact.
			if strings.Contains(strings.ToLower(line), target) {
				deleted = true
				continue // Skip this line
			}
			newLines = append(newLines, line)
		}

		if !deleted {
			return "Fact not found in memory.", nil
		}

		output := strings.Join(newLines, "\n")
		if err := os.WriteFile(filePath, []byte(output), 0644); err != nil {
			return "", fmt.Errorf("failed to update memory file: %v", err)
		}

		return fmt.Sprintf("Forgot facts containing: %s", content), nil

	case "query":
		// Simple grep
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("query cannot be empty")
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return "Memory is empty.", nil
			}
			return "", fmt.Errorf("failed to read memory: %v", err)
		}

		lines := strings.Split(string(data), "\n")
		var matches []string
		query := strings.ToLower(strings.TrimSpace(content))

		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), query) {
				matches = append(matches, line)
			}
		}

		if len(matches) == 0 {
			return fmt.Sprintf("No memory found for '%s'.", content), nil
		}
		return strings.Join(matches, "\n"), nil

	default:
		return "", fmt.Errorf("unknown action: %s. Supported: read, remember, forget, query", action)
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
