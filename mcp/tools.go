package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

var (
	memoryMu sync.Mutex
)

func compactMemoryText(input string, limit int) string {
	input = strings.TrimSpace(input)
	if limit <= 0 || len([]rune(input)) <= limit {
		return input
	}
	runes := []rune(input)
	return strings.TrimSpace(string(runes[:limit])) + "... (truncated)"
}

// GetCurrentTime returns the current local time in a readable format including timezone.
func GetCurrentTime() (string, error) {
	now := time.Now()
	// Format: 2026-02-06 09:02:06 Friday KST
	return fmt.Sprintf("Current Local Time: %s", now.Format("2006-01-02 15:04:05 Monday MST")), nil
}

// SearchWeb performs a web search using Google first, then falls back to DuckDuckGo Lite.
func SearchWeb(query string) (string, error) {
	originalQuery := query
	query = normalizeSearchQuery(query)
	log.Printf("[MCP] Searching Web for: %s", query)
	start := time.Now()
	traceArgs := []interface{}{"query", query}
	if query != originalQuery {
		traceArgs = append(traceArgs, "original_query", originalQuery)
	}
	EmitTrace("mcp", "search_web.start", "Starting web search", traceDetails(traceArgs...))

	client := &http.Client{Timeout: 10 * time.Second}
	if results, err := searchGoogle(query, client); err == nil && len(results) > 0 {
		EmitTrace("mcp", "search_web.complete", "Google search completed", traceDetails("query", query, "elapsed_ms", durationMs(start), "results", len(results), "provider", "google"))
		return strings.Join(results, "\n---\n"), nil
	} else if err != nil {
		log.Printf("[MCP] Google search failed, falling back to DuckDuckGo: %v", err)
	} else {
		log.Printf("[MCP] Google search returned no parsed results, falling back to DuckDuckGo")
	}

	results, err := searchDuckDuckGo(query, client)
	if err != nil {
		EmitTrace("mcp", "search_web.error", "Web search failed", traceDetails("query", query, "elapsed_ms", durationMs(start), "error", errorDetail(err)))
		return "", err
	}
	if len(results) == 0 {
		EmitTrace("mcp", "search_web.complete", "Web search returned no parsed results", traceDetails("query", query, "elapsed_ms", durationMs(start), "provider", "duckduckgo"))
		return "No results found or parsing failed.", nil
	}

	EmitTrace("mcp", "search_web.complete", "DuckDuckGo search completed", traceDetails("query", query, "elapsed_ms", durationMs(start), "results", len(results), "provider", "duckduckgo"))
	return strings.Join(results, "\n---\n"), nil
}

func searchGoogle(query string, client *http.Client) ([]string, error) {
	searchURL := fmt.Sprintf("https://www.google.com/search?hl=en&q=%s", url.QueryEscape(query))
	htmlContent, err := fetchSearchPage(client, searchURL)
	if err != nil {
		return nil, err
	}

	blockRegex := regexp.MustCompile(`(?s)<a href="/url\?q=(https?://[^"&]+)[^"]*".*?<h3[^>]*>(.*?)</h3>.*?</a>(.*?)</div>`)
	snippetRegex := regexp.MustCompile(`(?s)<div[^>]*data-sncf="1"[^>]*>(.*?)</div>`)
	matches := blockRegex.FindAllStringSubmatch(htmlContent, 5)

	var results []string
	for _, match := range matches {
		link := html.UnescapeString(match[1])
		title := cleanSearchText(match[2])
		snippet := ""
		if parts := snippetRegex.FindStringSubmatch(match[3]); len(parts) > 1 {
			snippet = cleanSearchText(parts[1])
		}
		if title == "" || link == "" {
			continue
		}
		results = append(results, fmt.Sprintf("Title: %s\nLink: %s\nSnippet: %s\n", title, link, snippet))
	}
	return results, nil
}

