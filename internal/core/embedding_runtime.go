package core

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"dinkisstyle-chat/internal/mcp"

	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/text/unicode/norm"
)

const (
	embeddingWrapperManifestName = "text_embedding_wrapper.json"
	embeddingWrapperModelName    = "text_to_embedding.onnx"
	embeddingDirectModelName     = "model.onnx"
	embeddingDefaultMaxTokens    = 512
)

type embeddingUsage string

const (
	embeddingUsageQuery    embeddingUsage = "query"
	embeddingUsageDocument embeddingUsage = "document"
)

type embeddingRuntimeInfo struct {
	Ready   bool
	Backend string
	Message string
}

var embeddingRuntimeState = struct {
	mu      sync.RWMutex
	runtime *embeddingRuntime
	info    embeddingRuntimeInfo
}{
	info: embeddingRuntimeInfo{
		Ready:   false,
		Backend: "",
		Message: "Embedding runtime is disabled.",
	},
}

type embeddingRuntime struct {
	modelID string
	backend string
	encoder embeddingEncoder
}

type embeddingEncoder interface {
	Encode(text string, usage embeddingUsage) ([]float64, error)
	Close() error
}

type embeddingWrapperManifest struct {
	ModelFile       string `json:"model_file"`
	InputName       string `json:"input_name"`
	OutputName      string `json:"output_name"`
	NormalizeOutput bool   `json:"normalize_output"`
}

type embeddingWrapperEncoder struct {
	session         *ort.DynamicAdvancedSession
	inputName       string
	outputName      string
	normalizeOutput bool
}

type embeddingDirectEncoder struct {
	session   *ort.DynamicAdvancedSession
	tokenizer *unigramTokenizer
	maxTokens int
}

type tokenizerFile struct {
	AddedTokens []tokenizerAddedToken `json:"added_tokens"`
	PreTok      tokenizerPreTokenizer `json:"pre_tokenizer"`
	Model       tokenizerModel        `json:"model"`
}

type tokenizerAddedToken struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
	Special bool   `json:"special"`
}

type tokenizerPreTokenizer struct {
	Type           string `json:"type"`
	Replacement    string `json:"replacement"`
	AddPrefixSpace bool   `json:"add_prefix_space"`
}

type tokenizerModel struct {
	Type  string           `json:"type"`
	UnkID int              `json:"unk_id"`
	Vocab []tokenizerPiece `json:"vocab"`
}

type tokenizerPiece struct {
	Token string
	Score float64
	ID    int
}

func (p *tokenizerPiece) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) != 2 {
		return fmt.Errorf("unexpected vocab entry length: %d", len(raw))
	}
	if err := json.Unmarshal(raw[0], &p.Token); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &p.Score); err != nil {
		return err
	}
	return nil
}

type unigramNode struct {
	children map[rune]*unigramNode
	piece    *tokenizerPiece
}

type unigramTokenizer struct {
	root           *unigramNode
	unkID          int
	bosID          int
	eosID          int
	padID          int
	maxTokens      int
	addPrefixSpace bool
	replacement    string
}

func setEmbeddingRuntimeInfo(info embeddingRuntimeInfo) {
	embeddingRuntimeState.mu.Lock()
	defer embeddingRuntimeState.mu.Unlock()
	embeddingRuntimeState.info = info
}

func getEmbeddingRuntimeInfo() embeddingRuntimeInfo {
	embeddingRuntimeState.mu.RLock()
	defer embeddingRuntimeState.mu.RUnlock()
	return embeddingRuntimeState.info
}

func setEmbeddingRuntime(rt *embeddingRuntime) {
	embeddingRuntimeState.mu.Lock()
	defer embeddingRuntimeState.mu.Unlock()
	if embeddingRuntimeState.runtime != nil {
		_ = embeddingRuntimeState.runtime.Close()
	}
	embeddingRuntimeState.runtime = rt
}

func clearEmbeddingRuntime() {
	embeddingRuntimeState.mu.Lock()
	defer embeddingRuntimeState.mu.Unlock()
	if embeddingRuntimeState.runtime != nil {
		_ = embeddingRuntimeState.runtime.Close()
		embeddingRuntimeState.runtime = nil
	}
}

func getEmbeddingRuntime() *embeddingRuntime {
	embeddingRuntimeState.mu.RLock()
	defer embeddingRuntimeState.mu.RUnlock()
	return embeddingRuntimeState.runtime
}

