// Created by DINKIssTyle on 2026. Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

package mcp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var (
	// db holds the global database connection pool.
	db                           *sql.DB
	memoryRetentionSchemaMu      sync.Mutex
	memoryRetentionSchemaChecked bool
)

const (
	memoryChunkSize    = 800
	memoryChunkOverlap = 120
)

const (
	memoryTierEphemeral = "ephemeral"
	memoryTierWorking   = "working"
	memoryTierCore      = "core"
)

const (
	retentionChatEventsDays        = 14
	retentionRequestExecutionsDays = 21
	retentionBackgroundJobsDays    = 7
	retentionEphemeralMemoryDays   = 14
	retentionWorkingMemoryDays     = 45
	retentionCoreMemoryDays        = 180
)

// InitDB initializes the SQLite database connection and creates necessary tables.
func InitDB(dbPath string) error {
	var err error

	// Ensure the directory exists
	if err = os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	log.Printf("[DB] Connecting to SQLite database at: %s", dbPath)
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // Keep it single connection to avoid SQLite busy locks
	db.SetMaxIdleConns(1)

	// Test connection
	if err = db.Ping(); err != nil {
		return fmt.Errorf("database unreachable: %w", err)
	}
	if _, err = db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	memoryRetentionSchemaMu.Lock()
	memoryRetentionSchemaChecked = false
	memoryRetentionSchemaMu.Unlock()

	// Initialize schema
	return createSchema()
}

// CloseDB closes the database connection.
func CloseDB() {
	if db != nil {
		log.Println("[DB] Closing SQLite database.")
		_ = db.Close()
	}
	memoryRetentionSchemaMu.Lock()
	memoryRetentionSchemaChecked = false
	memoryRetentionSchemaMu.Unlock()
}

// createSchema creates the memories table if it doesn't exist.
func createSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS memories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		full_text TEXT NOT NULL,
		hit_count INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		memory_type TEXT DEFAULT 'raw_interaction'
	);
	
	CREATE INDEX IF NOT EXISTS idx_memories_user_id ON memories(user_id);
	CREATE INDEX IF NOT EXISTS idx_memories_user_type ON memories(user_id, memory_type);

	CREATE TABLE IF NOT EXISTS memory_chunks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		memory_id INTEGER NOT NULL,
		user_id TEXT NOT NULL,
		chunk_index INTEGER NOT NULL,
		chunk_text TEXT NOT NULL,
		hit_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(memory_id) REFERENCES memories(id)
	);

	CREATE INDEX IF NOT EXISTS idx_memory_chunks_memory_id ON memory_chunks(memory_id, chunk_index);
	CREATE INDEX IF NOT EXISTS idx_memory_chunks_user_id ON memory_chunks(user_id, created_at DESC);

	CREATE VIRTUAL TABLE IF NOT EXISTS memory_chunks_fts
	USING fts5(
		chunk_text,
		memory_id UNINDEXED,
		user_id UNINDEXED,
		chunk_index UNINDEXED,
		tokenize = 'unicode61'
	);

	CREATE TABLE IF NOT EXISTS memory_chunk_embeddings (
		chunk_id INTEGER PRIMARY KEY,
		embedding_model TEXT NOT NULL DEFAULT '',
		embedding_dim INTEGER NOT NULL DEFAULT 0,
		embedding_blob BLOB,
		embedding_json TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(chunk_id) REFERENCES memory_chunks(id)
	);

	CREATE TABLE IF NOT EXISTS web_sources (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_id TEXT NOT NULL UNIQUE,
		user_id TEXT NOT NULL,
		tool_name TEXT NOT NULL DEFAULT '',
		query_text TEXT NOT NULL DEFAULT '',
		url TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL DEFAULT '',
		summary TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '',
		fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_web_sources_user_recent
	ON web_sources(user_id, last_used_at DESC, fetched_at DESC, id DESC);

	CREATE INDEX IF NOT EXISTS idx_web_sources_user_source
	ON web_sources(user_id, source_id);

	CREATE TABLE IF NOT EXISTS web_source_chunks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		chunk_index INTEGER NOT NULL,
		chunk_text TEXT NOT NULL,
		token_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(source_id) REFERENCES web_sources(source_id)
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_web_source_chunks_source_chunk
	ON web_source_chunks(source_id, chunk_index);

	CREATE INDEX IF NOT EXISTS idx_web_source_chunks_user_source
	ON web_source_chunks(user_id, source_id, chunk_index);

	CREATE VIRTUAL TABLE IF NOT EXISTS web_source_chunks_fts
	USING fts5(
		chunk_text,
		source_id UNINDEXED,
		user_id UNINDEXED,
		chunk_index UNINDEXED,
		tokenize = 'unicode61'
	);

	CREATE TABLE IF NOT EXISTS web_chunk_embeddings (
		chunk_id INTEGER PRIMARY KEY,
		embedding_model TEXT NOT NULL DEFAULT '',
		embedding_dim INTEGER NOT NULL DEFAULT 0,
		embedding_blob BLOB,
		embedding_json TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(chunk_id) REFERENCES web_source_chunks(id)
	);

	CREATE TABLE IF NOT EXISTS auth_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		token_hash TEXT NOT NULL UNIQUE,
		remember_me INTEGER NOT NULL DEFAULT 0,
		user_agent TEXT NOT NULL DEFAULT '',
		client_addr TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_id ON auth_sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires_at ON auth_sessions(expires_at);

	CREATE TABLE IF NOT EXISTS last_sessions (
		user_id TEXT PRIMARY KEY,
		last_user_message TEXT NOT NULL,
		last_assistant_message TEXT NOT NULL,
		mode TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS chat_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		session_key TEXT NOT NULL DEFAULT 'default',
		status TEXT NOT NULL DEFAULT 'idle',
		llm_mode TEXT NOT NULL DEFAULT 'standard',
		model_id TEXT NOT NULL DEFAULT '',
		current_job_id INTEGER,
		last_response_id TEXT NOT NULL DEFAULT '',
		summary_text TEXT NOT NULL DEFAULT '',
		turn_count INTEGER NOT NULL DEFAULT 0,
		estimated_chars INTEGER NOT NULL DEFAULT 0,
		last_input_tokens INTEGER NOT NULL DEFAULT 0,
		last_output_tokens INTEGER NOT NULL DEFAULT 0,
		peak_input_tokens INTEGER NOT NULL DEFAULT 0,
		token_budget INTEGER NOT NULL DEFAULT 0,
		risk_score REAL NOT NULL DEFAULT 0,
		risk_level TEXT NOT NULL DEFAULT 'low',
		last_reset_reason TEXT NOT NULL DEFAULT '',
		ui_state_json TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		cleared_at DATETIME
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_sessions_user_key
	ON chat_sessions(user_id, session_key);

	CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_updated
	ON chat_sessions(user_id, updated_at DESC);

	CREATE INDEX IF NOT EXISTS idx_chat_sessions_current_job
	ON chat_sessions(current_job_id);

	CREATE TABLE IF NOT EXISTS chat_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		user_id TEXT NOT NULL,
		event_seq INTEGER NOT NULL,
		role TEXT NOT NULL DEFAULT '',
		event_type TEXT NOT NULL,
		message_id TEXT NOT NULL DEFAULT '',
		turn_id TEXT NOT NULL DEFAULT '',
		payload_json TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(session_id) REFERENCES chat_sessions(id)
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_events_session_seq
	ON chat_events(session_id, event_seq);

	CREATE INDEX IF NOT EXISTS idx_chat_events_user_created
	ON chat_events(user_id, created_at DESC);

	CREATE INDEX IF NOT EXISTS idx_chat_events_session_created
	ON chat_events(session_id, created_at ASC);

	CREATE TABLE IF NOT EXISTS saved_turns (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		title TEXT NOT NULL,
		title_source TEXT NOT NULL DEFAULT 'fallback',
		auto_title_failures INTEGER NOT NULL DEFAULT 0,
		prompt_text TEXT NOT NULL,
		response_text TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_saved_turns_user_created
	ON saved_turns(user_id, created_at DESC);

	CREATE INDEX IF NOT EXISTS idx_saved_turns_user_title_source
	ON saved_turns(user_id, title_source, created_at DESC);

	CREATE TABLE IF NOT EXISTS request_patterns (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		intent_key TEXT NOT NULL,
		sample_query TEXT NOT NULL,
		query_fingerprint TEXT NOT NULL,
		hit_count INTEGER NOT NULL DEFAULT 1,
		first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_request_patterns_user_intent
	ON request_patterns(user_id, intent_key);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_request_patterns_user_fingerprint
	ON request_patterns(user_id, query_fingerprint);

	CREATE TABLE IF NOT EXISTS request_executions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		intent_key TEXT NOT NULL,
		request_pattern_id INTEGER,
		raw_query TEXT NOT NULL,
		normalized_query TEXT NOT NULL,
		tool_chain_json TEXT NOT NULL,
		tool_count INTEGER NOT NULL DEFAULT 0,
		total_latency_ms INTEGER NOT NULL DEFAULT 0,
		tool_latency_ms INTEGER NOT NULL DEFAULT 0,
		success INTEGER NOT NULL DEFAULT 0,
		fallback_used INTEGER NOT NULL DEFAULT 0,
		repeated_tool_blocked INTEGER NOT NULL DEFAULT 0,
		self_correction_used INTEGER NOT NULL DEFAULT 0,
		followup_within_2m INTEGER NOT NULL DEFAULT 0,
		user_feedback_score REAL NOT NULL DEFAULT 0,
		recipe_version TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(request_pattern_id) REFERENCES request_patterns(id)
	);

	CREATE INDEX IF NOT EXISTS idx_request_executions_user_intent
	ON request_executions(user_id, intent_key, created_at DESC);

	CREATE TABLE IF NOT EXISTS request_intent_stats (
		user_id TEXT NOT NULL,
		intent_key TEXT NOT NULL,
		total_count INTEGER NOT NULL DEFAULT 0,
		success_count INTEGER NOT NULL DEFAULT 0,
		avg_latency_ms REAL NOT NULL DEFAULT 0,
		last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, intent_key)
	);

	CREATE TABLE IF NOT EXISTS background_chat_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		job_type TEXT NOT NULL DEFAULT 'chat',
		status TEXT NOT NULL DEFAULT 'queued',
		llm_mode TEXT NOT NULL DEFAULT 'standard',
		model_id TEXT NOT NULL DEFAULT '',
		request_payload_json TEXT NOT NULL DEFAULT '{}',
		stream_state_json TEXT NOT NULL DEFAULT '{}',
		partial_text TEXT NOT NULL DEFAULT '',
		final_text TEXT NOT NULL DEFAULT '',
		error_text TEXT NOT NULL DEFAULT '',
		timeout_seconds INTEGER NOT NULL DEFAULT 300,
		started_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_background_chat_jobs_user_status
	ON background_chat_jobs(user_id, status, updated_at DESC);

	CREATE INDEX IF NOT EXISTS idx_background_chat_jobs_status_updated
	ON background_chat_jobs(status, updated_at DESC);

	CREATE TABLE IF NOT EXISTS procedure_recipes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		intent_key TEXT NOT NULL,
		recipe_name TEXT NOT NULL,
		trigger_hint TEXT NOT NULL DEFAULT '',
		tool_chain_template_json TEXT NOT NULL,
		preconditions_json TEXT NOT NULL DEFAULT '{}',
		avg_latency_ms REAL NOT NULL DEFAULT 0,
		success_rate REAL NOT NULL DEFAULT 0,
		quality_score REAL NOT NULL DEFAULT 0,
		final_score REAL NOT NULL DEFAULT 0,
		usage_count INTEGER NOT NULL DEFAULT 0,
		last_used_at DATETIME,
		active INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_procedure_recipes_user_intent
	ON procedure_recipes(user_id, intent_key, active, final_score DESC);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_procedure_recipes_user_intent_name
	ON procedure_recipes(user_id, intent_key, recipe_name);
	`
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	if err := migrateMemoriesSchema(); err != nil {
		return err
	}
	if err := migrateMemoryRetentionSchema(); err != nil {
		return err
	}
	if err := migrateChatSessionsSchema(); err != nil {
		return err
	}
	if err := migrateSavedTurnsSchema(); err != nil {
		return err
	}
	if err := runRetentionMaintenance(time.Now().UTC()); err != nil {
		log.Printf("[DB] retention maintenance warning: %v", err)
	}

	log.Println("[DB] Schema initialized successfully.")
	return nil
}

func migrateSavedTurnsSchema() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	rows, err := db.Query(`PRAGMA table_info(saved_turns)`)
	if err != nil {
		return fmt.Errorf("failed to inspect saved_turns schema: %w", err)
	}
	defer rows.Close()

	hasAutoTitleFailures := false
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return fmt.Errorf("failed to scan saved_turns schema: %w", err)
		}
		if name == "auto_title_failures" {
			hasAutoTitleFailures = true
		}
	}

	if hasAutoTitleFailures {
		return nil
	}

	if _, err := db.Exec(`ALTER TABLE saved_turns ADD COLUMN auto_title_failures INTEGER NOT NULL DEFAULT 0`); err != nil {
		return fmt.Errorf("failed to add auto_title_failures to saved_turns: %w", err)
	}
	if _, err := db.Exec(`UPDATE saved_turns SET auto_title_failures = 0 WHERE auto_title_failures IS NULL`); err != nil {
		return fmt.Errorf("failed to backfill auto_title_failures: %w", err)
	}
	return nil
}

func migrateChatSessionsSchema() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	rows, err := db.Query(`PRAGMA table_info(chat_sessions)`)
	if err != nil {
		return fmt.Errorf("failed to inspect chat_sessions schema: %w", err)
	}
	defer rows.Close()

	hasUIStateJSON := false
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return fmt.Errorf("failed to scan chat_sessions schema: %w", err)
		}
		if name == "ui_state_json" {
			hasUIStateJSON = true
		}
	}

	if hasUIStateJSON {
		return nil
	}

	if _, err := db.Exec(`ALTER TABLE chat_sessions ADD COLUMN ui_state_json TEXT NOT NULL DEFAULT '{}'`); err != nil {
		return fmt.Errorf("failed to add ui_state_json to chat_sessions: %w", err)
	}
	if _, err := db.Exec(`UPDATE chat_sessions SET ui_state_json = '{}' WHERE TRIM(COALESCE(ui_state_json, '')) = ''`); err != nil {
		return fmt.Errorf("failed to backfill ui_state_json: %w", err)
	}
	return nil
}

func migrateMemoriesSchema() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	rows, err := db.Query(`PRAGMA table_info(memories)`)
	if err != nil {
		return fmt.Errorf("failed to inspect memories schema: %w", err)
	}
	defer rows.Close()

	hasSummary := false
	hasKeywords := false
	hasMemoryType := false

	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return fmt.Errorf("failed to scan memories schema: %w", err)
		}
		switch name {
		case "summary":
			hasSummary = true
		case "keywords":
			hasKeywords = true
		case "memory_type":
			hasMemoryType = true
		}
	}

	if !hasSummary && !hasKeywords && hasMemoryType {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start memories migration: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS memories_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			full_text TEXT NOT NULL,
			hit_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			memory_type TEXT DEFAULT 'raw_interaction'
		)`); err != nil {
		return fmt.Errorf("failed to create migrated memories table: %w", err)
	}

	if hasMemoryType {
		_, err = tx.Exec(`
			INSERT INTO memories_new (id, user_id, full_text, hit_count, created_at, memory_type)
			SELECT id, user_id, full_text, COALESCE(hit_count, 0), created_at, COALESCE(memory_type, 'raw_interaction')
			FROM memories`)
	} else {
		_, err = tx.Exec(`
			INSERT INTO memories_new (id, user_id, full_text, hit_count, created_at, memory_type)
			SELECT id, user_id, full_text, COALESCE(hit_count, 0), created_at, 'raw_interaction'
			FROM memories`)
	}
	if err != nil {
		return fmt.Errorf("failed to copy memories into migrated table: %w", err)
	}

	if _, err = tx.Exec(`DROP TABLE memories`); err != nil {
		return fmt.Errorf("failed to drop old memories table: %w", err)
	}
	if _, err = tx.Exec(`ALTER TABLE memories_new RENAME TO memories`); err != nil {
		return fmt.Errorf("failed to rename migrated memories table: %w", err)
	}
	if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_memories_user_id ON memories(user_id)`); err != nil {
		return fmt.Errorf("failed to recreate idx_memories_user_id: %w", err)
	}
	if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_memories_user_type ON memories(user_id, memory_type)`); err != nil {
		return fmt.Errorf("failed to recreate idx_memories_user_type: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit memories migration: %w", err)
	}
	return nil
}