func searchDuckDuckGo(query string, client *http.Client) ([]string, error) {
	searchURL := fmt.Sprintf("https://lite.duckduckgo.com/lite/?q=%s", url.QueryEscape(query))
	htmlContent, err := fetchSearchPage(client, searchURL)
	if err != nil {
		return nil, err
	}

	linkRegex := regexp.MustCompile(`(?s)href="(.*?)" class='result-link'>(.*?)</a>`)
	snippetRegex := regexp.MustCompile(`(?s)class='result-snippet'>(.*?)</td>`)

	matches := linkRegex.FindAllStringSubmatch(htmlContent, 5)
	snippets := snippetRegex.FindAllStringSubmatch(htmlContent, 5)

	count := len(matches)
	if len(snippets) < count {
		count = len(snippets)
	}

	var results []string
	for i := 0; i < count; i++ {
		link := cleanSearchText(matches[i][1])
		title := cleanSearchText(matches[i][2])
		snippet := cleanSearchText(snippets[i][1])
		if title == "" || link == "" {
			continue
		}
		results = append(results, fmt.Sprintf("Title: %s\nLink: %s\nSnippet: %s\n", title, link, snippet))
	}

	return results, nil
}

func fetchSearchPage(client *http.Client, searchURL string) (string, error) {
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("search provider returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	htmlContent := string(body)
	preview := htmlContent
	if len(preview) > 500 {
		preview = preview[:500]
	}
	log.Printf("[MCP-DEBUG] Search HTML Preview: %s", preview)
	return htmlContent, nil
}

func cleanSearchText(input string) string {
	input = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(input, " ")
	input = html.UnescapeString(input)
	input = strings.ReplaceAll(input, "\u00a0", " ")
	input = strings.Join(strings.Fields(input), " ")
	return strings.TrimSpace(input)
}

// SearchNamuwiki searches Namuwiki by constructing a direct URL and reading the page.
func SearchNamuwiki(keyword string) (string, error) {
	log.Printf("[MCP] Searching Namuwiki for: %s", keyword)

	// Construct Namuwiki URL: https://namu.wiki/w/Keyword
	// Namuwiki uses direct path for terms
	encodedKeyword := url.PathEscape(keyword)
	targetURL := fmt.Sprintf("https://namu.wiki/w/%s", encodedKeyword)

	// Reuse ReadPage to fetch content
	// Namuwiki relies heavily on JS, so ReadPage (chromedp) is perfect.
	content, err := ReadPage(targetURL)
	if err != nil {
		return "", fmt.Errorf("failed to read Namuwiki page: %v", err)
	}

	return content, nil
}

// SearchNaver performs a search on Naver and returns the page content.
// Specialized for dictionary, Korea-related content, weather, and news.
func SearchNaver(query string) (string, error) {
	log.Printf("[MCP] Searching Naver for: %s", query)

	searchURL := fmt.Sprintf("https://search.naver.com/search.naver?&sm=top_hty&fbm=0&ie=utf8&query=%s", url.QueryEscape(query))

	// Reuse ReadPage to fetch content
	content, err := ReadPage(searchURL)
	if err != nil {
		return "", fmt.Errorf("failed to search Naver: %v", err)
	}

	return content, nil
}

// ReadPage fetches the text content of a URL using a headless browser with anti-detection.
func ReadPage(pageURL string) (string, error) {
	log.Printf("[MCP] Reading Page (Advanced + Anti-Detection): %s", pageURL)
	start := time.Now()
	EmitTrace("mcp", "read_web_page.start", "Starting page read", traceDetails("url", pageURL))

	// 1. Anti-Detection: Configure browser with stealth flags
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-features", "TranslateUI"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	timeout := readPageTimeoutForURL(pageURL)
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	var res string
	err := chromedp.Run(ctx,
		// 2. Anti-Detection: Override navigator.webdriver before any page loads
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(`
				Object.defineProperty(navigator, 'webdriver', {get: () => false});
				if (!window.chrome) { window.chrome = {}; }
				if (!window.chrome.runtime) { window.chrome.runtime = {}; }
				Object.defineProperty(navigator, 'plugins', {get: () => [1, 2, 3, 4, 5]});
				Object.defineProperty(navigator, 'languages', {get: () => ['ko-KR', 'ko', 'en-US', 'en']});
			`).Do(ctx)
			return err
		}),

		chromedp.Navigate(pageURL),

		// 3. Anti-Detection: Wait briefly for challenge pages, but keep the total budget bounded.
		chromedp.ActionFunc(func(ctx context.Context) error {
			maxChallengeWait := challengeWaitIterations(timeout)
			for i := 0; i < maxChallengeWait; i++ {
				var title string
				if err := chromedp.Evaluate(`document.title`, &title).Do(ctx); err != nil {
					return nil // Page might not be ready yet
				}
				titleLower := strings.ToLower(title)
				// Cloudflare challenge pages have these titles
				if strings.Contains(titleLower, "just a moment") ||
					strings.Contains(titleLower, "attention required") ||
					strings.Contains(titleLower, "checking your browser") ||
					strings.Contains(titleLower, "please wait") {
					log.Printf("[MCP] Cloudflare challenge detected (title: %s), waiting... (%d/%d)", title, i+1, maxChallengeWait)
					time.Sleep(1 * time.Second)
					continue
				}
				// Challenge passed or not a Cloudflare page
				break
			}
			return nil
		}),

		// Wait for page content to settle after challenge
		chromedp.Sleep(2*time.Second),

		// 4. Auto-scroll logic to trigger lazy loading
		chromedp.Evaluate(`
			(async () => {
				const distance = 400;
				const delay = 100;
				for (let i = 0; i < 15; i++) {
					window.scrollBy(0, distance);
					await new Promise(r => setTimeout(r, delay));
					if ((window.innerHeight + window.scrollY) >= document.body.offsetHeight) break;
				}
				window.scrollTo(0, 0); // Scroll back to top for extraction
			})()
		`, nil),
		chromedp.Sleep(1*time.Second),

		// 5. Smart Extraction Logic
		chromedp.Evaluate(`
			(() => {
				const noiseSelectors = [
					'nav', 'footer', 'aside', 'header', 'script', 'style', 'iframe',
					'.ads', '.menu', '.sidebar', '.nav', '.footer', '.advertisement',
					'.social-share', '.comments-section', '.related-posts'
				];
				const contentSelectors = [
					'article', 'main', '[role="main"]', '.content', '.post-content', 
					'.article-body', '.article-content', '#content', '.entry-content'
				];

				// Try to find the main content root
				let root = null;
				for (const s of contentSelectors) {
					const el = document.querySelector(s);
					if (el && el.innerText.length > 200) { // Ensure it's substantial
						root = el;
						break;
					}
				}
				if (!root) root = document.body;

				// Clone or work on a fragment to clean up
				const tempDiv = document.createElement('div');
				tempDiv.innerHTML = root.innerHTML;

				// Remove noise
				noiseSelectors.forEach(s => {
					const elements = tempDiv.querySelectorAll(s);
					elements.forEach(el => el.remove());
				});

				// Basic HTML to Markdown converter
				function toMarkdown(node) {
					let text = "";
					for (let child of node.childNodes) {
						if (child.nodeType === 3) { // Text node
							text += child.textContent;
						} else if (child.nodeType === 1) { // Element node
							const tag = child.tagName.toLowerCase();
							const inner = toMarkdown(child);
							switch(tag) {
								case 'h1': text += "\n# " + inner + "\n"; break;
								case 'h2': text += "\n## " + inner + "\n"; break;
								case 'h3': text += "\n### " + inner + "\n"; break;
								case 'p': text += "\n" + inner + "\n"; break;
								case 'br': text += "\n"; break;
								case 'b': case 'strong': text += "**" + inner + "**"; break;
								case 'i': case 'em': text += "*" + inner + "*"; break;
								case 'a': text += "[" + inner + "](" + child.href + ")"; break;
								case 'li': text += "\n- " + inner; break;
								case 'code': text += String.fromCharCode(96) + inner + String.fromCharCode(96); break;
								case 'pre': text += "\n" + String.fromCharCode(96,96,96) + "\n" + inner + "\n" + String.fromCharCode(96,96,96) + "\n"; break;
								default: text += inner;
							}
						}
					}
					return text;
				}

				return toMarkdown(tempDiv).replace(/\n\s*\n/g, "\n\n").trim();
			})()
		`, &res),
	)

	if err != nil {
		EmitTrace("mcp", "read_web_page.error", "Page read failed", traceDetails("url", pageURL, "elapsed_ms", durationMs(start), "error", errorDetail(err)))
		return "", fmt.Errorf("failed to read page: %v", err)
	}

	// truncate if too long (simple protection)
	if len(res) > 30000 {
		res = res[:30000] + "... (truncated)"
	}

	EmitTrace("mcp", "read_web_page.complete", "Page read completed", traceDetails("url", pageURL, "elapsed_ms", durationMs(start), "chars", len(res)))
	return res, nil
}

