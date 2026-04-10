package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuildEndpointURLIncludesClientID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/mcp/sse", nil)
	req.Host = "example.com"

	endpoint := buildEndpointURL(req, "42")
	if !strings.Contains(endpoint, "client_id=42") {
		t.Fatalf("expected client_id in endpoint, got %q", endpoint)
	}
}

func TestSendToClientRoutesOnlyToMatchingClient(t *testing.T) {
	clientsMu.Lock()
	clients = make(map[string]chan string)
	clientsMu.Unlock()

	chA := make(chan string, 1)
	chB := make(chan string, 1)
	idA := AddClient(chA)
	idB := AddClient(chB)
	t.Cleanup(func() {
		RemoveClient(idA)
		RemoveClient(idB)
	})

	if !SendToClient(idA, "hello-a") {
		t.Fatalf("expected message to be delivered to client A")
	}

	select {
	case got := <-chA:
		if got != "hello-a" {
			t.Fatalf("unexpected message for client A: %q", got)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for client A message")
	}

	select {
	case got := <-chB:
		t.Fatalf("client B should not receive client A message, got %q", got)
	default:
	}
}

func TestHandleMessagesRequiresClientID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(`{"jsonrpc":"2.0","method":"ping","id":1}`))
	rr := httptest.NewRecorder()

	HandleMessages(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request when client_id missing, got %d", rr.Code)
	}
}

func TestHandleMessagesRoutesResponseToRequestedClient(t *testing.T) {
	clientsMu.Lock()
	clients = make(map[string]chan string)
	clientsMu.Unlock()

	chA := make(chan string, 2)
	chB := make(chan string, 2)
	idA := AddClient(chA)
	idB := AddClient(chB)
	t.Cleanup(func() {
		RemoveClient(idA)
		RemoveClient(idB)
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp/messages?client_id="+idA, strings.NewReader(`{"jsonrpc":"2.0","method":"ping","id":7}`))
	rr := httptest.NewRecorder()

	HandleMessages(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected accepted response, got %d", rr.Code)
	}

	select {
	case got := <-chA:
		var res JSONRPCResponse
		if err := json.Unmarshal([]byte(got), &res); err != nil {
			t.Fatalf("failed to decode routed response: %v", err)
		}
		if res.ID.(float64) != 7 {
			t.Fatalf("expected response id 7, got %#v", res.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for client A response")
	}

	select {
	case got := <-chB:
		t.Fatalf("client B should not receive routed response, got %q", got)
	default:
	}
}
