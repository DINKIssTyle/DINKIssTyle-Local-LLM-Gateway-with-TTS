package core

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ManagedModelDownloadState struct {
	Key             string  `json:"key"`
	Kind            string  `json:"kind"`
	ModelID         string  `json:"modelId"`
	DisplayName     string  `json:"displayName"`
	Active          bool    `json:"active"`
	Finished        bool    `json:"finished"`
	Success         bool    `json:"success"`
	Status          string  `json:"status"`
	Message         string  `json:"message"`
	CurrentFile     string  `json:"currentFile"`
	FilesCompleted  int     `json:"filesCompleted"`
	FilesTotal      int     `json:"filesTotal"`
	BytesDownloaded int64   `json:"bytesDownloaded"`
	BytesTotal      int64   `json:"bytesTotal"`
	ProgressPct     float64 `json:"progressPct"`
}

var managedDownloadStore = struct {
	mu    sync.RWMutex
	items map[string]ManagedModelDownloadState
}{
	items: make(map[string]ManagedModelDownloadState),
}

func cloneManagedDownloadState(state ManagedModelDownloadState) ManagedModelDownloadState {
	return state
}

func getManagedDownloadState(key string) (ManagedModelDownloadState, bool) {
	managedDownloadStore.mu.RLock()
	defer managedDownloadStore.mu.RUnlock()
	state, ok := managedDownloadStore.items[key]
	return cloneManagedDownloadState(state), ok
}

func listManagedDownloadStates() []ManagedModelDownloadState {
	managedDownloadStore.mu.RLock()
	defer managedDownloadStore.mu.RUnlock()
	out := make([]ManagedModelDownloadState, 0, len(managedDownloadStore.items))
	for _, state := range managedDownloadStore.items {
		out = append(out, cloneManagedDownloadState(state))
	}
	return out
}

func putManagedDownloadState(state ManagedModelDownloadState) {
	managedDownloadStore.mu.Lock()
	managedDownloadStore.items[state.Key] = cloneManagedDownloadState(state)
	managedDownloadStore.mu.Unlock()
	if globalApp != nil && globalApp.ctx != nil {
		wruntime.EventsEmit(globalApp.ctx, "managed-model-download", state)
	}
}

func finishManagedDownloadState(state ManagedModelDownloadState) {
	state.Active = false
	state.Finished = true
	putManagedDownloadState(state)
	if globalApp != nil && globalApp.ctx != nil {
		wruntime.EventsEmit(globalApp.ctx, "managed-model-download-finished", state)
	}
}

func (a *App) GetManagedModelDownloads() []ManagedModelDownloadState {
	return listManagedDownloadStates()
}

func (a *App) StartManagedModelDownload(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("empty model key")
	}
	if state, ok := getManagedDownloadState(key); ok && state.Active {
		return fmt.Errorf("download already in progress for %s", key)
	}

	status, specs, afterDownload, err := a.getManagedModelDownloadPlan(key)
	if err != nil {
		return err
	}

	initial := ManagedModelDownloadState{
		Key:         status.Key,
		Kind:        status.Kind,
		ModelID:     status.ModelID,
		DisplayName: status.DisplayName,
		Active:      true,
		Status:      "queued",
		Message:     "Preparing download...",
		FilesTotal:  len(specs),
	}
	putManagedDownloadState(initial)

	go func() {
		downloader := NewDownloader()
		state := initial
		err := downloader.DownloadFiles(specs, func(progress DownloadProgress) {
			displayCompleted := progress.FilesCompleted
			if displayCompleted < 1 {
				displayCompleted = 1
			}
			if displayCompleted > max(progress.FilesTotal, 1) {
				displayCompleted = max(progress.FilesTotal, 1)
			}
			state.Status = "downloading"
			state.Message = fmt.Sprintf("Downloading %s (%d/%d)", progress.CurrentFile, displayCompleted, max(progress.FilesTotal, 1))
			state.CurrentFile = progress.CurrentFile
			state.FilesCompleted = progress.FilesCompleted
			state.FilesTotal = progress.FilesTotal
			state.BytesDownloaded = progress.BytesDownloaded
			state.BytesTotal = progress.BytesTotal
			state.ProgressPct = calculateDownloadProgressPct(progress.BytesDownloaded, progress.BytesTotal, progress.FilesCompleted, progress.FilesTotal)
			putManagedDownloadState(state)
		})
		if err != nil {
			state.Success = false
			state.Status = "failed"
			state.Message = err.Error()
			finishManagedDownloadState(state)
			return
		}

		state.FilesCompleted = len(specs)
		state.FilesTotal = len(specs)
		state.ProgressPct = 100
		state.Status = "finalizing"
		state.Message = "Finalizing model setup..."
		putManagedDownloadState(state)

		if afterDownload != nil {
			if err := afterDownload(); err != nil {
				state.Success = false
				state.Status = "failed"
				state.Message = err.Error()
				finishManagedDownloadState(state)
				return
			}
		}

		state.Success = true
		state.Status = "ready"
		state.Message = "Download complete."
		finishManagedDownloadState(state)
	}()

	return nil
}

