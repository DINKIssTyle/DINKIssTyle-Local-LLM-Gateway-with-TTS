package mcp

import (
	"dinkisstyle-chat/internal/config"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

// Global state for SSE clients
var (
	clients         = make(map[string]chan string)
	clientsMu       sync.Mutex
	nextClientIDSeq atomic.Uint64
)

func AddClient(ch chan string) string {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	id := strconv.FormatUint(nextClientIDSeq.Add(1), 10)
	clients[id] = ch
	log.Printf("[MCP-DEBUG] Total Clients: %d", len(clients))
	return id
}

func RemoveClient(clientID string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	ch, ok := clients[clientID]
	if !ok {
		return
	}
	delete(clients, clientID)
	close(ch)
}

// SendToClient sends a message to a specific SSE client and returns whether it was delivered.
func SendToClient(clientID, msg string) bool {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	ch, ok := clients[clientID]
	if !ok {
		return false
	}
	select {
	case ch <- msg:
		return true
	default:
		log.Printf("[MCP-DEBUG] SendToClient SKIPPED for client=%s (channel full)", clientID)
		return false
	}
}

func buildEndpointURL(r *http.Request, clientID string) string {
	scheme := "http"
	if r != nil && r.TLS != nil {
		scheme = "https"
	}
	host := ""
	if r != nil {
		host = r.Host
	}
	endpointURL := fmt.Sprintf("%s://%s/mcp/messages", scheme, host)
	if strings.TrimSpace(clientID) == "" {
		return endpointURL
	}
	values := url.Values{}
	values.Set("client_id", clientID)
	return endpointURL + "?" + values.Encode()
}

func extractClientID(r *http.Request) string {
	if r == nil {
		return ""
	}
	if clientID := strings.TrimSpace(r.URL.Query().Get("client_id")); clientID != "" {
		return clientID
	}
	if clientID := strings.TrimSpace(r.Header.Get("X-MCP-Client-ID")); clientID != "" {
		return clientID
	}
	return ""
}

// buildResponse constructs a JSON-RPC response for a given request
func buildResponse(req *JSONRPCRequest, userID string, enableMemory bool, disabledTools []string, disallowedCmds []string, disallowedDirs []string, clientID string) *JSONRPCResponse {
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
				"version": config.AppVersion,
			},
		}

	case "tools/list":
		// TODO: Filter tools list based on disabledTools?
		// For now, list all, but fail on call.
		res.Result = map[string]interface{}{
			"tools": GetToolList(),
		}

	case "tools/call":
		handleToolCall(req, res, userID, enableMemory, disabledTools, disallowedCmds, disallowedDirs, clientID)

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
	clientID := AddClient(messageChan)
	defer func() {
		log.Printf("[MCP-DEBUG] SSE CLOSED for %s", r.RemoteAddr)
		RemoveClient(clientID)
	}()

	endpointURL := buildEndpointURL(r, clientID)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()
	log.Printf("[MCP-DEBUG] Advertised Endpoint: %s", endpointURL)

	// If we captured an initial request, process it immediately in the stream
	if initialReq != nil {
		ctx := GetContext()
		res := buildResponse(initialReq, ctx.UserID, ctx.EnableMemory, ctx.DisabledTools, ctx.DisallowedCmds, ctx.DisallowedDirs, clientID)
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

	clientID := extractClientID(r)
	if clientID == "" {
		http.Error(w, "Missing client_id", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)

	go func() {
		time.Sleep(50 * time.Millisecond)
		ctx := GetContext()
		res := buildResponse(&req, ctx.UserID, ctx.EnableMemory, ctx.DisabledTools, ctx.DisallowedCmds, ctx.DisallowedDirs, clientID)
		if res != nil {
			respBytes, _ := json.Marshal(res)
			delivered := SendToClient(clientID, string(respBytes))
			log.Printf("[MCP-DEBUG] Routed %s to client=%s delivered=%v", req.Method, clientID, delivered)
		}
	}()
}

func handleToolCall(req *JSONRPCRequest, res *JSONRPCResponse, userID string, enableMemory bool, disabledTools []string, disallowedCmds []string, disallowedDirs []string, clientID string) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		res.Error = &JSONRPCError{Code: -32602, Message: "Invalid parameters"}
		return
	}

	log.Printf("[MCP] Tool Call: %s", params.Name)

	content, err := ExecuteToolByName(params.Name, params.Arguments, userID, enableMemory, disabledTools)
	if err != nil {
		res.Result = map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
			},
			"isError": true,
		}
		if strings.TrimSpace(clientID) != "" {
			SendToClient(clientID, fmt.Sprintf(`{"type": "tool_call.failure", "tool": "%s", "reason": %q}`, params.Name, err.Error()))
		}
		return
	}

	res.Result = map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": content},
		},
	}
	if strings.TrimSpace(clientID) != "" {
		SendToClient(clientID, fmt.Sprintf(`{"type": "tool_call.success", "tool": %q}`, params.Name))
	}
}
