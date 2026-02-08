package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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
	currentUserID       = "default"
	currentEnableMemory = false
	contextMu           sync.RWMutex
)

func SetContext(userID string, enableMemory bool) {
	contextMu.Lock()
	defer contextMu.Unlock()
	currentUserID = userID
	currentEnableMemory = enableMemory
	log.Printf("[MCP] Set Context -> User: %s, Memory: %v", userID, enableMemory)
}

func GetContext() (string, bool) {
	contextMu.RLock()
	defer contextMu.RUnlock()
	return currentUserID, currentEnableMemory
}

func GetToolList() []Tool {
	return []Tool{
		{
			Name:        "search_web",
			Description: "Search the internet using DuckDuckGo. Use this to find current information.",
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
			Description: "Read the text content of a specific URL. Use this ONLY when the user provides a URL or explicitly asks to read a specific page. DO NOT use this for describing images or identifying people in photos unless specifically requested.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{"type": "string", "description": "URL to visit"},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:        "personal_memory",
			Description: "Manage the user's long-term personal memory. Actions: 'remember' to save facts (server auto-extracts key), 'forget' to remove (log preserved), 'query' for fast lookup, 'read' for full index. Data is protected with Append-Only logging.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action: 'remember' (save fact, auto-key), 'forget' (remove from index), 'query' (fast lookup), 'read' (full memory).",
						"enum":        []string{"remember", "forget", "query", "read", "search"},
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "For 'remember': the fact to save. For 'forget'/'query': the key to lookup.",
					},
				},
				"required": []string{"action"},
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
			Name:        "read_user_document",
			Description: "Read a specific document from the user's memory folder. Available files: personal.md, work.md, index.md, log.md. Use index.md first to understand available information.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"filename": map[string]interface{}{
						"type":        "string",
						"description": "The filename to read (e.g., 'personal.md', 'work.md', 'index.md')",
					},
				},
				"required": []string{"filename"},
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
func buildResponse(req *JSONRPCRequest, userID string, enableMemory bool) *JSONRPCResponse {
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
		res.Result = map[string]interface{}{
			"tools": GetToolList(),
		}

	case "tools/call":
		handleToolCall(req, res, userID, enableMemory)

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
		userID, enableMemory := GetContext()
		res := buildResponse(initialReq, userID, enableMemory)
		if res != nil {
			respBytes, _ := json.Marshal(res)
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(respBytes))
			flusher.Flush()
			log.Printf("[MCP-DEBUG] Inline Response sent for %s", initialReq.Method)
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
		userID, enableMemory := GetContext()
		res := buildResponse(&req, userID, enableMemory)
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
func ExecuteToolByName(toolName string, argumentsJSON []byte, userID string, enableMemory bool) (string, error) {
	log.Printf("[MCP] ExecuteToolByName: %s (User: %s, Memory: %v)", toolName, userID, enableMemory)

	switch toolName {
	case "search_web":
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for search_web: %v", err)
		}
		return SearchWeb(args.Query)

	case "read_web_page":
		var args struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for read_web_page: %v", err)
		}
		return ReadPage(args.URL)

	case "personal_memory":
		if !enableMemory {
			return "", fmt.Errorf("memory feature is disabled by user settings")
		}
		var args struct {
			Action  string `json:"action"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for personal_memory: %v", err)
		}
		filePath, err := GetUserMemoryPath(userID)
		if err != nil {
			return "", fmt.Errorf("error resolving memory path: %v", err)
		}
		return ManageMemory(filePath, args.Action, args.Content)

	case "get_current_time":
		return GetCurrentTime()

	case "read_user_document":
		if !enableMemory {
			return "", fmt.Errorf("memory feature is disabled by user settings")
		}
		var args struct {
			Filename string `json:"filename"`
		}
		if err := json.Unmarshal(argumentsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments for read_user_document: %v", err)
		}
		if args.Filename == "" {
			args.Filename = "index.md"
		}
		if args.Filename == "index.md" {
			if err := GenerateIndexMD(userID); err != nil {
				log.Printf("[MCP] Failed to regenerate index.md: %v", err)
			}
		}
		return ReadUserDocument(userID, args.Filename)

	default:
		return "", fmt.Errorf("tool not found: %s", toolName)
	}
}

// Helper to handle tool calls

func handleToolCall(req *JSONRPCRequest, res *JSONRPCResponse, userID string, enableMemory bool) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		res.Error = &JSONRPCError{Code: -32602, Message: "Invalid parameters"}
		return
	}

	log.Printf("[MCP] Tool Call: %s", params.Name)

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
		} else {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": content},
				},
			}
		}
	} else if params.Name == "read_web_page" {
		var args struct {
			URL string `json:"url"`
		}
		json.Unmarshal(params.Arguments, &args)
		content, err := ReadPage(args.URL)
		if err != nil {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			}
		} else {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": content},
				},
			}
		}
	} else if params.Name == "personal_memory" {
		if !enableMemory {
			res.Error = &JSONRPCError{Code: -32601, Message: "Memory feature is disabled by user settings."}
			return
		}

		var args struct {
			Action  string `json:"action"`
			Content string `json:"content"`
		}
		json.Unmarshal(params.Arguments, &args)

		filePath, err := GetUserMemoryPath(userID)
		if err != nil {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error resolving memory path: %v", err)},
				},
				"isError": true,
			}
			return
		}

		content, err := ManageMemory(filePath, args.Action, args.Content)
		if err != nil {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			}
		} else {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": content},
				},
			}
		}

	} else if params.Name == "get_current_time" {
		content, _ := GetCurrentTime()
		res.Result = map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": content},
			},
		}
	} else if params.Name == "read_user_document" {
		if !enableMemory {
			res.Error = &JSONRPCError{Code: -32601, Message: "Memory feature is disabled by user settings."}
			return
		}

		var args struct {
			Filename string `json:"filename"`
		}
		json.Unmarshal(params.Arguments, &args)

		if args.Filename == "" {
			// Default to index.md
			args.Filename = "index.md"
		}

		// Regenerate index.md if requested
		if args.Filename == "index.md" {
			if err := GenerateIndexMD(userID); err != nil {
				log.Printf("[MCP] Failed to regenerate index.md: %v", err)
			}
		}

		content, err := ReadUserDocument(userID, args.Filename)
		if err != nil {
			// If document not found, list available files
			files, _ := ListUserMemoryFiles(userID)
			availableFiles := strings.Join(files, ", ")
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v. Available files: %s", err, availableFiles)},
				},
				"isError": true,
			}
		} else {
			res.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": content},
				},
			}
		}
	} else {
		res.Error = &JSONRPCError{Code: -32601, Message: "Tool not found"}
	}
}
