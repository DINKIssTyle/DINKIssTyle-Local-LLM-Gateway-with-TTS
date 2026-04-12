package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type EmbeddingModelConfig struct {
	Provider string `json:"provider"`
	ModelID  string `json:"modelId"`
	Enabled  bool   `json:"enabled"`
}

type ManagedModelStatus struct {
	Key          string  `json:"key"`
	Kind         string  `json:"kind"`
	DisplayName  string  `json:"displayName"`
	Provider     string  `json:"provider"`
	ModelID      string  `json:"modelId"`
	Backend      string  `json:"backend,omitempty"`
	Installed    bool    `json:"installed"`
	Active       bool    `json:"active"`
	Status       string  `json:"status"`
	Message      string  `json:"message"`
	InstallDir   string  `json:"installDir"`
	DownloadURL  string  `json:"downloadUrl,omitempty"`
	CanDownload  bool    `json:"canDownload"`
	Downloading  bool    `json:"downloading"`
	CurrentFile  string  `json:"currentFile,omitempty"`
	ProgressPct  float64 `json:"progressPct,omitempty"`
	ProgressText string  `json:"progressText,omitempty"`
}

var embeddingConfig = EmbeddingModelConfig{
	Provider: "local",
	ModelID:  "multilingual-e5-small",
	Enabled:  true,
}

func defaultEmbeddingConfig() EmbeddingModelConfig {
	return EmbeddingModelConfig{
		Provider: "local",
		ModelID:  "multilingual-e5-small",
		Enabled:  true,
	}
}

func normalizeEmbeddingConfig(cfg EmbeddingModelConfig) EmbeddingModelConfig {
	if strings.TrimSpace(cfg.Provider) == "" {
		cfg.Provider = "local"
	}
	if strings.TrimSpace(cfg.ModelID) == "" {
		cfg.ModelID = "multilingual-e5-small"
	}
	return cfg
}

func getEmbeddingModelConfigPath(modelID string) string {
	return filepath.Join(getEmbeddingModelInstallDir(modelID), "model.json")
}

func getEmbeddingModelRequiredFiles(modelID string) []string {
	switch strings.TrimSpace(modelID) {
	case "multilingual-e5-small":
		return []string{"model.onnx", "tokenizer.json", "config.json", "sentencepiece.bpe.model", "special_tokens_map.json"}
	default:
		return []string{"model.onnx"}
	}
}

func isEmbeddingModelInstalled(modelID string) bool {
	for _, name := range getEmbeddingModelRequiredFiles(modelID) {
		if _, err := os.Stat(filepath.Join(getEmbeddingModelInstallDir(modelID), name)); err != nil {
			return false
		}
	}
	return true
}

func getEmbeddingModelStatus(cfg EmbeddingModelConfig) ManagedModelStatus {
	cfg = normalizeEmbeddingConfig(cfg)
	installDir := getEmbeddingModelInstallDir(cfg.ModelID)
	installed := isEmbeddingModelInstalled(cfg.ModelID)
	rtInfo := getEmbeddingRuntimeInfo()

	status := "missing"
	message := "Model files are not installed yet."
	active := false
	if installed && cfg.Enabled && rtInfo.Ready {
		status = "ready"
		message = rtInfo.Message
		active = true
	} else if installed && cfg.Enabled {
		status = "pending"
		message = rtInfo.Message
	} else if installed {
		status = "installed"
		message = "Model files are installed but not enabled."
	} else if cfg.Enabled {
		status = "pending"
		message = "Enabled in settings, but model files are not installed yet."
	}

	return ManagedModelStatus{
		Key:         "embedding:" + cfg.ModelID,
		Kind:        "embedding",
		DisplayName: "Multilingual E5 Small",
		Provider:    cfg.Provider,
		ModelID:     cfg.ModelID,
		Backend:     rtInfo.Backend,
		Installed:   installed,
		Active:      active,
		Status:      status,
		Message:     message,
		InstallDir:  installDir,
		DownloadURL: "https://huggingface.co/intfloat/multilingual-e5-small",
		CanDownload: true,
	}
}

