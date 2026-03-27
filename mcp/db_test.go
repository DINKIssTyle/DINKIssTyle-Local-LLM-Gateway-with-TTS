package mcp

import (
	"os"
	"strings"
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
	id1, err := InsertMemory(userID, "Full text of conversation about golang and sqlite.")
	if err != nil {
		t.Fatalf("InsertMemory 1 failed: %v", err)
	}

	id2, err := InsertMemory(userID, "Full text about wails.")
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
	id, err := InsertMemory(userID, "사용자 이름, user name 은 박노민 입니다.")
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

func TestMemoryChunkTableExists(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "memory_chunk_schema_*.db")
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

	var tableName string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'memory_chunks'`).Scan(&tableName); err != nil {
		t.Fatalf("failed to find memory_chunks table: %v", err)
	}
	if tableName != "memory_chunks" {
		t.Fatalf("expected memory_chunks table, got %q", tableName)
	}
}

func TestInsertMemoryCreatesChunks(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "memory_chunk_insert_*.db")
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

	userID := "chunk_user"
	longText := ""
	for i := 0; i < 40; i++ {
		longText += "This is a long memory sentence that should be chunked for retrieval. "
	}

	id, err := InsertMemory(userID, longText)
	if err != nil {
		t.Fatalf("InsertMemory failed: %v", err)
	}

	var chunkCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_chunks WHERE memory_id = ?`, id).Scan(&chunkCount); err != nil {
		t.Fatalf("failed to count chunks: %v", err)
	}
	if chunkCount < 2 {
		t.Fatalf("expected multiple chunks, got %d", chunkCount)
	}
}

func TestSearchMemoryChunkMatchesFindsRelevantChunk(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "memory_chunk_search_*.db")
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

	userID := "chunk_search_user"
	longText := strings.Repeat("prefix text that does not matter much. ", 40) + "박노민이라는 이름이 뒤쪽 청크에 들어 있습니다."
	if _, err := InsertMemory(userID, longText); err != nil {
		t.Fatalf("InsertMemory failed: %v", err)
	}

	results, err := SearchMemoryChunkMatchesMultiQuery(userID, []string{"박노민"}, 10)
	if err != nil {
		t.Fatalf("SearchMemoryChunkMatchesMultiQuery failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected chunk search results, got 0")
	}
	if !strings.Contains(results[0].ChunkText, "박노민") {
		t.Fatalf("expected matching chunk text to contain query, got %q", results[0].ChunkText)
	}
}

func TestIncrementMemoryChunkHitCount(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "memory_chunk_hit_*.db")
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

	userID := "chunk_hit_user"
	if _, err := InsertMemory(userID, strings.Repeat("hit count memory content. ", 40)); err != nil {
		t.Fatalf("InsertMemory failed: %v", err)
	}

	var chunkID int64
	if err := db.QueryRow(`SELECT id FROM memory_chunks LIMIT 1`).Scan(&chunkID); err != nil {
		t.Fatalf("failed to fetch chunk id: %v", err)
	}

	if err := IncrementMemoryChunkHitCount(chunkID); err != nil {
		t.Fatalf("IncrementMemoryChunkHitCount failed: %v", err)
	}

	var hitCount int
	if err := db.QueryRow(`SELECT hit_count FROM memory_chunks WHERE id = ?`, chunkID).Scan(&hitCount); err != nil {
		t.Fatalf("failed to read chunk hit count: %v", err)
	}
	if hitCount != 1 {
		t.Fatalf("expected chunk hit count 1, got %d", hitCount)
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

func TestChatSessionTablesExist(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "chat_session_schema_*.db")
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

	var chatSessionsTable string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'chat_sessions'`).Scan(&chatSessionsTable); err != nil {
		t.Fatalf("failed to find chat_sessions table: %v", err)
	}
	if chatSessionsTable != "chat_sessions" {
		t.Fatalf("expected chat_sessions table, got %q", chatSessionsTable)
	}

	var chatEventsTable string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'chat_events'`).Scan(&chatEventsTable); err != nil {
		t.Fatalf("failed to find chat_events table: %v", err)
	}
	if chatEventsTable != "chat_events" {
		t.Fatalf("expected chat_events table, got %q", chatEventsTable)
	}
}

func nowForTest() time.Time {
	return time.Now().UTC()
}