func calculateDownloadProgressPct(downloaded, total int64, filesCompleted, filesTotal int) float64 {
	if total > 0 {
		pct := float64(downloaded) / float64(total) * 100
		if pct < 0 {
			return 0
		}
		if pct > 100 {
			return 100
		}
		return pct
	}
	if filesTotal <= 0 {
		return 0
	}
	pct := float64(filesCompleted) / float64(filesTotal) * 100
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

func (a *App) getManagedModelDownloadPlan(key string) (ManagedModelStatus, []DownloadFileSpec, func() error, error) {
	switch key {
	case "tts:supertonic":
		status := getTTSModelStatus()
		assetsDir := getWritableTTSAssetsDir()
		baseURL := "https://huggingface.co/Supertone/supertonic-3/resolve/main"
		specs := []DownloadFileSpec{
			{URL: baseURL + "/onnx/duration_predictor.onnx", DestPath: filepath.Join(assetsDir, legacyTTSOnnxDirName, "duration_predictor.onnx"), Label: "duration_predictor.onnx"},
			{URL: baseURL + "/onnx/text_encoder.onnx", DestPath: filepath.Join(assetsDir, legacyTTSOnnxDirName, "text_encoder.onnx"), Label: "text_encoder.onnx"},
			{URL: baseURL + "/onnx/vector_estimator.onnx", DestPath: filepath.Join(assetsDir, legacyTTSOnnxDirName, "vector_estimator.onnx"), Label: "vector_estimator.onnx"},
			{URL: baseURL + "/onnx/vocoder.onnx", DestPath: filepath.Join(assetsDir, legacyTTSOnnxDirName, "vocoder.onnx"), Label: "vocoder.onnx"},
			{URL: baseURL + "/onnx/tts.json", DestPath: filepath.Join(assetsDir, legacyTTSOnnxDirName, "tts.json"), Label: "tts.json"},
			{URL: baseURL + "/onnx/unicode_indexer.json", DestPath: filepath.Join(assetsDir, legacyTTSOnnxDirName, "unicode_indexer.json"), Label: "unicode_indexer.json"},
			{URL: baseURL + "/LICENSE", DestPath: filepath.Join(assetsDir, "LICENSE"), Label: "LICENSE"},
			{URL: baseURL + "/voice_styles/M1.json", DestPath: filepath.Join(assetsDir, legacyTTSVoiceStylesDir, "M1.json"), Label: "M1.json"},
			{URL: baseURL + "/voice_styles/M2.json", DestPath: filepath.Join(assetsDir, legacyTTSVoiceStylesDir, "M2.json"), Label: "M2.json"},
			{URL: baseURL + "/voice_styles/M3.json", DestPath: filepath.Join(assetsDir, legacyTTSVoiceStylesDir, "M3.json"), Label: "M3.json"},
			{URL: baseURL + "/voice_styles/M4.json", DestPath: filepath.Join(assetsDir, legacyTTSVoiceStylesDir, "M4.json"), Label: "M4.json"},
			{URL: baseURL + "/voice_styles/M5.json", DestPath: filepath.Join(assetsDir, legacyTTSVoiceStylesDir, "M5.json"), Label: "M5.json"},
			{URL: baseURL + "/voice_styles/F1.json", DestPath: filepath.Join(assetsDir, legacyTTSVoiceStylesDir, "F1.json"), Label: "F1.json"},
			{URL: baseURL + "/voice_styles/F2.json", DestPath: filepath.Join(assetsDir, legacyTTSVoiceStylesDir, "F2.json"), Label: "F2.json"},
			{URL: baseURL + "/voice_styles/F3.json", DestPath: filepath.Join(assetsDir, legacyTTSVoiceStylesDir, "F3.json"), Label: "F3.json"},
			{URL: baseURL + "/voice_styles/F4.json", DestPath: filepath.Join(assetsDir, legacyTTSVoiceStylesDir, "F4.json"), Label: "F4.json"},
			{URL: baseURL + "/voice_styles/F5.json", DestPath: filepath.Join(assetsDir, legacyTTSVoiceStylesDir, "F5.json"), Label: "F5.json"},
		}
		return status, specs, func() error {
			if err := InitTTS(assetsDir, 4); err != nil {
				return fmt.Errorf("download succeeded but TTS init failed: %w", err)
			}
			if a.ctx != nil {
				wruntime.EventsEmit(a.ctx, "assets-ready")
			}
			return nil
		}, nil
	case "embedding:" + jevModelID:
		cfg := defaultEmbeddingConfig()
		status := getEmbeddingModelStatus(cfg)
		installDir := getEmbeddingModelInstallDir(jevModelID)
		specs := []DownloadFileSpec{{URL: jevModelURL + "/resolve/main/" + jevModelFile, DestPath: filepath.Join(installDir, jevModelFile), Label: jevModelFile}, {URL: jevModelURL + "/resolve/main/README.md", DestPath: filepath.Join(installDir, "README.md"), Label: "README.md"}}
		return status, specs, func() error {
			if err := installJevDependencies(); err != nil {
				return err
			}
			cfg.UseJevDecisions = a.GetEmbeddingModelConfig().UseJevDecisions
			if err := a.SetEmbeddingModelConfig(cfg); err != nil {
				return err
			}
			_, err := a.ExportEmbeddingModelManifest()
			return err
		}, nil
	case "stt:whisper-base":
		status := getSTTModelStatusFor(defaultSTTModelID)
		installDir := getWritableSTTAssetsDir("whisper-base")
		baseURL := "https://huggingface.co/onnx-community/whisper-base/resolve/main"
		specs := []DownloadFileSpec{
			{URL: baseURL + "/config.json", DestPath: filepath.Join(installDir, "config.json"), Label: "config.json"},
			{URL: baseURL + "/tokenizer.json", DestPath: filepath.Join(installDir, "tokenizer.json"), Label: "tokenizer.json"},
			{URL: baseURL + "/vocab.json", DestPath: filepath.Join(installDir, "vocab.json"), Label: "vocab.json"},
			{URL: baseURL + "/onnx/encoder_model.onnx", DestPath: filepath.Join(installDir, "encoder_model.onnx"), Label: "encoder_model.onnx"},
			{URL: baseURL + "/onnx/decoder_model_merged.onnx", DestPath: filepath.Join(installDir, "decoder_model_merged.onnx"), Label: "decoder_model_merged.onnx"},
		}
		for _, name := range []string{"preprocessor_config.json", "tokenizer_config.json", "generation_config.json", "special_tokens_map.json", "merges.txt", "added_tokens.json", "normalizer.json"} {
			specs = append(specs, DownloadFileSpec{URL: baseURL + "/" + name, DestPath: filepath.Join(installDir, name), Label: name})
		}
		specs = append(specs, DownloadFileSpec{URL: "https://raw.githubusercontent.com/snakers4/silero-vad/066409f3d9db45a30d21dfcbf9147826ffdd7052/src/silero_vad/data/silero_vad.onnx", DestPath: filepath.Join(installDir, "silero_vad.onnx"), Label: "Silero VAD speech filter"})

		return status, specs, func() error {
			if err := installSTTDependencies(); err != nil {
				return err
			}
			if a.ctx != nil {
				wruntime.EventsEmit(a.ctx, "assets-ready")
			}
			return nil
		}, nil
	case "stt:whisper-large-v3-turbo-mlx":
		if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
			return ManagedModelStatus{}, nil, nil, fmt.Errorf("MLX Whisper requires macOS on Apple Silicon")
		}
		dir := getWritableSTTAssetsDir(mlxSTTModelID)
		specs := []DownloadFileSpec{}
		for _, name := range []string{"config.json", "weights.safetensors"} {
			specs = append(specs, DownloadFileSpec{URL: mlxSTTModelURL + "/resolve/main/" + name, DestPath: filepath.Join(dir, name), Label: name})
		}
		specs = append(specs, DownloadFileSpec{URL: "https://raw.githubusercontent.com/snakers4/silero-vad/066409f3d9db45a30d21dfcbf9147826ffdd7052/src/silero_vad/data/silero_vad.onnx", DestPath: filepath.Join(dir, "silero_vad.onnx"), Label: "Silero VAD speech filter"})
		return getSTTModelStatusFor(mlxSTTModelID), specs, func() error { return installMLXSTTDependencies() }, nil

	default:
		return ManagedModelStatus{}, nil, nil, fmt.Errorf("unknown managed model key: %s", key)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
