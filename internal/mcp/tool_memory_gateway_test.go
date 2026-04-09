package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSynthesizeMemoryContextRetriesAndCapsTokens(t *testing.T) {
	originalEndpoint := memorySynthesisEndpoint
	t.Cleanup(func() {
		memorySynthesisEndpoint = originalEndpoint
	})

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "temporary overload", http.StatusServiceUnavailable)
			return
		}

		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected content type: %s", got)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload["max_tokens"] != float64(memorySynthesisMaxTokens) {
			t.Fatalf("expected max_tokens %d, got %#v", memorySynthesisMaxTokens, payload["max_tokens"])
		}
		if payload["stream"] != false {
			t.Fatalf("expected stream=false, got %#v", payload["stream"])
		}

		fmt.Fprint(w, `{"choices":[{"message":{"content":"relevant fact"}}]}`)
	}))
	defer server.Close()

	memorySynthesisEndpoint = server.URL
	result, err := SynthesizeMemoryContext("user", "what is my preference?", strings.Repeat("한글 memory ", 600))
	if err != nil {
		t.Fatalf("SynthesizeMemoryContext failed: %v", err)
	}
	if result != "relevant fact" {
		t.Fatalf("unexpected synthesis result: %q", result)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestSafeUserDocumentPathRejectsTraversal(t *testing.T) {
	baseDir := t.TempDir()

	if _, err := safeUserDocumentPath(baseDir, "../secrets.txt"); err == nil {
		t.Fatalf("expected traversal path to be rejected")
	}
	if _, err := safeUserDocumentPath(baseDir, "..\\secrets.txt"); err == nil {
		t.Fatalf("expected windows-style traversal path to be rejected")
	}
}

func TestReadUserDocumentRejectsAbsoluteAndTraversalPaths(t *testing.T) {
	userID := "test-user"
	baseDir, err := GetUserMemoryDir(userID)
	if err != nil {
		t.Fatalf("GetUserMemoryDir failed: %v", err)
	}

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("failed to create user memory dir: %v", err)
	}

	goodPath := filepath.Join(baseDir, "note.md")
	if err := os.WriteFile(goodPath, []byte("ok"), 0o644); err != nil {
		t.Fatalf("failed to write test document: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(goodPath)
	})

	if _, err := ReadUserDocument(userID, "note.md"); err != nil {
		t.Fatalf("expected safe path to succeed: %v", err)
	}
	if _, err := ReadUserDocument(userID, "../note.md"); err == nil {
		t.Fatalf("expected traversal path to fail")
	}
	if _, err := ReadUserDocument(userID, goodPath); err == nil {
		t.Fatalf("expected absolute path to fail")
	}
}

func TestCompactMemoryTextByEstimatedTokensTreatsKoreanAsHeavier(t *testing.T) {
	asciiOriginal := strings.Repeat("a", 20)
	ascii := compactMemoryTextByEstimatedTokens(asciiOriginal, 10)
	if !strings.HasSuffix(ascii, "... (truncated)") {
		t.Fatalf("expected truncated ascii text, got %q", ascii)
	}
	asciiCore := strings.TrimSuffix(ascii, "... (truncated)")

	koreanOriginal := strings.Repeat("가", 20)
	korean := compactMemoryTextByEstimatedTokens(koreanOriginal, 10)
	if !strings.HasSuffix(korean, "... (truncated)") {
		t.Fatalf("expected truncated korean text, got %q", korean)
	}
	koreanCore := strings.TrimSuffix(korean, "... (truncated)")
	if estimateTokenCount(koreanCore) > 10 {
		t.Fatalf("expected korean output to fit token budget, got %d tokens", estimateTokenCount(koreanCore))
	}
	if len([]rune(koreanCore)) >= len([]rune(asciiCore)) {
		t.Fatalf("expected korean text to truncate earlier than ascii for the same token budget")
	}
}