func loadEmbeddingRuntime(cfg EmbeddingModelConfig) (*embeddingRuntime, embeddingRuntimeInfo, error) {
	cfg = normalizeEmbeddingConfig(cfg)
	modelDir := getEmbeddingModelInstallDir(cfg.ModelID)
	if err := InitializeONNXRuntime(); err != nil {
		info := embeddingRuntimeInfo{
			Ready:   false,
			Backend: "",
			Message: fmt.Sprintf("ONNX Runtime init failed: %v", err),
		}
		return nil, info, err
	}

	if rt, ok, err := tryLoadWrapperEmbeddingRuntime(cfg.ModelID, modelDir); err == nil && ok {
		info := embeddingRuntimeInfo{
			Ready:   true,
			Backend: rt.backend,
			Message: "Embedding runtime is ready with text-to-embedding wrapper ONNX.",
		}
		return rt, info, nil
	}

	rt, err := loadDirectEmbeddingRuntime(cfg.ModelID, modelDir)
	if err != nil {
		info := embeddingRuntimeInfo{
			Ready:   false,
			Backend: "",
			Message: fmt.Sprintf("Embedding runtime failed to load: %v", err),
		}
		return nil, info, err
	}
	info := embeddingRuntimeInfo{
		Ready:   true,
		Backend: rt.backend,
		Message: "Embedding runtime is ready with local tokenizer + direct ONNX encoder.",
	}
	return rt, info, nil
}

func tryLoadWrapperEmbeddingRuntime(modelID, modelDir string) (*embeddingRuntime, bool, error) {
	manifestPath := filepath.Join(modelDir, embeddingWrapperManifestName)
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, false, err
	}
	var manifest embeddingWrapperManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, false, fmt.Errorf("invalid wrapper manifest: %w", err)
	}
	modelFile := strings.TrimSpace(manifest.ModelFile)
	if modelFile == "" {
		modelFile = embeddingWrapperModelName
	}
	inputName := strings.TrimSpace(manifest.InputName)
	if inputName == "" {
		inputName = "text"
	}
	outputName := strings.TrimSpace(manifest.OutputName)
	if outputName == "" {
		outputName = "embedding"
	}

	session, err := ort.NewDynamicAdvancedSession(
		filepath.Join(modelDir, modelFile),
		[]string{inputName},
		[]string{outputName},
		nil,
	)
	if err != nil {
		return nil, false, err
	}

	return &embeddingRuntime{
		modelID: modelID,
		backend: "wrapper-onnx-string",
		encoder: &embeddingWrapperEncoder{
			session:         session,
			inputName:       inputName,
			outputName:      outputName,
			normalizeOutput: manifest.NormalizeOutput,
		},
	}, true, nil
}

func loadDirectEmbeddingRuntime(modelID, modelDir string) (*embeddingRuntime, error) {
	tokenizer, err := loadUnigramTokenizer(filepath.Join(modelDir, "tokenizer.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer.json: %w", err)
	}
	session, err := ort.NewDynamicAdvancedSession(
		filepath.Join(modelDir, embeddingDirectModelName),
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"last_hidden_state"},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load direct encoder model: %w", err)
	}

	return &embeddingRuntime{
		modelID: modelID,
		backend: "direct-onnx-unigram",
		encoder: &embeddingDirectEncoder{
			session:   session,
			tokenizer: tokenizer,
			maxTokens: tokenizer.maxTokens,
		},
	}, nil
}

func (r *embeddingRuntime) Close() error {
	if r == nil || r.encoder == nil {
		return nil
	}
	return r.encoder.Close()
}

func (r *embeddingRuntime) Build(text string, usage embeddingUsage) ([]float64, string, error) {
	if r == nil || r.encoder == nil {
		return nil, "", fmt.Errorf("embedding runtime is not loaded")
	}
	vector, err := r.encoder.Encode(text, usage)
	if err != nil {
		return nil, "", err
	}
	return vector, fmt.Sprintf("%s:%s", r.modelID, r.backend), nil
}

func (e *embeddingWrapperEncoder) Close() error {
	if e.session != nil {
		return e.session.Destroy()
	}
	return nil
}

func (e *embeddingWrapperEncoder) Encode(text string, usage embeddingUsage) ([]float64, error) {
	inputTensor, err := ort.NewStringTensor(ort.NewShape(1))
	if err != nil {
		return nil, err
	}
	defer inputTensor.Destroy()

	if err := inputTensor.SetContents([]string{prepareEmbeddingText(text, usage)}); err != nil {
		return nil, err
	}

	outputs := []ort.Value{nil}
	if err := e.session.Run([]ort.Value{inputTensor}, outputs); err != nil {
		return nil, err
	}
	if outputs[0] == nil {
		return nil, fmt.Errorf("wrapper model returned no embedding output")
	}
	defer outputs[0].Destroy()

	outputTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("wrapper model returned unexpected output type")
	}
	vector := make([]float64, len(outputTensor.GetData()))
	for i, value := range outputTensor.GetData() {
		vector[i] = float64(value)
	}
	if e.normalizeOutput {
		normalizeFloat64Vector(vector)
	}
	return vector, nil
}

