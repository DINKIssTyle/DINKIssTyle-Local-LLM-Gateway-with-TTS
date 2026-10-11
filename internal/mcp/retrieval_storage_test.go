package mcp

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openRetrievalTestDB(t *testing.T) string {
	t.Helper()
	CloseDB()
	path := filepath.Join(t.TempDir(), "memory.db")
	if err := InitDB(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(CloseDB)
	previous := getBufferedEmbeddingProvider()
	SetBufferedEmbeddingProvider(BufferedEmbeddingProvider{ModelName: "test", BuildWithUsage: func(text string, _ BufferedEmbeddingUsage) ([]float64, string, error) {
		if strings.Contains(text, "beta") {
			return []float64{0, 1}, "test", nil
		}
		return []float64{1, 0}, "test", nil
	}})
	t.Cleanup(func() { SetBufferedEmbeddingProvider(previous); SetMemoryChoiceProvider(nil) })
	return path
}

func TestDatabaseBackupIncludesWAL(t *testing.T) {
	path := openRetrievalTestDB(t)
	if _, err := InsertMemory("alice", "alpha private memory"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("expected active WAL: %v", err)
	}
	backup, err := BackupDatabase("test")
	if err != nil {
		t.Fatal(err)
	}
	connection, err := sql.Open("sqlite", backup)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	var content string
	if err := connection.QueryRow(`SELECT full_text FROM memories WHERE user_id='alice'`).Scan(&content); err != nil {
		t.Fatal(err)
	}
	if content != "alpha private memory" {
		t.Fatal(content)
	}
	info, _ := os.Stat(backup)
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("backup permissions: %v", info.Mode())
	}
	if _, err := BackupSQLiteFile(path, "relocation test"); err != nil {
		t.Fatal(err)
	}
}

func TestRetrievalMigrationBacksUpBeforeConversionAndIsIdempotent(t *testing.T) {
	path := openRetrievalTestDB(t)
	if _, err := InsertMemory("alice", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE memory_chunk_embeddings SET embedding_blob=NULL; DELETE FROM app_meta WHERE key='retrieval_storage_version'`); err != nil {
		t.Fatal(err)
	}
	CloseDB()
	if err := InitDB(path); err != nil {
		t.Fatal(err)
	}
	backups, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "backups", "*.bak"))
	if len(backups) != 1 {
		t.Fatalf("backups: %v", backups)
	}
	old, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	var oldBlob, newBlob []byte
	if err := old.QueryRow(`SELECT embedding_blob FROM memory_chunk_embeddings`).Scan(&oldBlob); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT embedding_blob FROM memory_chunk_embeddings`).Scan(&newBlob); err != nil {
		t.Fatal(err)
	}
	if len(oldBlob) != 0 || len(newBlob) != 8 {
		t.Fatalf("old/new blobs: %d/%d", len(oldBlob), len(newBlob))
	}
	CloseDB()
	if err := InitDB(path); err != nil {
		t.Fatal(err)
	}
	backups, _ = filepath.Glob(filepath.Join(filepath.Dir(path), "backups", "*.bak"))
	if len(backups) != 1 {
		t.Fatalf("repeat migration made backup: %v", backups)
	}
}