func migrateMemoryRetentionSchema() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	rows, err := db.Query(`PRAGMA table_info(memories)`)
	if err != nil {
		return fmt.Errorf("failed to inspect memories retention schema: %w", err)
	}
	defer rows.Close()

	hasLastAccessed := false
	hasImportance := false
	hasPinned := false
	hasTier := false

	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return fmt.Errorf("failed to scan memories retention schema: %w", err)
		}
		switch name {
		case "last_accessed_at":
			hasLastAccessed = true
		case "importance_score":
			hasImportance = true
		case "pinned":
			hasPinned = true
		case "memory_tier":
			hasTier = true
		}
	}

	if !hasLastAccessed {
		if _, err := db.Exec(`ALTER TABLE memories ADD COLUMN last_accessed_at DATETIME`); err != nil {
			return fmt.Errorf("failed to add last_accessed_at to memories: %w", err)
		}
	}
	if !hasImportance {
		if _, err := db.Exec(`ALTER TABLE memories ADD COLUMN importance_score REAL`); err != nil {
			return fmt.Errorf("failed to add importance_score to memories: %w", err)
		}
	}
	if !hasPinned {
		if _, err := db.Exec(`ALTER TABLE memories ADD COLUMN pinned INTEGER`); err != nil {
			return fmt.Errorf("failed to add pinned to memories: %w", err)
		}
	}
	if !hasTier {
		if _, err := db.Exec(`ALTER TABLE memories ADD COLUMN memory_tier TEXT`); err != nil {
			return fmt.Errorf("failed to add memory_tier to memories: %w", err)
		}
	}

	rows, err = db.Query(`
		SELECT id, full_text, memory_type, created_at
		FROM memories`)
	if err != nil {
		return fmt.Errorf("failed to list memories for retention backfill: %w", err)
	}
	defer rows.Close()

	type backfillRow struct {
		ID         int64
		FullText   string
		MemoryType string
		CreatedAt  time.Time
	}
	var items []backfillRow
	for rows.Next() {
		var item backfillRow
		if err := rows.Scan(&item.ID, &item.FullText, &item.MemoryType, &item.CreatedAt); err != nil {
			return fmt.Errorf("failed to scan memory retention backfill row: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range items {
		tier, importance, pinned := classifyMemoryRetention(item.FullText, item.MemoryType)
		if _, err := db.Exec(`
			UPDATE memories
			SET last_accessed_at = COALESCE(last_accessed_at, created_at, ?),
			    importance_score = ?,
			    pinned = CASE WHEN pinned != 0 THEN pinned ELSE ? END,
			    memory_tier = ?
			WHERE id = ?`,
			item.CreatedAt.UTC(), importance, boolToInt(pinned), tier, item.ID,
		); err != nil {
			return fmt.Errorf("failed to backfill memory retention fields: %w", err)
		}
	}

	return nil
}

func ensureMemoryRetentionSchema() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	memoryRetentionSchemaMu.Lock()
	defer memoryRetentionSchemaMu.Unlock()

	if memoryRetentionSchemaChecked {
		return nil
	}
	if err := migrateMemoryRetentionSchema(); err != nil {
		return err
	}
	memoryRetentionSchemaChecked = true
	return nil
}

