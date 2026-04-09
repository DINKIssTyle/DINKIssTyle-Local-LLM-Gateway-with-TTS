package mcp

import (
	"database/sql"
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

func TestShouldForgetMemoryRespectsConfiguredRetentionDays(t *testing.T) {
	original := GetMemoryRetentionConfig()
	t.Cleanup(func() {
		SetMemoryRetentionConfig(original)
	})

	now := time.Now().UTC()
	oldMemory := MemoryEntry{
		CreatedAt:       now.AddDate(0, 0, -200),
		LastAccessedAt:  now.AddDate(0, 0, -200),
		ImportanceScore: 0.10,
		HitCount:        0,
	}

	SetMemoryRetentionConfig(MemoryRetentionConfig{
		CoreDays:      0,
		WorkingDays:   0,
		EphemeralDays: 14,
	})

	coreMemory := oldMemory
	coreMemory.MemoryTier = memoryTierCore
	if shouldForgetMemory(coreMemory, now) {
		t.Fatalf("expected core memory to be retained when CoreDays is 0")
	}

	workingMemory := oldMemory
	workingMemory.MemoryTier = memoryTierWorking
	if shouldForgetMemory(workingMemory, now) {
		t.Fatalf("expected working memory to be retained when WorkingDays is 0")
	}

	ephemeralMemory := oldMemory
	ephemeralMemory.MemoryTier = memoryTierEphemeral
	if !shouldForgetMemory(ephemeralMemory, now) {
		t.Fatalf("expected ephemeral memory to be pruned when EphemeralDays is 14")
	}

	SetMemoryRetentionConfig(MemoryRetentionConfig{
		CoreDays:      180,
		WorkingDays:   45,
		EphemeralDays: 14,
	})
	if !shouldForgetMemory(coreMemory, now) {
		t.Fatalf("expected core memory to be pruned when CoreDays is configured")
	}
	if !shouldForgetMemory(workingMemory, now) {
		t.Fatalf("expected working memory to be pruned when WorkingDays is configured")
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

	var ftsTable string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'memory_chunks_fts'`).Scan(&ftsTable); err != nil {
		t.Fatalf("failed to find memory_chunks_fts table: %v", err)
	}
	if ftsTable != "memory_chunks_fts" {
		t.Fatalf("expected memory_chunks_fts table, got %q", ftsTable)
	}

	var savedTurnChunkTable string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'saved_turn_chunks'`).Scan(&savedTurnChunkTable); err != nil {
		t.Fatalf("failed to find saved_turn_chunks table: %v", err)
	}
	if savedTurnChunkTable != "saved_turn_chunks" {
		t.Fatalf("expected saved_turn_chunks table, got %q", savedTurnChunkTable)
	}

	var savedTurnFTSTable string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'saved_turn_chunks_fts'`).Scan(&savedTurnFTSTable); err != nil {
		t.Fatalf("failed to find saved_turn_chunks_fts table: %v", err)
	}
	if savedTurnFTSTable != "saved_turn_chunks_fts" {
		t.Fatalf("expected saved_turn_chunks_fts table, got %q", savedTurnFTSTable)
	}
}

func TestInitDBConfiguresSQLitePragmas(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "memory_pragmas_*.db")
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

	var journalMode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("failed to read journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("expected WAL journal mode, got %q", journalMode)
	}

	var busyTimeout int
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("failed to read busy_timeout: %v", err)
	}
	if busyTimeout != sqliteBusyTimeout {
		t.Fatalf("expected busy_timeout %d, got %d", sqliteBusyTimeout, busyTimeout)
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

	var ftsCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_chunks_fts`).Scan(&ftsCount); err != nil {
		t.Fatalf("failed to count memory fts rows: %v", err)
	}
	if ftsCount != chunkCount {
		t.Fatalf("expected fts rows to match chunk count, got fts=%d chunks=%d", ftsCount, chunkCount)
	}

	var embeddingCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_chunk_embeddings WHERE embedding_model = ?`, webEmbeddingModel).Scan(&embeddingCount); err != nil {
		t.Fatalf("failed to count memory embeddings: %v", err)
	}
	if embeddingCount != chunkCount {
		t.Fatalf("expected embeddings for every memory chunk, got embeddings=%d chunks=%d", embeddingCount, chunkCount)
	}

	var tier string
	var importance float64
	var pinned int
	if err := db.QueryRow(`SELECT memory_tier, importance_score, pinned FROM memories WHERE id = ?`, id).Scan(&tier, &importance, &pinned); err != nil {
		t.Fatalf("failed to read memory retention fields: %v", err)
	}
	if tier == "" {
		t.Fatalf("expected memory tier to be set")
	}
	if importance <= 0 {
		t.Fatalf("expected positive importance score, got %f", importance)
	}
	if pinned != 0 {
		t.Fatalf("expected raw interaction memory to be unpinned")
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

func TestSaveSavedTurnCreatesChunksAndSearches(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "saved_turn_chunk_search_*.db")
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

	userID := "saved_turn_user"
	entry, err := SaveSavedTurn(userID,
		"예비군 교육 및 훈련은 몇 년 동안 진행되나요?",
		"예비군 훈련은 일반적으로 일정 기간 동안 단계적으로 진행됩니다.",
	)
	if err != nil {
		t.Fatalf("SaveSavedTurn failed: %v", err)
	}

	var chunkCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM saved_turn_chunks WHERE saved_turn_id = ?`, entry.ID).Scan(&chunkCount); err != nil {
		t.Fatalf("failed to count saved turn chunks: %v", err)
	}
	if chunkCount == 0 {
		t.Fatalf("expected saved turn chunks to be created")
	}

	var embeddingCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM saved_turn_chunk_embeddings`).Scan(&embeddingCount); err != nil {
		t.Fatalf("failed to count saved turn embeddings: %v", err)
	}
	if embeddingCount != chunkCount {
		t.Fatalf("expected embeddings for every saved turn chunk, got embeddings=%d chunks=%d", embeddingCount, chunkCount)
	}

	results, err := SearchSavedTurnChunkMatches(userID, "예비군 훈련 기간", 10)
	if err != nil {
		t.Fatalf("SearchSavedTurnChunkMatches failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected saved turn chunk search results, got 0")
	}
	if results[0].ID != entry.ID {
		t.Fatalf("expected saved turn id %d, got %d", entry.ID, results[0].ID)
	}
}

func TestSearchMemoryChunkMatchesPreTokenizesKoreanSuffixes(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "memory_chunk_korean_*.db")
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

	userID := "memory_chunk_korean_user"
	id, err := InsertMemory(userID, "임베딩 모델을 최적화했습니다.")
	if err != nil {
		t.Fatalf("InsertMemory failed: %v", err)
	}

	results, err := SearchMemoryChunkMatches(userID, "모델", 10)
	if err != nil {
		t.Fatalf("SearchMemoryChunkMatches failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected korean chunk search results, got 0")
	}
	if results[0].ID != id {
		t.Fatalf("expected result ID %d, got %d", id, results[0].ID)
	}

	results, err = SearchMemoryChunkMatches(userID, "최적화", 10)
	if err != nil {
		t.Fatalf("SearchMemoryChunkMatches failed for 최적화: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected korean stem search results for 최적화, got 0")
	}
}

func TestSearchSavedTurnChunkMatchesPreTokenizesKoreanSuffixes(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "saved_turn_korean_*.db")
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

	userID := "saved_turn_korean_user"
	entry, err := SaveSavedTurn(
		userID,
		"임베딩 파이프라인을 어떻게 다듬었나요?",
		"임베딩 모델을 최적화했습니다. 검색 정확도도 함께 개선했습니다.",
	)
	if err != nil {
		t.Fatalf("SaveSavedTurn failed: %v", err)
	}

	results, err := SearchSavedTurnChunkMatches(userID, "모델", 10)
	if err != nil {
		t.Fatalf("SearchSavedTurnChunkMatches failed for 모델: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected korean saved turn results for 모델, got 0")
	}
	if results[0].ID != entry.ID {
		t.Fatalf("expected saved turn id %d, got %d", entry.ID, results[0].ID)
	}

	results, err = SearchSavedTurnChunkMatches(userID, "최적화", 10)
	if err != nil {
		t.Fatalf("SearchSavedTurnChunkMatches failed for 최적화: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected korean saved turn results for 최적화, got 0")
	}
}

func TestDeleteSavedTurnRemovesChunkArtifacts(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "saved_turn_delete_*.db")
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

	userID := "saved_turn_delete_user"
	entry, err := SaveSavedTurn(userID, "첫째 아들 이름은?", "첫째 아들의 이름은 테스트입니다.")
	if err != nil {
		t.Fatalf("SaveSavedTurn failed: %v", err)
	}

	if err := DeleteSavedTurn(userID, entry.ID); err != nil {
		t.Fatalf("DeleteSavedTurn failed: %v", err)
	}

	var chunks int
	if err := db.QueryRow(`SELECT COUNT(*) FROM saved_turn_chunks WHERE saved_turn_id = ?`, entry.ID).Scan(&chunks); err != nil {
		t.Fatalf("failed to count saved turn chunks after delete: %v", err)
	}
	if chunks != 0 {
		t.Fatalf("expected saved turn chunks to be deleted, got %d", chunks)
	}

	var ftsRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM saved_turn_chunks_fts`).Scan(&ftsRows); err != nil {
		t.Fatalf("failed to count saved turn fts rows after delete: %v", err)
	}
	if ftsRows != 0 {
		t.Fatalf("expected saved turn fts rows to be deleted, got %d", ftsRows)
	}
}

