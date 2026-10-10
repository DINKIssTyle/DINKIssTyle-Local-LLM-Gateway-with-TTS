package mcp

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// BenchmarkMemorySearchPipeline isolates the existing SQLite retrieval path.
// The embedding provider is a constant-time stub, not an ONNX or Jev model.
// Fixtures use synthetic text and dense 384-dimensional vectors; no user DB is read.
func BenchmarkMemorySearchPipeline(b *testing.B) {
	for _, count := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("chunks_%d", count), func(b *testing.B) {
			if db != nil {
				b.Fatal("benchmark requires an isolated database")
			}
			if err := InitDB(filepath.Join(b.TempDir(), "memory.db")); err != nil {
				b.Fatal(err)
			}
			b.Cleanup(CloseDB)
			provider := getBufferedEmbeddingProvider()
			b.Cleanup(func() { SetBufferedEmbeddingProvider(provider) })
			queryVector := make([]float64, 384)
			for i := range queryVector {
				queryVector[i] = 1 / math.Sqrt(float64(len(queryVector)))
			}
			calls := 0
			SetBufferedEmbeddingProvider(BufferedEmbeddingProvider{
				BuildWithUsage: func(_ string, _ BufferedEmbeddingUsage) ([]float64, string, error) {
					calls++
					return queryVector, "benchmark-384", nil
				},
			})
			seedMemorySearchBenchmark(b, count)
			stages := []struct {
				name string
				run  func() (int, error)
			}{
				{"fts", func() (int, error) {
					matches, err := searchMemoryChunkMatchesFTS("benchmark", "topic42", 30)
					return len(matches), err
				}},
				{"vector", func() (int, error) {
					matches, err := searchMemoryChunkMatchesVector("benchmark", "topic42", 30)
					return len(matches), err
				}},
				{"candidates", func() (int, error) {
					matches, err := buildMemoryCandidates("benchmark", "topic42", 8)
					return len(matches), err
				}},
			}
			for _, stage := range stages {
				b.Run(stage.name, func(b *testing.B) {
					calls = 0
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						n, err := stage.run()
						if err != nil {
							b.Fatal(err)
						}
						if n == 0 {
							b.Fatal("fixture returned no matches")
						}
					}
					b.StopTimer()
					b.ReportMetric(float64(calls)/float64(b.N), "query_embeddings/op")
				})
			}
		})
	}
}

func seedMemorySearchBenchmark(b *testing.B, count int) {
	b.Helper()
	tx, err := db.Begin()
	if err != nil {
		b.Fatal(err)
	}
	defer tx.Rollback()
	sqls := []string{
		`INSERT INTO memories (id, user_id, full_text, created_at) VALUES (?, 'benchmark', ?, ?)`,
		`INSERT INTO memory_chunks (id, memory_id, user_id, chunk_index, chunk_text, created_at) VALUES (?, ?, 'benchmark', 0, ?, ?)`,
		`INSERT INTO memory_chunks_fts (rowid, chunk_text, memory_id, user_id, chunk_index) VALUES (?, ?, ?, 'benchmark', 0)`,
		`INSERT INTO memory_chunk_embeddings (chunk_id, embedding_model, embedding_dim, embedding_json) VALUES (?, 'benchmark-384', 384, ?)`,
	}
	// Keep prepared statements inside the single fixture transaction.
	created := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	for index, query := range sqls {
		stmt, err := tx.Prepare(query)
		if err != nil {
			b.Fatal(err)
		}
		defer stmt.Close()
		for i := 0; i < count; i++ {
			text := fmt.Sprintf("Synthetic memory item %d topic%d ", i, i%100)
			var args []any
			switch index {
			case 0:
				args = []any{i + 1, text + strings.Repeat("Synthetic context. ", 160), created}
			case 1:
				args = []any{i + 1, i + 1, text, created}
			case 2:
				args = []any{i + 1, buildFTSIndexedText(text), i + 1}
			case 3:
				vector := make([]float64, 384)
				var norm float64
				for j := range vector {
					vector[j] = float64(((i+1)*31+j*17)%997+1) / 997
					norm += vector[j] * vector[j]
				}
				norm = math.Sqrt(norm)
				for j := range vector {
					vector[j] /= norm
				}
				raw, err := json.Marshal(vector)
				if err != nil {
					b.Fatal(err)
				}
				args = []any{i + 1, string(raw)}
			}
			if _, err := stmt.Exec(args...); err != nil {
				b.Fatal(err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
}
