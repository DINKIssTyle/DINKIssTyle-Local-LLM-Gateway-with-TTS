package mcp

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type cachedVector struct {
	chunkID int64
	index   int
	created time.Time
	model   string
	vector  []float32
}
type vectorSnapshot struct {
	epoch string
	rows  []cachedVector
}

var vectorCache = struct {
	sync.Mutex
	database *sql.DB
	entries  map[string]vectorSnapshot
}{entries: make(map[string]vectorSnapshot)}

func resetVectorCache() {
	vectorCache.Lock()
	defer vectorCache.Unlock()
	vectorCache.entries = make(map[string]vectorSnapshot)
	vectorCache.database = nil
}

// Vectors are isolated by user and source. Only ids and ranking metadata are
// cached; full source text is fetched for the winning chunks after ranking.
func loadVectorSnapshot(userID string, saved bool) ([]cachedVector, error) {
	vectorCache.Lock()
	defer vectorCache.Unlock()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if vectorCache.database != db {
		vectorCache.entries = make(map[string]vectorSnapshot)
		vectorCache.database = db
	}
	key := fmt.Sprintf("%t:%s", saved, userID)
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var epoch string
	if err := tx.QueryRow(`SELECT value FROM app_meta WHERE key='retrieval_epoch'`).Scan(&epoch); err != nil {
		return nil, err
	}
	if snapshot, ok := vectorCache.entries[key]; ok && snapshot.epoch == epoch {
		return snapshot.rows, nil
	}
	table, parent, foreign := "memory", "memories", "memory_id"
	if saved {
		table, parent, foreign = "saved_turn", "saved_turns", "saved_turn_id"
	}
	rows, err := tx.Query(fmt.Sprintf(`SELECT c.id,c.chunk_index,p.created_at,e.embedding_model,e.embedding_dim,e.embedding_blob,CASE WHEN e.embedding_blob IS NULL THEN e.embedding_json ELSE '' END
		FROM %s_chunks c JOIN %s p ON p.id=c.%s JOIN %s_chunk_embeddings e ON e.chunk_id=c.id
		WHERE c.user_id=? AND p.user_id=? ORDER BY c.id`, table, parent, foreign, table), userID, userID)
	if err != nil {
		return nil, err
	}
	var cached []cachedVector
	for rows.Next() {
		var row cachedVector
		var dimension int
		var blob []byte
		var raw string
		if err := rows.Scan(&row.chunkID, &row.index, &row.created, &row.model, &dimension, &blob, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		if len(blob) == 0 {
			vector, err := parseBufferedEmbeddingJSON(raw)
			if err != nil {
				continue
			}
			blob = encodeEmbeddingBlob(vector)
		}
		row.vector, err = decodeEmbeddingBlob(blob, dimension)
		if err != nil {
			continue
		}
		cached = append(cached, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	// Bound retained users. Existing readers hold immutable snapshots safely.
	if len(vectorCache.entries) >= 8 {
		vectorCache.entries = make(map[string]vectorSnapshot)
	}
	vectorCache.entries[key] = vectorSnapshot{epoch, cached}
	return cached, nil
}

type vectorHit struct {
	cachedVector
	score float64
}

func rankCachedVectors(userID, query string, limit int, saved bool) ([]vectorHit, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := loadVectorSnapshot(userID, saved)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	vector, model := queryEmbedding(query)
	if len(vector) == 0 {
		return nil, nil
	}
	var hits []vectorHit
	better := func(a, b vectorHit) bool {
		if math.Abs(a.score-b.score) < 1e-9 {
			if a.created.Equal(b.created) {
				return a.index < b.index
			}
			return a.created.After(b.created)
		}
		return a.score > b.score
	}
	for _, row := range rows {
		if len(row.vector) != len(vector) || (strings.TrimSpace(model) != "" && strings.TrimSpace(row.model) != "" && row.model != model) {
			continue
		}
		var score float64
		for i, v := range row.vector {
			score += float64(v) * vector[i]
		}
		if score <= 0 {
			continue
		}
		hit := vectorHit{row, score}
		if len(hits) == limit && !better(hit, hits[len(hits)-1]) {
			continue
		}
		position := sort.Search(len(hits), func(i int) bool { return better(hit, hits[i]) })
		hits = append(hits, vectorHit{})
		copy(hits[position+1:], hits[position:])
		hits[position] = hit
		if len(hits) > limit {
			hits = hits[:limit]
		}
	}
	return hits, nil
}

var queryEmbeddings = struct {
	sync.Mutex
	entries map[string]queryEmbeddingValue
}{entries: make(map[string]queryEmbeddingValue)}

type queryEmbeddingValue struct {
	vector []float64
	model  string
}

func queryEmbedding(text string) ([]float64, string) {
	queryEmbeddings.Lock()
	defer queryEmbeddings.Unlock()
	if value, ok := queryEmbeddings.entries[text]; ok {
		return value.vector, value.model
	}
	vector, model := buildBufferedEmbedding(text, BufferedEmbeddingUsageQuery)
	if len(queryEmbeddings.entries) >= 64 {
		queryEmbeddings.entries = make(map[string]queryEmbeddingValue)
	}
	// A runtime failure falls back to token hashes; do not persist that fallback
	// when a semantic model is configured, so a recovered worker is retried.
	provider := getBufferedEmbeddingProvider()
	if len(vector) > 0 && !(model == webEmbeddingModel && (provider.Build != nil || provider.BuildWithUsage != nil)) {
		queryEmbeddings.entries[text] = queryEmbeddingValue{vector, model}
	}
	return vector, model
}
