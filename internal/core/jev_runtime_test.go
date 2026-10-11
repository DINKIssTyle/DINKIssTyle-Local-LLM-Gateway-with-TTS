package core

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dinkisstyle-chat/internal/mcp"
)

// Run with DKST_JEV_TEST_PYTHON and DKST_JEV_TEST_MODEL to exercise the actual
// installed runtime. Normal tests never download models or access user data.
func TestJevRuntimeIntegration(t *testing.T) {
	python, model := os.Getenv("DKST_JEV_TEST_PYTHON"), os.Getenv("DKST_JEV_TEST_MODEL")
	if python == "" || model == "" {
		t.Skip("local MediaPipe integration assets not specified")
	}
	worker, err := startJevEncoder(python, model, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { worker.Close() })
	for _, usage := range []embeddingUsage{embeddingUsageQuery, embeddingUsageDocument} {
		started := time.Now()
		vector, err := worker.Encode("사용자는 macOS에서 Go로 프로그램을 개발합니다.", usage)
		if err != nil {
			t.Fatal(err)
		}
		var norm float64
		for _, v := range vector {
			norm += v * v
		}
		if len(vector) != 256 || math.Abs(norm-1) > 1e-6 {
			t.Fatalf("invalid normalized MRL vector: dim=%d norm=%f", len(vector), norm)
		}
		t.Logf("%s embedding: %v", usage, time.Since(started))
	}
	// The previous 512-token worker fails this 800-character Korean fixture
	// with 1,608 tokens. Exercise both query and document paths with the real SDK.
	korean800 := string([]rune(strings.Repeat("한글문자뿕똠쀍꿿뢟🙂", 90))[:800])
	longDocument := strings.Repeat("뿕똠쀍꿿뢟🙂", 300) + "문서의 마지막 정보는 서울입니다."
	for _, fixture := range []struct {
		text  string
		usage embeddingUsage
	}{
		{korean800, embeddingUsageQuery},
		{korean800, embeddingUsageDocument},
		{longDocument, embeddingUsageDocument},
	} {
		started := time.Now()
		vector, err := worker.Encode(fixture.text, fixture.usage)
		if err != nil {
			t.Fatalf("long %s embedding: %v", fixture.usage, err)
		}
		var norm float64
		for _, v := range vector {
			norm += v * v
		}
		if len(vector) != 256 || math.Abs(norm-1) > 1e-6 {
			t.Fatalf("invalid long embedding: dim=%d norm=%f", len(vector), norm)
		}
		t.Logf("long %s embedding (%d characters): %v", fixture.usage, len([]rune(fixture.text)), time.Since(started))
	}
	// Run the same atomic reindex used by model activation against an isolated
	// database containing long memory and saved-conversation chunks.
	if err := mcp.InitDB(filepath.Join(t.TempDir(), "memory.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mcp.CloseDB)
	t.Cleanup(func() { installEmbeddingProvider(getEmbeddingRuntime()) })
	mcp.SetBufferedEmbeddingProvider(mcp.BufferedEmbeddingProvider{})
	if _, err := mcp.InsertMemory("integration", longDocument); err != nil {
		t.Fatal(err)
	}
	if _, err := mcp.SaveSavedTurn("integration", korean800, longDocument); err != nil {
		t.Fatal(err)
	}
	modelName := jevModelID + ":" + jevEmbeddingBackend
	provider := mcp.BufferedEmbeddingProvider{ModelName: modelName, BuildWithUsage: func(text string, usage mcp.BufferedEmbeddingUsage) ([]float64, string, error) {
		vector, err := worker.Encode(text, embeddingUsage(usage))
		return vector, modelName, err
	}}
	if err := mcp.ReindexStoredEmbeddings(modelName, func(text string) ([]float64, string, error) {
		return provider.BuildWithUsage(text, mcp.BufferedEmbeddingUsageDocument)
	}, provider); err != nil {
		t.Fatal(err)
	}
	if matches, err := mcp.SearchMemoryChunkMatches("integration", "서울", 8); err != nil || len(matches) == 0 {
		t.Fatalf("reindexed retrieval: %d matches, %v", len(matches), err)
	}
	selected, err := worker.Choose("사용자는 어떤 운영체제를 사용하나요?", map[string]string{"mac": "사용자는 macOS를 사용합니다.", "food": "사용자는 피자를 좋아합니다."})
	if err != nil {
		t.Fatal(err)
	}
	if selected != "mac" && selected != "food" {
		t.Fatalf("invalid decision choice: %q", selected)
	}
	t.Logf("JEV selected %q (semantic quality is not asserted by this protocol test)", selected)
	if err := worker.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := worker.Encode("closed", embeddingUsageQuery); err == nil {
		t.Fatal("closed worker accepted inference")
	}
}
