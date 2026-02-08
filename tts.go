/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 *
 * Supertonic TTS Integration
 * Based on: https://github.com/supertone-inc/supertonic
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/sjzar/go-lame"
	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/text/unicode/norm"
)

// Available languages for multilingual TTS
var AvailableLangs = []string{"en", "ko", "es", "pt", "fr"}

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
	fmt.Printf("Loading TTS models (CPU mode, Threads: %d)\n", threads)

	dpPath := filepath.Join(onnxDir, "duration_predictor.onnx")
	textEncPath := filepath.Join(onnxDir, "text_encoder.onnx")
	vectorEstPath := filepath.Join(onnxDir, "vector_estimator.onnx")
	vocoderPath := filepath.Join(onnxDir, "vocoder.onnx")

	// Optimization: Set session options for multi-threading
	if threads <= 0 {
		threads = 4
	}
	so, _ := ort.NewSessionOptions()
	defer so.Destroy()
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
		maxLen = 300
		if lang == "ko" {
			maxLen = 120
		}
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

func (tts *TextToSpeech) infer(ctx context.Context, textList []string, langList []string, style *Style, totalStep int, speed float32) ([]float32, []float32, error) {
	bsz := len(textList)

	textIDs, textMask := tts.textProcessor.Call(textList, langList)
	textIDsShape := []int64{int64(bsz), int64(len(textIDs[0]))}
	textMaskShape := []int64{int64(bsz), 1, int64(len(textMask[0][0]))}

	textIDsTensor := intArrayToTensor(textIDs, textIDsShape)
	defer textIDsTensor.Destroy()
	textMaskTensor := arrayToTensor(textMask, textMaskShape)
	defer textMaskTensor.Destroy()

	// Predict duration
	dpOutputs := make([]ort.ArbitraryTensor, 1)
	err := tts.dpOrt.Run(
		[]ort.ArbitraryTensor{textIDsTensor, style.DpTensor, textMaskTensor},
		dpOutputs,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to run duration predictor: %w", err)
	}
	durTensor := dpOutputs[0].(*ort.Tensor[float32])
	defer durTensor.Destroy()
	durOnnx := durTensor.GetData()

	for i := range durOnnx {
		durOnnx[i] /= speed
	}

	// Encode text
	textIDsTensor2 := intArrayToTensor(textIDs, textIDsShape)
	defer textIDsTensor2.Destroy()
	textEncOutputs := make([]ort.ArbitraryTensor, 1)
	err = tts.textEncOrt.Run(
		[]ort.ArbitraryTensor{textIDsTensor2, style.TtlTensor, textMaskTensor},
		textEncOutputs,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to run text encoder: %w", err)
	}
	textEmbTensor := textEncOutputs[0].(*ort.Tensor[float32])
	defer textEmbTensor.Destroy()

	// Sample noisy latent
	xt, latentMask := tts.sampleNoisyLatent(durOnnx)
	latentShape := []int64{int64(bsz), int64(len(xt[0])), int64(len(xt[0][0]))}
	latentMaskShape := []int64{int64(bsz), 1, int64(len(latentMask[0][0]))}

	totalStepArray := make([]float32, bsz)
	for b := 0; b < bsz; b++ {
		totalStepArray[b] = float32(totalStep)
	}
	scalarShape := []int64{int64(bsz)}
	totalStepTensor, _ := ort.NewTensor(scalarShape, totalStepArray)
	defer totalStepTensor.Destroy()

	// Optimization: Move invariant tensor creation OUT of the loop
	latentMaskTensor := arrayToTensor(latentMask, latentMaskShape)
	defer latentMaskTensor.Destroy()
	textMaskTensor2 := arrayToTensor(textMask, textMaskShape)
	defer textMaskTensor2.Destroy()

	// Denoising loop
	for step := 0; step < totalStep; step++ {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		currentStepArray := make([]float32, bsz)
		for b := 0; b < bsz; b++ {
			currentStepArray[b] = float32(step)
		}

		currentStepTensor, _ := ort.NewTensor(scalarShape, currentStepArray)
		noisyLatentTensor := arrayToTensor(xt, latentShape)

		vectorEstOutputs := make([]ort.ArbitraryTensor, 1)
		err = tts.vectorEstOrt.Run(
			[]ort.ArbitraryTensor{noisyLatentTensor, textEmbTensor, style.TtlTensor, latentMaskTensor, textMaskTensor2,
				currentStepTensor, totalStepTensor},
			vectorEstOutputs,
		)
		if err != nil {
			currentStepTensor.Destroy()
			noisyLatentTensor.Destroy()
			return nil, nil, fmt.Errorf("failed to run vector estimator: %w", err)
		}

		denoisedTensor := vectorEstOutputs[0].(*ort.Tensor[float32])
		denoisedData := denoisedTensor.GetData()

		idx := 0
		for b := 0; b < bsz; b++ {
			for d := 0; d < len(xt[b]); d++ {
				for t := 0; t < len(xt[b][d]); t++ {
					xt[b][d][t] = float64(denoisedData[idx])
					idx++
				}
			}
		}

		noisyLatentTensor.Destroy()
		currentStepTensor.Destroy()
		denoisedTensor.Destroy()
	}

	// Generate waveform
	finalLatentTensor := arrayToTensor(xt, latentShape)
	defer finalLatentTensor.Destroy()

	vocoderOutputs := make([]ort.ArbitraryTensor, 1)
	err = tts.vocoderOrt.Run(
		[]ort.ArbitraryTensor{finalLatentTensor},
		vocoderOutputs,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to run vocoder: %w", err)
	}

	wavBatchTensor := vocoderOutputs[0].(*ort.Tensor[float32])
	defer wavBatchTensor.Destroy()
	wav := wavBatchTensor.GetData()

	return wav, durOnnx, nil
}

