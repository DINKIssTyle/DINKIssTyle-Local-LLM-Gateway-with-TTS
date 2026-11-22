package toolruntime

import (
	"context"
	"dinkisstyle-chat/internal/mcp"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestTranscriptToolsAreAccountScopedReadOnlyAndPaginated(t *testing.T) {
	if err := mcp.InitDB(filepath.Join(t.TempDir(), "transcripts.db")); err != nil {
		t.Fatal(err)
	}
	defer mcp.CloseDB()
	for i := 0; i < 22; i++ {
		if err := mcp.SaveTranscript(mcp.TranscriptEntry{UserID: "alice", SegmentID: string(rune('a' + i)), StartedAt: "2026-11-20T15:00:00Z", EndedAt: "2026-11-20T15:00:01Z", Text: "회의 원문"}); err != nil {
			t.Fatal(err)
		}
	}
	registry := NewDefaultRegistry()
	ctx := context.Background()
	result, err := registry.Call(ctx, ExecutionContext{UserID: "alice"}, "read_transcripts", json.RawMessage(`{"date":"2026-11-21"}`))
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Entries    []mcp.TranscriptEntry
		NextOffset int
	}
	if err := json.Unmarshal([]byte(result.Content), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Entries) != 20 || data.NextOffset != 20 {
		t.Fatalf("page: %+v", data)
	}
	result, err = registry.Call(ctx, ExecutionContext{UserID: "bob"}, "search_transcripts", json.RawMessage(`{"query":"회의"}`))
	if err != nil || strings.Contains(result.Content, "회의 원문") {
		t.Fatalf("cross-user leak: %s %v", result.Content, err)
	}
	if _, err := registry.Call(ctx, ExecutionContext{}, "read_transcripts", json.RawMessage(`{"date":"2026-11-21"}`)); err == nil {
		t.Fatal("anonymous transcript access")
	}
	for _, tool := range registry.List(ExecutionContext{UserID: "alice"}) {
		if strings.Contains(tool.Name, "transcripts") && (!tool.Metadata.ReadOnly || tool.Metadata.SideEffecting) {
			t.Fatal("transcript tool can mutate records")
		}
	}
}