func classifyMemoryRetention(fullText, memoryType string) (tier string, importance float64, pinned bool) {
	text := strings.ToLower(strings.TrimSpace(fullText))
	memType := strings.ToLower(strings.TrimSpace(memoryType))
	if strings.Contains(text, "내 이름") || strings.Contains(text, "제 이름") || strings.Contains(text, "my name") || strings.Contains(text, "call me") {
		return memoryTierCore, 0.95, false
	}
	if strings.Contains(text, "prefer") || strings.Contains(text, "preference") || strings.Contains(text, "선호") || strings.Contains(text, "좋아") || strings.Contains(text, "싫어") {
		return memoryTierCore, 0.85, false
	}
	if strings.Contains(text, "project") || strings.Contains(text, "repository") || strings.Contains(text, "repo") || strings.Contains(text, "프로젝트") || strings.Contains(text, "github") {
		return memoryTierWorking, 0.65, false
	}
	if memType == "raw_interaction" {
		return memoryTierEphemeral, 0.25, false
	}
	return memoryTierWorking, 0.45, false
}

func runRetentionMaintenance(now time.Time) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := pruneExpiredAuthSessions(now); err != nil {
		return err
	}
	if err := pruneOldBackgroundJobs(now); err != nil {
		return err
	}
	if err := pruneOldRequestExecutions(now); err != nil {
		return err
	}
	if err := pruneOldChatEvents(now); err != nil {
		return err
	}
	if err := pruneAgedMemories(now); err != nil {
		return err
	}
	return nil
}

func pruneExpiredAuthSessions(now time.Time) error {
	_, err := db.Exec(`DELETE FROM auth_sessions WHERE expires_at <= ?`, now.UTC())
	if err != nil {
		return fmt.Errorf("failed to prune expired auth sessions: %w", err)
	}
	return nil
}

func pruneOldBackgroundJobs(now time.Time) error {
	cutoff := now.AddDate(0, 0, -retentionBackgroundJobsDays).UTC()
	_, err := db.Exec(`
		DELETE FROM background_chat_jobs
		WHERE status IN ('completed', 'failed', 'cancelled')
		  AND updated_at < ?`, cutoff)
	if err != nil {
		return fmt.Errorf("failed to prune old background jobs: %w", err)
	}
	return nil
}

func pruneOldRequestExecutions(now time.Time) error {
	cutoff := now.AddDate(0, 0, -retentionRequestExecutionsDays).UTC()
	_, err := db.Exec(`DELETE FROM request_executions WHERE created_at < ?`, cutoff)
	if err != nil {
		return fmt.Errorf("failed to prune old request executions: %w", err)
	}
	return nil
}

func pruneOldChatEvents(now time.Time) error {
	cutoff := now.AddDate(0, 0, -retentionChatEventsDays).UTC()
	_, err := db.Exec(`DELETE FROM chat_events WHERE created_at < ?`, cutoff)
	if err != nil {
		return fmt.Errorf("failed to prune old chat events: %w", err)
	}
	return nil
}

func pruneAgedMemories(now time.Time) error {
	rows, err := db.Query(`
		SELECT id, user_id, full_text, hit_count, created_at, memory_type,
		       COALESCE(last_accessed_at, created_at),
		       COALESCE(importance_score, 0.25),
		       COALESCE(pinned, 0),
		       COALESCE(memory_tier, 'ephemeral')
		FROM memories`)
	if err != nil {
		return fmt.Errorf("failed to query memories for retention pruning: %w", err)
	}
	defer rows.Close()

	var deleteIDs []struct {
		ID     int64
		UserID string
	}
	for rows.Next() {
		var memory MemoryEntry
		var pinned int
		var lastAccessedRaw string
		if err := rows.Scan(
			&memory.ID,
			&memory.UserID,
			&memory.FullText,
			&memory.HitCount,
			&memory.CreatedAt,
			&memory.MemoryType,
			&lastAccessedRaw,
			&memory.ImportanceScore,
			&pinned,
			&memory.MemoryTier,
		); err != nil {
			return fmt.Errorf("failed to scan memory pruning row: %w", err)
		}
		memory.LastAccessedAt = parseSQLiteTime(lastAccessedRaw, memory.CreatedAt)
		memory.Pinned = pinned != 0
		if shouldForgetMemory(memory, now) {
			deleteIDs = append(deleteIDs, struct {
				ID     int64
				UserID string
			}{ID: memory.ID, UserID: memory.UserID})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range deleteIDs {
		if err := DeleteMemory(item.UserID, item.ID); err != nil {
			return err
		}
	}
	return nil
}

func shouldForgetMemory(memory MemoryEntry, now time.Time) bool {
	if memory.Pinned {
		return false
	}
	lastTouched := memory.LastAccessedAt
	if lastTouched.IsZero() {
		lastTouched = memory.CreatedAt
	}
	ageDays := now.Sub(lastTouched).Hours() / 24
	retentionScore := memory.ImportanceScore + math.Min(float64(memory.HitCount), 8)*0.08

	switch memory.MemoryTier {
	case memoryTierCore:
		return ageDays > retentionCoreMemoryDays && retentionScore < 0.75
	case memoryTierWorking:
		return ageDays > retentionWorkingMemoryDays && retentionScore < 0.65
	default:
		return ageDays > retentionEphemeralMemoryDays && retentionScore < 0.55
	}
}

func parseSQLiteTime(raw string, fallback time.Time) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return fallback
}

type AuthSessionEntry struct {
	ID         int64
	UserID     string
	TokenHash  string
	RememberMe bool
	UserAgent  string
	ClientAddr string
	CreatedAt  time.Time
	LastUsedAt time.Time
	ExpiresAt  time.Time
}

type LastSessionEntry struct {
	UserID               string
	LastUserMessage      string
	LastAssistantMessage string
	Mode                 string
	UpdatedAt            time.Time
}

type ChatSessionEntry struct {
	ID               int64
	UserID           string
	SessionKey       string
	Status           string
	LLMMode          string
	ModelID          string
	CurrentJobID     sql.NullInt64
	LastResponseID   string
	SummaryText      string
	TurnCount        int
	EstimatedChars   int
	LastInputTokens  int
	LastOutputTokens int
	PeakInputTokens  int
	TokenBudget      int
	RiskScore        float64
	RiskLevel        string
	LastResetReason  string
	UIStateJSON      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ClearedAt        sql.NullTime
}

type ChatEventEntry struct {
	ID          int64
	SessionID   int64
	UserID      string
	EventSeq    int
	Role        string
	EventType   string
	MessageID   string
	TurnID      string
	PayloadJSON string
	CreatedAt   time.Time
}

type SavedTurnEntry struct {
	ID                int64     `json:"id"`
	UserID            string    `json:"user_id"`
	Title             string    `json:"title"`
	TitleSource       string    `json:"title_source"`
	AutoTitleFailures int       `json:"auto_title_failures"`
	Processing        bool      `json:"processing,omitempty"`
	PromptText        string    `json:"prompt_text"`
	ResponseText      string    `json:"response_text"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type RequestExecutionEntry struct {
	UserID               string
	IntentKey            string
	RequestPatternID     sql.NullInt64
	RawQuery             string
	NormalizedQuery      string
	ToolChainJSON        string
	ToolCount            int
	TotalLatencyMS       int64
	ToolLatencyMS        int64
	Success              bool
	FallbackUsed         bool
	RepeatedToolBlocked  bool
	SelfCorrectionUsed   bool
	FollowupWithinTwoMin bool
	UserFeedbackScore    float64
	RecipeVersion        string
	CreatedAt            time.Time
}

type ProcedureRecipeEntry struct {
	ID                    int64
	UserID                string
	IntentKey             string
	RecipeName            string
	TriggerHint           string
	ToolChainTemplateJSON string
	PreconditionsJSON     string
	AvgLatencyMS          float64
	SuccessRate           float64
	QualityScore          float64
	FinalScore            float64
	UsageCount            int
	LastUsedAt            sql.NullTime
	Active                bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func InsertAuthSession(userID, tokenHash string, rememberMe bool, userAgent, clientAddr string, expiresAt time.Time) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
	INSERT INTO auth_sessions (user_id, token_hash, remember_me, user_agent, client_addr, expires_at)
	VALUES (?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query, userID, tokenHash, boolToInt(rememberMe), userAgent, clientAddr, expiresAt.UTC())
	if err != nil {
		return fmt.Errorf("failed to insert auth session: %w", err)
	}
	return nil
}

func GetAuthSessionByTokenHash(tokenHash string) (AuthSessionEntry, error) {
	var s AuthSessionEntry
	if db == nil {
		return s, fmt.Errorf("database not initialized")
	}

	query := `
	SELECT id, user_id, token_hash, remember_me, user_agent, client_addr, created_at, last_used_at, expires_at
	FROM auth_sessions
	WHERE token_hash = ?`

	var rememberInt int
	err := db.QueryRow(query, tokenHash).Scan(
		&s.ID,
		&s.UserID,
		&s.TokenHash,
		&rememberInt,
		&s.UserAgent,
		&s.ClientAddr,
		&s.CreatedAt,
		&s.LastUsedAt,
		&s.ExpiresAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return s, err
		}
		return s, fmt.Errorf("failed to fetch auth session: %w", err)
	}

	s.RememberMe = rememberInt != 0
	return s, nil
}

func TouchAuthSession(tokenHash string, usedAt time.Time) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := db.Exec(`UPDATE auth_sessions SET last_used_at = ? WHERE token_hash = ?`, usedAt.UTC(), tokenHash)
	if err != nil {
		return fmt.Errorf("failed to update auth session: %w", err)
	}
	return nil
}

func DeleteAuthSession(tokenHash string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := db.Exec(`DELETE FROM auth_sessions WHERE token_hash = ?`, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to delete auth session: %w", err)
	}
	return nil
}

func DeleteAuthSessionsByUser(userID string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := db.Exec(`DELETE FROM auth_sessions WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user auth sessions: %w", err)
	}
	return nil
}