func (tts *TextToSpeech) sampleNoisyLatent(durOnnx []float32) ([][][]float64, [][][]float64) {
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

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	noisyLatent := make([][][]float64, bsz)
	for b := 0; b < bsz; b++ {
		batch := make([][]float64, latentDim)
		for d := 0; d < latentDim; d++ {
			row := make([]float64, latentLen)
			for t := 0; t < latentLen; t++ {
				const eps = 1e-10
				u1 := math.Max(eps, rng.Float64())
				u2 := rng.Float64()
				row[t] = math.Sqrt(-2.0*math.Log(u1)) * math.Cos(2.0*math.Pi*u2)
			}
			batch[d] = row
		}
		noisyLatent[b] = batch
	}

	latentMask := tts.getLatentMask(wavLengths)

	for b := 0; b < bsz; b++ {
		for d := 0; d < latentDim; d++ {
			for t := 0; t < latentLen; t++ {
				if t < len(latentMask[b][0]) {
					noisyLatent[b][d][t] *= latentMask[b][0][t]
				}
			}
		}
	}

	return noisyLatent, latentMask
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

// GenerateWAV generates WAV bytes from audio data
func GenerateWAV(audioData []float32, sampleRate int) ([]byte, error) {
	// Create temp file (wav.NewEncoder needs io.WriteSeeker)
	tmpFile, err := os.CreateTemp("", "tts_*.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	intData := make([]int, len(audioData))
	for i, sample := range audioData {
		clamped := math.Max(-1.0, math.Min(1.0, float64(sample)))
		intData[i] = int(clamped * 32767)
	}

	encoder := wav.NewEncoder(tmpFile, sampleRate, 16, 1, 1)
	audioBuf := &audio.IntBuffer{
		Data:           intData,
		Format:         &audio.Format{SampleRate: sampleRate, NumChannels: 1},
		SourceBitDepth: 16,
	}

	if err := encoder.Write(audioBuf); err != nil {
		return nil, err
	}

	if err := encoder.Close(); err != nil {
		return nil, err
	}

	// Read back the file
	tmpFile.Seek(0, 0)
	return os.ReadFile(tmpFile.Name())
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
	text = norm.NFKD.String(text)

	// Remove all characters except allowed ones (Letters, Numbers, Basic Punctuation)
	// We use \p{L} to support accented characters for French, Spanish, Portuguese, etc.
	// This still excludes symbols like arrows (→) or math symbols.
	allowedPattern := regexp.MustCompile(`[^\p{L}\p{N}\s.,!?:;'"\(\)\[\]\-]`)
	text = allowedPattern.ReplaceAllString(text, " ")

	replacements := map[string]string{
		"_": " ", "\u201C": "\"", "\u201D": "\"", "\u2018": "'", "\u2019": "'",
	}
	for old, new := range replacements {
		text = strings.ReplaceAll(text, old, new)
	}

	// First normalize all whitespace to single spaces
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// Add natural pauses after punctuation marks by duplicating punctuation
	// This technique works with many TTS engines to create brief pauses
	// Period: add comma for pause between sentences
	text = regexp.MustCompile(`\.(\s+)([A-Z가-힣])`).ReplaceAllString(text, "., $2")
	// Exclamation/Question: add period for pause
	text = regexp.MustCompile(`([!?])(\s+)`).ReplaceAllString(text, "$1, ")
	// Colon/Semicolon: add comma for medium pause
	text = regexp.MustCompile(`([:;])(\s*)`).ReplaceAllString(text, "$1, ")
	// Equals sign in context: ensure space
	text = regexp.MustCompile(`\s*=\s*`).ReplaceAllString(text, " - ")

	if text != "" && !regexp.MustCompile(`[.!?;:,'"]$`).MatchString(text) {
		text += "."
	}

	if !isValidLang(lang) {
		lang = "en"
	}

	text = fmt.Sprintf("<%s>%s</%s>", lang, text, lang)
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
		maxLen = 300
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}

	// First, split by paragraphs
	paragraphs := regexp.MustCompile(`\n\s*\n+`).Split(text, -1)
	var finalChunks []string

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// Split by sentence endings (. ! ? ; :) but keep the punctuation
		// Protecting abbreviations like Mr. Mrs. etc.
		temp := p
		abbrevs := []string{"Mr.", "Mrs.", "Dr.", "vs.", "e.g.", "i.e."}
		for _, abbr := range abbrevs {
			safe := strings.ReplaceAll(abbr, ".", "\u0000")
			temp = strings.ReplaceAll(temp, abbr, safe)
		}

		// Re-split with a more robust boundary
		re := regexp.MustCompile(`([.!?])\s+`)
		temp = re.ReplaceAllString(temp, "$1|")
		temp = strings.ReplaceAll(temp, "\u0000", ".")
		sentences := strings.Split(temp, "|")

		var currentChunk strings.Builder
		currentLen := 0

		for _, s := range sentences {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}

			sRunes := []rune(s)
			// If a SINGLE sentence is longer than maxLen, split it by words/spaces
			if len(sRunes) > maxLen {
				// Flush current before splitting long one
				if currentLen > 0 {
					finalChunks = append(finalChunks, currentChunk.String())
					currentChunk.Reset()
					currentLen = 0
				}

				// Split long sentence by spaces
				words := strings.Fields(s)
				var wordChunk strings.Builder
				wordLen := 0
				for _, w := range words {
					wRunes := []rune(w)
					if wordLen+len(wRunes)+1 > maxLen && wordLen > 0 {
						finalChunks = append(finalChunks, wordChunk.String())
						wordChunk.Reset()
						wordLen = 0
					}
					if wordLen > 0 {
						wordChunk.WriteString(" ")
						wordLen++
					}
					wordChunk.WriteString(w)
					wordLen += len(wRunes)
				}
				if wordChunk.Len() > 0 {
					finalChunks = append(finalChunks, wordChunk.String())
				}
				continue
			}

			// Typical grouping logic
			if currentLen+len(sRunes)+1 > maxLen && currentLen > 0 {
				finalChunks = append(finalChunks, currentChunk.String())
				currentChunk.Reset()
				currentLen = 0
			}

			if currentLen > 0 {
				currentChunk.WriteString(" ")
				currentLen++
			}
			currentChunk.WriteString(s)
			currentLen += len(sRunes)
		}

		if currentChunk.Len() > 0 {
			finalChunks = append(finalChunks, currentChunk.String())
		}
	}

	if len(finalChunks) == 0 {
		return []string{text}
	}

	return finalChunks
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

func arrayToTensor(array [][][]float64, shape []int64) *ort.Tensor[float32] {
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
		panic(err)
	}

	return tensor
}

func intArrayToTensor(array [][]int64, shape []int64) *ort.Tensor[int64] {
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
		panic(err)
	}

	return tensor
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

	libName := "onnxruntime.dll"
	if runtime.GOOS == "darwin" {
		libName = "libonnxruntime.dylib"
	} else if runtime.GOOS == "linux" {
		libName = "libonnxruntime.so"
	}

	libPath := GetResourcePath(filepath.Join("onnxruntime", libName))
	log.Printf("Attempting to load ONNX Runtime library from: %s", libPath)

	if _, err := os.Stat(libPath); err != nil {
		log.Printf("Library file not found at: %s", libPath)
		// Fallback to checking beside executable if getResourcePath failed for some reason
		exe, _ := os.Executable()
		fallback := filepath.Join(filepath.Dir(exe), libName)
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