func TestRetentionMaintenancePrunesOldEphemeralMemoryAndLogs(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "memory_retention_*.db")
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

	oldTime := time.Now().UTC().AddDate(0, 0, -(DefaultMemoryRetentionConfig().EphemeralDays + 10))
	result, err := db.Exec(`
		INSERT INTO memories (
			user_id, full_text, hit_count, created_at, memory_type, last_accessed_at, importance_score, pinned, memory_tier
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"retention_user",
		"temporary raw interaction that should fade away",
		0,
		oldTime,
		"raw_interaction",
		oldTime,
		0.20,
		0,
		memoryTierEphemeral,
	)
	if err != nil {
		t.Fatalf("failed to insert retention test memory: %v", err)
	}
	memoryID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to get retention test memory id: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO chat_sessions (user_id, session_key) VALUES (?, ?)`, "retention_user", "default"); err != nil {
		t.Fatalf("failed to insert chat session: %v", err)
	}
	var sessionID int64
	if err := db.QueryRow(`SELECT id FROM chat_sessions WHERE user_id = ? AND session_key = ?`, "retention_user", "default").Scan(&sessionID); err != nil {
		t.Fatalf("failed to read chat session id: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO chat_events (session_id, user_id, event_seq, role, event_type, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, "retention_user", 1, "user", "message.created", oldTime); err != nil {
		t.Fatalf("failed to insert old chat event: %v", err)
	}

	if err := runRetentionMaintenance(time.Now().UTC()); err != nil {
		t.Fatalf("runRetentionMaintenance failed: %v", err)
	}

	var remainingMemories int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories WHERE id = ?`, memoryID).Scan(&remainingMemories); err != nil {
		t.Fatalf("failed to count retained memory: %v", err)
	}
	if remainingMemories != 0 {
		t.Fatalf("expected old ephemeral memory to be pruned")
	}

	var remainingEvents int
	if err := db.QueryRow(`SELECT COUNT(*) FROM chat_events`).Scan(&remainingEvents); err != nil {
		t.Fatalf("failed to count chat events: %v", err)
	}
	if remainingEvents != 0 {
		t.Fatalf("expected old chat events to be pruned")
	}
}

func TestRetentionMaintenanceUsesShortTTLForStreamingEvents(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "retention_chat_events_short_*.db")
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

	if _, err := db.Exec(`INSERT INTO chat_sessions (user_id, session_key) VALUES (?, ?)`, "retention_user", "default"); err != nil {
		t.Fatalf("failed to insert chat session: %v", err)
	}
	var sessionID int64
	if err := db.QueryRow(`SELECT id FROM chat_sessions WHERE user_id = ? AND session_key = ?`, "retention_user", "default").Scan(&sessionID); err != nil {
		t.Fatalf("failed to read chat session id: %v", err)
	}

	shortExpired := time.Now().UTC().Add(-(time.Duration(retentionChatEventsShortHours) * time.Hour)).Add(-5 * time.Minute)
	longFresh := time.Now().UTC().Add(-2 * time.Hour)

	if _, err := db.Exec(`INSERT INTO chat_events (session_id, user_id, event_seq, role, event_type, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, "retention_user", 1, "assistant", "message.delta", shortExpired); err != nil {
		t.Fatalf("failed to insert expired message.delta: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO chat_events (session_id, user_id, event_seq, role, event_type, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, "retention_user", 2, "assistant", "request.complete", longFresh); err != nil {
		t.Fatalf("failed to insert fresh request.complete: %v", err)
	}

	if err := runRetentionMaintenance(time.Now().UTC()); err != nil {
		t.Fatalf("runRetentionMaintenance failed: %v", err)
	}

	var deltaCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM chat_events WHERE event_type = 'message.delta'`).Scan(&deltaCount); err != nil {
		t.Fatalf("failed to count message.delta rows: %v", err)
	}
	if deltaCount != 0 {
		t.Fatalf("expected message.delta to be pruned after short ttl, got %d", deltaCount)
	}

	var completeCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM chat_events WHERE event_type = 'request.complete'`).Scan(&completeCount); err != nil {
		t.Fatalf("failed to count request.complete rows: %v", err)
	}
	if completeCount != 1 {
		t.Fatalf("expected request.complete to survive long ttl, got %d", completeCount)
	}
}