func PurgeExpiredAuthSessions(now time.Time) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := db.Exec(`DELETE FROM auth_sessions WHERE expires_at <= ?`, now.UTC())
	if err != nil {
		return fmt.Errorf("failed to purge expired auth sessions: %w", err)
	}
	return nil
}

func UpsertLastSession(userID, userMessage, assistantMessage, mode string, updatedAt time.Time) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}

	query := `
	INSERT INTO last_sessions (user_id, last_user_message, last_assistant_message, mode, updated_at)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(user_id) DO UPDATE SET
		last_user_message = excluded.last_user_message,
		last_assistant_message = excluded.last_assistant_message,
		mode = excluded.mode,
		updated_at = excluded.updated_at`

	_, err := db.Exec(query, userID, userMessage, assistantMessage, mode, updatedAt.UTC())
	if err != nil {
		return fmt.Errorf("failed to upsert last session: %w", err)
	}
	return nil
}

func GetLastSession(userID string) (LastSessionEntry, error) {
	var entry LastSessionEntry
	if db == nil {
		return entry, fmt.Errorf("database not initialized")
	}

	query := `
	SELECT user_id, last_user_message, last_assistant_message, mode, updated_at
	FROM last_sessions
	WHERE user_id = ?`

	err := db.QueryRow(query, userID).Scan(
		&entry.UserID,
		&entry.LastUserMessage,
		&entry.LastAssistantMessage,
		&entry.Mode,
		&entry.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return entry, err
		}
		return entry, fmt.Errorf("failed to fetch last session: %w", err)
	}

	return entry, nil
}

func DeleteLastSession(userID string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := db.Exec(`DELETE FROM last_sessions WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete last session: %w", err)
	}
	return nil
}

func UpsertChatSession(entry ChatSessionEntry) (ChatSessionEntry, error) {
	var saved ChatSessionEntry
	if db == nil {
		return saved, fmt.Errorf("database not initialized")
	}

	entry.UserID = strings.TrimSpace(entry.UserID)
	entry.SessionKey = strings.TrimSpace(entry.SessionKey)
	entry.Status = strings.TrimSpace(entry.Status)
	entry.LLMMode = strings.TrimSpace(entry.LLMMode)
	entry.ModelID = strings.TrimSpace(entry.ModelID)
	entry.LastResponseID = strings.TrimSpace(entry.LastResponseID)
	entry.RiskLevel = strings.TrimSpace(entry.RiskLevel)
	entry.LastResetReason = strings.TrimSpace(entry.LastResetReason)
	entry.UIStateJSON = strings.TrimSpace(entry.UIStateJSON)
	if entry.UserID == "" {
		return saved, fmt.Errorf("user id is required")
	}
	if entry.SessionKey == "" {
		entry.SessionKey = "default"
	}
	if entry.Status == "" {
		entry.Status = "idle"
	}
	if entry.LLMMode == "" {
		entry.LLMMode = "standard"
	}
	if entry.RiskLevel == "" {
		entry.RiskLevel = "low"
	}
	if entry.UIStateJSON == "" {
		entry.UIStateJSON = "{}"
	}

	query := `
	INSERT INTO chat_sessions (
		user_id, session_key, status, llm_mode, model_id, current_job_id,
		last_response_id, summary_text, turn_count, estimated_chars,
		last_input_tokens, last_output_tokens, peak_input_tokens, token_budget,
		risk_score, risk_level, last_reset_reason, ui_state_json, updated_at, cleared_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id, session_key) DO UPDATE SET
		status = excluded.status,
		llm_mode = excluded.llm_mode,
		model_id = excluded.model_id,
		current_job_id = excluded.current_job_id,
		last_response_id = excluded.last_response_id,
		summary_text = excluded.summary_text,
		turn_count = excluded.turn_count,
		estimated_chars = excluded.estimated_chars,
		last_input_tokens = excluded.last_input_tokens,
		last_output_tokens = excluded.last_output_tokens,
		peak_input_tokens = excluded.peak_input_tokens,
		token_budget = excluded.token_budget,
		risk_score = excluded.risk_score,
		risk_level = excluded.risk_level,
		last_reset_reason = excluded.last_reset_reason,
		ui_state_json = excluded.ui_state_json,
		updated_at = excluded.updated_at,
		cleared_at = excluded.cleared_at`

	updatedAt := time.Now().UTC()
	_, err := db.Exec(
		query,
		entry.UserID,
		entry.SessionKey,
		entry.Status,
		entry.LLMMode,
		entry.ModelID,
		nullInt64Value(entry.CurrentJobID),
		entry.LastResponseID,
		entry.SummaryText,
		entry.TurnCount,
		entry.EstimatedChars,
		entry.LastInputTokens,
		entry.LastOutputTokens,
		entry.PeakInputTokens,
		entry.TokenBudget,
		entry.RiskScore,
		entry.RiskLevel,
		entry.LastResetReason,
		entry.UIStateJSON,
		updatedAt,
		nullTimeValue(entry.ClearedAt),
	)
	if err != nil {
		return saved, fmt.Errorf("failed to upsert chat session: %w", err)
	}
	return GetChatSession(entry.UserID, entry.SessionKey)
}

func GetChatSession(userID, sessionKey string) (ChatSessionEntry, error) {
	var entry ChatSessionEntry
	if db == nil {
		return entry, fmt.Errorf("database not initialized")
	}

	userID = strings.TrimSpace(userID)
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		sessionKey = "default"
	}

	query := `
	SELECT id, user_id, session_key, status, llm_mode, model_id, current_job_id,
		last_response_id, summary_text, turn_count, estimated_chars,
		last_input_tokens, last_output_tokens, peak_input_tokens, token_budget,
		risk_score, risk_level, last_reset_reason, ui_state_json, created_at, updated_at, cleared_at
	FROM chat_sessions
	WHERE user_id = ? AND session_key = ?`

	err := db.QueryRow(query, userID, sessionKey).Scan(
		&entry.ID,
		&entry.UserID,
		&entry.SessionKey,
		&entry.Status,
		&entry.LLMMode,
		&entry.ModelID,
		&entry.CurrentJobID,
		&entry.LastResponseID,
		&entry.SummaryText,
		&entry.TurnCount,
		&entry.EstimatedChars,
		&entry.LastInputTokens,
		&entry.LastOutputTokens,
		&entry.PeakInputTokens,
		&entry.TokenBudget,
		&entry.RiskScore,
		&entry.RiskLevel,
		&entry.LastResetReason,
		&entry.UIStateJSON,
		&entry.CreatedAt,
		&entry.UpdatedAt,
		&entry.ClearedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return entry, err
		}
		return entry, fmt.Errorf("failed to fetch chat session: %w", err)
	}
	return entry, nil
}

func GetCurrentChatSession(userID string) (ChatSessionEntry, error) {
	return GetChatSession(userID, "default")
}

func ClearChatSessionEvents(userID string, sessionID int64) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}
	if sessionID <= 0 {
		return fmt.Errorf("session id is required")
	}

	if _, err := db.Exec(`DELETE FROM chat_events WHERE user_id = ? AND session_id = ?`, userID, sessionID); err != nil {
		return fmt.Errorf("failed to clear chat session events: %w", err)
	}
	return nil
}

func AppendChatEvent(userID string, sessionID int64, role, eventType, messageID, turnID, payloadJSON string) (ChatEventEntry, error) {
	var entry ChatEventEntry
	if db == nil {
		return entry, fmt.Errorf("database not initialized")
	}
	userID = strings.TrimSpace(userID)
	eventType = strings.TrimSpace(eventType)
	if userID == "" {
		return entry, fmt.Errorf("user id is required")
	}
	if sessionID <= 0 {
		return entry, fmt.Errorf("session id is required")
	}
	if eventType == "" {
		return entry, fmt.Errorf("event type is required")
	}

	tx, err := db.Begin()
	if err != nil {
		return entry, fmt.Errorf("failed to begin chat event transaction: %w", err)
	}
	defer tx.Rollback()

	var nextSeq int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(event_seq), 0) + 1 FROM chat_events WHERE session_id = ?`, sessionID).Scan(&nextSeq); err != nil {
		return entry, fmt.Errorf("failed to compute chat event sequence: %w", err)
	}

	res, err := tx.Exec(`
		INSERT INTO chat_events (session_id, user_id, event_seq, role, event_type, message_id, turn_id, payload_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID,
		userID,
		nextSeq,
		strings.TrimSpace(role),
		eventType,
		strings.TrimSpace(messageID),
		strings.TrimSpace(turnID),
		defaultJSONValue(payloadJSON),
	)
	if err != nil {
		return entry, fmt.Errorf("failed to insert chat event: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return entry, fmt.Errorf("failed to fetch chat event id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return entry, fmt.Errorf("failed to commit chat event transaction: %w", err)
	}

	return GetChatEvent(userID, id)
}

func GetChatEvent(userID string, eventID int64) (ChatEventEntry, error) {
	var entry ChatEventEntry
	if db == nil {
		return entry, fmt.Errorf("database not initialized")
	}

	err := db.QueryRow(`
		SELECT id, session_id, user_id, event_seq, role, event_type, message_id, turn_id, payload_json, created_at
		FROM chat_events
		WHERE user_id = ? AND id = ?`,
		strings.TrimSpace(userID),
		eventID,
	).Scan(
		&entry.ID,
		&entry.SessionID,
		&entry.UserID,
		&entry.EventSeq,
		&entry.Role,
		&entry.EventType,
		&entry.MessageID,
		&entry.TurnID,
		&entry.PayloadJSON,
		&entry.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return entry, err
		}
		return entry, fmt.Errorf("failed to fetch chat event: %w", err)
	}
	return entry, nil
}

func ListChatEvents(userID string, sessionID int64, afterSeq, limit int) ([]ChatEventEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 {
		limit = 200
	}
	if afterSeq < 0 {
		afterSeq = 0
	}

	rows, err := db.Query(`
		SELECT id, session_id, user_id, event_seq, role, event_type, message_id, turn_id, payload_json, created_at
		FROM chat_events
		WHERE user_id = ? AND session_id = ? AND event_seq > ?
		  AND created_at >= COALESCE((SELECT cleared_at FROM chat_sessions WHERE id = ?), '0001-01-01 00:00:00')
		ORDER BY event_seq ASC
		LIMIT ?`,
		strings.TrimSpace(userID),
		sessionID,
		afterSeq,
		sessionID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list chat events: %w", err)
	}
	defer rows.Close()

	var entries []ChatEventEntry
	for rows.Next() {
		var entry ChatEventEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.SessionID,
			&entry.UserID,
			&entry.EventSeq,
			&entry.Role,
			&entry.EventType,
			&entry.MessageID,
			&entry.TurnID,
			&entry.PayloadJSON,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan chat event: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate chat events: %w", err)
	}
	return entries, nil
}

func CountChatEvents(userID string, sessionID int64) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	var count int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM chat_events
		WHERE user_id = ? AND session_id = ?
		  AND created_at >= COALESCE((SELECT cleared_at FROM chat_sessions WHERE id = ?), '0001-01-01 00:00:00')`,
		strings.TrimSpace(userID),
		sessionID,
		sessionID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count chat events: %w", err)
	}
	return count, nil
}

