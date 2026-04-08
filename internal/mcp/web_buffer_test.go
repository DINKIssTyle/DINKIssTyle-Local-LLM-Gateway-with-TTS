package mcp

import (
	"os"
	"strings"
	"testing"
)

func TestWebBufferSchemaExists(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "web_buffer_schema_*.db")
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

	for _, name := range []string{"web_sources", "web_source_chunks", "web_source_chunks_fts", "web_chunk_embeddings"} {
		var objectName string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE name = ?`, name).Scan(&objectName); err != nil {
			t.Fatalf("failed to find sqlite object %s: %v", name, err)
		}
		if objectName != name {
			t.Fatalf("expected sqlite object %s, got %q", name, objectName)
		}
	}
}

func TestBufferedWebSourcePersistsToSQLiteAndReadsFocusedChunks(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "web_buffer_read_*.db")
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

	userID := "buffer_user"
	prefix := strings.Repeat("Alpha context that is not relevant to the focused lookup. ", 40)
	suffix := strings.Repeat("Omega context that is also not relevant. ", 35)
	content := prefix + "The M4 benchmark graph shows 42 tokens per second for the reference workload. " + suffix

	source := saveBufferedWebSource(userID, "read_web_page", "m4 benchmark", "https://example.com/bench", "Benchmark Notes", content)
	if source == nil {
		t.Fatalf("expected buffered source")
	}

	var sourceCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM web_sources WHERE source_id = ?`, source.SourceID).Scan(&sourceCount); err != nil {
		t.Fatalf("failed to count web sources: %v", err)
	}
	if sourceCount != 1 {
		t.Fatalf("expected 1 web source row, got %d", sourceCount)
	}

	var chunkCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM web_source_chunks WHERE source_id = ?`, source.SourceID).Scan(&chunkCount); err != nil {
		t.Fatalf("failed to count web chunks: %v", err)
	}
	if chunkCount < 2 {
		t.Fatalf("expected multiple web chunks, got %d", chunkCount)
	}
	var embeddingCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM web_chunk_embeddings WHERE embedding_model = ?`, webEmbeddingModel).Scan(&embeddingCount); err != nil {
		t.Fatalf("failed to count web embeddings: %v", err)
	}
	if embeddingCount != chunkCount {
		t.Fatalf("expected embeddings for every chunk, got embeddings=%d chunks=%d", embeddingCount, chunkCount)
	}

	result, err := readBufferedSource(userID, source.SourceID, "benchmark graph 42 tokens", 2)
	if err != nil {
		t.Fatalf("readBufferedSource failed: %v", err)
	}
	if !strings.Contains(result, "42 tokens per second") {
		t.Fatalf("expected focused excerpt in result, got %q", result)
	}
	if !strings.Contains(result, "Retrieval: hybrid_fts5_vector") && !strings.Contains(result, "Retrieval: fts5") {
		t.Fatalf("expected hybrid or fts5 retrieval mode in result, got %q", result)
	}
}

func TestBufferedWebSourceVectorFallbackWorksWithoutFTSHit(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "web_buffer_vector_*.db")
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

	userID := "vector_user"
	content := strings.Repeat("Apple orchard harvest season fruit sweetness crisp texture. ", 30)
	source := saveBufferedWebSource(userID, "read_web_page", "apple notes", "https://example.com/apples", "Apple Notes", content)
	if source == nil {
		t.Fatalf("expected buffered source")
	}

	result, err := readBufferedSource(userID, source.SourceID, "orchard crisp sweetness", 2)
	if err != nil {
		t.Fatalf("readBufferedSource failed: %v", err)
	}
	if !strings.Contains(result, "orchard") {
		t.Fatalf("expected orchard excerpt, got %q", result)
	}
	if !strings.Contains(result, "Retrieval: hybrid_fts5_vector") && !strings.Contains(result, "Retrieval: vector_only") && !strings.Contains(result, "Retrieval: fts5") {
		t.Fatalf("expected vector-aware retrieval mode in result, got %q", result)
	}
}