func readPageTimeoutForURL(pageURL string) time.Duration {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return 25 * time.Second
	}

	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "":
		return 25 * time.Second
	case strings.Contains(host, "wikipedia.org"),
		strings.Contains(host, "wikimedia.org"),
		strings.Contains(host, "docs."),
		strings.Contains(host, ".gov"),
		strings.Contains(host, ".edu"),
		strings.Contains(host, "developer."),
		strings.Contains(host, "openai.com"):
		return 35 * time.Second
	case strings.Contains(host, "instagram.com"),
		strings.Contains(host, "facebook.com"),
		strings.Contains(host, "x.com"),
		strings.Contains(host, "twitter.com"),
		strings.Contains(host, "mydramalist.com"),
		strings.Contains(host, "tiktok.com"):
		return 18 * time.Second
	default:
		return 25 * time.Second
	}
}

func challengeWaitIterations(timeout time.Duration) int {
	seconds := int(timeout / time.Second)
	switch {
	case seconds >= 35:
		return 12
	case seconds >= 25:
		return 9
	default:
		return 6
	}
}

func normalizeSearchQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return query
	}

	var symbolCount int
	for _, r := range query {
		if unicode.IsSymbol(r) {
			symbolCount++
		}
	}

	// Heuristic: if the query is visibly polluted with symbol-heavy mojibake,
	// strip symbol runes but keep letters, numbers, marks, spaces and punctuation.
	if symbolCount == 0 || symbolCount*4 < len([]rune(query)) {
		return query
	}

	var b strings.Builder
	for _, r := range query {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), unicode.IsMark(r), unicode.IsSpace(r), unicode.IsPunct(r):
			b.WriteRune(r)
		}
	}

	cleaned := strings.Join(strings.Fields(b.String()), " ")
	if cleaned == "" {
		return query
	}
	return cleaned
}