func UpsertRequestPattern(userID, intentKey, sampleQuery, queryFingerprint string) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	queryFingerprint = strings.TrimSpace(queryFingerprint)
	if queryFingerprint == "" {
		return 0, fmt.Errorf("query fingerprint is required")
	}

	query := `
	INSERT INTO request_patterns (user_id, intent_key, sample_query, query_fingerprint, hit_count, first_seen_at, last_seen_at)
	VALUES (?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	ON CONFLICT(user_id, query_fingerprint) DO UPDATE SET
		intent_key = excluded.intent_key,
		sample_query = excluded.sample_query,
		hit_count = request_patterns.hit_count + 1,
		last_seen_at = CURRENT_TIMESTAMP`

	if _, err := db.Exec(query, userID, intentKey, sampleQuery, queryFingerprint); err != nil {
		return 0, fmt.Errorf("failed to upsert request pattern: %w", err)
	}

	var id int64
	if err := db.QueryRow(
		`SELECT id FROM request_patterns WHERE user_id = ? AND query_fingerprint = ?`,
		userID,
		queryFingerprint,
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("failed to fetch request pattern id: %w", err)
	}

	return id, nil
}

func InsertRequestExecution(entry RequestExecutionEntry) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
	INSERT INTO request_executions (
		user_id, intent_key, request_pattern_id, raw_query, normalized_query,
		tool_chain_json, tool_count, total_latency_ms, tool_latency_ms, success,
		fallback_used, repeated_tool_blocked, self_correction_used,
		followup_within_2m, user_feedback_score, recipe_version, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(
		query,
		entry.UserID,
		entry.IntentKey,
		entry.RequestPatternID,
		entry.RawQuery,
		entry.NormalizedQuery,
		entry.ToolChainJSON,
		entry.ToolCount,
		entry.TotalLatencyMS,
		entry.ToolLatencyMS,
		boolToInt(entry.Success),
		boolToInt(entry.FallbackUsed),
		boolToInt(entry.RepeatedToolBlocked),
		boolToInt(entry.SelfCorrectionUsed),
		boolToInt(entry.FollowupWithinTwoMin),
		entry.UserFeedbackScore,
		entry.RecipeVersion,
		entry.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert request execution: %w", err)
	}

	return nil
}

func UpsertRequestIntentStat(userID, intentKey string, success bool, latencyMS int64, seenAt time.Time) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
	INSERT INTO request_intent_stats (
		user_id, intent_key, total_count, success_count, avg_latency_ms, last_seen_at
	) VALUES (?, ?, 1, ?, ?, ?)
	ON CONFLICT(user_id, intent_key) DO UPDATE SET
		total_count = request_intent_stats.total_count + 1,
		success_count = request_intent_stats.success_count + excluded.success_count,
		avg_latency_ms = (
			(request_intent_stats.avg_latency_ms * request_intent_stats.total_count) + excluded.avg_latency_ms
		) / (request_intent_stats.total_count + 1),
		last_seen_at = excluded.last_seen_at`

	_, err := db.Exec(query, userID, intentKey, boolToInt(success), float64(latencyMS), seenAt.UTC())
	if err != nil {
		return fmt.Errorf("failed to upsert request intent stats: %w", err)
	}

	return nil
}

func UpsertProcedureRecipe(entry ProcedureRecipeEntry) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
	INSERT INTO procedure_recipes (
		user_id, intent_key, recipe_name, trigger_hint, tool_chain_template_json,
		preconditions_json, avg_latency_ms, success_rate, quality_score, final_score,
		usage_count, last_used_at, active, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	ON CONFLICT(user_id, intent_key, recipe_name) DO UPDATE SET
		trigger_hint = excluded.trigger_hint,
		tool_chain_template_json = excluded.tool_chain_template_json,
		preconditions_json = excluded.preconditions_json,
		avg_latency_ms = excluded.avg_latency_ms,
		success_rate = excluded.success_rate,
		quality_score = excluded.quality_score,
		final_score = excluded.final_score,
		usage_count = excluded.usage_count,
		last_used_at = excluded.last_used_at,
		active = excluded.active,
		updated_at = CURRENT_TIMESTAMP`

	_, err := db.Exec(
		query,
		entry.UserID,
		entry.IntentKey,
		entry.RecipeName,
		entry.TriggerHint,
		entry.ToolChainTemplateJSON,
		entry.PreconditionsJSON,
		entry.AvgLatencyMS,
		entry.SuccessRate,
		entry.QualityScore,
		entry.FinalScore,
		entry.UsageCount,
		entry.LastUsedAt,
		boolToInt(entry.Active),
	)
	if err != nil {
		return fmt.Errorf("failed to upsert procedure recipe: %w", err)
	}

	return nil
}

func RefreshProcedureRecipes(userID, intentKey string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	userID = strings.TrimSpace(userID)
	intentKey = strings.TrimSpace(intentKey)
	if userID == "" || intentKey == "" {
		return fmt.Errorf("user id and intent key are required")
	}

	rows, err := db.Query(`
		SELECT tool_chain_json, total_latency_ms, success, fallback_used, repeated_tool_blocked,
		       self_correction_used, created_at
		FROM request_executions
		WHERE user_id = ? AND intent_key = ?
		ORDER BY created_at DESC
		LIMIT 50`,
		userID,
		intentKey,
	)
	if err != nil {
		return fmt.Errorf("failed to query request executions: %w", err)
	}
	defer rows.Close()

	type executionSample struct {
		ToolChainJSON      string
		TotalLatencyMS     int64
		Success            bool
		FallbackUsed       bool
		RepeatedBlocked    bool
		SelfCorrectionUsed bool
		CreatedAt          time.Time
	}

	var samples []executionSample
	for rows.Next() {
		var sample executionSample
		var successInt int
		var fallbackInt int
		var repeatedInt int
		var selfCorrectionInt int
		if err := rows.Scan(
			&sample.ToolChainJSON,
			&sample.TotalLatencyMS,
			&successInt,
			&fallbackInt,
			&repeatedInt,
			&selfCorrectionInt,
			&sample.CreatedAt,
		); err != nil {
			return fmt.Errorf("failed to scan request execution: %w", err)
		}
		sample.Success = successInt != 0
		sample.FallbackUsed = fallbackInt != 0
		sample.RepeatedBlocked = repeatedInt != 0
		sample.SelfCorrectionUsed = selfCorrectionInt != 0
		samples = append(samples, sample)
	}

	if len(samples) == 0 {
		return nil
	}

	type recipeAgg struct {
		ToolChainJSON  string
		UsageCount     int
		SuccessCount   int
		TotalLatencyMS int64
		QualityTotal   float64
		LastUsedAt     time.Time
	}

	aggregates := make(map[string]*recipeAgg)
	for _, sample := range samples {
		recipeName := deriveRecipeName(intentKey, sample.ToolChainJSON)
		agg, ok := aggregates[recipeName]
		if !ok {
			agg = &recipeAgg{ToolChainJSON: sample.ToolChainJSON}
			aggregates[recipeName] = agg
		}
		agg.UsageCount++
		if sample.Success {
			agg.SuccessCount++
		}
		agg.TotalLatencyMS += sample.TotalLatencyMS
		agg.QualityTotal += computeExecutionQuality(sample.Success, sample.TotalLatencyMS, sample.FallbackUsed, sample.RepeatedBlocked, sample.SelfCorrectionUsed)
		if sample.CreatedAt.After(agg.LastUsedAt) {
			agg.LastUsedAt = sample.CreatedAt
		}
	}

	for recipeName, agg := range aggregates {
		avgLatency := float64(agg.TotalLatencyMS) / float64(agg.UsageCount)
		successRate := float64(agg.SuccessCount) / float64(agg.UsageCount)
		qualityScore := agg.QualityTotal / float64(agg.UsageCount)
		speedScore := latencyToSpeedScore(avgLatency)
		finalScore := successRate*0.45 + qualityScore*0.35 + speedScore*0.20

		entry := ProcedureRecipeEntry{
			UserID:                userID,
			IntentKey:             intentKey,
			RecipeName:            recipeName,
			TriggerHint:           intentKey,
			ToolChainTemplateJSON: agg.ToolChainJSON,
			PreconditionsJSON:     "{}",
			AvgLatencyMS:          avgLatency,
			SuccessRate:           successRate,
			QualityScore:          qualityScore,
			FinalScore:            finalScore,
			UsageCount:            agg.UsageCount,
			LastUsedAt:            sql.NullTime{Time: agg.LastUsedAt, Valid: !agg.LastUsedAt.IsZero()},
			Active:                true,
		}

		if err := UpsertProcedureRecipe(entry); err != nil {
			return err
		}
	}

	return nil
}

