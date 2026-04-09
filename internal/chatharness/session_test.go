package chatharness

import (
	"os"
	"testing"

	"dinkisstyle-chat/internal/mcp"
)

func TestSessionTrackerSkipsDeltaSnapshotPersistenceUntilCompletion(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "session_tracker_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	dbPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(dbPath)

	if err := mcp.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer mcp.CloseDB()

	sessionEntry, err := mcp.UpsertChatSession(mcp.ChatSessionEntry{
		UserID:      "chat_user",
		SessionKey:  "default",
		Status:      "running",
		UIStateJSON: "{}",
	})
	if err != nil {
		t.Fatalf("UpsertChatSession failed: %v", err)
	}

	tracker := NewSessionTracker(
		"chat_user",
		"turn-1",
		sessionEntry,
		true,
		SessionUISnapshot{ToolCards: map[string]SessionToolCardSnapshot{}, Messages: []SessionMessageSnapshot{}},
		"{}",
	)
	state := SessionPersistState{Status: "running", UIStateJSON: "{}"}

	tracker.AppendEvent(state, "user", "message.created", map[string]interface{}{"content": "hello"})

	current, err := mcp.GetCurrentChatSession("chat_user")
	if err != nil {
		t.Fatalf("GetChatSession after user message failed: %v", err)
	}
	snapshotAfterUser := ParseUISnapshot(current.UIStateJSON)
	if len(snapshotAfterUser.Messages) != 1 || snapshotAfterUser.Messages[0].UserContent != "hello" {
		t.Fatalf("expected user message in snapshot, got %+v", snapshotAfterUser.Messages)
	}

	tracker.AppendEvent(state, "assistant", "message.delta", map[string]interface{}{
		"content":      "partial",
		"full_content": "partial",
	})

	current, err = mcp.GetCurrentChatSession("chat_user")
	if err != nil {
		t.Fatalf("GetChatSession after delta failed: %v", err)
	}
	snapshotAfterDelta := ParseUISnapshot(current.UIStateJSON)
	if len(snapshotAfterDelta.Messages) != 1 {
		t.Fatalf("expected 1 snapshot message after delta, got %d", len(snapshotAfterDelta.Messages))
	}
	if snapshotAfterDelta.Messages[0].AssistantContent != "" {
		t.Fatalf("expected assistant content to stay empty until completion, got %q", snapshotAfterDelta.Messages[0].AssistantContent)
	}
	eventsAfterDelta, err := mcp.ListChatEvents("chat_user", current.ID, 0, 10)
	if err != nil {
		t.Fatalf("ListChatEvents after delta failed: %v", err)
	}
	if len(eventsAfterDelta) != 1 {
		t.Fatalf("expected only user event to persist before completion, got %d", len(eventsAfterDelta))
	}
	if eventsAfterDelta[0].EventType != "message.created" {
		t.Fatalf("expected only message.created before completion, got %+v", eventsAfterDelta)
	}

	tracker.AppendEvent(state, "assistant", "request.complete", map[string]interface{}{
		"final_assistant_content": "final answer",
	})

	current, err = mcp.GetCurrentChatSession("chat_user")
	if err != nil {
		t.Fatalf("GetChatSession after completion failed: %v", err)
	}
	snapshotAfterComplete := ParseUISnapshot(current.UIStateJSON)
	if len(snapshotAfterComplete.Messages) != 1 {
		t.Fatalf("expected 1 snapshot message after completion, got %d", len(snapshotAfterComplete.Messages))
	}
	if snapshotAfterComplete.Messages[0].AssistantContent != "final answer" {
		t.Fatalf("expected final assistant content after completion, got %q", snapshotAfterComplete.Messages[0].AssistantContent)
	}
	eventsAfterComplete, err := mcp.ListChatEvents("chat_user", current.ID, 0, 10)
	if err != nil {
		t.Fatalf("ListChatEvents after completion failed: %v", err)
	}
	if len(eventsAfterComplete) != 2 {
		t.Fatalf("expected user and completion events after completion, got %d", len(eventsAfterComplete))
	}
	if eventsAfterComplete[1].EventType != "request.complete" {
		t.Fatalf("expected request.complete as final persisted event, got %+v", eventsAfterComplete)
	}
}