// ManageMemory is deprecated. All memory is handled via SQLite (SearchMemoryDB / ReadMemoryDB).

// SearchMemoryDB calls the SQLite db to search memory by keyword
func SearchMemoryDB(userID, query string) (string, error) {
	log.Printf("[MCP] SearchMemoryDB: User=%s, Query=%s", userID, query)
	rewrittenQuery := rewriteMemoryQuery(query)
	searchQueries := buildSearchQueries(query, rewrittenQuery)
	results, err := SearchMemoriesMultiQuery(userID, searchQueries)
	if err != nil {
		return "", fmt.Errorf("db search failed: %v", err)
	}
	savedTurns, err := searchSavedTurnsMultiQuery(userID, searchQueries)
	if err != nil {
		return "", fmt.Errorf("saved turn search failed: %v", err)
	}

	if len(results) == 0 && len(savedTurns) == 0 {
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
	for _, r := range results {
		memoryType := strings.TrimSpace(r.MemoryType)
		if memoryType == "" {
			memoryType = "raw_interaction"
		}
		sb.WriteString(fmt.Sprintf("\n--- MEMORY ID: %d | DATE: %s | TYPE: %s ---\n", r.ID, r.CreatedAt.Format("2006-01-02"), memoryType))
		sb.WriteString(fmt.Sprintf("FULL TEXT:\n%s\n", r.FullText))
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

	var allResults []MemoryEntry
	seenIDs := make(map[int64]bool)

	// Step 1: Search with top 3 keywords (Priority)
	searchWords := keywords
	if len(searchWords) > 3 {
		searchWords = searchWords[:3]
	}

	runSearch := func(words []string) {
		for _, kw := range words {
			results, err := SearchMemories(userID, kw)
			if err == nil {
				if len(results) > 0 {
					log.Printf("[MCP] AutoSearchMemory: Keyword %q found %d results", kw, len(results))
				}
				for _, r := range results {
					if !seenIDs[r.ID] {
						allResults = append(allResults, r)
						seenIDs[r.ID] = true
					}
				}
			}

			savedTurns, err := SearchSavedTurns(userID, kw, 5)
			if err == nil {
				for _, turn := range savedTurns {
					if seenIDs[turn.ID+1_000_000_000] {
						continue
					}
					allResults = append(allResults, MemoryEntry{
						ID:         turn.ID + 1_000_000_000,
						UserID:     turn.UserID,
						FullText:   fmt.Sprintf("User Prompt:\n%s\n\nAssistant Response:\n%s", turn.PromptText, turn.ResponseText),
						HitCount:   0,
						CreatedAt:  turn.CreatedAt,
						MemoryType: "saved_turn",
					})
					seenIDs[turn.ID+1_000_000_000] = true
				}
			}
		}
	}

	runSearch(searchWords)

	// Step 2: Retry with remaining keywords if no results found
	if len(allResults) == 0 && len(keywords) > 3 {
		log.Printf("[MCP] AutoSearchMemory: No results in Step 1. Retrying with next keywords.")
		nextWords := keywords[3:]
		if len(nextWords) > 5 {
			nextWords = nextWords[:5]
		}
		runSearch(nextWords)
	}

	if len(allResults) == 0 {
		return ""
	}

	// Limit to top 3 for synthesis to keep the prompt compact.
	limit := 3
	if len(allResults) < limit {
		limit = len(allResults)
	}

	var rawContextSb strings.Builder
	for i := 0; i < limit; i++ {
		r := allResults[i]
		memoryType := strings.TrimSpace(r.MemoryType)
		if memoryType == "" {
			memoryType = "raw_interaction"
		}
		rawContextSb.WriteString(fmt.Sprintf("\n--- MEMORY ID: %d | DATE: %s | TYPE: %s ---\n", r.ID, r.CreatedAt.Format("2006-01-02"), memoryType))
		rawContextSb.WriteString(fmt.Sprintf("Content: %s\n", compactMemoryText(r.FullText, 400)))

		// Increment hit count only for actual memory records.
		if r.ID < 1_000_000_000 {
			_ = IncrementHitCount(r.ID)
		}
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

// ExecuteCommand runs a shell command with restrictions
func ExecuteCommand(command string, disallowedCmds []string, disallowedDirs []string) (string, error) {
	log.Printf("[MCP] ExecuteCommand: %s", command)

	// 1. Basic Security Checks
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("command is empty")
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("command is empty")
	}
	baseCmd := parts[0]

	// 2. Check Disallowed Commands
	for _, disallowed := range disallowedCmds {
		if strings.EqualFold(baseCmd, disallowed) {
			return "", fmt.Errorf("permission denied: command '%s' is not allowed", baseCmd)
		}
	}

	// 3. Check Disallowed Directories (Command Arguments)
	// Iterate through arguments to see if they reference disallowed paths
	for _, arg := range parts[1:] {
		// Clean the path
		argClean := filepath.Clean(arg)
		for _, dir := range disallowedDirs {
			// Check if arg starts with disallowed dir (simple check)
			// TODO: Enhance with better path resolution
			if strings.HasPrefix(argClean, filepath.Clean(dir)) {
				return "", fmt.Errorf("permission denied: directory '%s' is restricted", dir)
			}
		}
	}

	// 4. Execution
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}

	// Capture Output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Error: %v\nOutput: %s", err, string(output)), nil
	}

	return string(output), nil
}
