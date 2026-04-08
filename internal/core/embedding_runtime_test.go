package core

import "testing"

func TestNormalizeTokenizerText(t *testing.T) {
	got := normalizeTokenizerText("  hello   world  ", true, "▁")
	if got != "▁hello▁world" {
		t.Fatalf("unexpected normalized text: %q", got)
	}
}

func TestUnigramTokenizerEncodeAddsSpecialTokens(t *testing.T) {
	root := &unigramNode{children: make(map[rune]*unigramNode)}
	for i, piece := range []tokenizerPiece{
		{Token: "▁hello", Score: 10},
		{Token: "▁", Score: 1},
		{Token: "h", Score: 1},
		{Token: "e", Score: 1},
		{Token: "l", Score: 1},
		{Token: "o", Score: 1},
	} {
		p := piece
		p.ID = i + 10
		insertUnigramPiece(root, &p)
	}
	tokenizer := &unigramTokenizer{
		root:           root,
		unkID:          3,
		bosID:          0,
		eosID:          2,
		padID:          1,
		maxTokens:      16,
		addPrefixSpace: true,
		replacement:    "▁",
	}

	ids := tokenizer.Encode("hello", 16)
	if len(ids) != 3 {
		t.Fatalf("expected bos + piece + eos, got %v", ids)
	}
	if ids[0] != 0 || ids[1] != 10 || ids[2] != 2 {
		t.Fatalf("unexpected token sequence: %v", ids)
	}
}

func TestPrepareEmbeddingTextUsesE5Prefixes(t *testing.T) {
	if got := prepareEmbeddingText("weather in seoul", embeddingUsageQuery); got != "query: weather in seoul" {
		t.Fatalf("unexpected query prefix: %q", got)
	}
	if got := prepareEmbeddingText("city weather summary", embeddingUsageDocument); got != "passage: city weather summary" {
		t.Fatalf("unexpected document prefix: %q", got)
	}
}

func TestLoadDirectEmbeddingRuntimeWithInstalledModel(t *testing.T) {
	modelDir := getEmbeddingModelInstallDir("multilingual-e5-small")
	if !isEmbeddingModelInstalled("multilingual-e5-small") {
		t.Skip("multilingual-e5-small is not installed on this machine")
	}
	if err := InitializeONNXRuntime(); err != nil {
		t.Skipf("onnx runtime is not available in test environment: %v", err)
	}
	rt, err := loadDirectEmbeddingRuntime("multilingual-e5-small", modelDir)
	if err != nil {
		t.Fatalf("failed to load direct embedding runtime: %v", err)
	}
	defer rt.Close()

	vector, modelName, err := rt.Build("서울의 봄 날씨 요약", embeddingUsageDocument)
	if err != nil {
		t.Fatalf("failed to build embedding: %v", err)
	}
	if len(vector) == 0 {
		t.Fatalf("expected non-empty embedding vector")
	}
	if modelName == "" {
		t.Fatalf("expected non-empty model name")
	}
}
