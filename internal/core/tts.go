/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 *
 * Supertonic TTS Integration
 * Based on: https://github.com/supertone-inc/supertonic
 */

package core

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/sjzar/go-lame"
	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/text/unicode/norm"
)

var (
	reTTSHeading        = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+?)([.!?]?)\s*$`)
	reTTSBulletList     = regexp.MustCompile(`(?m)^\s*[-*+]\s+(.+?)\s*$`)
	reTTSOrderedList    = regexp.MustCompile(`(?m)^\s*(\d+)[\.\)]\s+(.+?)\s*$`)
	reTTSHorizontalRule = regexp.MustCompile(`(?m)^([-*_]){3,}\s*$`)
	reTTSParagraphBreak = regexp.MustCompile(`\n\s*\n+`)
	reTTSLineBreak      = regexp.MustCompile(`([^\s.!?])\n`)
	reTTSNewlines       = regexp.MustCompile(`\n+`)
	reTTSAllowedChars   = regexp.MustCompile(`[^\p{L}\p{M}\p{N}\s.,!?:;'"\(\)\[\]\-]`)
	reTTSWhitespace     = regexp.MustCompile(`\s+`)
	reTTSPeriodPause    = regexp.MustCompile(`\.(\s+)([A-Z가-힣])`)
	reTTSExclaimPause   = regexp.MustCompile(`([!?])(\s+)`)
	reTTSColonPause     = regexp.MustCompile(`([:;])(\s*)`)
	reTTSEqualsSign     = regexp.MustCompile(`\s*=\s*`)
	reTTSTrailingPunct  = regexp.MustCompile(`[.!?;:,'"]$`)
	ttsPunctReplacer    = strings.NewReplacer(
		"_", " ", "“", "\"", "”", "\"", "‘", "'", "’", "'",
		"–", "-", "—", "-", "−", "-", "。", ".", "！", "!", "？", "?",
	)
)

// Available languages for Supertonic 3 multilingual TTS.
var AvailableLangs = []string{
	"en", "ko", "ja", "ar", "bg", "cs", "da", "de",
	"el", "es", "et", "fi", "fr", "hi", "hr", "hu",
	"id", "it", "lt", "lv", "nl", "pl", "pt", "ro",
	"ru", "sk", "sl", "sv", "tr", "uk", "vi",
}

// Config structures for TTS
type SpecProcessorConfig struct {
	NFFT      int     `json:"n_fft"`
	WinLength int     `json:"win_length"`
	HopLength int     `json:"hop_length"`
	NMels     int     `json:"n_mels"`
	Eps       float64 `json:"eps"`
	NormMean  float64 `json:"norm_mean"`
	NormStd   float64 `json:"norm_std"`
}

type EncoderConfig struct {
	SpecProcessor SpecProcessorConfig `json:"spec_processor"`
}

type AEConfig struct {
	SampleRate    int           `json:"sample_rate"`
	BaseChunkSize int           `json:"base_chunk_size"`
	Encoder       EncoderConfig `json:"encoder"`
}

type StyleTokenLayerConfig struct {
	NStyle        int `json:"n_style"`
	StyleValueDim int `json:"style_value_dim"`
}

type StyleEncoderConfig struct {
	StyleTokenLayer StyleTokenLayerConfig `json:"style_token_layer"`
}

type ProjOutConfig struct {
	Idim int `json:"idim"`
	Odim int `json:"odim"`
}

type TextEncoderConfig struct {
	ProjOut ProjOutConfig `json:"proj_out"`
}

type TTLConfig struct {
	ChunkCompressFactor int                `json:"chunk_compress_factor"`
	LatentDim           int                `json:"latent_dim"`
	StyleEncoder        StyleEncoderConfig `json:"style_encoder"`
	TextEncoder         TextEncoderConfig  `json:"text_encoder"`
}

type DPStyleEncoderConfig struct {
	StyleTokenLayer StyleTokenLayerConfig `json:"style_token_layer"`
}

type DPConfig struct {
	LatentDim           int                  `json:"latent_dim"`
	ChunkCompressFactor int                  `json:"chunk_compress_factor"`
	StyleEncoder        DPStyleEncoderConfig `json:"style_encoder"`
}

