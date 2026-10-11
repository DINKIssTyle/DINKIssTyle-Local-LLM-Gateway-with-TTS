package mcp

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const retrievalStorageVersion = "1"

var databasePath string
var databaseBackupMu sync.Mutex

// BackupDatabase takes a consistent SQLite snapshot, including committed WAL
// pages. A failed or unverifiable snapshot must prevent any migration.
func BackupDatabase(reason string) (string, error) {
	databaseBackupMu.Lock()
	defer databaseBackupMu.Unlock()
	if db == nil || databasePath == "" {
		return "", fmt.Errorf("database not initialized")
	}
	return backupSQLiteConnection(db, databasePath, reason)
}

// BackupSQLiteFile is used before legacy database files are moved or replaced.
func BackupSQLiteFile(source, reason string) (string, error) {
	databaseBackupMu.Lock()
	defer databaseBackupMu.Unlock()
	if _, err := os.Stat(source); err != nil {
		return "", err
	}
	connection, err := sql.Open("sqlite", source)
	if err != nil {
		return "", err
	}
	defer connection.Close()
	if _, err := connection.Exec(fmt.Sprintf(`PRAGMA busy_timeout=%d`, sqliteBusyTimeout)); err != nil {
		return "", err
	}
	return backupSQLiteConnection(connection, source, reason)
}

func backupSQLiteConnection(connection *sql.DB, source, reason string) (string, error) {
	dir := filepath.Join(filepath.Dir(source), "backups")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, filepath.Base(source)+"."+time.Now().UTC().Format("20060102T150405Z")+".*.bak")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		return "", err
	}
	// VACUUM INTO requires a nonexistent or empty destination.
	if _, err := connection.Exec(`VACUUM INTO ?`, path); err != nil {
		return "", fmt.Errorf("database backup failed: %w", err)
	}
	check, err := sql.Open("sqlite", path)
	if err != nil {
		return "", err
	}
	var result string
	err = check.QueryRow(`PRAGMA integrity_check`).Scan(&result)
	check.Close()
	if err != nil || result != "ok" {
		return "", fmt.Errorf("backup integrity check failed (%s): %v", result, err)
	}
	f, err = os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		return "", err
	}
	err = f.Sync()
	f.Close()
	if err != nil {
		return "", err
	}
	log.Printf("[DB] Verified backup before %s: %s", reason, path)
	return path, nil
}

func backupBeforeSchemaMigration() error {
	var tables int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&tables); err != nil {
		return err
	}
	if tables == 0 {
		return nil
	}
	var version string
	// Older databases may not have app_meta yet.
	_ = db.QueryRow(`SELECT value FROM app_meta WHERE key='retrieval_storage_version'`).Scan(&version)
	var ftsVersion string
	_ = db.QueryRow(`SELECT value FROM app_meta WHERE key='fts_index_version'`).Scan(&ftsVersion)
	if version == retrievalStorageVersion && ftsVersion == ftsIndexVersion {
		return nil
	}
	_, err := BackupDatabase("retrieval schema migration")
	return err
}

func encodeEmbeddingBlob(vector []float64) []byte {
	blob := make([]byte, len(vector)*4)
	for i, v := range vector {
		binary.LittleEndian.PutUint32(blob[i*4:], math.Float32bits(float32(v)))
	}
	return blob
}