func getTTSModelStatus() ManagedModelStatus {
	assetsDir := getWritableTTSAssetsDir()
	installed := globalApp != nil && globalApp.CheckAssets()
	status := "missing"
	message := "TTS assets are not installed yet."
	if installed {
		status = "ready"
		message = "TTS assets are installed and ready to use."
	}
	return ManagedModelStatus{
		Key:         "tts:supertonic",
		Kind:        "tts",
		DisplayName: "Supertonic 2",
		Provider:    "local",
		ModelID:     "supertonic-2",
		Installed:   installed,
		Active:      installed,
		Status:      status,
		Message:     message,
		InstallDir:  assetsDir,
		DownloadURL: "https://huggingface.co/Supertone/supertonic-2",
		CanDownload: true,
	}
}

func currentEmbeddingModelConfig() EmbeddingModelConfig {
	return normalizeEmbeddingConfig(embeddingConfig)
}

func applyEmbeddingRuntimeConfig() {
	cfg := currentEmbeddingModelConfig()
	status := getEmbeddingModelStatus(cfg)
	assetsEnabled := globalApp == nil || globalApp.enableTTS
	if assetsEnabled && status.Installed && cfg.Enabled {
		rt, info, err := loadEmbeddingRuntime(cfg)
		if err == nil {
			setEmbeddingRuntime(rt)
			setEmbeddingRuntimeInfo(info)
			installEmbeddingProvider(rt)
			return
		}
		clearEmbeddingRuntime()
		setEmbeddingRuntimeInfo(info)
		installEmbeddingProvider(nil)
		return
	}
	clearEmbeddingRuntime()
	setEmbeddingRuntimeInfo(embeddingRuntimeInfo{
		Ready:   false,
		Backend: "",
		Message: "Embedding runtime is disabled.",
	})
	installEmbeddingProvider(nil)
}

func (a *App) GetEmbeddingModelConfig() EmbeddingModelConfig {
	return currentEmbeddingModelConfig()
}

func (a *App) SetEmbeddingModelConfig(cfg EmbeddingModelConfig) {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	embeddingConfig = normalizeEmbeddingConfig(cfg)
	applyEmbeddingRuntimeConfig()
	a.saveConfig()
}

func (a *App) GetManagedModels() []ManagedModelStatus {
	items := []ManagedModelStatus{
		getTTSModelStatus(),
		getEmbeddingModelStatus(currentEmbeddingModelConfig()),
	}
	for i := range items {
		if state, ok := getManagedDownloadState(items[i].Key); ok {
			items[i].Downloading = state.Active
			items[i].CurrentFile = state.CurrentFile
			items[i].ProgressPct = state.ProgressPct
			items[i].ProgressText = state.Message
			if state.Active {
				items[i].Status = "downloading"
				items[i].Message = state.Message
			} else if state.Finished && !state.Success {
				items[i].Status = "failed"
				items[i].Message = state.Message
			}
		}
	}
	return items
}

func (a *App) GetModelsRootDir() string {
	return GetModelsRootDir()
}

func (a *App) EnsureModelsRootDir() (string, error) {
	root := GetModelsRootDir()
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	return root, nil
}

func (a *App) ExportEmbeddingModelManifest() (string, error) {
	cfg := currentEmbeddingModelConfig()
	installDir := getEmbeddingModelInstallDir(cfg.ModelID)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return "", err
	}
	manifest := map[string]interface{}{
		"provider":                  cfg.Provider,
		"model_id":                  cfg.ModelID,
		"download_url":              "https://huggingface.co/intfloat/multilingual-e5-small",
		"required_files":            getEmbeddingModelRequiredFiles(cfg.ModelID),
		"optional_wrapper_manifest": embeddingWrapperManifestName,
		"download_files": map[string]string{
			"config.json":             "https://huggingface.co/intfloat/multilingual-e5-small/resolve/main/config.json",
			"tokenizer.json":          "https://huggingface.co/intfloat/multilingual-e5-small/resolve/main/tokenizer.json",
			"sentencepiece.bpe.model": "https://huggingface.co/intfloat/multilingual-e5-small/resolve/main/sentencepiece.bpe.model",
			"special_tokens_map.json": "https://huggingface.co/intfloat/multilingual-e5-small/resolve/main/special_tokens_map.json",
			"model.onnx":              "https://huggingface.co/intfloat/multilingual-e5-small/resolve/main/onnx/model.onnx",
		},
		"wrapper_example": map[string]interface{}{
			"model_file":        embeddingWrapperModelName,
			"input_name":        "text",
			"output_name":       "embedding",
			"normalize_output":  true,
			"manifest_filename": embeddingWrapperManifestName,
		},
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	path := getEmbeddingModelConfigPath(cfg.ModelID)
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return "", err
	}
	return path, nil
}