func TestBackupFailurePreventsMigration(t *testing.T) {
	path := openRetrievalTestDB(t)
	if _, err := InsertMemory("alice", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE memory_chunk_embeddings SET embedding_blob=NULL; DELETE FROM app_meta WHERE key='retrieval_storage_version'`); err != nil {
		t.Fatal(err)
	}
	CloseDB()
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "backups"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := InitDB(path); err == nil {
		t.Fatal("migration succeeded without backup")
	}
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	var blob []byte
	if err := old.QueryRow(`SELECT embedding_blob FROM memory_chunk_embeddings`).Scan(&blob); err != nil {
		t.Fatal(err)
	}
	if len(blob) != 0 {
		t.Fatal("migration modified source after failed backup")
	}
}

func TestInvalidLegacyVectorKeepsSourceAndCanBeReindexed(t *testing.T) {
	path := openRetrievalTestDB(t)
	if _, err := InsertMemory("alice", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE memory_chunk_embeddings SET embedding_json='',embedding_blob=NULL; DELETE FROM app_meta WHERE key='retrieval_storage_version'`); err != nil {
		t.Fatal(err)
	}
	CloseDB()
	if err := InitDB(path); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM memories WHERE user_id='alice'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("source lost: %d %v", count, err)
	}
	if err := ReindexStoredEmbeddings("test", func(string) ([]float64, string, error) { return []float64{1, 0}, "test", nil }); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM memory_chunk_embeddings WHERE length(embedding_blob)=8`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("invalid vector not repaired: %d %v", count, err)
	}
}

func TestMigrationAddsBlobColumnToOlderSchema(t *testing.T) {
	path := openRetrievalTestDB(t)
	if _, err := InsertMemory("alice", "alpha"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"json", "insert", "delete", "update"} {
		if _, err := db.Exec(`DROP TRIGGER IF EXISTS retrieval_memory_chunk_embeddings_` + name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE memory_chunk_embeddings DROP COLUMN embedding_blob; DELETE FROM app_meta WHERE key='retrieval_storage_version'`); err != nil {
		t.Fatal(err)
	}
	CloseDB()
	if err := InitDB(path); err != nil {
		t.Fatal(err)
	}
	var size int
	if err := db.QueryRow(`SELECT length(embedding_blob) FROM memory_chunk_embeddings`).Scan(&size); err != nil || size != 8 {
		t.Fatalf("old column migration: %d %v", size, err)
	}
	backups, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "backups", "*.bak"))
	if len(backups) != 1 {
		t.Fatal(backups)
	}
}

func TestVectorCacheIsolationInvalidationAndModelMismatch(t *testing.T) {
	openRetrievalTestDB(t)
	alice, err := InsertMemory("alice", "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InsertMemory("bob", "alpha secret"); err != nil {
		t.Fatal(err)
	}
	results, err := searchMemoryChunkMatchesVector("alice", "alpha", 8)
	if err != nil || len(results) != 1 || results[0].ID != alice {
		t.Fatalf("isolation: %+v %v", results, err)
	}
	if _, err := db.Exec(`UPDATE memory_chunk_embeddings SET embedding_json='[0,1]' WHERE chunk_id IN (SELECT id FROM memory_chunks WHERE user_id='alice')`); err != nil {
		t.Fatal(err)
	}
	results, err = searchMemoryChunkMatchesVector("alice", "alpha", 8)
	if err != nil || len(results) != 0 {
		t.Fatalf("stale cache/blob: %+v %v", results, err)
	}
	results, err = searchMemoryChunkMatchesVector("alice", "beta", 8)
	if err != nil || len(results) != 1 {
		t.Fatalf("updated vector: %+v %v", results, err)
	}
	SetBufferedEmbeddingProvider(BufferedEmbeddingProvider{ModelName: "other", Build: func(string) ([]float64, string, error) { return []float64{0, 1}, "other", nil }})
	results, err = searchMemoryChunkMatchesVector("alice", "beta", 8)
	if err != nil || len(results) != 0 {
		t.Fatalf("mixed embedding models: %+v %v", results, err)
	}
	if err := DeleteMemory("alice", alice); err != nil {
		t.Fatal(err)
	}
	results, err = searchMemoryChunkMatchesVector("alice", "beta", 8)
	if err != nil || len(results) != 0 {
		t.Fatalf("deleted cached row: %+v %v", results, err)
	}
}