type TTSConfig struct {
	AE  AEConfig  `json:"ae"`
	TTL TTLConfig `json:"ttl"`
	DP  DPConfig  `json:"dp"`
}

// VoiceStyleData holds voice style JSON structure
type VoiceStyleData struct {
	StyleTTL struct {
		Data [][][]float64 `json:"data"`
		Dims []int64       `json:"dims"`
		Type string        `json:"type"`
	} `json:"style_ttl"`
	StyleDP struct {
		Data [][][]float64 `json:"data"`
		Dims []int64       `json:"dims"`
		Type string        `json:"type"`
	} `json:"style_dp"`
}

// Style holds style tensors
type Style struct {
	TtlTensor *ort.Tensor[float32]
	DpTensor  *ort.Tensor[float32]
}

func (s *Style) Destroy() {
	if s.TtlTensor != nil {
		s.TtlTensor.Destroy()
	}
	if s.DpTensor != nil {
		s.DpTensor.Destroy()
	}
}

// UnicodeProcessor for text processing
type UnicodeProcessor struct {
	indexer []int64
}

func NewUnicodeProcessor(unicodeIndexerPath string) (*UnicodeProcessor, error) {
	indexer, err := loadJSONInt64(unicodeIndexerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load unicode indexer: %w", err)
	}
	return &UnicodeProcessor{indexer: indexer}, nil
}

func (up *UnicodeProcessor) Call(textList []string, langList []string) ([][]int64, [][][]float64) {
	processedTexts := make([]string, len(textList))
	for i, text := range textList {
		processedTexts[i] = preprocessText(text, langList[i])
	}

	textLengths := make([]int64, len(processedTexts))
	maxLen := 0
	for i, text := range processedTexts {
		textLengths[i] = int64(len([]rune(text)))
		if int(textLengths[i]) > maxLen {
			maxLen = int(textLengths[i])
		}
	}

	textIDs := make([][]int64, len(processedTexts))
	for i, text := range processedTexts {
		row := make([]int64, maxLen)
		runes := []rune(text)
		for j, r := range runes {
			unicodeVal := int(r)
			if unicodeVal < len(up.indexer) {
				row[j] = up.indexer[unicodeVal]
			} else {
				row[j] = -1
			}
		}
		textIDs[i] = row
	}

	textMask := lengthToMask(textLengths, maxLen)
	return textIDs, textMask
}

// TextToSpeech generates speech from text
type TextToSpeech struct {
	cfg           TTSConfig
	textProcessor *UnicodeProcessor
	dpOrt         *ort.DynamicAdvancedSession
	textEncOrt    *ort.DynamicAdvancedSession
	vectorEstOrt  *ort.DynamicAdvancedSession
	vocoderOrt    *ort.DynamicAdvancedSession
	SampleRate    int
	baseChunkSize int
	chunkCompress int
	ldim          int
}

func (tts *TextToSpeech) Destroy() {
	if tts.dpOrt != nil {
		tts.dpOrt.Destroy()
	}
	if tts.textEncOrt != nil {
		tts.textEncOrt.Destroy()
	}
	if tts.vectorEstOrt != nil {
		tts.vectorEstOrt.Destroy()
	}
	if tts.vocoderOrt != nil {
		tts.vocoderOrt.Destroy()
	}
}