func (e *embeddingDirectEncoder) Close() error {
	if e.session != nil {
		return e.session.Destroy()
	}
	return nil
}

func (e *embeddingDirectEncoder) Encode(text string, usage embeddingUsage) ([]float64, error) {
	tokenIDs := e.tokenizer.Encode(prepareEmbeddingText(text, usage), e.maxTokens)
	if len(tokenIDs) == 0 {
		return nil, fmt.Errorf("tokenizer returned no tokens")
	}
	attentionMask := make([]int64, len(tokenIDs))
	tokenTypeIDs := make([]int64, len(tokenIDs))
	for i := range attentionMask {
		attentionMask[i] = 1
	}

	shape := ort.NewShape(1, int64(len(tokenIDs)))
	inputIDsTensor, err := ort.NewTensor(shape, tokenIDs)
	if err != nil {
		return nil, err
	}
	defer inputIDsTensor.Destroy()

	attentionMaskTensor, err := ort.NewTensor(shape, attentionMask)
	if err != nil {
		return nil, err
	}
	defer attentionMaskTensor.Destroy()

	tokenTypeTensor, err := ort.NewTensor(shape, tokenTypeIDs)
	if err != nil {
		return nil, err
	}
	defer tokenTypeTensor.Destroy()

	outputs := []ort.Value{nil}
	if err := e.session.Run(
		[]ort.Value{inputIDsTensor, attentionMaskTensor, tokenTypeTensor},
		outputs,
	); err != nil {
		return nil, err
	}
	if outputs[0] == nil {
		return nil, fmt.Errorf("direct encoder returned no hidden state output")
	}
	defer outputs[0].Destroy()

	outputTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("direct encoder returned unexpected output type")
	}
	return meanPoolLastHiddenState(outputTensor, attentionMask)
}

func meanPoolLastHiddenState(tensor *ort.Tensor[float32], attentionMask []int64) ([]float64, error) {
	shape := tensor.GetShape()
	if len(shape) != 3 {
		return nil, fmt.Errorf("unexpected hidden state shape: %v", shape)
	}
	seqLen := int(shape[1])
	hiddenSize := int(shape[2])
	if seqLen <= 0 || hiddenSize <= 0 {
		return nil, fmt.Errorf("invalid hidden state shape: %v", shape)
	}
	data := tensor.GetData()
	if len(data) < seqLen*hiddenSize {
		return nil, fmt.Errorf("hidden state tensor is shorter than expected")
	}

	vector := make([]float64, hiddenSize)
	var count float64
	for tokenIndex := 0; tokenIndex < seqLen && tokenIndex < len(attentionMask); tokenIndex++ {
		if attentionMask[tokenIndex] == 0 {
			continue
		}
		count += 1
		base := tokenIndex * hiddenSize
		for dim := 0; dim < hiddenSize; dim++ {
			vector[dim] += float64(data[base+dim])
		}
	}
	if count == 0 {
		return nil, fmt.Errorf("attention mask produced zero active tokens")
	}
	for i := range vector {
		vector[i] /= count
	}
	normalizeFloat64Vector(vector)
	return vector, nil
}

func normalizeFloat64Vector(vector []float64) {
	var magnitude float64
	for _, value := range vector {
		magnitude += value * value
	}
	if magnitude == 0 {
		return
	}
	magnitude = math.Sqrt(magnitude)
	for i := range vector {
		vector[i] /= magnitude
	}
}

func prepareEmbeddingText(text string, usage embeddingUsage) string {
	text = strings.TrimSpace(text)
	switch usage {
	case embeddingUsageQuery:
		return "query: " + text
	case embeddingUsageDocument:
		return "passage: " + text
	default:
		return text
	}
}

