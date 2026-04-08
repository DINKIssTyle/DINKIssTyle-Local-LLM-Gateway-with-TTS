package core

import (
	"fmt"
	"path/filepath"
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
		assetsDir := filepath.Join(GetAppDataDir(), "assets")
		specs := []DownloadFileSpec{
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/onnx/duration_predictor.onnx", DestPath: filepath.Join(assetsDir, "onnx", "duration_predictor.onnx"), Label: "duration_predictor.onnx"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/onnx/text_encoder.onnx", DestPath: filepath.Join(assetsDir, "onnx", "text_encoder.onnx"), Label: "text_encoder.onnx"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/onnx/vector_estimator.onnx", DestPath: filepath.Join(assetsDir, "onnx", "vector_estimator.onnx"), Label: "vector_estimator.onnx"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/onnx/vocoder.onnx", DestPath: filepath.Join(assetsDir, "onnx", "vocoder.onnx"), Label: "vocoder.onnx"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/onnx/tts.json", DestPath: filepath.Join(assetsDir, "onnx", "tts.json"), Label: "tts.json"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/onnx/unicode_indexer.json", DestPath: filepath.Join(assetsDir, "onnx", "unicode_indexer.json"), Label: "unicode_indexer.json"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/LICENSE", DestPath: filepath.Join(assetsDir, "LICENSE"), Label: "LICENSE"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/voice_styles/M1.json", DestPath: filepath.Join(assetsDir, "voice_styles", "M1.json"), Label: "M1.json"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/voice_styles/M2.json", DestPath: filepath.Join(assetsDir, "voice_styles", "M2.json"), Label: "M2.json"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/voice_styles/M3.json", DestPath: filepath.Join(assetsDir, "voice_styles", "M3.json"), Label: "M3.json"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/voice_styles/M4.json", DestPath: filepath.Join(assetsDir, "voice_styles", "M4.json"), Label: "M4.json"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/voice_styles/M5.json", DestPath: filepath.Join(assetsDir, "voice_styles", "M5.json"), Label: "M5.json"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/voice_styles/F1.json", DestPath: filepath.Join(assetsDir, "voice_styles", "F1.json"), Label: "F1.json"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/voice_styles/F2.json", DestPath: filepath.Join(assetsDir, "voice_styles", "F2.json"), Label: "F2.json"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/voice_styles/F3.json", DestPath: filepath.Join(assetsDir, "voice_styles", "F3.json"), Label: "F3.json"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/voice_styles/F4.json", DestPath: filepath.Join(assetsDir, "voice_styles", "F4.json"), Label: "F4.json"},
			{URL: "https://huggingface.co/Supertone/supertonic-2/resolve/main/voice_styles/F5.json", DestPath: filepath.Join(assetsDir, "voice_styles", "F5.json"), Label: "F5.json"},
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
	case "embedding:multilingual-e5-small":
		cfg := currentEmbeddingModelConfig()
		status := getEmbeddingModelStatus(cfg)
		installDir := getEmbeddingModelInstallDir("multilingual-e5-small")
		specs := []DownloadFileSpec{
			{URL: "https://huggingface.co/intfloat/multilingual-e5-small/resolve/main/config.json", DestPath: filepath.Join(installDir, "config.json"), Label: "config.json"},
			{URL: "https://huggingface.co/intfloat/multilingual-e5-small/resolve/main/tokenizer.json", DestPath: filepath.Join(installDir, "tokenizer.json"), Label: "tokenizer.json"},
			{URL: "https://huggingface.co/intfloat/multilingual-e5-small/resolve/main/sentencepiece.bpe.model", DestPath: filepath.Join(installDir, "sentencepiece.bpe.model"), Label: "sentencepiece.bpe.model"},
			{URL: "https://huggingface.co/intfloat/multilingual-e5-small/resolve/main/special_tokens_map.json", DestPath: filepath.Join(installDir, "special_tokens_map.json"), Label: "special_tokens_map.json"},
			{URL: "https://huggingface.co/intfloat/multilingual-e5-small/resolve/main/onnx/model.onnx", DestPath: filepath.Join(installDir, "model.onnx"), Label: "model.onnx"},
		}
		return status, specs, func() error {
			if _, err := a.ExportEmbeddingModelManifest(); err != nil {
				return err
			}
			cfg := currentEmbeddingModelConfig()
			cfg.Enabled = true
			a.SetEmbeddingModelConfig(cfg)
			return nil
		}, nil
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