func TestInitDBMigratesLegacyMemoriesRetentionColumns(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "legacy_memories_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	dbPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(dbPath)

	legacyDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open legacy db: %v", err)
	}
	if _, err := legacyDB.Exec(`
		CREATE TABLE memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			full_text TEXT NOT NULL,
			hit_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			memory_type TEXT DEFAULT 'raw_interaction'
		);
	`); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("failed to create legacy memories table: %v", err)
	}
	oldTime := time.Now().UTC().AddDate(0, 0, -5)
	if _, err := legacyDB.Exec(`
		INSERT INTO memories (user_id, full_text, hit_count, created_at, memory_type)
		VALUES (?, ?, ?, ?, ?)`,
		"legacy_user",
		"my name is Dinki and I prefer concise responses",
		3,
		oldTime,
		"raw_interaction",
	); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("failed to seed legacy memory: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("failed to close legacy db: %v", err)
	}

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed on legacy schema: %v", err)
	}
	defer CloseDB()

	rows, err := db.Query(`PRAGMA table_info(memories)`)
	if err != nil {
		t.Fatalf("failed to inspect migrated memories schema: %v", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("failed to scan migrated schema row: %v", err)
		}
		columns[name] = true
	}

	for _, name := range []string{"last_accessed_at", "importance_score", "pinned", "memory_tier"} {
		if !columns[name] {
			t.Fatalf("expected migrated column %q to exist", name)
		}
	}

	var lastAccessedRaw string
	var importance float64
	var pinned int
	var tier string
	if err := db.QueryRow(`
		SELECT COALESCE(last_accessed_at, created_at), COALESCE(importance_score, 0), COALESCE(pinned, 0), COALESCE(memory_tier, '')
		FROM memories
		WHERE user_id = ?`,
		"legacy_user",
	).Scan(&lastAccessedRaw, &importance, &pinned, &tier); err != nil {
		t.Fatalf("failed to read migrated legacy memory: %v", err)
	}
	if parseSQLiteTime(lastAccessedRaw, time.Time{}).IsZero() {
		t.Fatalf("expected last_accessed_at to be backfilled")
	}
	if importance <= 0 {
		t.Fatalf("expected importance to be backfilled, got %v", importance)
	}
	if tier == "" {
		t.Fatalf("expected memory tier to be backfilled")
	}
	if pinned != 0 && pinned != 1 {
		t.Fatalf("unexpected pinned value %d", pinned)
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

func TestChatSessionHelpers(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "chat_session_helpers_*.db")
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

	saved, err := UpsertChatSession(ChatSessionEntry{
		UserID:           "chat_user",
		SessionKey:       "default",
		Status:           "running",
		LLMMode:          "stateful",
		ModelID:          "test-model",
		LastResponseID:   "resp_123",
		SummaryText:      "summary text",
		TurnCount:        2,
		EstimatedChars:   120,
		LastInputTokens:  55,
		LastOutputTokens: 34,
		PeakInputTokens:  60,
		TokenBudget:      1000,
		RiskScore:        0.42,
		RiskLevel:        "medium",
		LastResetReason:  "manual",
		UIStateJSON:      `{"tool_cards":{"turn-1":{"state":"success","tool_name":"get_current_time","history":[{"tool":"Get Current Time","detail":"checked"}]}}}`,
	})
	if err != nil {
		t.Fatalf("UpsertChatSession failed: %v", err)
	}
	if saved.ID == 0 {
		t.Fatalf("expected saved chat session to have id")
	}

	current, err := GetCurrentChatSession("chat_user")
	if err != nil {
		t.Fatalf("GetCurrentChatSession failed: %v", err)
	}
	if current.ModelID != "test-model" || current.Status != "running" {
		t.Fatalf("unexpected chat session state: %+v", current)
	}
	if !strings.Contains(current.UIStateJSON, `"tool_cards"`) {
		t.Fatalf("expected ui state json to round-trip, got %q", current.UIStateJSON)
	}

	event1, err := AppendChatEvent("chat_user", current.ID, "user", "message.created", "msg-user-1", "turn-1", `{"content":"hello"}`)
	if err != nil {
		t.Fatalf("AppendChatEvent user failed: %v", err)
	}
	event2, err := AppendChatEvent("chat_user", current.ID, "assistant", "message.delta", "msg-ai-1", "turn-1", `{"content":"hi"}`)
	if err != nil {
		t.Fatalf("AppendChatEvent assistant failed: %v", err)
	}

	if event1.EventSeq != 1 || event2.EventSeq != 2 {
		t.Fatalf("expected sequential event ids, got %d and %d", event1.EventSeq, event2.EventSeq)
	}

	events, err := ListChatEvents("chat_user", current.ID, 0, 10)
	if err != nil {
		t.Fatalf("ListChatEvents failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 chat events, got %d", len(events))
	}
	if events[0].EventType != "message.created" || events[1].EventType != "message.delta" {
		t.Fatalf("unexpected chat events: %+v", events)
	}
}

func TestInitDBDropsDeprecatedLastSessionsTable(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "legacy_last_sessions_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	dbPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(dbPath)

	legacyDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open legacy db: %v", err)
	}
	if _, err := legacyDB.Exec(`
		CREATE TABLE last_sessions (
			user_id TEXT PRIMARY KEY,
			last_user_message TEXT NOT NULL,
			last_assistant_message TEXT NOT NULL,
			mode TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		legacyDB.Close()
		t.Fatalf("failed to create legacy last_sessions table: %v", err)
	}
	legacyDB.Close()

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer CloseDB()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'last_sessions'`).Scan(&count); err != nil {
		t.Fatalf("failed to inspect sqlite_master for last_sessions: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected last_sessions table to be dropped, got count=%d", count)
	}
}

func nowForTest() time.Time {
	return time.Now().UTC()
}
