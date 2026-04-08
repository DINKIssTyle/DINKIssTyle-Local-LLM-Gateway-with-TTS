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
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type ToolContext struct {
	UserID         string
	EnableMemory   bool
	DisabledTools  []string
	LocationInfo   string
	DisallowedCmds []string
	DisallowedDirs []string
}

type ToolHost interface {
	ExecuteCommand(command string) (string, error)
	SendKeys(keys []string) (string, error)
	ReadTerminalTail(lines int, maxWaitMs int, idleMs int) (string, error)
}

type ToolHostFuncs struct {
	ExecuteCommandFunc   func(command string) (string, error)
	SendKeysFunc         func(keys []string) (string, error)
	ReadTerminalTailFunc func(lines int, maxWaitMs int, idleMs int) (string, error)
}

type ToolHooks struct {
	Trace                   func(source, stage, message string, details map[string]interface{})
	SearchMemory            func(userID, query string) (string, error)
	ReadMemory              func(userID string, memoryID int64) (string, error)
	DeleteMemory            func(userID string, memoryID int64) (string, error)
	BufferWebResult         func(userID, toolName, query, pageURL, title, content string) (string, error)
	BufferWebFallback       func(userID, failedTool, target string, err error) string
	ReadBufferedSource      func(userID, sourceID, query string, maxChunks int) (string, error)
	NormalizeSearchQuery    func(query string) string
	ReadPageTimeoutForURL   func(pageURL string) time.Duration
	ChallengeWaitIterations func(timeout time.Duration) int
}

func (h ToolHostFuncs) ExecuteCommand(command string) (string, error) {
	if h.ExecuteCommandFunc == nil {
		return "", fmt.Errorf("command executor not configured")
	}
	return h.ExecuteCommandFunc(command)
}

func (h ToolHostFuncs) SendKeys(keys []string) (string, error) {
	if h.SendKeysFunc == nil {
		return "", fmt.Errorf("terminal key executor not configured")
	}
	return h.SendKeysFunc(keys)
}

func (h ToolHostFuncs) ReadTerminalTail(lines int, maxWaitMs int, idleMs int) (string, error) {
	if h.ReadTerminalTailFunc == nil {
		return "", fmt.Errorf("terminal tail reader not configured")
	}
	return h.ReadTerminalTailFunc(lines, maxWaitMs, idleMs)
}

var (
	currentHost           ToolHost
	hostMu                sync.RWMutex
	currentContext        = ToolContext{UserID: "default"}
	contextMu             sync.RWMutex
	currentHooks          ToolHooks
	hooksMu               sync.RWMutex
	verboseLoggingEnabled atomic.Uint32
)

func SetVerboseLoggingEnabled(enabled bool) {
	if enabled {
		verboseLoggingEnabled.Store(1)
		return
	}
	verboseLoggingEnabled.Store(0)
}

func logVerbosef(format string, args ...interface{}) {
	if verboseLoggingEnabled.Load() == 1 {
		log.Printf(format, args...)
	}
}

func SetHost(host ToolHost) {
	hostMu.Lock()
	defer hostMu.Unlock()
	currentHost = host
}

func SetToolHooks(hooks ToolHooks) {
	hooksMu.Lock()
	defer hooksMu.Unlock()
	currentHooks = hooks
}

func getToolHooks() ToolHooks {
	hooksMu.RLock()
	defer hooksMu.RUnlock()
	return currentHooks
}

func getHost() ToolHost {
	hostMu.RLock()
	defer hostMu.RUnlock()
	return currentHost
}

func SetContext(userID string, enableMemory bool, disabledTools []string, locationInfo string, disallowedCmds []string, disallowedDirs []string) {
	contextMu.Lock()
	defer contextMu.Unlock()
	currentContext = ToolContext{
		UserID:         userID,
		EnableMemory:   enableMemory,
		DisabledTools:  append([]string(nil), disabledTools...),
		LocationInfo:   locationInfo,
		DisallowedCmds: append([]string(nil), disallowedCmds...),
		DisallowedDirs: append([]string(nil), disallowedDirs...),
	}
	log.Printf("[MCP] Set Context -> User: %s, Memory: %v, DisabledTools: %v, Location: %s, DisallowedCmds: %v, DisallowedDirs: %v", userID, enableMemory, disabledTools, locationInfo, disallowedCmds, disallowedDirs)
}