// LoadTextToSpeech loads TTS components
func LoadTextToSpeech(onnxDir string, cfg TTSConfig, threads int) (*TextToSpeech, error) {
	threads = normalizeTTSThreads(threads)
	fmt.Printf("Loading TTS models (CPU mode, Threads: %d)\n", threads)

	dpPath := filepath.Join(onnxDir, "duration_predictor.onnx")
	textEncPath := filepath.Join(onnxDir, "text_encoder.onnx")
	vectorEstPath := filepath.Join(onnxDir, "vector_estimator.onnx")
	vocoderPath := filepath.Join(onnxDir, "vocoder.onnx")

	// Optimization: Set session options for multi-threading
	so, _ := ort.NewSessionOptions()
	defer so.Destroy()
	so.SetGraphOptimizationLevel(ort.GraphOptimizationLevelEnableAll)
	so.SetIntraOpNumThreads(threads) // Use configured threads
	so.SetInterOpNumThreads(1)

	dpOrt, err := ort.NewDynamicAdvancedSession(dpPath, []string{"text_ids", "style_dp", "text_mask"},
		[]string{"duration"}, so)
	if err != nil {
		return nil, fmt.Errorf("failed to load duration predictor: %w", err)
	}

	textEncOrt, err := ort.NewDynamicAdvancedSession(textEncPath, []string{"text_ids", "style_ttl", "text_mask"},
		[]string{"text_emb"}, so)
	if err != nil {
		return nil, fmt.Errorf("failed to load text encoder: %w", err)
	}

	vectorEstOrt, err := ort.NewDynamicAdvancedSession(vectorEstPath,
		[]string{"noisy_latent", "text_emb", "style_ttl", "latent_mask", "text_mask", "current_step", "total_step"},
		[]string{"denoised_latent"}, so)
	if err != nil {
		return nil, fmt.Errorf("failed to load vector estimator: %w", err)
	}

	vocoderOrt, err := ort.NewDynamicAdvancedSession(vocoderPath, []string{"latent"},
		[]string{"wav_tts"}, so)
	if err != nil {
		return nil, fmt.Errorf("failed to load vocoder: %w", err)
	}

	unicodeIndexerPath := filepath.Join(onnxDir, "unicode_indexer.json")
	textProcessor, err := NewUnicodeProcessor(unicodeIndexerPath)
	if err != nil {
		return nil, err
	}

	return &TextToSpeech{
		cfg:           cfg,
		textProcessor: textProcessor,
		dpOrt:         dpOrt,
		textEncOrt:    textEncOrt,
		vectorEstOrt:  vectorEstOrt,
		vocoderOrt:    vocoderOrt,
		SampleRate:    cfg.AE.SampleRate,
		baseChunkSize: cfg.AE.BaseChunkSize,
		chunkCompress: cfg.TTL.ChunkCompressFactor,
		ldim:          cfg.TTL.LatentDim,
	}, nil
}

func normalizeTTSThreads(threads int) int {
	if threads < 1 || threads > 4 {
		return 4
	}
	return threads
}

func normalizeTTSSpeed(speed float32) float32 {
	if speed <= 0 {
		return 0.9
	}
	if speed < 0.7 {
		return 0.7
	}
	if speed > 2.0 {
		return 2.0
	}
	return speed
}

// LoadVoiceStyle loads voice style from JSON file
func LoadVoiceStyle(voiceStylePath string) (*Style, error) {
	data, err := os.ReadFile(voiceStylePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read voice style file: %w", err)
	}

	var voiceStyle VoiceStyleData
	if err := json.Unmarshal(data, &voiceStyle); err != nil {
		return nil, fmt.Errorf("failed to parse voice style JSON: %w", err)
	}

	ttlDims := voiceStyle.StyleTTL.Dims
	dpDims := voiceStyle.StyleDP.Dims

	// Flatten TTL data
	ttlFlat := make([]float32, 0)
	for _, batch := range voiceStyle.StyleTTL.Data {
		for _, row := range batch {
			for _, val := range row {
				ttlFlat = append(ttlFlat, float32(val))
			}
		}
	}

	// Flatten DP data
	dpFlat := make([]float32, 0)
	for _, batch := range voiceStyle.StyleDP.Data {
		for _, row := range batch {
			for _, val := range row {
				dpFlat = append(dpFlat, float32(val))
			}
		}
	}

	ttlTensor, err := ort.NewTensor(ttlDims, ttlFlat)
	if err != nil {
		return nil, fmt.Errorf("failed to create TTL tensor: %w", err)
	}

	dpTensor, err := ort.NewTensor(dpDims, dpFlat)
	if err != nil {
		ttlTensor.Destroy()
		return nil, fmt.Errorf("failed to create DP tensor: %w", err)
	}

	return &Style{TtlTensor: ttlTensor, DpTensor: dpTensor}, nil
}

