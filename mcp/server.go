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
			Description: "Read the text content of a specific URL. Use this to read articles or documentation.",
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
			Description: "Manage the user's long-term personal memory (v14 - Enhanced). Actions: 'remember' to save facts (server auto-extracts key), 'forget' to remove (log preserved), 'query' for fast lookup, 'read' for full index. Data is protected with Append-Only logging.",
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
						"description": "For 'remember': the fact to save (e.g., 'My name is DINKI' or 'Name: DINKI'). For 'forget'/'query': the key to lookup.",
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
}

func RemoveClient(ch chan string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	delete(clients, ch)
	close(ch)
}

// Broadcast sends a message to all connected SSE clients
func Broadcast(msg string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for ch := range clients {
		select {
		case ch <- msg:
		default:
			// Non-blocking send
		}
	}
}

// HandleSSE handles the connection for MCP (Server-Sent Events)
func HandleSSE(w http.ResponseWriter, r *http.Request) {
	log.Printf("[MCP-DEBUG] HandleSSE called from %s Method=%s", r.RemoteAddr, r.Method)
	for k, v := range r.Header {
		log.Printf("[MCP-DEBUG] Header %s: %v", k, v)
	}

	// Read and log body (LM Studio sends initialize here)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[MCP-DEBUG] Failed to read body: %v", err)
	}
	r.Body.Close()

	if len(bodyBytes) > 0 {
		log.Printf("[MCP-DEBUG] Initial Request Body: %s", string(bodyBytes))
	}

	// Process initial request if exists
	if len(bodyBytes) > 0 {
		var req JSONRPCRequest
		if err := json.Unmarshal(bodyBytes, &req); err == nil {
			if req.Method == "initialize" {
				// Handle initialize via SSE (Keep existing flow)
				log.Println("[MCP-DEBUG] Handling initial initialize request")

				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")
				w.Header().Set("Access-Control-Allow-Origin", "*")

				log.Println("[MCP] New SSE Client Connected")

				messageChan := make(chan string, 50) // Increased buffer
				AddClient(messageChan)
				defer func() {
					log.Println("[MCP-DEBUG] Removing Client and Closing Connection")
					RemoveClient(messageChan)
				}()

				// Send endpoint event
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				host := strings.Replace(r.Host, "localhost", "127.0.0.1", 1)
				endpointURL := fmt.Sprintf("%s://%s/mcp/messages", scheme, host)
				log.Printf("[MCP-DEBUG] Advertised Endpoint: %s", endpointURL)

				_, err = fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
				if err != nil {
					log.Printf("[MCP-DEBUG] Failed to write endpoint event: %v", err)
					return
				}
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
					log.Println("[MCP-DEBUG] Flushed endpoint event")
				}

				// Send initialize response via SSE
				res := JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"protocolVersion": "2024-11-05",
						"capabilities": map[string]interface{}{
							"tools": map[string]interface{}{},
						},
						"serverInfo": map[string]string{
							"name":    "DKST Local Gateway",
							"version": "1.0.0",
						},
					},
				}
				respBytes, _ := json.Marshal(res)
				messageChan <- string(respBytes)

				// Start keepalive ticker
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()

				// SSE Loop
				for {
					select {
					case msg := <-messageChan:
						log.Printf("[MCP-DEBUG] Sending SSE Message: %s", msg)
						fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
						if f, ok := w.(http.Flusher); ok {
							f.Flush()
						}
					case <-ticker.C:
						fmt.Fprintf(w, ": keepalive\n\n")
						if f, ok := w.(http.Flusher); ok {
							f.Flush()
						}
					case <-r.Context().Done():
						log.Println("[MCP] Client Disconnected (Context Done)")
						return
					}
				}
			} else {
				// Handle other messages as standard POST RPC (HandleMessages logic)
				log.Printf("[MCP-DEBUG] Handling direct RPC on SSE endpoint: %s", req.Method)
				// Reconstruct request for helper or just handle inline
				// Handling inline for simplicity and access to 'req'
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Access-Control-Allow-Origin", "*")

				// Capture User ID and Memory Setting from Global Context
				userID, enableMemory := GetContext()

				var res JSONRPCResponse
				res.JSONRPC = "2.0"
				res.ID = req.ID

				switch req.Method {
				case "tools/list":
					res.Result = map[string]interface{}{
						"tools": GetToolList(),
					}
				case "notifications/initialized":
					// Just ack
					return
				case "notifications/cancelled":
					return
				case "ping":
					res.Result = map[string]string{}
				default:
					// For tools/call, we need the RawMessage.
					// Since we parsed into 'req', we have req.Params
					if req.Method == "tools/call" {
						handleToolCall(&req, &res, userID, enableMemory)
					} else {
						res.Error = &JSONRPCError{Code: -32601, Message: "Method not found"}
					}
				}
				json.NewEncoder(w).Encode(res)
				return
			}
		}
	}
}

// HandleMessages processes incoming JSON-RPC messages from the client
func HandleMessages(w http.ResponseWriter, r *http.Request) {
	log.Printf("[MCP-DEBUG] HandleMessages called from %s Method=%s", r.RemoteAddr, r.Method)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		log.Printf("[MCP-DEBUG] Method Not Allowed: %s", r.Method)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	// Capture body for debugging if decode fails
	// But decoding directly is efficient. Let's decode and log error.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[MCP-DEBUG] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Capture User ID and Memory Setting from Global Context
	userID, enableMemory := GetContext()

	log.Printf("[MCP] Received Method: %s (ID: %v) User: %s Memory: %v", req.Method, req.ID, userID, enableMemory)

	// Acknowledge receipt immediately (MCP Spec)
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Accepted"))

	// Process asynchronously and send response via SSE
	go func() {
		var res JSONRPCResponse
		res.JSONRPC = "2.0"
		res.ID = req.ID

		switch req.Method {
		case "initialize":
			res.Result = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
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
			// Pass context manually since we are in a goroutine
			handleToolCall(&req, &res, userID, enableMemory)

		case "notifications/initialized":
			log.Println("[MCP] Client Initialized")
			return

		default:
			if req.Method == "ping" {
				res.Result = map[string]string{}
			} else {
				res.Error = &JSONRPCError{Code: -32601, Message: "Method not found"}
			}
		}

		// Send Response via SSE
		respBytes, _ := json.Marshal(res)
		Broadcast(string(respBytes))
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