func decodeEmbeddingBlob(blob []byte, dimension int) ([]float32, error) {
	if dimension <= 0 || dimension > 8192 || len(blob) != dimension*4 {
		return nil, fmt.Errorf("invalid embedding dimensions")
	}
	vector := make([]float32, dimension)
	for i := range vector {
		vector[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
		if math.IsNaN(float64(vector[i])) || math.IsInf(float64(vector[i]), 0) {
			return nil, fmt.Errorf("non-finite embedding")
		}
	}
	return vector, nil
}

func migrateRetrievalStorage() error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"memory_chunk_embeddings", "saved_turn_chunk_embeddings", "web_chunk_embeddings"} {
		columns, err := tx.Query(`PRAGMA table_info(` + table + `)`)
		if err != nil {
			return err
		}
		hasBlob := false
		for columns.Next() {
			var cid, notNull, primary int
			var name, columnType string
			var defaultValue any
			if err := columns.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primary); err != nil {
				columns.Close()
				return err
			}
			if name == "embedding_blob" {
				hasBlob = true
			}
		}
		if err := columns.Err(); err != nil {
			columns.Close()
			return err
		}
		columns.Close()
		if !hasBlob {
			if _, err := tx.Exec(`ALTER TABLE ` + table + ` ADD COLUMN embedding_blob BLOB`); err != nil {
				return err
			}
		}
		// JSON remains readable by old versions and allows rollback to the backup.
		rows, err := tx.Query(`SELECT chunk_id, embedding_json FROM ` + table + ` WHERE embedding_blob IS NULL`)
		if err != nil {
			return err
		}
		type conversion struct {
			id   int64
			blob []byte
		}
		var batch []conversion
		skipped := 0
		for rows.Next() {
			var id int64
			var raw string
			var vector []float64
			if err := rows.Scan(&id, &raw); err != nil {
				rows.Close()
				return err
			}
			if err := json.Unmarshal([]byte(raw), &vector); err != nil {
				skipped++
				continue
			}
			blob := encodeEmbeddingBlob(vector)
			if _, err := decodeEmbeddingBlob(blob, len(vector)); err != nil {
				skipped++
				continue
			}
			batch = append(batch, conversion{id, blob})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if skipped > 0 {
			log.Printf("[DB] Kept %d invalid legacy vectors in %s; source text preserved for reindex", skipped, table)
		}
		for _, row := range batch {
			if _, err := tx.Exec(`UPDATE `+table+` SET embedding_blob=? WHERE chunk_id=?`, row.blob, row.id); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO app_meta(key,value) VALUES ('retrieval_epoch','0')`); err != nil {
		return err
	}
	for _, table := range []string{"memories", "saved_turns"} {
		statement := fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS retrieval_%s_metadata AFTER UPDATE OF user_id,created_at ON %s BEGIN UPDATE app_meta SET value=CAST(value AS INTEGER)+1 WHERE key='retrieval_epoch'; END`, table, table)
		if _, err := tx.Exec(statement); err != nil {
			return err
		}
	}
	// These triggers also invalidate cached vectors after external SQL writes.
	for _, table := range []string{"memory_chunks", "saved_turn_chunks", "web_source_chunks", "memory_chunk_embeddings", "saved_turn_chunk_embeddings", "web_chunk_embeddings"} {
		if strings.HasSuffix(table, "embeddings") {
			// SQL clients may update only the legacy JSON column.
			statement := fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS retrieval_%s_json AFTER UPDATE OF embedding_json ON %s WHEN OLD.embedding_json<>NEW.embedding_json AND OLD.embedding_blob IS NEW.embedding_blob BEGIN UPDATE %s SET embedding_blob=NULL WHERE chunk_id=NEW.chunk_id; END`, table, table, table)
			if _, err := tx.Exec(statement); err != nil {
				return err
			}
		}
		for _, event := range []string{"INSERT", "DELETE", "UPDATE"} {
			update := event
			if event == "UPDATE" {
				if strings.HasSuffix(table, "embeddings") {
					update += " OF embedding_json, embedding_blob, embedding_model, embedding_dim"
				} else {
					update += " OF user_id, chunk_text, chunk_index"
				}
			}
			statement := fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS retrieval_%s_%s AFTER %s ON %s BEGIN UPDATE app_meta SET value=CAST(value AS INTEGER)+1 WHERE key='retrieval_epoch'; END`, table, strings.ToLower(event), update, table)
			if _, err := tx.Exec(statement); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`INSERT INTO app_meta(key,value) VALUES ('retrieval_storage_version',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, retrievalStorageVersion); err != nil {
		return err
	}
	return tx.Commit()
}