func GetTopProcedureRecipe(userID, intentKey string) (ProcedureRecipeEntry, error) {
	var entry ProcedureRecipeEntry
	if db == nil {
		return entry, fmt.Errorf("database not initialized")
	}

	query := `
	SELECT id, user_id, intent_key, recipe_name, trigger_hint, tool_chain_template_json,
	       preconditions_json, avg_latency_ms, success_rate, quality_score, final_score,
	       usage_count, last_used_at, active, created_at, updated_at
	FROM procedure_recipes
	WHERE user_id = ? AND intent_key = ? AND active = 1
	ORDER BY final_score DESC, usage_count DESC, updated_at DESC
	LIMIT 1`

	var activeInt int
	err := db.QueryRow(query, userID, intentKey).Scan(
		&entry.ID,
		&entry.UserID,
		&entry.IntentKey,
		&entry.RecipeName,
		&entry.TriggerHint,
		&entry.ToolChainTemplateJSON,
		&entry.PreconditionsJSON,
		&entry.AvgLatencyMS,
		&entry.SuccessRate,
		&entry.QualityScore,
		&entry.FinalScore,
		&entry.UsageCount,
		&entry.LastUsedAt,
		&activeInt,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)
	if err != nil {
		return entry, err
	}
	entry.Active = activeInt != 0
	return entry, nil
}

func deriveRecipeName(intentKey, toolChainJSON string) string {
	var events []struct {
		Tool string `json:"tool"`
	}
	if err := json.Unmarshal([]byte(toolChainJSON), &events); err != nil || len(events) == 0 {
		if strings.TrimSpace(intentKey) == "" {
			return "direct_answer"
		}
		return intentKey + "_direct"
	}

	parts := make([]string, 0, len(events))
	for _, event := range events {
		name := strings.TrimSpace(event.Tool)
		if name == "" {
			continue
		}
		parts = append(parts, name)
	}
	if len(parts) == 0 {
		return intentKey + "_direct"
	}
	return strings.Join(parts, "__")
}

func computeExecutionQuality(success bool, latencyMS int64, fallbackUsed bool, repeatedBlocked bool, selfCorrectionUsed bool) float64 {
	score := 0.0
	if success {
		score += 1.0
	} else {
		score -= 1.0
	}

	switch {
	case latencyMS > 0 && latencyMS <= 1500:
		score += 0.5
	case latencyMS <= 3000:
		score += 0.2
	}

	if fallbackUsed {
		score -= 0.2
	}
	if repeatedBlocked {
		score -= 0.2
	}
	if selfCorrectionUsed {
		score -= 0.3
	}

	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func latencyToSpeedScore(avgLatencyMS float64) float64 {
	switch {
	case avgLatencyMS <= 0:
		return 0
	case avgLatencyMS <= 1500:
		return 1.0
	case avgLatencyMS <= 3000:
		return 0.8
	case avgLatencyMS <= 5000:
		return 0.5
	case avgLatencyMS <= 8000:
		return 0.2
	default:
		return 0
	}
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullTimeValue(value sql.NullTime) interface{} {
	if value.Valid {
		return value.Time.UTC()
	}
	return nil
}

func nullInt64Value(value sql.NullInt64) interface{} {
	if value.Valid {
		return value.Int64
	}
	return nil
}

func defaultJSONValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "{}"
	}
	return trimmed
}

func buildSavedTurnFallbackTitle(responseText string) string {
	text := strings.TrimSpace(responseText)
	if text == "" {
		return "Saved response"
	}

	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= 42 {
		return text
	}
	return strings.TrimSpace(string(runes[:42])) + "..."
}

func SaveSavedTurn(userID, promptText, responseText string) (SavedTurnEntry, error) {
	var entry SavedTurnEntry
	if db == nil {
		return entry, fmt.Errorf("database not initialized")
	}

	userID = strings.TrimSpace(userID)
	promptText = strings.TrimSpace(promptText)
	responseText = strings.TrimSpace(responseText)
	if userID == "" {
		return entry, fmt.Errorf("user id is required")
	}
	if promptText == "" || responseText == "" {
		return entry, fmt.Errorf("prompt and response are required")
	}

	title := buildSavedTurnFallbackTitle(responseText)
	now := time.Now().UTC()
	query := `
	INSERT INTO saved_turns (
		user_id, title, title_source, auto_title_failures, prompt_text, response_text, created_at, updated_at
	) VALUES (?, ?, 'fallback', 0, ?, ?, ?, ?)`

	result, err := db.Exec(query, userID, title, promptText, responseText, now, now)
	if err != nil {
		return entry, fmt.Errorf("failed to save turn: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return entry, fmt.Errorf("failed to fetch saved turn id: %w", err)
	}

	return GetSavedTurn(userID, id)
}

func ListSavedTurns(userID string, limit int) ([]SavedTurnEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 {
		limit = 100
	}

	rows, err := db.Query(`
		SELECT id, user_id, title, title_source, auto_title_failures, prompt_text, response_text, created_at, updated_at
		FROM saved_turns
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list saved turns: %w", err)
	}
	defer rows.Close()

	var entries []SavedTurnEntry
	for rows.Next() {
		var entry SavedTurnEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.Title,
			&entry.TitleSource,
			&entry.AutoTitleFailures,
			&entry.PromptText,
			&entry.ResponseText,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan saved turn: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func SearchSavedTurns(userID, queryStr string, limit int) ([]SavedTurnEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 {
		limit = 20
	}

	queryStr = strings.TrimSpace(queryStr)
	if queryStr == "" {
		return ListSavedTurns(userID, limit)
	}

	pattern := "%" + queryStr + "%"
	rows, err := db.Query(`
		SELECT id, user_id, title, title_source, auto_title_failures, prompt_text, response_text, created_at, updated_at
		FROM saved_turns
		WHERE user_id = ?
		  AND (title LIKE ? OR prompt_text LIKE ? OR response_text LIKE ?)
		ORDER BY created_at DESC
		LIMIT ?`, userID, pattern, pattern, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search saved turns: %w", err)
	}
	defer rows.Close()

	var entries []SavedTurnEntry
	for rows.Next() {
		var entry SavedTurnEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.Title,
			&entry.TitleSource,
			&entry.AutoTitleFailures,
			&entry.PromptText,
			&entry.ResponseText,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan saved turn: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func GetSavedTurn(userID string, turnID int64) (SavedTurnEntry, error) {
	var entry SavedTurnEntry
	if db == nil {
		return entry, fmt.Errorf("database not initialized")
	}

	err := db.QueryRow(`
		SELECT id, user_id, title, title_source, auto_title_failures, prompt_text, response_text, created_at, updated_at
		FROM saved_turns
		WHERE id = ? AND user_id = ?`, turnID, userID).Scan(
		&entry.ID,
		&entry.UserID,
		&entry.Title,
		&entry.TitleSource,
		&entry.AutoTitleFailures,
		&entry.PromptText,
		&entry.ResponseText,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return entry, fmt.Errorf("saved turn not found")
		}
		return entry, fmt.Errorf("failed to get saved turn: %w", err)
	}
	return entry, nil
}

func DeleteSavedTurn(userID string, turnID int64) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	res, err := db.Exec(`DELETE FROM saved_turns WHERE id = ? AND user_id = ?`, turnID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete saved turn: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to verify deletion: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("saved turn not found or not owned by user")
	}
	return nil
}

func GetNextSavedTurnPendingTitle(userID string) (SavedTurnEntry, error) {
	var entry SavedTurnEntry
	if db == nil {
		return entry, fmt.Errorf("database not initialized")
	}

	err := db.QueryRow(`
		SELECT id, user_id, title, title_source, auto_title_failures, prompt_text, response_text, created_at, updated_at
		FROM saved_turns
		WHERE user_id = ? AND title_source = 'fallback' AND auto_title_failures < 3
		ORDER BY created_at ASC
		LIMIT 1`, userID).Scan(
		&entry.ID,
		&entry.UserID,
		&entry.Title,
		&entry.TitleSource,
		&entry.AutoTitleFailures,
		&entry.PromptText,
		&entry.ResponseText,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return entry, err
		}
		return entry, fmt.Errorf("failed to load pending saved turn title: %w", err)
	}
	return entry, nil
}

func UpdateSavedTurnTitle(userID string, turnID int64, title string, titleSource string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	title = strings.TrimSpace(title)
	titleSource = strings.TrimSpace(titleSource)
	if title == "" {
		return fmt.Errorf("title is required")
	}
	if titleSource == "" {
		titleSource = "generated"
	}

	res, err := db.Exec(`
		UPDATE saved_turns
		SET title = ?, title_source = ?, auto_title_failures = 0, updated_at = ?
		WHERE id = ? AND user_id = ?`, title, titleSource, time.Now().UTC(), turnID, userID)
	if err != nil {
		return fmt.Errorf("failed to update saved turn title: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to verify title update: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("saved turn not found or not owned by user")
	}
	return nil
}

func IncrementSavedTurnAutoTitleFailures(userID string, turnID int64) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	now := time.Now().UTC()
	res, err := db.Exec(`
		UPDATE saved_turns
		SET auto_title_failures = auto_title_failures + 1, updated_at = ?
		WHERE id = ? AND user_id = ?`, now, turnID, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to increment saved turn auto title failures: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to verify auto title failure increment: %w", err)
	}
	if rowsAffected == 0 {
		return 0, fmt.Errorf("saved turn not found or not owned by user")
	}

	var failures int
	if err := db.QueryRow(`
		SELECT auto_title_failures
		FROM saved_turns
		WHERE id = ? AND user_id = ?`, turnID, userID).Scan(&failures); err != nil {
		return 0, fmt.Errorf("failed to read saved turn auto title failures: %w", err)
	}
	return failures, nil
}

// InsertMemory saves a new memory entry into the database.
func InsertMemory(userID, fullText string) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	if err := ensureMemoryRetentionSchema(); err != nil {
		return 0, err
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to start memory insert: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	tier, importance, pinned := classifyMemoryRetention(fullText, "raw_interaction")
	now := time.Now().UTC()
	result, err := tx.Exec(`
		INSERT INTO memories (user_id, full_text, hit_count, memory_type, last_accessed_at, importance_score, pinned, memory_tier)
		VALUES (?, ?, 0, 'raw_interaction', ?, ?, ?, ?)`, userID, fullText, now, importance, boolToInt(pinned), tier)
	if err != nil {
		return 0, fmt.Errorf("failed to insert memory: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get inserted memory id: %w", err)
	}

	if err = insertMemoryChunksTx(tx, id, userID, fullText, now); err != nil {
		return 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit memory insert: %w", err)
	}

	return id, nil
}

func insertMemoryChunksTx(tx *sql.Tx, memoryID int64, userID, fullText string, createdAt time.Time) error {
	chunks := chunkBufferedContent(fullText, memoryChunkSize, memoryChunkOverlap)
	if len(chunks) == 0 {
		return nil
	}

	stmt, err := tx.Prepare(`
		INSERT INTO memory_chunks (memory_id, user_id, chunk_index, chunk_text, hit_count, created_at)
		VALUES (?, ?, ?, ?, 0, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare memory chunk insert: %w", err)
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		result, err := stmt.Exec(memoryID, userID, chunk.Index, chunk.Text, createdAt)
		if err != nil {
			return fmt.Errorf("failed to insert memory chunk %d: %w", chunk.Index, err)
		}
		chunkID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to read memory chunk id: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT INTO memory_chunks_fts(rowid, chunk_text, memory_id, user_id, chunk_index)
			VALUES (?, ?, ?, ?, ?)
		`, chunkID, chunk.Text, memoryID, userID, chunk.Index); err != nil {
			return fmt.Errorf("failed to index memory chunk: %w", err)
		}
		if err := upsertMemoryChunkEmbeddingTx(tx, chunkID, chunk.Text); err != nil {
			return err
		}
	}

	return nil
}

func upsertMemoryChunkEmbeddingTx(tx *sql.Tx, chunkID int64, text string) error {
	vector, modelName := buildBufferedEmbedding(text, BufferedEmbeddingUsageDocument)
	if len(vector) == 0 {
		return nil
	}
	if strings.TrimSpace(modelName) == "" {
		modelName = webEmbeddingModel
	}
	embeddingJSON, err := json.Marshal(vector)
	if err != nil {
		return fmt.Errorf("failed to marshal memory embedding: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO memory_chunk_embeddings (
			chunk_id, embedding_model, embedding_dim, embedding_json, updated_at
		) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(chunk_id) DO UPDATE SET
			embedding_model = excluded.embedding_model,
			embedding_dim = excluded.embedding_dim,
			embedding_json = excluded.embedding_json,
			updated_at = excluded.updated_at
	`, chunkID, modelName, len(vector), string(embeddingJSON), time.Now().UTC()); err != nil {
		return fmt.Errorf("failed to store memory chunk embedding: %w", err)
	}
	return nil
}

// IncrementHitCount increases the hit counter for a specific memory entry.
func IncrementHitCount(memoryID int64) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := ensureMemoryRetentionSchema(); err != nil {
		return err
	}
	query := "UPDATE memories SET hit_count = hit_count + 1, last_accessed_at = ? WHERE id = ?"
	_, err := db.Exec(query, time.Now().UTC(), memoryID)
	return err
}

func IncrementMemoryChunkHitCount(chunkID int64) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	query := "UPDATE memory_chunks SET hit_count = hit_count + 1 WHERE id = ?"
	_, err := db.Exec(query, chunkID)
	return err
}

