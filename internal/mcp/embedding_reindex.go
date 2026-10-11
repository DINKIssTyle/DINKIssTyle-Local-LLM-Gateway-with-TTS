package mcp

import (
	"encoding/json"
	"fmt"
	"time"
)

// ReindexStoredEmbeddings builds a replacement without modifying live vectors.
// Activation is atomic and is rejected if any source changed during inference.
// A verified database backup is required before replacing an existing model.
func ReindexStoredEmbeddings(model string, build func(string) ([]float64, string, error), activation ...BufferedEmbeddingProvider) error {
	if db == nil {
		if len(activation) > 0 {
			SetBufferedEmbeddingProvider(activation[0])
		}
		return nil
	}
	var epoch string
	if err := db.QueryRow(`SELECT value FROM app_meta WHERE key='retrieval_epoch'`).Scan(&epoch); err != nil {
		return err
	}
	type replacement struct {
		table  string
		id     int64
		text   string
		vector []float64
		raw    []byte
	}
	var replacements []replacement
	for _, source := range []struct{ chunks, embeddings string }{
		{"memory_chunks", "memory_chunk_embeddings"},
		{"saved_turn_chunks", "saved_turn_chunk_embeddings"},
		{"web_source_chunks", "web_chunk_embeddings"},
	} {
		rows, err := db.Query(`SELECT c.id,c.chunk_text FROM `+source.chunks+` c LEFT JOIN `+source.embeddings+` e ON e.chunk_id=c.id WHERE e.chunk_id IS NULL OR e.embedding_model<>? OR e.embedding_blob IS NULL`, model)
		if err != nil {
			return err
		}
		for rows.Next() {
			row := replacement{table: source.embeddings}
			if err := rows.Scan(&row.id, &row.text); err != nil {
				rows.Close()
				return err
			}
			replacements = append(replacements, row)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
	}
	if len(replacements) == 0 {
		if len(activation) > 0 {
			SetBufferedEmbeddingProvider(activation[0])
		}
		return nil
	}
	if _, err := BackupDatabase("embedding model reindex"); err != nil {
		return err
	}
	for i := range replacements {
		row := &replacements[i]
		vector, actual, err := build(row.text)
		if err != nil {
			return fmt.Errorf("reindex chunk %d failed: %w", row.id, err)
		}
		if actual != model {
			return fmt.Errorf("reindex returned a different embedding model")
		}
		if _, err := decodeEmbeddingBlob(encodeEmbeddingBlob(vector), len(vector)); err != nil {
			return err
		}
		row.vector = vector
		row.raw, err = json.Marshal(vector)
		if err != nil {
			return err
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current string
	if err := tx.QueryRow(`SELECT value FROM app_meta WHERE key='retrieval_epoch'`).Scan(&current); err != nil {
		return err
	}
	if current != epoch {
		return fmt.Errorf("memories changed during reindex; previous vectors preserved, retry when chat is idle")
	}
	for _, row := range replacements {
		_, err := tx.Exec(`INSERT INTO `+row.table+` (chunk_id,embedding_model,embedding_dim,embedding_json,embedding_blob,updated_at) VALUES (?,?,?,?,?,?) ON CONFLICT(chunk_id) DO UPDATE SET embedding_model=excluded.embedding_model,embedding_dim=excluded.embedding_dim,embedding_json=excluded.embedding_json,embedding_blob=excluded.embedding_blob,updated_at=excluded.updated_at`, row.id, model, len(row.vector), string(row.raw), encodeEmbeddingBlob(row.vector), time.Now().UTC())
		if err != nil {
			return err
		}
	}
	// Publish the new encoder while the DB connection is still held. Queued
	// memory writes obtain it after this transaction, so they cannot append old
	// model vectors immediately after the replacement commits.
	previous := getBufferedEmbeddingProvider()
	if len(activation) > 0 {
		SetBufferedEmbeddingProvider(activation[0])
	}
	if err := tx.Commit(); err != nil {
		if len(activation) > 0 {
			SetBufferedEmbeddingProvider(previous)
		}
		return err
	}
	return nil
}