// Call synthesizes speech from text
func (tts *TextToSpeech) Call(ctx context.Context, text string, lang string, style *Style, totalStep int, speed float32, maxLen int) ([]float32, float32, error) {
	if maxLen == 0 {
		maxLen = 1000
	}
	// Logic adapted from Supertonic's Python implementation
	// Chunk text to avoid degradation on long inputs
	chunks := chunkText(text, maxLen)

	fmt.Printf("%s TTS: Split text into %d chunks\n", time.Now().Format("15:04:05.000"), len(chunks))

	var combinedWav []float32
	var totalDuration float32

	// Silence padding (0.3s)
	silenceSamples := int(0.3 * float32(tts.SampleRate))
	silence := make([]float32, silenceSamples)

	for i, chunk := range chunks {
		fmt.Printf("%s Processing chunk %d/%d: %s\n", time.Now().Format("15:04:05.000"), i+1, len(chunks), chunk)
		start := time.Now()
		wav, duration, err := tts.infer(ctx, []string{chunk}, []string{lang}, style, totalStep, speed)
		elapsed := time.Since(start).Seconds()
		if err != nil {
			return nil, 0, err
		}

		// Calculate actual samples based on duration
		dur := duration[0]
		wavLen := int(float32(tts.SampleRate) * dur)

		// Safety check for bounds
		if wavLen > len(wav) {
			wavLen = len(wav)
		}
		wavChunk := wav[:wavLen]

		if i > 0 {
			combinedWav = append(combinedWav, silence...)
			totalDuration += 0.3
		}

		combinedWav = append(combinedWav, wavChunk...)
		totalDuration += dur
		fmt.Printf("%s TTS: Chunk %d/%d completed (Audio: %.2fs, Processing: %.2fs)\n", time.Now().Format("15:04:05.000"), i+1, len(chunks), dur, elapsed)
	}

	return combinedWav, totalDuration, nil
}

// CallPrechunked synthesizes one client-prepared playback chunk without
// applying the SDK's semantic chunker a second time. The HTTP handler checks
// the language-specific safety limit before calling this method.
func (tts *TextToSpeech) CallPrechunked(ctx context.Context, text string, lang string, style *Style, totalStep int, speed float32) ([]float32, float32, error) {
	wav, duration, err := tts.infer(ctx, []string{text}, []string{lang}, style, totalStep, speed)
	if err != nil {
		return nil, 0, err
	}

	dur := duration[0]
	wavLen := int(float32(tts.SampleRate) * dur)
	if wavLen > len(wav) {
		wavLen = len(wav)
	}
	return wav[:wavLen], dur, nil
}

