// Created by DINKIssTyle on 2026. Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

package mcp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	// db holds the global database connection pool.
	db *sql.DB
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

	// Initialize schema
	return createSchema()
}

// CloseDB closes the database connection.
func CloseDB() {
	if db != nil {
		log.Println("[DB] Closing SQLite database.")
		_ = db.Close()
	}
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

	CREATE TABLE IF NOT EXISTS saved_turns (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		title TEXT NOT NULL,
		title_source TEXT NOT NULL DEFAULT 'fallback',
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

	log.Println("[DB] Schema initialized successfully.")
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

type SavedTurnEntry struct {
	ID           int64     `json:"id"`
	UserID       string    `json:"user_id"`
	Title        string    `json:"title"`
	TitleSource  string    `json:"title_source"`
	PromptText   string    `json:"prompt_text"`
	ResponseText string    `json:"response_text"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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
		user_id, title, title_source, prompt_text, response_text, created_at, updated_at
	) VALUES (?, ?, 'fallback', ?, ?, ?, ?)`

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
		SELECT id, user_id, title, title_source, prompt_text, response_text, created_at, updated_at
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
		SELECT id, user_id, title, title_source, prompt_text, response_text, created_at, updated_at
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
		SELECT id, user_id, title, title_source, prompt_text, response_text, created_at, updated_at
		FROM saved_turns
		WHERE id = ? AND user_id = ?`, turnID, userID).Scan(
		&entry.ID,
		&entry.UserID,
		&entry.Title,
		&entry.TitleSource,
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
		SELECT id, user_id, title, title_source, prompt_text, response_text, created_at, updated_at
		FROM saved_turns
		WHERE user_id = ? AND title_source = 'fallback'
		ORDER BY created_at ASC
		LIMIT 1`, userID).Scan(
		&entry.ID,
		&entry.UserID,
		&entry.Title,
		&entry.TitleSource,
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
		SET title = ?, title_source = ?, updated_at = ?
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

// InsertMemory saves a new memory entry into the database.
func InsertMemory(userID, fullText string) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	query := `
	INSERT INTO memories (user_id, full_text, hit_count, memory_type)
	VALUES (?, ?, 0, 'raw_interaction')`

	result, err := db.Exec(query, userID, fullText)
	if err != nil {
		return 0, fmt.Errorf("failed to insert memory: %w", err)
	}

	return result.LastInsertId()
}

// IncrementHitCount increases the hit counter for a specific memory entry.
func IncrementHitCount(memoryID int64) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	query := "UPDATE memories SET hit_count = hit_count + 1 WHERE id = ?"
	_, err := db.Exec(query, memoryID)
	return err
}

// MemoryEntry represents a single memory record.
type MemoryEntry struct {
	ID         int64     `json:"id"`
	UserID     string    `json:"user_id"`
	FullText   string    `json:"full_text"`
	HitCount   int       `json:"hit_count"`
	CreatedAt  time.Time `json:"created_at"`
	MemoryType string    `json:"memory_type"`
}

// SearchMemories searches full_text memories belonging to the user.
func SearchMemories(userID, queryStr string) ([]MemoryEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	searchPattern := "%" + queryStr + "%"

	query := `
	SELECT id, user_id, full_text, hit_count, created_at, memory_type
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
		if err := rows.Scan(&m.ID, &m.UserID, &m.FullText, &m.HitCount, &m.CreatedAt, &m.MemoryType); err != nil {
			return nil, err
		}
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

// SearchMemoriesByRecent gets the most recent N memories for a user.
func SearchMemoriesByRecent(userID string, limit int) ([]MemoryEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
	SELECT id, user_id, full_text, hit_count, created_at, memory_type
	FROM memories
	WHERE user_id = ?
	ORDER BY created_at DESC
	LIMIT ?`

	rows, err := db.Query(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("recent memories failed: %w", err)
	}
	defer rows.Close()

	var results []MemoryEntry
	for rows.Next() {
		var m MemoryEntry
		if err := rows.Scan(&m.ID, &m.UserID, &m.FullText, &m.HitCount, &m.CreatedAt, &m.MemoryType); err != nil {
			return nil, err
		}
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

	query := `
	SELECT id, user_id, full_text, hit_count, created_at, memory_type
	FROM memories
	WHERE id = ? AND user_id = ?`

	err := db.QueryRow(query, memoryID, userID).Scan(&m.ID, &m.UserID, &m.FullText, &m.HitCount, &m.CreatedAt, &m.MemoryType)
	if err != nil {
		if err == sql.ErrNoRows {
			return m, fmt.Errorf("memory not found")
		}
		return m, fmt.Errorf("failed to read memory: %w", err)
	}

	return m, nil
}

// DeleteMemory removes an existing memory entry.
func DeleteMemory(userID string, memoryID int64) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
	DELETE FROM memories 
	WHERE id = ? AND user_id = ?`

	res, err := db.Exec(query, memoryID, userID)
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

	return nil
}
