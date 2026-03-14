// Created by DINKIssTyle on 2026. Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

package mcp

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
		summary TEXT NOT NULL,
		keywords TEXT NOT NULL,
		full_text TEXT NOT NULL,
		hit_count INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_memories_user_id ON memories(user_id);
	`
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Migration: Add hit_count if it doesn't exist (for existing DBs)
	_, _ = db.Exec("ALTER TABLE memories ADD COLUMN hit_count INTEGER DEFAULT 0")

	log.Println("[DB] Schema initialized successfully.")
	return nil
}

// InsertMemory saves a new memory entry into the database.
func InsertMemory(userID, summary, keywords, fullText string) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	query := `
	INSERT INTO memories (user_id, summary, keywords, full_text, hit_count)
	VALUES (?, ?, ?, ?, 0)`

	result, err := db.Exec(query, userID, summary, keywords, fullText)
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
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	Summary   string    `json:"summary"`
	Keywords  string    `json:"keywords"`
	FullText  string    `json:"full_text"`
	HitCount  int       `json:"hit_count"`
	CreatedAt time.Time `json:"created_at"`
}

// SearchMemories searches for memories belonging to user where keywords or summary match.
func SearchMemories(userID, queryStr string) ([]MemoryEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Use LIKE for simple substring match on keywords, summary, or full_text.
	// For advanced search, FTS5 could be used later.
	searchPattern := "%" + queryStr + "%"

	query := `
	SELECT id, user_id, summary, keywords, full_text, hit_count, created_at
	FROM memories
	WHERE user_id = ? AND (keywords LIKE ? OR summary LIKE ? OR full_text LIKE ?)
	ORDER BY created_at DESC
	LIMIT 10`

	rows, err := db.Query(query, userID, searchPattern, searchPattern, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	defer rows.Close()

	var results []MemoryEntry
	for rows.Next() {
		var m MemoryEntry
		if err := rows.Scan(&m.ID, &m.UserID, &m.Summary, &m.Keywords, &m.FullText, &m.HitCount, &m.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, m)
	}

	return results, nil
}

// SearchMemoriesByRecent gets the most recent N memories for a user.
func SearchMemoriesByRecent(userID string, limit int) ([]MemoryEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
	SELECT id, user_id, summary, keywords, hit_count, created_at
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
		if err := rows.Scan(&m.ID, &m.UserID, &m.Summary, &m.Keywords, &m.HitCount, &m.CreatedAt); err != nil {
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
	SELECT id, user_id, summary, keywords, full_text, hit_count, created_at
	FROM memories
	WHERE id = ? AND user_id = ?`

	err := db.QueryRow(query, memoryID, userID).Scan(&m.ID, &m.UserID, &m.Summary, &m.Keywords, &m.FullText, &m.HitCount, &m.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return m, fmt.Errorf("memory not found")
		}
		return m, fmt.Errorf("failed to read memory: %w", err)
	}

	return m, nil
}

// UpdateMemory modifies an existing memory entry.
func UpdateMemory(userID string, memoryID int64, summary string, keywords string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
	UPDATE memories 
	SET summary = ?, keywords = ?
	WHERE id = ? AND user_id = ?`

	res, err := db.Exec(query, summary, keywords, memoryID, userID)
	if err != nil {
		return fmt.Errorf("failed to update memory: %w", err)
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