func (tts *TextToSpeech) infer(ctx context.Context, textList []string, langList []string, style *Style, totalStep int, speed float32) ([]float32, []float32, error) {
	bsz := len(textList)
	runOptions, err := ort.NewRunOptions()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create ONNX run options: %w", err)
	}
	stopCancellationWatch := make(chan struct{})
	cancellationWatchDone := make(chan struct{})
	go func() {
		defer close(cancellationWatchDone)
		select {
		case <-ctx.Done():
			_ = runOptions.Terminate()
		case <-stopCancellationWatch:
		}
	}()
	defer func() {
		close(stopCancellationWatch)
		<-cancellationWatchDone
		_ = runOptions.Destroy()
	}()

	textIDs, textMask := tts.textProcessor.Call(textList, langList)
	textIDsShape := []int64{int64(bsz), int64(len(textIDs[0]))}
	textMaskShape := []int64{int64(bsz), 1, int64(len(textMask[0][0]))}

	textIDsTensor, err := intArrayToTensor(textIDs, textIDsShape)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create text ids tensor: %w", err)
	}
	defer textIDsTensor.Destroy()
	textMaskTensor, err := arrayToTensor(textMask, textMaskShape)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create text mask tensor: %w", err)
	}
	defer textMaskTensor.Destroy()

	// Predict duration
	dpOutputs := make([]ort.ArbitraryTensor, 1)
	err = tts.dpOrt.RunWithOptions(
		[]ort.ArbitraryTensor{textIDsTensor, style.DpTensor, textMaskTensor},
		dpOutputs,
		runOptions,
	)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, fmt.Errorf("failed to run duration predictor: %w", err)
	}
	durTensor := dpOutputs[0].(*ort.Tensor[float32])
	durOnnx := slices.Clone(durTensor.GetData())
	durTensor.Destroy()

	for i := range durOnnx {
		durOnnx[i] /= speed
	}

	// Encode text (reuse textIDsTensor and textMaskTensor)
	textEncOutputs := make([]ort.ArbitraryTensor, 1)
	err = tts.textEncOrt.RunWithOptions(
		[]ort.ArbitraryTensor{textIDsTensor, style.TtlTensor, textMaskTensor},
		textEncOutputs,
		runOptions,
	)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, fmt.Errorf("failed to run text encoder: %w", err)
	}
	textEmbTensor := textEncOutputs[0].(*ort.Tensor[float32])
	defer textEmbTensor.Destroy()

	// Sample noisy latent directly into flat float32 buffer
	latentFlat, latentShape, latentMask := tts.sampleNoisyLatentFlat(durOnnx)
	latentMaskShape := []int64{int64(bsz), 1, int64(len(latentMask[0][0]))}

	totalStepArray := make([]float32, bsz)
	for b := 0; b < bsz; b++ {
		totalStepArray[b] = float32(totalStep)
	}
	scalarShape := []int64{int64(bsz)}
	totalStepTensor, err := ort.NewTensor(scalarShape, totalStepArray)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create total step tensor: %w", err)
	}
	defer totalStepTensor.Destroy()

	// Optimization: Move invariant tensor creation OUT of the loop
	latentMaskTensor, err := arrayToTensor(latentMask, latentMaskShape)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create latent mask tensor: %w", err)
	}
	defer latentMaskTensor.Destroy()

	latentTensor, err := ort.NewTensor(latentShape, latentFlat)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create noisy latent tensor: %w", err)
	}
	defer func() {
		if latentTensor != nil {
			latentTensor.Destroy()
		}
	}()

	currentStepArray := make([]float32, bsz)

	// Denoising loop: pass denoised output tensor directly into the next step and vocoder
	for step := 0; step < totalStep; step++ {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		for b := 0; b < bsz; b++ {
			currentStepArray[b] = float32(step)
		}

		currentStepTensor, err := ort.NewTensor(scalarShape, currentStepArray)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create current step tensor: %w", err)
		}

		vectorEstOutputs := make([]ort.ArbitraryTensor, 1)
		err = tts.vectorEstOrt.RunWithOptions(
			[]ort.ArbitraryTensor{latentTensor, textEmbTensor, style.TtlTensor, latentMaskTensor, textMaskTensor,
				currentStepTensor, totalStepTensor},
			vectorEstOutputs,
			runOptions,
		)
		currentStepTensor.Destroy()
		if err != nil {
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			return nil, nil, fmt.Errorf("failed to run vector estimator: %w", err)
		}

		denoisedTensor := vectorEstOutputs[0].(*ort.Tensor[float32])
		latentTensor.Destroy()
		latentTensor = denoisedTensor
	}

	// Generate waveform directly from final denoised latent tensor
	vocoderOutputs := make([]ort.ArbitraryTensor, 1)
	err = tts.vocoderOrt.RunWithOptions(
		[]ort.ArbitraryTensor{latentTensor},
		vocoderOutputs,
		runOptions,
	)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, fmt.Errorf("failed to run vocoder: %w", err)
	}

	wavBatchTensor := vocoderOutputs[0].(*ort.Tensor[float32])
	wav := slices.Clone(wavBatchTensor.GetData())
	wavBatchTensor.Destroy()

	return wav, durOnnx, nil
}