func GetContext() ToolContext {
	contextMu.RLock()
	defer contextMu.RUnlock()
	return ToolContext{
		UserID:         currentContext.UserID,
		EnableMemory:   currentContext.EnableMemory,
		DisabledTools:  append([]string(nil), currentContext.DisabledTools...),
		LocationInfo:   currentContext.LocationInfo,
		DisallowedCmds: append([]string(nil), currentContext.DisallowedCmds...),
		DisallowedDirs: append([]string(nil), currentContext.DisallowedDirs...),
	}
}

func GetToolList() []Tool {
	tools := []Tool{
		{
			Name:        "search_web",
			Description: "Search the internet for current information. This tool tries Google first and automatically falls back to DuckDuckGo Lite if needed.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "Search query"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "read_web_page",
			Description: "Fetch and buffer the text content of a specific URL for the current user. This returns a source handle plus summary, not the full page. After this tool, use read_buffered_source with the source_id and the user's question to inspect relevant excerpts.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{"type": "string", "description": "URL to visit"},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:        "read_buffered_source",
			Description: "Read relevant excerpts from a previously buffered web source for the current user. Prefer this after read_web_page, naver_search, namu_wiki, or a buffered search result. If source_id is omitted, the most recent buffered source for this user is used.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source_id":  map[string]interface{}{"type": "string", "description": "Buffered source ID returned by another web tool. Optional if you want the latest source."},
					"query":      map[string]interface{}{"type": "string", "description": "Focused question or keywords to retrieve the most relevant excerpts."},
					"max_chunks": map[string]interface{}{"type": "integer", "description": "Maximum number of excerpts to return. Optional."},
				},
			},
		},
		{
			Name:        "get_current_time",
			Description: "Get the current local date and time. Use this when you need to know the current date, time, or day of the week for scheduling or age calculations.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "search_memory",
			Description: "Search the user's long-term SQLite memory using full-text matching. Results may include raw chat memories and saved turns. Use this whenever the user asks about their personal info, prior chats, preferences, or past facts.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The keyword or short phrase to search in past memories.",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "read_memory",
			Description: "Read the full transcription/context of a specific memory by its ID. You MUST call this IMMEDIATELY after 'search_memory' to understand the actual details of the memory.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"memory_id": map[string]interface{}{
						"type":        "integer",
						"description": "The numeric ID of the memory to read.",
					},
				},
				"required": []string{"memory_id"},
			},
		},
		{
			Name:        "delete_memory",
			Description: "Delete an existing memory entry if it is completely wrong or no longer needed. You MUST use search_memory first to find the correct memory_id.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"memory_id": map[string]interface{}{
						"type":        "integer",
						"description": "The ID of the memory to delete.",
					},
				},
				"required": []string{"memory_id"},
			},
		},
		{
			Name:        "naver_search",
			Description: "Search Naver (Korean portal). Specialized for dictionary, Korea-related content, weather, and news. Use this for specific Korean context.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "The search query for Naver"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "namu_wiki",
			Description: "Search and read definitions from Namuwiki (Korean Wiki). Use this for Korean pop culture, history, or slang definitions. Input must be the exact keyword/title.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"keyword": map[string]interface{}{"type": "string", "description": "The exact keyword to search on Namuwiki"},
				},
				"required": []string{"keyword"},
			},
		},
		{
			Name:        "get_current_location",
			Description: "Get the user's current location (City, Region, Country) provided by their device. Use this for location-specific queries like weather or local news.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "send_keys",
			Description: "Send raw key presses to the active terminal. Use this for ESC, ENTER, CTRL_C and editor interactions.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"keys": map[string]interface{}{
						"type":        "array",
						"description": "Ordered list of keys like ESC, ENTER, CTRL_C, or plain text chunks such as :q!",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"keys"},
			},
		},
		{
			Name:        "read_terminal_tail",
			Description: "Read the latest output from the active terminal. Use this after long-running commands like builds or tests to inspect whether they succeeded or failed.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"lines":     map[string]interface{}{"type": "integer", "description": "How many recent lines to return. Defaults to 40."},
					"maxWaitMs": map[string]interface{}{"type": "integer", "description": "How long to wait for terminal output to go idle before reading. Defaults to 0."},
					"idleMs":    map[string]interface{}{"type": "integer", "description": "How long output must stay quiet to count as idle. Defaults to 1200."},
				},
			},
		},
		{
			Name:        "execute_command",
			Description: "Execute a shell command on the host (with restrictions). Use this to run system commands.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The shell command to execute.",
					},
				},
				"required": []string{"command"},
			},
		},
	}
	filtered := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		if isToolConfiguredEnabled(tool.Name) {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func compactMemoryText(input string, limit int) string {
	input = strings.TrimSpace(input)
	if limit <= 0 || len([]rune(input)) <= limit {
		return input
	}
	runes := []rune(input)
	return strings.TrimSpace(string(runes[:limit])) + "... (truncated)"
}

func emitTraceEvent(source, stage, message string, details map[string]interface{}) {
	hooks := getToolHooks()
	if hooks.Trace != nil {
		hooks.Trace(source, stage, message, details)
	}
}

func traceDetailsMap(args ...interface{}) map[string]interface{} {
	details := make(map[string]interface{})
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		details[key] = args[i+1]
	}
	return details
}