func loadUnigramTokenizer(path string) (*unigramTokenizer, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file tokenizerFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	if !strings.EqualFold(file.Model.Type, "Unigram") {
		return nil, fmt.Errorf("unsupported tokenizer model type: %s", file.Model.Type)
	}

	bosID, eosID, padID := 0, 2, 1
	for _, token := range file.AddedTokens {
		switch token.Content {
		case "<s>":
			bosID = token.ID
		case "</s>":
			eosID = token.ID
		case "<pad>":
			padID = token.ID
		}
	}

	root := &unigramNode{children: make(map[rune]*unigramNode)}
	for i := range file.Model.Vocab {
		piece := &file.Model.Vocab[i]
		piece.ID = i
		if strings.HasPrefix(piece.Token, "<") && strings.HasSuffix(piece.Token, ">") {
			continue
		}
		insertUnigramPiece(root, piece)
	}

	replacement := file.PreTok.Replacement
	if replacement == "" {
		replacement = "▁"
	}

	return &unigramTokenizer{
		root:           root,
		unkID:          file.Model.UnkID,
		bosID:          bosID,
		eosID:          eosID,
		padID:          padID,
		maxTokens:      embeddingDefaultMaxTokens,
		addPrefixSpace: file.PreTok.AddPrefixSpace,
		replacement:    replacement,
	}, nil
}

func insertUnigramPiece(root *unigramNode, piece *tokenizerPiece) {
	node := root
	for _, r := range piece.Token {
		if node.children[r] == nil {
			node.children[r] = &unigramNode{children: make(map[rune]*unigramNode)}
		}
		node = node.children[r]
	}
	node.piece = piece
}

func (t *unigramTokenizer) Encode(text string, maxTokens int) []int64 {
	if maxTokens <= 0 {
		maxTokens = t.maxTokens
	}
	normalized := normalizeTokenizerText(text, t.addPrefixSpace, t.replacement)
	pieces := t.segmentUnigram(normalized)
	tokenIDs := make([]int64, 0, minInt(maxTokens, len(pieces)+2))
	tokenIDs = append(tokenIDs, int64(t.bosID))
	for _, piece := range pieces {
		if len(tokenIDs) >= maxTokens-1 {
			break
		}
		tokenIDs = append(tokenIDs, int64(piece.ID))
	}
	tokenIDs = append(tokenIDs, int64(t.eosID))
	return tokenIDs
}

func normalizeTokenizerText(text string, addPrefixSpace bool, replacement string) string {
	text = norm.NFKC.String(strings.TrimSpace(text))
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return replacement
	}
	if addPrefixSpace && !strings.HasPrefix(text, " ") {
		text = " " + text
	}
	return strings.ReplaceAll(text, " ", replacement)
}

func (t *unigramTokenizer) segmentUnigram(text string) []tokenizerPiece {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return nil
	}
	bestScore := make([]float64, n+1)
	bestPrev := make([]int, n+1)
	bestPiece := make([]*tokenizerPiece, n+1)
	for i := range bestScore {
		bestScore[i] = math.Inf(-1)
		bestPrev[i] = -1
	}
	bestScore[0] = 0

	for start := 0; start < n; start++ {
		if math.IsInf(bestScore[start], -1) {
			continue
		}
		node := t.root
		for end := start; end < n; end++ {
			next := node.children[runes[end]]
			if next == nil {
				break
			}
			node = next
			if node.piece != nil {
				score := bestScore[start] + node.piece.Score
				if score > bestScore[end+1] {
					bestScore[end+1] = score
					bestPrev[end+1] = start
					bestPiece[end+1] = node.piece
				}
			}
		}
		if bestPrev[start+1] == -1 {
			unknown := &tokenizerPiece{ID: t.unkID, Token: string(runes[start]), Score: -100}
			score := bestScore[start] + unknown.Score
			if score > bestScore[start+1] {
				bestScore[start+1] = score
				bestPrev[start+1] = start
				bestPiece[start+1] = unknown
			}
		}
	}

	out := make([]tokenizerPiece, 0, n)
	for index := n; index > 0; {
		piece := bestPiece[index]
		if piece == nil {
			_, width := utf8.DecodeRuneInString(string(runes[index-1]))
			if width <= 0 {
				width = 1
			}
			out = append(out, tokenizerPiece{ID: t.unkID, Token: string(runes[index-1])})
			index--
			continue
		}
		out = append(out, *piece)
		index = bestPrev[index]
		if index < 0 {
			break
		}
	}
	reverseTokenizerPieces(out)
	return out
}

func reverseTokenizerPieces(items []tokenizerPiece) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func installEmbeddingProvider(rt *embeddingRuntime) {
	if rt == nil {
		mcp.SetBufferedEmbeddingProvider(mcp.BufferedEmbeddingProvider{})
		return
	}
	mcp.SetBufferedEmbeddingProvider(mcp.BufferedEmbeddingProvider{
		ModelName: rt.modelID,
		BuildWithUsage: func(text string, usage mcp.BufferedEmbeddingUsage) ([]float64, string, error) {
			vector, modelName, err := rt.Build(text, embeddingUsage(usage))
			return vector, modelName, err
		},
	})
}