func (tts *TextToSpeech) sampleNoisyLatentFlat(durOnnx []float32) ([]float32, []int64, [][][]float64) {
	bsz := len(durOnnx)
	maxDur := float64(0)
	for _, d := range durOnnx {
		if float64(d) > maxDur {
			maxDur = float64(d)
		}
	}

	wavLenMax := maxDur * float64(tts.SampleRate)
	wavLengths := make([]int64, bsz)
	for i, d := range durOnnx {
		wavLengths[i] = int64(float64(d) * float64(tts.SampleRate))
	}

	chunkSize := tts.baseChunkSize * tts.chunkCompress
	latentLen := int((wavLenMax + float64(chunkSize) - 1) / float64(chunkSize))
	latentDim := tts.ldim * tts.chunkCompress

	latentMask := tts.getLatentMask(wavLengths)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	flat := make([]float32, bsz*latentDim*latentLen)
	idx := 0
	for b := 0; b < bsz; b++ {
		maskRow := latentMask[b][0]
		maskLen := len(maskRow)
		for d := 0; d < latentDim; d++ {
			for t := 0; t < latentLen; t++ {
				val := rng.NormFloat64()
				if t < maskLen {
					val *= maskRow[t]
				}
				flat[idx] = float32(val)
				idx++
			}
		}
	}

	return flat, []int64{int64(bsz), int64(latentDim), int64(latentLen)}, latentMask
}

func (tts *TextToSpeech) getLatentMask(wavLengths []int64) [][][]float64 {
	baseChunkSize := int64(tts.baseChunkSize)
	chunkCompressFactor := int64(tts.chunkCompress)
	latentSize := baseChunkSize * chunkCompressFactor

	latentLengths := make([]int64, len(wavLengths))
	maxLen := int64(0)
	for i, wavLen := range wavLengths {
		latentLengths[i] = (wavLen + latentSize - 1) / latentSize
		if latentLengths[i] > maxLen {
			maxLen = latentLengths[i]
		}
	}

	return lengthToMask(latentLengths, int(maxLen))
}

// LoadTTSConfig loads configuration from JSON file
func LoadTTSConfig(onnxDir string) (TTSConfig, error) {
	cfgPath := filepath.Join(onnxDir, "tts.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return TTSConfig{}, err
	}

	var cfg TTSConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return TTSConfig{}, err
	}

	return cfg, nil
}

// GenerateWAV generates 16-bit mono PCM WAV bytes directly in a single allocation.
func GenerateWAV(audioData []float32, sampleRate int) ([]byte, error) {
	if sampleRate <= 0 {
		return nil, fmt.Errorf("invalid sample rate: %d", sampleRate)
	}
	const (
		numChannels   = 1
		bitsPerSample = 16
		headerSize    = 44
	)
	dataSize := len(audioData) * (bitsPerSample / 8)
	buf := make([]byte, headerSize+dataSize)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+dataSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16) // PCM fmt chunk size
	binary.LittleEndian.PutUint16(buf[20:22], 1)  // PCM format
	binary.LittleEndian.PutUint16(buf[22:24], numChannels)
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	byteRate := sampleRate * numChannels * (bitsPerSample / 8)
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	blockAlign := numChannels * (bitsPerSample / 8)
	binary.LittleEndian.PutUint16(buf[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:36], bitsPerSample)
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))

	offset := headerSize
	for _, sample := range audioData {
		clamped := math.Max(-1.0, math.Min(1.0, float64(sample)))
		val := int16(clamped * 32767)
		binary.LittleEndian.PutUint16(buf[offset:offset+2], uint16(val))
		offset += 2
	}

	return buf, nil
}

// GenerateMP3 generates MP3 bytes from audio data using LAME encoder
func GenerateMP3(audioData []float32, sampleRate int, bitrate int) ([]byte, error) {
	// Convert float32 samples to int16 PCM bytes (little-endian)
	pcmBytes := make([]byte, len(audioData)*2)
	for i, sample := range audioData {
		clamped := math.Max(-1.0, math.Min(1.0, float64(sample)))
		val := int16(clamped * 32767)
		pcmBytes[i*2] = byte(val)
		pcmBytes[i*2+1] = byte(val >> 8)
	}

	// Create LAME encoder
	enc := lame.Init()
	if enc == nil {
		return nil, fmt.Errorf("failed to initialize LAME encoder")
	}
	defer enc.Close()

	// Set encoder parameters
	enc.SetInSamplerate(sampleRate)
	enc.SetNumChannels(1)
	enc.SetBitrate(bitrate) // Variable bitrate
	enc.SetQuality(2)       // High quality (0=best, 9=worst)

	if enc.InitParams() < 0 {
		return nil, fmt.Errorf("failed to init LAME params")
	}

	// Encode PCM to MP3
	mp3Data := enc.Encode(pcmBytes)

	// Flush encoder
	flush := enc.Flush()

	return append(mp3Data, flush...), nil
}

