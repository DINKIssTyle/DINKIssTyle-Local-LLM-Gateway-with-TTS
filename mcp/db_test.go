package mcp

import (
	"os"
	"testing"
	"time"
)

func TestDBCreationAndSearch(t *testing.T) {
	// 1. Setup temporary DB
	tmpFile, err := os.CreateTemp("", "memory_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	dbPath := tmpFile.Name()
	tmpFile.Close() // Close so sqlite can open it
	defer os.Remove(dbPath)

	// 2. Initialize DB
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer CloseDB()

	userID := "test_user_123"

	// 3. Insert Memories
	id1, err := InsertMemory(userID, "Used golang and sqlite for db.", "golang,sqlite,db", "Full text of conversation about golang and sqlite.")
	if err != nil {
		t.Fatalf("InsertMemory 1 failed: %v", err)
	}

	id2, err := InsertMemory(userID, "Talked about wails desktop.", "wails,desktop,gui", "Full text about wails.")
	if err != nil {
		t.Fatalf("InsertMemory 2 failed: %v", err)
	}

	if id1 == 0 || id2 == 0 {
		t.Fatalf("Expected valid IDs, got %d, %d", id1, id2)
	}

	// 4. Search Memories (Keyword match)
	results, err := SearchMemories(userID, "golang")
	if err != nil {
		t.Fatalf("SearchMemories failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result for 'golang', got %d", len(results))
	}
	if results[0].ID != id1 {
		t.Fatalf("Expected result ID %d, got %d", id1, results[0].ID)
	}

	// 5. Read Full Memory
	mem, err := ReadMemory(userID, id1)
	if err != nil {
		t.Fatalf("ReadMemory failed: %v", err)
	}
	if mem.FullText != "Full text of conversation about golang and sqlite." {
		t.Fatalf("Expected full text, got: %s", mem.FullText)
	}
}

func TestSearchMemoriesMultiQueryFindsTokenizedRewrite(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "memory_test_multi_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	dbPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(dbPath)

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer CloseDB()

	userID := "test_user_multi"
	id, err := InsertMemory(userID, "User's name is Park Nomin.", "name,이름,박노민,user name", "사용자 이름, user name 은 박노민 입니다.")
	if err != nil {
		t.Fatalf("InsertMemory failed: %v", err)
	}

	results, err := SearchMemoriesMultiQuery(userID, []string{
		"user nickname name profile user",
		"이름",
		"박노민",
	})
	if err != nil {
		t.Fatalf("SearchMemoriesMultiQuery failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("Expected at least 1 result, got 0")
	}
	if results[0].ID != id {
		t.Fatalf("Expected result ID %d, got %d", id, results[0].ID)
	}
}

func TestLastSessionUpsertAndFetch(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "last_session_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	dbPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(dbPath)

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer CloseDB()

	userID := "session_user"

	if err := UpsertLastSession(userID, "hello", "hi there", "stateful", nowForTest()); err != nil {
		t.Fatalf("UpsertLastSession failed: %v", err)
	}

	entry, err := GetLastSession(userID)
	if err != nil {
		t.Fatalf("GetLastSession failed: %v", err)
	}
	if entry.LastUserMessage != "hello" {
		t.Fatalf("expected last user message to be hello, got %q", entry.LastUserMessage)
	}
	if entry.LastAssistantMessage != "hi there" {
		t.Fatalf("expected last assistant message to be hi there, got %q", entry.LastAssistantMessage)
	}
	if entry.Mode != "stateful" {
		t.Fatalf("expected mode to be stateful, got %q", entry.Mode)
	}

	if err := UpsertLastSession(userID, "updated user", "updated assistant", "standard", nowForTest()); err != nil {
		t.Fatalf("UpsertLastSession update failed: %v", err)
	}

	updated, err := GetLastSession(userID)
	if err != nil {
		t.Fatalf("GetLastSession after update failed: %v", err)
	}
	if updated.LastUserMessage != "updated user" || updated.LastAssistantMessage != "updated assistant" {
		t.Fatalf("last session was not updated correctly: %+v", updated)
	}
	if updated.Mode != "standard" {
		t.Fatalf("expected mode to be standard after update, got %q", updated.Mode)
	}

	if err := DeleteLastSession(userID); err != nil {
		t.Fatalf("DeleteLastSession failed: %v", err)
	}
	if _, err := GetLastSession(userID); err == nil {
		t.Fatalf("expected GetLastSession to fail after delete")
	}
}

func nowForTest() time.Time {
	return time.Now().UTC()
}
