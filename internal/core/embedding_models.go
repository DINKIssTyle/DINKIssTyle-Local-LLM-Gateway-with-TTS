package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dinkisstyle-chat/internal/mcp"
)

type EmbeddingModelConfig struct {
	Provider        string `json:"provider"`
	ModelID         string `json:"modelId"`
	Enabled         bool   `json:"enabled"`
	UseJevDecisions bool   `json:"useJevDecisions"`
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
	case jevModelID:
		return []string{jevModelFile}
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
	rt := getEmbeddingRuntime()
	selected := rt != nil && rt.modelID == cfg.ModelID
	configured := currentEmbeddingModelConfig().ModelID == cfg.ModelID
	if installed && cfg.Enabled && rtInfo.Ready && selected && rt.IsUsable() {
		status = "ready"
		message = rtInfo.Message
		active = true
	} else if installed && cfg.Enabled && configured {
		status = "pending"
		message = rtInfo.Message
		if selected && !rt.IsUsable() {
			message = "Local embedding worker stopped. Activate or download the model again."
		}
	} else if installed {
		status = "installed"
		message = "Model files are installed but not enabled."
	} else if cfg.Enabled {
		status = "pending"
		message = "Enabled in settings, but model files are not installed yet."
	}

	displayName, downloadURL := "Multilingual E5 Small", "https://huggingface.co/intfloat/multilingual-e5-small"
	if cfg.ModelID == jevModelID {
		displayName, downloadURL = "EmbeddingGemma 2 + JEV (local)", jevModelURL
	}
	backend := ""
	if selected {
		backend = rtInfo.Backend
	}
	return ManagedModelStatus{
		Key:         "embedding:" + cfg.ModelID,
		Kind:        "embedding",
		DisplayName: displayName,
		Provider:    cfg.Provider,
		ModelID:     cfg.ModelID,
		Backend:     backend,
		Installed:   installed,
		Active:      active,
		Status:      status,
		Message:     message,
		InstallDir:  installDir,
		DownloadURL: downloadURL,
		CanDownload: true,
	}
}

func getTTSModelStatus() ManagedModelStatus {
	assetsDir := getTTSAssetsDir()
	installed := hasRequiredTTSAssets(assetsDir)
	status := "missing"
	message := "TTS assets are not installed yet."
	if installed {
		status = "ready"
		message = "TTS assets are installed and ready to use."
	}
	return ManagedModelStatus{
		Key:         "tts:supertonic",
		Kind:        "tts",
		DisplayName: "Supertonic 3",
		Provider:    "local",
		ModelID:     "supertonic-3",
		Installed:   installed,
		Active:      installed,
		Status:      status,
		Message:     message,
		InstallDir:  assetsDir,
		DownloadURL: "https://huggingface.co/Supertone/supertonic-3",
		CanDownload: true,
	}
}

func currentEmbeddingModelConfig() EmbeddingModelConfig {
	return normalizeEmbeddingConfig(embeddingConfig)
}

func applyEmbeddingRuntimeConfig() error {
	cfg := currentEmbeddingModelConfig()
	status := getEmbeddingModelStatus(cfg)
	assetsEnabled := globalApp == nil || globalApp.enableTTS
	if assetsEnabled && status.Installed && cfg.Enabled {
		if rt := getEmbeddingRuntime(); rt != nil && rt.modelID == cfg.ModelID && getEmbeddingRuntimeInfo().Ready && rt.IsUsable() {
			installJevChoiceProvider(rt, cfg.UseJevDecisions)
			return nil
		}
		rt, info, err := loadEmbeddingRuntime(cfg)
		if err == nil {
			modelName := fmt.Sprintf("%s:%s", rt.modelID, rt.backend)
			err = mcp.ReindexStoredEmbeddings(modelName, func(text string) ([]float64, string, error) { return rt.Build(text, embeddingUsageDocument) }, embeddingProviderForRuntime(rt))
			if err != nil {
				rt.Close()
				info.Ready = false
				info.Message = "Previous embeddings preserved: " + err.Error()
				setEmbeddingRuntimeInfo(info)
				return err
			}
			setEmbeddingRuntime(rt)
			setEmbeddingRuntimeInfo(info)
			installJevChoiceProvider(rt, cfg.UseJevDecisions)
			return nil
		}
		setEmbeddingRuntimeInfo(info)
		return err
	}
	mcp.SetMemoryChoiceProvider(nil)
	installEmbeddingProvider(nil)
	clearEmbeddingRuntime()
	setEmbeddingRuntimeInfo(embeddingRuntimeInfo{
		Ready:   false,
		Backend: "",
		Message: "Embedding runtime is disabled.",
	})
	return nil
}

func (a *App) GetEmbeddingModelConfig() EmbeddingModelConfig {
	return currentEmbeddingModelConfig()
}

func (a *App) SetEmbeddingModelConfig(cfg EmbeddingModelConfig) error {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	cfg = normalizeEmbeddingConfig(cfg)
	previous := embeddingConfig
	previousInfo := getEmbeddingRuntimeInfo()
	embeddingConfig = cfg
	if cfg.Enabled && !isEmbeddingModelInstalled(cfg.ModelID) {
		embeddingConfig = previous
		return fmt.Errorf("download the embedding model in model manager before activating it")
	}
	if err := applyEmbeddingRuntimeConfig(); err != nil {
		embeddingConfig = previous
		setEmbeddingRuntimeInfo(previousInfo)
		return err
	}
	a.saveConfig()
	return nil
}

func (a *App) GetManagedModels() []ManagedModelStatus {
	items := []ManagedModelStatus{
		getTTSModelStatus(),
		getEmbeddingModelStatus(EmbeddingModelConfig{Provider: "local", ModelID: "multilingual-e5-small", Enabled: currentEmbeddingModelConfig().Enabled}),
		getEmbeddingModelStatus(EmbeddingModelConfig{Provider: "local", ModelID: jevModelID, Enabled: currentEmbeddingModelConfig().Enabled}),
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
	if cfg.ModelID == jevModelID {
		manifest = map[string]interface{}{"provider": "local", "model_id": jevModelID, "download_url": jevModelURL, "required_files": getEmbeddingModelRequiredFiles(jevModelID), "download_files": map[string]string{jevModelFile: jevModelURL + "/resolve/main/" + jevModelFile}, "runtime": "mediapipe==1.1.0, Python 3.12", "dimension": 256, "api_key_required": false}
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