// GenerateAudio generates audio bytes in the specified format
// GenerateAudio generates audio bytes in the specified format
func GenerateAudio(audioData []float32, sampleRate int, format string) ([]byte, string, error) {
	switch strings.ToLower(format) {
	case "mp3-low":
		// Low bandwidth mode: 32kbps
		data, err := GenerateMP3(audioData, sampleRate, 32)
		return data, "audio/mpeg", err
	case "mp3-medium":
		// Medium bandwidth mode: 96kbps
		data, err := GenerateMP3(audioData, sampleRate, 96)
		return data, "audio/mpeg", err
	case "mp3", "mp3-high":
		// Standard/High quality: 128kbps
		data, err := GenerateMP3(audioData, sampleRate, 128)
		return data, "audio/mpeg", err
	case "wav":
		fallthrough
	default:
		data, err := GenerateWAV(audioData, sampleRate)
		return data, "audio/wav", err
	}
}

// Helper functions
func preprocessText(text string, lang string) string {
	// Keep syllables composed while applying language-sensitive punctuation rules.
	text = norm.NFKC.String(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Convert markdown structure into explicit pause boundaries before stripping symbols.
	text = reTTSHeading.ReplaceAllString(text, "$2$3\n\n")
	text = reTTSBulletList.ReplaceAllString(text, "$1.\n")
	text = reTTSOrderedList.ReplaceAllString(text, "$1. $2.\n")
	text = reTTSHorizontalRule.ReplaceAllString(text, "\n\n")

	text = reTTSParagraphBreak.ReplaceAllString(text, ". . ")
	text = reTTSLineBreak.ReplaceAllString(text, "$1. ")
	text = reTTSNewlines.ReplaceAllString(text, ", ")

	// Map punctuation before filtering so contractions and ranges survive.
	text = ttsPunctReplacer.Replace(text)
	text = reTTSAllowedChars.ReplaceAllString(text, " ")

	// First normalize all whitespace to single spaces
	text = reTTSWhitespace.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// Add natural pauses after punctuation marks by duplicating punctuation
	// This technique works with many TTS engines to create brief pauses
	// Period: add comma for pause between sentences
	text = reTTSPeriodPause.ReplaceAllString(text, "., $2")
	// Exclamation/Question: add period for pause
	text = reTTSExclaimPause.ReplaceAllString(text, "$1, ")
	// Colon/Semicolon: add comma for medium pause
	text = reTTSColonPause.ReplaceAllString(text, "$1, ")
	// Equals sign in context: ensure space
	text = reTTSEqualsSign.ReplaceAllString(text, " - ")

	if text != "" && !reTTSTrailingPunct.MatchString(text) {
		text += "."
	}

	if !isValidLang(lang) {
		lang = "en"
	}

	// The model consumes decomposed Unicode, including accent marks and Hangul Jamo.
	text = fmt.Sprintf("<%s>%s</%s>", lang, norm.NFKD.String(text), lang)
	return text
}

func isValidLang(lang string) bool {
	for _, l := range AvailableLangs {
		if l == lang {
			return true
		}
	}
	return false
}

func chunkText(text string, maxLen int) []string {
	if maxLen <= 0 {
		maxLen = 1000
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}

	// Keep paragraph prosody intact. Only paragraphs above the hard limit are
	// wrapped at whitespace; punctuation is never used as a chunk boundary.
	paragraphs := reTTSParagraphBreak.Split(text, -1)
	var finalChunks []string

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		finalChunks = append(finalChunks, hardWrapTTSParagraph(p, maxLen)...)
	}

	if len(finalChunks) == 0 {
		return []string{text}
	}

	return finalChunks
}

