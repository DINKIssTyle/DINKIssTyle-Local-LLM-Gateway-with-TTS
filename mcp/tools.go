package mcp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
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