// MemoryEntry represents a single memory record.
type MemoryEntry struct {
	ID              int64     `json:"id"`
	UserID          string    `json:"user_id"`
	FullText        string    `json:"full_text"`
	HitCount        int       `json:"hit_count"`
	CreatedAt       time.Time `json:"created_at"`
	MemoryType      string    `json:"memory_type"`
	LastAccessedAt  time.Time `json:"last_accessed_at,omitempty"`
	ImportanceScore float64   `json:"importance_score,omitempty"`
	Pinned          bool      `json:"pinned,omitempty"`
	MemoryTier      string    `json:"memory_tier,omitempty"`
}

type MemoryChunkMatch struct {
	MemoryEntry
	ChunkID     int64   `json:"chunk_id"`
	ChunkIndex  int     `json:"chunk_index"`
	ChunkText   string  `json:"chunk_text"`
	FTSScore    float64 `json:"fts_score,omitempty"`
	VectorScore float64 `json:"vector_score,omitempty"`
	HybridScore float64 `json:"hybrid_score,omitempty"`
}

// SearchMemories searches full_text memories belonging to the user.
func SearchMemories(userID, queryStr string) ([]MemoryEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if err := ensureMemoryRetentionSchema(); err != nil {
		return nil, err
	}

	searchPattern := "%" + queryStr + "%"

	query := `
	SELECT id, user_id, full_text, hit_count, created_at, memory_type,
	       COALESCE(last_accessed_at, created_at), COALESCE(importance_score, 0.25), COALESCE(pinned, 0), COALESCE(memory_tier, 'ephemeral')
	FROM memories
	WHERE user_id = ? AND full_text LIKE ?
	ORDER BY created_at DESC
	LIMIT 10`

	rows, err := db.Query(query, userID, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	defer rows.Close()

	var results []MemoryEntry
	for rows.Next() {
		var m MemoryEntry
		var pinned int
		var lastAccessedRaw string
		if err := rows.Scan(&m.ID, &m.UserID, &m.FullText, &m.HitCount, &m.CreatedAt, &m.MemoryType, &lastAccessedRaw, &m.ImportanceScore, &pinned, &m.MemoryTier); err != nil {
			return nil, err
		}
		m.LastAccessedAt = parseSQLiteTime(lastAccessedRaw, m.CreatedAt)
		m.Pinned = pinned != 0
		results = append(results, m)
	}

	return results, nil
}

// SearchMemoriesMultiQuery searches with multiple candidate queries and merges results.
func SearchMemoriesMultiQuery(userID string, queryStrs []string) ([]MemoryEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	seen := make(map[int64]bool)
	var merged []MemoryEntry

	for _, queryStr := range queryStrs {
		trimmed := strings.TrimSpace(queryStr)
		if trimmed == "" {
			continue
		}

		results, err := SearchMemories(userID, trimmed)
		if err != nil {
			return nil, err
		}

		for _, result := range results {
			if seen[result.ID] {
				continue
			}
			seen[result.ID] = true
			merged = append(merged, result)
			if len(merged) >= 10 {
				return merged, nil
			}
		}
	}

	return merged, nil
}

func SearchMemoryChunkMatches(userID, queryStr string, limit int) ([]MemoryChunkMatch, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	trimmed := strings.TrimSpace(queryStr)
	if trimmed == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	results, err := hybridSearchMemoryChunkMatches(userID, trimmed, limit)
	if err != nil {
		return nil, err
	}
	if len(results) > 0 {
		return results, nil
	}
	return searchMemoryChunkMatchesLike(userID, trimmed, limit)
}

func searchMemoryChunkMatchesLike(userID, queryStr string, limit int) ([]MemoryChunkMatch, error) {
	searchPattern := "%" + queryStr + "%"
	rows, err := db.Query(`
		SELECT
			m.id, m.user_id, m.full_text, m.hit_count, m.created_at, m.memory_type,
			mc.id, mc.chunk_index, mc.chunk_text
		FROM memory_chunks mc
		INNER JOIN memories m ON m.id = mc.memory_id
		WHERE mc.user_id = ? AND mc.chunk_text LIKE ?
		ORDER BY m.created_at DESC, mc.chunk_index ASC
		LIMIT ?`, userID, searchPattern, limit)
	if err != nil {
		return nil, fmt.Errorf("memory chunk like search failed: %w", err)
	}
	defer rows.Close()

	var results []MemoryChunkMatch
	for rows.Next() {
		var match MemoryChunkMatch
		if err := rows.Scan(
			&match.ID,
			&match.UserID,
			&match.FullText,
			&match.HitCount,
			&match.CreatedAt,
			&match.MemoryType,
			&match.ChunkID,
			&match.ChunkIndex,
			&match.ChunkText,
		); err != nil {
			return nil, fmt.Errorf("failed to scan memory chunk like match: %w", err)
		}
		results = append(results, match)
	}
	return results, rows.Err()
}

func searchMemoryChunkMatchesFTS(userID, queryStr string, limit int) ([]MemoryChunkMatch, error) {
	ftsQuery := buildBufferedFTSQuery(queryStr)
	if strings.TrimSpace(ftsQuery) == "" {
		return nil, nil
	}
	rows, err := db.Query(`
		SELECT
			m.id, m.user_id, m.full_text, m.hit_count, m.created_at, m.memory_type,
			mc.id, mc.chunk_index, mc.chunk_text, bm25(memory_chunks_fts)
		FROM memory_chunks_fts
		JOIN memory_chunks mc ON mc.id = memory_chunks_fts.rowid
		JOIN memories m ON m.id = mc.memory_id
		WHERE memory_chunks_fts MATCH ?
		  AND mc.user_id = ?
		ORDER BY bm25(memory_chunks_fts), m.created_at DESC, mc.chunk_index ASC
		LIMIT ?
	`, ftsQuery, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("memory chunk fts search failed: %w", err)
	}
	defer rows.Close()

	var results []MemoryChunkMatch
	for rows.Next() {
		var match MemoryChunkMatch
		var bm25 float64
		if err := rows.Scan(
			&match.ID,
			&match.UserID,
			&match.FullText,
			&match.HitCount,
			&match.CreatedAt,
			&match.MemoryType,
			&match.ChunkID,
			&match.ChunkIndex,
			&match.ChunkText,
			&bm25,
		); err != nil {
			return nil, fmt.Errorf("failed to scan memory fts match: %w", err)
		}
		match.FTSScore = normalizeFTSScore(bm25)
		results = append(results, match)
	}
	return results, rows.Err()
}

func searchMemoryChunkMatchesVector(userID, queryStr string, limit int) ([]MemoryChunkMatch, error) {
	queryVector, queryModel := buildBufferedEmbedding(queryStr, BufferedEmbeddingUsageQuery)
	if len(queryVector) == 0 {
		return nil, nil
	}

	rows, err := db.Query(`
		SELECT
			m.id, m.user_id, m.full_text, m.hit_count, m.created_at, m.memory_type,
			mc.id, mc.chunk_index, mc.chunk_text, e.embedding_json, e.embedding_model
		FROM memory_chunks mc
		JOIN memories m ON m.id = mc.memory_id
		JOIN memory_chunk_embeddings e ON e.chunk_id = mc.id
		WHERE mc.user_id = ?
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("memory chunk vector search failed: %w", err)
	}
	defer rows.Close()

	var ranked []MemoryChunkMatch
	for rows.Next() {
		var match MemoryChunkMatch
		var embeddingJSON string
		var embeddingModel string
		if err := rows.Scan(
			&match.ID,
			&match.UserID,
			&match.FullText,
			&match.HitCount,
			&match.CreatedAt,
			&match.MemoryType,
			&match.ChunkID,
			&match.ChunkIndex,
			&match.ChunkText,
			&embeddingJSON,
			&embeddingModel,
		); err != nil {
			return nil, fmt.Errorf("failed to scan memory vector match: %w", err)
		}
		if strings.TrimSpace(queryModel) != "" && strings.TrimSpace(embeddingModel) != "" && embeddingModel != queryModel {
			continue
		}
		vector, err := parseBufferedEmbeddingJSON(embeddingJSON)
		if err != nil {
			continue
		}
		score := cosineSimilarity(queryVector, vector)
		if score <= 0 {
			continue
		}
		match.VectorScore = score
		ranked = append(ranked, match)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if math.Abs(ranked[i].VectorScore-ranked[j].VectorScore) < 1e-9 {
			if ranked[i].CreatedAt.Equal(ranked[j].CreatedAt) {
				return ranked[i].ChunkIndex < ranked[j].ChunkIndex
			}
			return ranked[i].CreatedAt.After(ranked[j].CreatedAt)
		}
		return ranked[i].VectorScore > ranked[j].VectorScore
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked, nil
}

func hybridSearchMemoryChunkMatches(userID, queryStr string, limit int) ([]MemoryChunkMatch, error) {
	ftsLimit := max(limit*3, limit)
	ftsMatches, err := searchMemoryChunkMatchesFTS(userID, queryStr, ftsLimit)
	if err != nil {
		ftsMatches = nil
	}
	vectorMatches, err := searchMemoryChunkMatchesVector(userID, queryStr, ftsLimit)
	if err != nil {
		vectorMatches = nil
	}
	if len(ftsMatches) == 0 && len(vectorMatches) == 0 {
		return nil, nil
	}

	merged := make(map[string]MemoryChunkMatch, len(ftsMatches)+len(vectorMatches))
	for _, match := range ftsMatches {
		match.HybridScore = match.FTSScore
		key := fmt.Sprintf("%d:%d", match.ID, match.ChunkIndex)
		merged[key] = match
	}
	for _, match := range vectorMatches {
		key := fmt.Sprintf("%d:%d", match.ID, match.ChunkIndex)
		existing, ok := merged[key]
		if ok {
			existing.VectorScore = match.VectorScore
			existing.HybridScore = (existing.FTSScore * 0.65) + (match.VectorScore * 0.35)
			merged[key] = existing
			continue
		}
		match.HybridScore = match.VectorScore * 0.35
		merged[key] = match
	}

	ranked := make([]MemoryChunkMatch, 0, len(merged))
	for _, match := range merged {
		ranked = append(ranked, match)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if math.Abs(ranked[i].HybridScore-ranked[j].HybridScore) < 1e-9 {
			if ranked[i].CreatedAt.Equal(ranked[j].CreatedAt) {
				return ranked[i].ChunkIndex < ranked[j].ChunkIndex
			}
			return ranked[i].CreatedAt.After(ranked[j].CreatedAt)
		}
		return ranked[i].HybridScore > ranked[j].HybridScore
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked, nil
}

func SearchMemoryChunkMatchesMultiQuery(userID string, queryStrs []string, limit int) ([]MemoryChunkMatch, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 {
		limit = 10
	}

	seen := make(map[string]bool)
	var merged []MemoryChunkMatch

	for _, queryStr := range queryStrs {
		trimmed := strings.TrimSpace(queryStr)
		if trimmed == "" {
			continue
		}

		results, err := SearchMemoryChunkMatches(userID, trimmed, limit)
		if err != nil {
			return nil, err
		}

		for _, result := range results {
			key := fmt.Sprintf("%d:%d", result.ID, result.ChunkIndex)
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, result)
			if len(merged) >= limit {
				return merged, nil
			}
		}
	}

	return merged, nil
}

// SearchMemoriesByRecent gets the most recent N memories for a user.
func SearchMemoriesByRecent(userID string, limit int) ([]MemoryEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if err := ensureMemoryRetentionSchema(); err != nil {
		return nil, err
	}

	query := `
	SELECT id, user_id, full_text, hit_count, created_at, memory_type,
	       COALESCE(last_accessed_at, created_at), COALESCE(importance_score, 0.25), COALESCE(pinned, 0), COALESCE(memory_tier, 'ephemeral')
	FROM memories
	WHERE user_id = ?
	ORDER BY pinned DESC, importance_score DESC, last_accessed_at DESC, created_at DESC
	LIMIT ?`

	rows, err := db.Query(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("recent memories failed: %w", err)
	}
	defer rows.Close()

	var results []MemoryEntry
	for rows.Next() {
		var m MemoryEntry
		var pinned int
		var lastAccessedRaw string
		if err := rows.Scan(&m.ID, &m.UserID, &m.FullText, &m.HitCount, &m.CreatedAt, &m.MemoryType, &lastAccessedRaw, &m.ImportanceScore, &pinned, &m.MemoryTier); err != nil {
			return nil, err
		}
		m.LastAccessedAt = parseSQLiteTime(lastAccessedRaw, m.CreatedAt)
		m.Pinned = pinned != 0
		results = append(results, m)
	}

	return results, nil
}

// ReadMemory fetches the full_text of a specific memory by ID.
func ReadMemory(userID string, memoryID int64) (MemoryEntry, error) {
	var m MemoryEntry
	if db == nil {
		return m, fmt.Errorf("database not initialized")
	}
	if err := ensureMemoryRetentionSchema(); err != nil {
		return m, err
	}

	query := `
	SELECT id, user_id, full_text, hit_count, created_at, memory_type,
	       COALESCE(last_accessed_at, created_at), COALESCE(importance_score, 0.25), COALESCE(pinned, 0), COALESCE(memory_tier, 'ephemeral')
	FROM memories
	WHERE id = ? AND user_id = ?`

	var pinned int
	var lastAccessedRaw string
	err := db.QueryRow(query, memoryID, userID).Scan(&m.ID, &m.UserID, &m.FullText, &m.HitCount, &m.CreatedAt, &m.MemoryType, &lastAccessedRaw, &m.ImportanceScore, &pinned, &m.MemoryTier)
	if err != nil {
		if err == sql.ErrNoRows {
			return m, fmt.Errorf("memory not found")
		}
		return m, fmt.Errorf("failed to read memory: %w", err)
	}
	m.LastAccessedAt = parseSQLiteTime(lastAccessedRaw, m.CreatedAt)
	m.Pinned = pinned != 0

	return m, nil
}

// DeleteMemory removes an existing memory entry.
func DeleteMemory(userID string, memoryID int64) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start memory delete: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	rows, err := tx.Query(`SELECT id FROM memory_chunks WHERE memory_id = ? AND user_id = ?`, memoryID, userID)
	if err != nil {
		return fmt.Errorf("failed to query memory chunks for delete: %w", err)
	}
	var chunkIDs []int64
	for rows.Next() {
		var chunkID int64
		if scanErr := rows.Scan(&chunkID); scanErr != nil {
			rows.Close()
			return fmt.Errorf("failed to scan memory chunk id for delete: %w", scanErr)
		}
		chunkIDs = append(chunkIDs, chunkID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("failed to iterate memory chunk ids for delete: %w", err)
	}
	rows.Close()

	for _, chunkID := range chunkIDs {
		if _, err := tx.Exec(`DELETE FROM memory_chunk_embeddings WHERE chunk_id = ?`, chunkID); err != nil {
			return fmt.Errorf("failed to delete memory chunk embedding: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM memory_chunks_fts WHERE rowid = ?`, chunkID); err != nil {
			return fmt.Errorf("failed to delete memory chunk fts row: %w", err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM memory_chunks WHERE memory_id = ? AND user_id = ?`, memoryID, userID); err != nil {
		return fmt.Errorf("failed to delete memory chunks: %w", err)
	}

	res, err := tx.Exec(`
		DELETE FROM memories
		WHERE id = ? AND user_id = ?`, memoryID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete memory: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("memory not found or not owned by user")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit memory delete: %w", err)
	}

	return nil
}