func TestReindexFailurePreservesLiveVectors(t *testing.T) {
	openRetrievalTestDB(t)
	if _, err := InsertMemory("alice", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := InsertMemory("alice", "beta"); err != nil {
		t.Fatal(err)
	}
	calls := 0
	err := ReindexStoredEmbeddings("new", func(string) ([]float64, string, error) {
		calls++
		if calls == 2 {
			return nil, "", errors.New("inference failed")
		}
		return []float64{1, 0}, "new", nil
	})
	if err == nil {
		t.Fatal("expected failed inference")
	}
	var count int
	db.QueryRow(`SELECT count(*) FROM memory_chunk_embeddings WHERE embedding_model='test'`).Scan(&count)
	if count != 2 {
		t.Fatalf("live vectors replaced on failure: %d", count)
	}
	err = ReindexStoredEmbeddings("new", func(string) ([]float64, string, error) { return []float64{0, 1}, "new", nil })
	if err != nil {
		t.Fatal(err)
	}
	db.QueryRow(`SELECT count(*) FROM memory_chunk_embeddings WHERE embedding_model='new' AND length(embedding_blob)=8`).Scan(&count)
	if count != 2 {
		t.Fatalf("successful activation: %d", count)
	}
}

func TestReindexRejectsConcurrentMemoryChanges(t *testing.T) {
	openRetrievalTestDB(t)
	if _, err := InsertMemory("alice", "alpha"); err != nil {
		t.Fatal(err)
	}
	err := ReindexStoredEmbeddings("new", func(string) ([]float64, string, error) {
		_, err := db.Exec(`UPDATE memory_chunks SET chunk_text='changed' WHERE user_id='alice'`)
		return []float64{0, 1}, "new", err
	})
	if err == nil {
		t.Fatal("activated embeddings built from stale sources")
	}
	var model string
	db.QueryRow(`SELECT embedding_model FROM memory_chunk_embeddings`).Scan(&model)
	if model != "test" {
		t.Fatal(model)
	}
}

func TestQueryEmbeddingReusedAcrossSourcesAndResetOnProviderChange(t *testing.T) {
	openRetrievalTestDB(t)
	if _, err := InsertMemory("alice", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveSavedTurn("alice", "alpha question", "alpha answer"); err != nil {
		t.Fatal(err)
	}
	calls := 0
	provider := BufferedEmbeddingProvider{ModelName: "test", BuildWithUsage: func(string, BufferedEmbeddingUsage) ([]float64, string, error) {
		calls++
		return []float64{1, 0}, "test", nil
	}}
	SetBufferedEmbeddingProvider(provider)
	if _, err := buildMemoryCandidatesMode("alice", "alpha", 8, false); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("query embeddings=%d, want 1", calls)
	}
	SetBufferedEmbeddingProvider(provider)
	queryEmbedding("alpha")
	if calls != 2 {
		t.Fatal("provider change retained query cache")
	}
	context := FastMemoryContext("alice", "alpha")
	if context.UsedSynthesis || context.Context == "" || !strings.Contains(context.Context, "saved_turn") {
		t.Fatalf("missing raw memory/saved turn context: %+v", context)
	}
	SetBufferedEmbeddingProvider(provider)
	calls = 0
	context = FastMemoryContextWithOptions("alice", "alpha", false)
	if calls != 0 || context.Context == "" {
		t.Fatalf("disabled embeddings should use FTS without inference: calls=%d context=%+v", calls, context)
	}
}

func TestReindexPublishesProviderBeforeQueuedWrites(t *testing.T) {
	openRetrievalTestDB(t)
	if _, err := InsertMemory("alice", "alpha"); err != nil {
		t.Fatal(err)
	}
	provider := BufferedEmbeddingProvider{ModelName: "new", Build: func(string) ([]float64, string, error) { return []float64{0, 1}, "new", nil }}
	if err := ReindexStoredEmbeddings("new", provider.Build, provider); err != nil {
		t.Fatal(err)
	}
	if _, err := InsertMemory("alice", "new memory"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM memory_chunk_embeddings WHERE embedding_model='new'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("new writes used previous model: %d", count)
	}
}
