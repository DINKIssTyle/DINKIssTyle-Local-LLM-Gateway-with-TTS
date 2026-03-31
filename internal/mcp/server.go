package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// Simplified MCP Server implementation

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
	ID      interface{}   `json:"id"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// Global state for SSE clients
var (
	clients   = make(map[chan string]bool)
	clientsMu sync.Mutex

	// Current Context (Hacky solution for local single-user gateway)
	// Since LM Studio doesn't pass user context back to MCP, we rely on the
	// most recent chat request setting these values.
	currentUserID         = "default"
	currentEnableMemory   = false
	currentDisabledTools  []string
	currentLocationInfo   string
	currentDisallowedCmds []string
	currentDisallowedDirs []string
	contextMu             sync.RWMutex
)

func SetContext(userID string, enableMemory bool, disabledTools []string, locationInfo string, disallowedCmds []string, disallowedDirs []string) {
	contextMu.Lock()
	defer contextMu.Unlock()
	currentUserID = userID
	currentEnableMemory = enableMemory
	currentDisabledTools = disabledTools
	currentLocationInfo = locationInfo
	currentDisallowedCmds = disallowedCmds
	currentDisallowedDirs = disallowedDirs
	log.Printf("[MCP] Set Context -> User: %s, Memory: %v, DisabledTools: %v, Location: %s, DisallowedCmds: %v, DisallowedDirs: %v", userID, enableMemory, disabledTools, locationInfo, disallowedCmds, disallowedDirs)
}

func GetContext() (string, bool, []string, string, []string, []string) {
	contextMu.RLock()
	defer contextMu.RUnlock()
	return currentUserID, currentEnableMemory, currentDisabledTools, currentLocationInfo, currentDisallowedCmds, currentDisallowedDirs
}

func GetToolList() []Tool {
	return []Tool{
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
}

func AddClient(ch chan string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	clients[ch] = true
	log.Printf("[MCP-DEBUG] Total Clients: %d", len(clients))
}

func RemoveClient(ch chan string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	delete(clients, ch)
	close(ch)
}

// Broadcast sends a message to all connected SSE clients and returns count sent
func Broadcast(msg string) int {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	count := 0
	for ch := range clients {
		select {
		case ch <- msg:
			count++
		default:
			log.Printf("[MCP-DEBUG] Broadcast SKIPPED for a client (channel full)")
		}
	}
	return count
}

// buildResponse constructs a JSON-RPC response for a given request
func buildResponse(req *JSONRPCRequest, userID string, enableMemory bool, disabledTools []string, disallowedCmds []string, disallowedDirs []string) *JSONRPCResponse {
	res := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		res.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{"listChanged": false},
			},
			"serverInfo": map[string]string{
				"name":    "DKST Local Gateway",
				"version": "1.0.0",
			},
		}

	case "tools/list":
		// TODO: Filter tools list based on disabledTools?
		// For now, list all, but fail on call.
		res.Result = map[string]interface{}{
			"tools": GetToolList(),
		}

	case "tools/call":
		handleToolCall(req, res, userID, enableMemory, disabledTools, disallowedCmds, disallowedDirs)

	case "notifications/initialized":
		log.Println("[MCP] Client Initialized")
		return nil

	default:
		if req.Method == "ping" {
			res.Result = map[string]string{}
		} else {
			res.Error = &JSONRPCError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}
		}
	}
	return res
}

func HandleSSE(w http.ResponseWriter, r *http.Request) {
	log.Printf("[MCP-DEBUG] HandleSSE (SSE Open) from %s Method=%s", r.RemoteAddr, r.Method)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var initialReq *JSONRPCRequest
	if r.Method == http.MethodPost {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil && len(bodyBytes) > 0 {
			var req JSONRPCRequest
			if err := json.Unmarshal(bodyBytes, &req); err == nil {
				initialReq = &req
				log.Printf("[MCP-DEBUG] Initial POST Captured: %s (ID: %v)", initialReq.Method, initialReq.ID)
			}
		}
		r.Body.Close()
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	messageChan := make(chan string, 200)
	AddClient(messageChan)
	defer func() {
		log.Printf("[MCP-DEBUG] SSE CLOSED for %s", r.RemoteAddr)
		RemoveClient(messageChan)
	}()

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	endpointURL := fmt.Sprintf("%s://%s/mcp/messages", scheme, r.Host)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()
	log.Printf("[MCP-DEBUG] Advertised Endpoint: %s", endpointURL)

	// If we captured an initial request, process it immediately in the stream
	if initialReq != nil {
		userID, enableMemory, disabledTools, locationInfo, disallowedCmds, disallowedDirs := GetContext()
		_ = locationInfo // Handle unused for now if not needed in buildResponse directly
		res := buildResponse(initialReq, userID, enableMemory, disabledTools, disallowedCmds, disallowedDirs)
		if res != nil {
			respBytes, _ := json.Marshal(res)
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(respBytes))
			flusher.Flush()
			log.Printf("[MCP-DEBUG] Inline Response sent for %s", initialReq.Method)

			// 🚀 CRITICAL FIX: If this was a POST request and we processed it inline,
			// don't enter the infinite SSE loop. Most one-off clients (like LM Studio tools)
			// use POST and expect a response, not a 10-minute stream.
			if initialReq.Method != "initialize" {
				log.Printf("[MCP-DEBUG] Short-circuiting POST request for %s", initialReq.Method)
				return
			}
		}
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-messageChan:
			log.Printf("[MCP-DEBUG] SSE PUSH -> %s", r.RemoteAddr)
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func HandleMessages(w http.ResponseWriter, r *http.Request) {
	log.Printf("[MCP-DEBUG] HandleMessages (POST) from %s", r.RemoteAddr)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body.Close()

	var req JSONRPCRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("[MCP-DEBUG] Request Method: %s (ID: %v)", req.Method, req.ID)

	w.WriteHeader(http.StatusAccepted)

	go func() {
		time.Sleep(50 * time.Millisecond)
		userID, enableMemory, disabledTools, locationInfo, disallowedCmds, disallowedDirs := GetContext()
		_ = locationInfo // Unused here
		res := buildResponse(&req, userID, enableMemory, disabledTools, disallowedCmds, disallowedDirs)
		if res != nil {
			respBytes, _ := json.Marshal(res)
			count := Broadcast(string(respBytes))
			log.Printf("[MCP-DEBUG] Broadcasted %s (to %d clients)", req.Method, count)
		}
	}()
}

// ExecuteToolByName executes a tool by name with given arguments JSON.
// This is used for text-based tool call parsing (when model outputs tool call as plain text).
// Returns the result text and an error if any.
func ExecuteToolByName(toolName string, argumentsJSON []byte, userID string, enableMemory bool, disabledTools []string) (string, error) {
	start := time.Now()
	// Re-fetch context to get location since signature change is hard across all callsites (or we can just use global if thread-safe enough for this hack)
	// Better: Use the global GetContext to retrieve the location for this execution
	_, _, _, locationInfo, disallowedCmds, disallowedDirs := GetContext()

	log.Printf("[MCP] ExecuteToolByName: %s (User: %s, Memory: %v, Loc: %s)", toolName, userID, enableMemory, locationInfo)
	EmitTrace("mcp", "tool.start", "Executing MCP tool", traceDetails(
		"tool", toolName,
		"user", userID,
		"args_bytes", len(argumentsJSON),
		"memory", enableMemory,
	))

	// Check if tool is disabled
	for _, disabled := range disabledTools {
		if disabled == toolName {
			EmitTrace("mcp", "tool.error", "Tool blocked by user policy", traceDetails("tool", toolName, "elapsed_ms", durationMs(start)))
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
			source := saveBufferedWebSource(userID, toolName, args.Query, "", fmt.Sprintf("Search: %s", compactMemoryText(args.Query, 80)), result)
			result = formatBufferedSourceHandle(source)
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
			source := saveBufferedWebSource(userID, toolName, "", args.URL, "", result)
			result = formatBufferedSourceHandle(source)
		} else {
			result = formatBufferedFallbackAfterToolError(userID, toolName, args.URL, err)
			EmitTrace("mcp", "tool.fallback", "Using buffered fallback after page read failure", traceDetails(
				"tool", toolName,
				"user", userID,
				"url", args.URL,
				"error", errorDetail(err),
			))
			err = nil
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
		result, err := readBufferedSource(userID, args.SourceID, args.Query, args.MaxChunks)
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
		result, err := SearchMemoryDB(userID, args.Query)
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
		result, err := ReadMemoryDB(userID, args.MemoryID)
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
		result, err := DeleteMemoryDB(userID, args.MemoryID)
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
			source := saveBufferedWebSource(userID, toolName, args.Keyword, fmt.Sprintf("https://namu.wiki/w/%s", args.Keyword), "", result)
			result = formatBufferedSourceHandle(source)
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
			source := saveBufferedWebSource(userID, toolName, args.Query, "", fmt.Sprintf("Naver: %s", compactMemoryText(args.Query, 80)), result)
			result = formatBufferedSourceHandle(source)
		}
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	case "get_current_location":
		if locationInfo == "" {
			result := "Location information not provided by client."
			emitToolResultTrace(toolName, start, result, nil)
			return result, nil
		}
		emitToolResultTrace(toolName, start, locationInfo, nil)
		return locationInfo, nil

	case "execute_command":
		var args map[string]string
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %v", err)
		}
		command, ok := args["command"]
		if !ok {
			return "", fmt.Errorf("argument 'command' is required")
		}
		result, err := ExecuteCommand(command, disallowedCmds, disallowedDirs)
		emitToolResultTrace(toolName, start, result, err)
		return result, err

	default:
		EmitTrace("mcp", "tool.error", "Unknown tool requested", traceDetails("tool", toolName, "elapsed_ms", durationMs(start)))
		return "", fmt.Errorf("tool not found: %s", toolName)
	}
}

func emitToolResultTrace(toolName string, start time.Time, result string, err error) {
	if err != nil {
		EmitTrace("mcp", "tool.error", "Tool execution failed", traceDetails(
			"tool", toolName,
			"elapsed_ms", durationMs(start),
			"error", errorDetail(err),
		))
		return
	}

	EmitTrace("mcp", "tool.complete", "Tool execution completed", traceDetails(
		"tool", toolName,
		"elapsed_ms", durationMs(start),
		"result_chars", len(result),
	))
}

// Helper to handle tool calls

func handleToolCall(req *JSONRPCRequest, res *JSONRPCResponse, userID string, enableMemory bool, disabledTools []string, disallowedCmds []string, disallowedDirs []string) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		res.Error = &JSONRPCError{Code: -32602, Message: "Invalid parameters"}
		return
	}

	log.Printf("[MCP] Tool Call: %s", params.Name)

	// Check if tool is disabled
	for _, disabled := range disabledTools {
		if disabled == params.Name {
			res.Error = &JSONRPCError{Code: -32601, Message: fmt.Sprintf("Tool '%s' is disabled for this user.", params.Name)}
			Broadcast(fmt.Sprintf(`{"type": "tool_call.failure", "tool": "%s", "reason": "Disabled by admin"}`, params.Name))
			return
		}
	}

	if params.Name == "search_web" {
		var args struct {
			Query string `json:"query"`
		}
		json.Unmarshal(params.Arguments, &args)
		content, err := SearchWeb(args.Query)
		if err != nil {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			}
			Broadcast(fmt.Sprintf(`{"type": "tool_call.failure", "tool": "search_web", "reason": "%v"}`, err))
		} else {
			source := saveBufferedWebSource(userID, "search_web", args.Query, "", fmt.Sprintf("Search: %s", compactMemoryText(args.Query, 80)), content)
			content = formatBufferedSourceHandle(source)
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": content},
				},
			}
			Broadcast(`{"type": "tool_call.success", "tool": "search_web"}`)
		}
	} else if params.Name == "read_web_page" {
		var args struct {
			URL string `json:"url"`
		}
		json.Unmarshal(params.Arguments, &args)
		content, err := ReadPage(args.URL)
		if err != nil {
			content = formatBufferedFallbackAfterToolError(userID, "read_web_page", args.URL, err)
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": content},
				},
			}
			EmitTrace("mcp", "tool.fallback", "Using buffered fallback after page read failure", traceDetails(
				"tool", "read_web_page",
				"user", userID,
				"url", args.URL,
				"error", errorDetail(err),
			))
			Broadcast(`{"type": "tool_call.success", "tool": "read_web_page"}`)
		} else {
			source := saveBufferedWebSource(userID, "read_web_page", "", args.URL, "", content)
			content = formatBufferedSourceHandle(source)
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": content},
				},
			}
			Broadcast(`{"type": "tool_call.success", "tool": "read_web_page"}`)
		}
	} else if params.Name == "read_buffered_source" {
		var args struct {
			SourceID  string `json:"source_id"`
			Query     string `json:"query"`
			MaxChunks int    `json:"max_chunks"`
		}
		json.Unmarshal(params.Arguments, &args)
		content, err := readBufferedSource(userID, args.SourceID, args.Query, args.MaxChunks)
		if err != nil {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			}
			Broadcast(fmt.Sprintf(`{"type": "tool_call.failure", "tool": "read_buffered_source", "reason": "%v"}`, err))
		} else {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": content},
				},
			}
			Broadcast(`{"type": "tool_call.success", "tool": "read_buffered_source"}`)
		}
	} else if params.Name == "search_memory" {
		if !enableMemory {
			res.Error = &JSONRPCError{Code: -32601, Message: "Memory feature is disabled by user settings."}
			Broadcast(`{"type": "tool_call.failure", "tool": "search_memory", "reason": "Memory disabled"}`)
			return
		}
		var args struct {
			Query string `json:"query"`
		}
		json.Unmarshal(params.Arguments, &args)

		content, err := SearchMemoryDB(userID, args.Query)
		if err != nil {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			}
			Broadcast(fmt.Sprintf(`{"type": "tool_call.failure", "tool": "search_memory", "reason": "%v"}`, err))
		} else {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": content},
				},
			}
			Broadcast(`{"type": "tool_call.success", "tool": "search_memory"}`)
		}
	} else if params.Name == "read_memory" {
		if !enableMemory {
			res.Error = &JSONRPCError{Code: -32601, Message: "Memory feature is disabled by user settings."}
			Broadcast(`{"type": "tool_call.failure", "tool": "read_memory", "reason": "Memory disabled"}`)
			return
		}
		var args struct {
			MemoryID int64 `json:"memory_id"`
		}
		json.Unmarshal(params.Arguments, &args)

		content, err := ReadMemoryDB(userID, args.MemoryID)
		if err != nil {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			}
			Broadcast(fmt.Sprintf(`{"type": "tool_call.failure", "tool": "read_memory", "reason": "%v"}`, err))
		} else {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": content},
				},
			}
			Broadcast(`{"type": "tool_call.success", "tool": "read_memory"}`)
		}

	} else if params.Name == "delete_memory" {
		if !enableMemory {
			res.Error = &JSONRPCError{Code: -32601, Message: "Memory feature is disabled by user settings."}
			Broadcast(`{"type": "tool_call.failure", "tool": "delete_memory", "reason": "Memory disabled"}`)
			return
		}
		var args struct {
			MemoryID int64 `json:"memory_id"`
		}
		json.Unmarshal(params.Arguments, &args)

		content, err := DeleteMemoryDB(userID, args.MemoryID)
		if err != nil {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			}
			Broadcast(fmt.Sprintf(`{"type": "tool_call.failure", "tool": "delete_memory", "reason": "%v"}`, err))
		} else {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": content},
				},
			}
			Broadcast(`{"type": "tool_call.success", "tool": "delete_memory"}`)
		}
	} else if params.Name == "get_current_time" {
		content, _ := GetCurrentTime()
		res.Result = map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": content},
			},
		}
		Broadcast(`{"type": "tool_call.success", "tool": "get_current_time"}`)
	} else if params.Name == "namu_wiki" {
		var args struct {
			Keyword string `json:"keyword"`
		}
		json.Unmarshal(params.Arguments, &args)
		content, err := SearchNamuwiki(args.Keyword)
		if err != nil {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			}
			Broadcast(fmt.Sprintf(`{"type": "tool_call.failure", "tool": "namu_wiki", "reason": "%v"}`, err))
		} else {
			source := saveBufferedWebSource(userID, "namu_wiki", args.Keyword, "", "", content)
			content = formatBufferedSourceHandle(source)
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": content},
				},
			}
			Broadcast(`{"type": "tool_call.success", "tool": "namu_wiki"}`)
		}
	} else if params.Name == "naver_search" {
		var args struct {
			Query string `json:"query"`
		}
		json.Unmarshal(params.Arguments, &args)
		content, err := SearchNaver(args.Query)
		if err != nil {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			}
			Broadcast(fmt.Sprintf(`{"type": "tool_call.failure", "tool": "naver_search", "reason": "%v"}`, err))
		} else {
			source := saveBufferedWebSource(userID, "naver_search", args.Query, "", fmt.Sprintf("Naver: %s", compactMemoryText(args.Query, 80)), content)
			content = formatBufferedSourceHandle(source)
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": content},
				},
			}
			Broadcast(`{"type": "tool_call.success", "tool": "naver_search"}`)
		}
	} else if params.Name == "get_current_location" {
		_, _, _, locationInfo, _, _ := GetContext()
		if locationInfo == "" {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": "Location information not provided by client."},
				},
			}
			Broadcast(`{"type": "tool_call.failure", "tool": "get_current_location", "reason": "No location info"}`)
		} else {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": locationInfo},
				},
			}
			Broadcast(`{"type": "tool_call.success", "tool": "get_current_location"}`)
		}

	} else if params.Name == "execute_command" {
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			res.Error = &JSONRPCError{Code: -32602, Message: "Invalid arguments"}
		} else if args.Command == "" {
			res.Error = &JSONRPCError{Code: -32602, Message: "Missing 'command' argument"}
		} else {
			output, err := ExecuteCommand(args.Command, disallowedCmds, disallowedDirs)
			if err != nil {
				res.Result = map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
					},
					"isError": true,
				}
				Broadcast(fmt.Sprintf(`{"type": "tool_call.failure", "tool": "execute_command", "reason": "%v"}`, err))
			} else {
				res.Result = map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": output},
					},
				}
				Broadcast(`{"type": "tool_call.success", "tool": "execute_command"}`)
			}
		}

	} else {
		res.Error = &JSONRPCError{Code: -32601, Message: "Tool not found"}
	}
}