func toolErrorDetail(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func toolDurationMs(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func normalizeToolSearchQuery(query string) string {
	hooks := getToolHooks()
	if hooks.NormalizeSearchQuery != nil {
		return hooks.NormalizeSearchQuery(query)
	}
	return defaultNormalizeSearchQuery(query)
}

func bufferToolResult(userID, toolName, query, pageURL, title, content string) (string, error) {
	hooks := getToolHooks()
	if hooks.BufferWebResult != nil {
		return hooks.BufferWebResult(userID, toolName, query, pageURL, title, content)
	}
	return content, nil
}

func bufferToolFallbackAfterError(userID, failedTool, target string, err error) (string, bool) {
	hooks := getToolHooks()
	if hooks.BufferWebFallback == nil {
		return "", false
	}
	return hooks.BufferWebFallback(userID, failedTool, target, err), true
}

func readBufferedToolSource(userID, sourceID, query string, maxChunks int) (string, error) {
	hooks := getToolHooks()
	if hooks.ReadBufferedSource == nil {
		return "", fmt.Errorf("buffered source reading is not supported")
	}
	return hooks.ReadBufferedSource(userID, sourceID, query, maxChunks)
}

func runSearchMemoryHook(userID, query string) (string, error) {
	hooks := getToolHooks()
	if hooks.SearchMemory == nil {
		return "", fmt.Errorf("memory search is not supported")
	}
	return hooks.SearchMemory(userID, query)
}

func runReadMemoryHook(userID string, memoryID int64) (string, error) {
	hooks := getToolHooks()
	if hooks.ReadMemory == nil {
		return "", fmt.Errorf("memory read is not supported")
	}
	return hooks.ReadMemory(userID, memoryID)
}

func runDeleteMemoryHook(userID string, memoryID int64) (string, error) {
	hooks := getToolHooks()
	if hooks.DeleteMemory == nil {
		return "", fmt.Errorf("memory delete is not supported")
	}
	return hooks.DeleteMemory(userID, memoryID)
}

func defaultReadPageTimeoutForURL(pageURL string) time.Duration {
	hooks := getToolHooks()
	if hooks.ReadPageTimeoutForURL != nil {
		return hooks.ReadPageTimeoutForURL(pageURL)
	}
	return defaultReadPageTimeoutForURL(pageURL)
}

func defaultChallengeWaitIterations(timeout time.Duration) int {
	hooks := getToolHooks()
	if hooks.ChallengeWaitIterations != nil {
		return hooks.ChallengeWaitIterations(timeout)
	}
	return defaultChallengeWaitIterations(timeout)
}

func ExecuteToolByName(toolName string, argumentsJSON []byte, userID string, enableMemory bool, disabledTools []string) (string, error) {
	start := time.Now()
	ctx := GetContext()
	if userID == "" {
		userID = ctx.UserID
	}
	if disabledTools == nil {
		disabledTools = ctx.DisabledTools
	}
	if !enableMemory {
		enableMemory = ctx.EnableMemory
	}

	log.Printf("[MCP] ExecuteToolByName: %s (User: %s, Memory: %v, Loc: %s)", toolName, userID, enableMemory, ctx.LocationInfo)
	if !isToolConfiguredEnabled(toolName) {
		return "", fmt.Errorf("tool '%s' is disabled by app config", toolName)
	}
	emitTraceEvent("mcp", "tool.start", "Executing MCP tool", traceDetailsMap(
		"tool", toolName,
		"user", userID,
		"args_bytes", len(argumentsJSON),
		"memory", enableMemory,
	))

	for _, disabled := range disabledTools {
		if disabled == toolName {
			emitTraceEvent("mcp", "tool.error", "Tool blocked by user policy", traceDetailsMap("tool", toolName, "elapsed_ms", toolDurationMs(start)))
			return "", fmt.Errorf("tool '%s' is disabled for this user", toolName)
		}
	}

	switch toolName {
	case "search_web":
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for search_web: %v", err)
		}
		result, err := SearchWeb(args.Query)
		if err == nil {
			result, err = bufferToolResult(userID, toolName, args.Query, "", fmt.Sprintf("Search: %s", compactMemoryText(args.Query, 80)), result)
		}
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	case "read_web_page":
		var args struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for read_web_page: %v", err)
		}
		result, err := ReadPage(args.URL)
		if err == nil {
			result, err = bufferToolResult(userID, toolName, "", args.URL, "", result)
		} else {
			if fallback, ok := bufferToolFallbackAfterError(userID, toolName, args.URL, err); ok {
				result = fallback
				emitTraceEvent("mcp", "tool.fallback", "Using buffered fallback after page read failure", traceDetailsMap(
					"tool", toolName,
					"user", userID,
					"url", args.URL,
					"error", toolErrorDetail(err),
				))
				err = nil
			}
		}
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	case "read_buffered_source":
		var args struct {
			SourceID  string `json:"source_id"`
			Query     string `json:"query"`
			MaxChunks int    `json:"max_chunks"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for read_buffered_source: %v", err)
		}
		result, err := readBufferedToolSource(userID, args.SourceID, args.Query, args.MaxChunks)
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	case "search_memory":
		if !enableMemory {
			return "", fmt.Errorf("memory feature is disabled by user settings")
		}
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for search_memory: %v", err)
		}
		result, err := runSearchMemoryHook(userID, args.Query)
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	case "read_memory":
		if !enableMemory {
			return "", fmt.Errorf("memory feature is disabled by user settings")
		}
		var args struct {
			MemoryID int64 `json:"memory_id"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for read_memory: %v", err)
		}
		result, err := runReadMemoryHook(userID, args.MemoryID)
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	case "delete_memory":
		if !enableMemory {
			return "", fmt.Errorf("memory feature is disabled by user settings")
		}
		var args struct {
			MemoryID int64 `json:"memory_id"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for delete_memory: %v", err)
		}
		result, err := runDeleteMemoryHook(userID, args.MemoryID)
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	case "get_current_time":
		result, err := GetCurrentTime()
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	case "namu_wiki":
		var args struct {
			Keyword string `json:"keyword"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for namu_wiki: %v", err)
		}
		result, err := SearchNamuwiki(args.Keyword)
		if err == nil {
			result, err = bufferToolResult(userID, toolName, args.Keyword, fmt.Sprintf("https://namu.wiki/w/%s", args.Keyword), "", result)
		}
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	case "naver_search":
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for naver_search: %v", err)
		}
		result, err := SearchNaver(args.Query)
		if err == nil {
			result, err = bufferToolResult(userID, toolName, args.Query, "", fmt.Sprintf("Naver: %s", compactMemoryText(args.Query, 80)), result)
		}
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	case "get_current_location":
		if ctx.LocationInfo == "" {
			result := "Location information not provided by client."
			emitToolResultTrace(toolName, start, result, nil)
			return result, nil
		}
		emitToolResultTrace(toolName, start, ctx.LocationInfo, nil)
		return ctx.LocationInfo, nil

	case "send_keys":
		var args struct {
			Keys []string `json:"keys"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for send_keys: %v", err)
		}
		if len(args.Keys) == 0 {
			return "", fmt.Errorf("keys cannot be empty")
		}
		host := getHost()
		if host == nil {
			return "", fmt.Errorf("terminal key executor not configured")
		}
		result, err := host.SendKeys(args.Keys)
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	case "read_terminal_tail":
		var args struct {
			Lines     int `json:"lines"`
			MaxWaitMs int `json:"maxWaitMs"`
			IdleMs    int `json:"idleMs"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for read_terminal_tail: %v", err)
		}
		host := getHost()
		if host == nil {
			return "", fmt.Errorf("terminal tail reader not configured")
		}
		result, err := host.ReadTerminalTail(args.Lines, args.MaxWaitMs, args.IdleMs)
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	case "execute_command":
		var args map[string]string
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %v", err)
		}
		command, ok := args["command"]
		if !ok {
			return "", fmt.Errorf("argument 'command' is required")
		}
		if host := getHost(); host != nil {
			result, err := host.ExecuteCommand(command)
			emitToolResultTrace(toolName, start, result, err)
			return result, err
		}
		result, err := ExecuteCommand(command, ctx.DisallowedCmds, ctx.DisallowedDirs)
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	default:
		emitTraceEvent("mcp", "tool.error", "Unknown tool requested", traceDetailsMap("tool", toolName, "elapsed_ms", toolDurationMs(start)))
		return "", fmt.Errorf("tool not found: %s", toolName)
	}
}

