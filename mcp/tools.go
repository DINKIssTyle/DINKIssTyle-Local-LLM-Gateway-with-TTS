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
// action: "read", "append", "rewrite"
func ManageMemory(filePath string, action string, content string) (string, error) {
	log.Printf("[MCP] ManageMemory Action: %s, Path: %s", action, filePath)

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}

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

	case "append":
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("content cannot be empty for append")
		}
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to open memory file: %v", err)
		}
		defer f.Close()

		timestamp := time.Now().Format("2006-01-02 15:04:05")
		entry := fmt.Sprintf("\n[%s] %s", timestamp, content)
		if _, err := f.WriteString(entry); err != nil {
			return "", fmt.Errorf("failed to append to memory: %v", err)
		}
		return "Memory updated successfully.", nil

	case "rewrite":
		// Safeguard: Never rewrite with empty content via tool calls
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("content cannot be empty for rewrite. If you want to clear memory, use a specific phrase or handle carefully.")
		}
		// Overwrite the file completely
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("failed to rewrite memory: %v", err)
		}
		return "Memory rewritten successfully.", nil

	case "search":
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("search query (content) cannot be empty")
		}
		// Simple case-insensitive line search to save context tokens
		data, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return "Memory is empty.", nil
			}
			return "", fmt.Errorf("failed to read memory for search: %v", err)
		}

		lines := strings.Split(string(data), "\n")
		var results []string
		query := strings.ToLower(content)

		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), query) {
				results = append(results, line)
			}
		}

		if len(results) == 0 {
			return fmt.Sprintf("No matches found for '%s'.", content), nil
		}
		return strings.Join(results, "\n"), nil

	case "replace":
		// Syntax: "old_text|new_text"
		parts := strings.Split(content, "|")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid replace syntax. Use 'old_text|new_text'")
		}
		oldText, newText := parts[0], parts[1]

		if strings.TrimSpace(oldText) == "" {
			return "", fmt.Errorf("old text cannot be empty")
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read memory for replacement: %v", err)
		}

		fileContent := string(data)
		if !strings.Contains(fileContent, oldText) {
			return fmt.Sprintf("Text '%s' not found in memory. Use 'search' to find the exact text first.", oldText), nil
		}

		newFileContent := strings.ReplaceAll(fileContent, oldText, newText)
		if err := os.WriteFile(filePath, []byte(newFileContent), 0644); err != nil {
			return "", fmt.Errorf("failed to save memory after replacement: %v", err)
		}
		return fmt.Sprintf("Successfully replaced '%s' with '%s'.", oldText, newText), nil

	case "upsert":
		// Syntax: "Key: Value"
		sepIdx := strings.Index(content, ":")
		if sepIdx == -1 {
			return "", fmt.Errorf("invalid upsert syntax. Use 'Key: Value'")
		}
		key := strings.TrimSpace(content[:sepIdx])
		if key == "" {
			return "", fmt.Errorf("key cannot be empty for upsert")
		}

		data, err := os.ReadFile(filePath)
		var fileContent string
		if err != nil {
			if !os.IsNotExist(err) {
				return "", fmt.Errorf("failed to read memory for upsert: %v", err)
			}
			fileContent = ""
		} else {
			fileContent = string(data)
		}

		timestamp := time.Now().Format("2006-01-02 15:04:05")
		newLine := fmt.Sprintf("[%s] %s", timestamp, strings.TrimSpace(content))

		lines := strings.Split(fileContent, "\n")
		found := false
		keyPattern := " " + key + ":"
		startPattern := key + ":"

		for i, line := range lines {
			// Check if line contains " Key:" or starts with "Key:"
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, startPattern) || strings.Contains(line, keyPattern) {
				lines[i] = newLine
				found = true
				break
			}
		}

		if !found {
			if len(fileContent) > 0 && !strings.HasSuffix(fileContent, "\n") {
				fileContent += "\n"
			}
			fileContent += newLine
		} else {
			fileContent = strings.Join(lines, "\n")
		}

		if err := os.WriteFile(filePath, []byte(fileContent), 0644); err != nil {
			return "", fmt.Errorf("failed to save memory after upsert: %v", err)
		}
		return fmt.Sprintf("Successfully upserted '%s'.", key), nil

	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

// GetUserMemoryPath resolves the memory file path based on OS and User ID.
func GetUserMemoryPath(userID string) (string, error) {
	if userID == "" {
		userID = "default"
	}

	var baseDir string
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(home, "Documents", "DKST LLM Chat")
	} else {
		// Windows/Linux: Executable directory
		ex, err := os.Executable()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(filepath.Dir(ex), "memory")
	}

	filename := fmt.Sprintf("%s.md", userID)
	return filepath.Join(baseDir, filename), nil
}