func hardWrapTTSParagraph(paragraph string, maxLen int) []string {
	runes := []rune(strings.TrimSpace(paragraph))
	var chunks []string

	for len(runes) > maxLen {
		splitAt := maxLen
		consumeAt := maxLen
		for i := maxLen - 1; i >= 0; i-- {
			if unicode.IsSpace(runes[i]) {
				splitAt = i
				consumeAt = i + 1
				break
			}
		}
		if splitAt == 0 {
			splitAt = maxLen
			consumeAt = maxLen
		}

		chunk := strings.TrimSpace(string(runes[:splitAt]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		runes = []rune(strings.TrimLeftFunc(string(runes[consumeAt:]), unicode.IsSpace))
	}

	if tail := strings.TrimSpace(string(runes)); tail != "" {
		chunks = append(chunks, tail)
	}
	return chunks
}

func lengthToMask(lengths []int64, maxLen int) [][][]float64 {
	bsz := len(lengths)
	mask := make([][][]float64, bsz)

	for i := 0; i < bsz; i++ {
		row := make([]float64, maxLen)
		for j := 0; j < maxLen; j++ {
			if int64(j) < lengths[i] {
				row[j] = 1.0
			} else {
				row[j] = 0.0
			}
		}
		mask[i] = [][]float64{row}
	}

	return mask
}

func loadJSONInt64(filePath string) ([]int64, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var result []int64
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func arrayToTensor(array [][][]float64, shape []int64) (*ort.Tensor[float32], error) {
	totalSize := int64(1)
	for _, dim := range shape {
		totalSize *= dim
	}

	flat := make([]float32, totalSize)
	idx := 0
	for b := 0; b < len(array); b++ {
		for d := 0; d < len(array[b]); d++ {
			for t := 0; t < len(array[b][d]); t++ {
				flat[idx] = float32(array[b][d][t])
				idx++
			}
		}
	}

	tensor, err := ort.NewTensor(shape, flat)
	if err != nil {
		return nil, err
	}

	return tensor, nil
}

func intArrayToTensor(array [][]int64, shape []int64) (*ort.Tensor[int64], error) {
	totalSize := int64(1)
	for _, dim := range shape {
		totalSize *= dim
	}

	flat := make([]int64, totalSize)
	idx := 0
	for b := 0; b < len(array); b++ {
		for t := 0; t < len(array[b]); t++ {
			flat[idx] = array[b][t]
			idx++
		}
	}

	tensor, err := ort.NewTensor(shape, flat)
	if err != nil {
		return nil, err
	}

	return tensor, nil
}

// InitializeONNXRuntime initializes ONNX Runtime environment
// InitializeONNXRuntime initializes ONNX Runtime environment
func InitializeONNXRuntime() error {
	// Debug logging - Disabled per user request
	// logFile, _ := os.OpenFile(GetResourcePath("debug.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	// if logFile != nil {
	// 	defer logFile.Close()
	// 	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	// }

	if err := ensureONNXRuntimeLibraryPresent(); err != nil {
		return fmt.Errorf("failed to prepare ONNX Runtime library: %w", err)
	}

	libPath := getONNXRuntimeLibraryPath()
	log.Printf("Attempting to load ONNX Runtime library from: %s", libPath)

	if _, err := os.Stat(libPath); err != nil {
		log.Printf("Library file not found at: %s", libPath)
		// Fallback to checking beside executable if getResourcePath failed for some reason
		exe, _ := os.Executable()
		fallback := filepath.Join(filepath.Dir(exe), getONNXRuntimeLibraryFileName())
		if _, err := os.Stat(fallback); err == nil {
			libPath = fallback
			log.Printf("Found library at fallback path: %s", libPath)
		}
	}

	ort.SetSharedLibraryPath(libPath)

	if err := ort.InitializeEnvironment(); err != nil {
		// Ignore if already initialized
		if strings.Contains(err.Error(), "already been initialized") {
			log.Printf("ONNX environment already initialized, proceeding")
		} else {
			log.Printf("Failed to initialize ONNX environment: %v", err)
			return fmt.Errorf("failed to initialize ONNX Runtime: %w", err)
		}
	}
	log.Println("ONNX Runtime initialized successfully")
	return nil
}