func emitToolResultTrace(toolName string, start time.Time, result string, err error) {
	if err != nil {
		emitTraceEvent("mcp", "tool.error", "Tool execution failed", traceDetailsMap(
			"tool", toolName,
			"elapsed_ms", toolDurationMs(start),
			"error", toolErrorDetail(err),
		))
		return
	}

	emitTraceEvent("mcp", "tool.complete", "Tool execution completed", traceDetailsMap(
		"tool", toolName,
		"elapsed_ms", toolDurationMs(start),
		"result_chars", len(result),
	))
}

// GetCurrentTime returns the current local time in a readable format including timezone.
func GetCurrentTime() (string, error) {
	now := time.Now()
	// Format: 2026-02-06 09:02:06 Friday KST
	return fmt.Sprintf("Current Local Time: %s", now.Format("2006-01-02 15:04:05 Monday MST")), nil
}

// SearchWeb performs a web search using DuckDuckGo Lite.
func SearchWeb(query string) (string, error) {
	originalQuery := query
	query = normalizeToolSearchQuery(query)
	log.Printf("[MCP] Searching Web for: %s", query)
	start := time.Now()
	traceArgs := []interface{}{"query", query}
	if query != originalQuery {
		traceArgs = append(traceArgs, "original_query", originalQuery)
	}
	emitTraceEvent("mcp", "search_web.start", "Starting web search", traceDetailsMap(traceArgs...))

	client := &http.Client{Timeout: 10 * time.Second}
	results, err := searchDuckDuckGo(query, client)
	if err != nil {
		emitTraceEvent("mcp", "search_web.error", "Web search failed", traceDetailsMap("query", query, "elapsed_ms", toolDurationMs(start), "error", toolErrorDetail(err)))
		return "", err
	}
	if len(results) == 0 {
		emitTraceEvent("mcp", "search_web.complete", "Web search returned no parsed results", traceDetailsMap("query", query, "elapsed_ms", toolDurationMs(start), "provider", "duckduckgo"))
		return "No results found or parsing failed.", nil
	}

	emitTraceEvent("mcp", "search_web.complete", "DuckDuckGo search completed", traceDetailsMap("query", query, "elapsed_ms", toolDurationMs(start), "results", len(results), "provider", "duckduckgo"))
	return strings.Join(results, "\n---\n"), nil
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
	emitTraceEvent("mcp", "read_web_page.start", "Starting page read", traceDetailsMap("url", pageURL))

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

	timeout := readPageTimeoutForToolURL(pageURL)
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
			maxChallengeWait := challengeWaitIterationsForTool(timeout)
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
		emitTraceEvent("mcp", "read_web_page.error", "Page read failed", traceDetailsMap("url", pageURL, "elapsed_ms", toolDurationMs(start), "error", toolErrorDetail(err)))
		return "", fmt.Errorf("failed to read page: %v", err)
	}

	// truncate if too long (simple protection)
	if len(res) > 30000 {
		res = res[:30000] + "... (truncated)"
	}

	emitTraceEvent("mcp", "read_web_page.complete", "Page read completed", traceDetailsMap("url", pageURL, "elapsed_ms", toolDurationMs(start), "chars", len(res)))
	return res, nil
}

func readPageTimeoutForToolURL(pageURL string) time.Duration {
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

func challengeWaitIterationsForTool(timeout time.Duration) int {
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

func defaultNormalizeSearchQuery(query string) string {
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
