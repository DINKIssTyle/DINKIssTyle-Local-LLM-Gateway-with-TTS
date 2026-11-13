package core

import (
	"dinkisstyle-chat/internal/mcp"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type STTModelConfig struct {
	Provider string `json:"provider"`
	ModelID  string `json:"modelId"`
	Enabled  bool   `json:"enabled"`
	Language string `json:"language"`
}

const (
	defaultSTTModelID = "whisper-base"
	mlxSTTModelID     = "whisper-large-v3-turbo-mlx"
	mlxSTTModelURL    = "https://huggingface.co/mlx-community/whisper-large-v3-turbo"
	whisperBaseURL    = "https://huggingface.co/onnx-community/whisper-base"
)

var defaultSTTConfig = STTModelConfig{
	Provider: "local",
	ModelID:  defaultSTTModelID,
	Enabled:  true,
	Language: "ko",
}

var sttLock sync.Mutex

func getRequiredSTTFiles(modelID string) []string {
	if modelID == mlxSTTModelID {
		return []string{"config.json", "weights.safetensors", "silero_vad.onnx"}
	}
	return []string{
		"config.json",
		"tokenizer.json",
		"vocab.json",
		"encoder_model.onnx",
		"decoder_model_merged.onnx",
		"preprocessor_config.json", "tokenizer_config.json", "generation_config.json",
		"special_tokens_map.json", "merges.txt", "added_tokens.json", "normalizer.json",
		"silero_vad.onnx",
	}
}

func isSTTModelInstalled(modelID string) bool {
	installDir := getWritableSTTAssetsDir(modelID)
	for _, req := range getRequiredSTTFiles(modelID) {
		if !fileExists(filepath.Join(installDir, req)) {
			return false
		}
	}
	return true
}

func selectedSTTModelID() string {
	if os.Getenv("DKST_STT_MODEL") == defaultSTTModelID {
		return defaultSTTModelID
	}
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" && isSTTModelInstalled(mlxSTTModelID) && fileExists(filepath.Join(sttRuntimeDir(), "installed-mlx")) {
		return mlxSTTModelID
	}
	return defaultSTTModelID
}
func getSTTModelStatus() ManagedModelStatus { return getSTTModelStatusFor(selectedSTTModelID()) }
func getSTTModelStatusFor(modelID string) ManagedModelStatus {
	marker := "installed"
	displayName := "Whisper Base (STT)"
	downloadURL := whisperBaseURL
	provider := "local"
	if modelID == mlxSTTModelID {
		marker = "installed-mlx"
		displayName = "Whisper Large-v3-Turbo (MLX / Apple Silicon)"
		downloadURL = mlxSTTModelURL
		provider = "mlx"
	}

	installDir := getWritableSTTAssetsDir(modelID)
	installed := isSTTModelInstalled(modelID)

	status := "missing"
	message := "STT model files are not installed yet."
	active := false
	supported := modelID != mlxSTTModelID || (runtime.GOOS == "darwin" && runtime.GOARCH == "arm64")

	if supported && installed && fileExists(sttPythonPath()) && fileExists(filepath.Join(sttRuntimeDir(), marker)) {
		status = "ready"
		message = "STT model is installed and ready to transcribe."
		active = true
	}

	if installed && !active {
		status = "runtime-missing"
		message = "Whisper runtime is missing. Download STT again to install runtime dependencies."
	}
	if !supported {
		status = "unsupported"
		message = "MLX Whisper requires Apple Silicon macOS."
	}
	return ManagedModelStatus{
		Key:         "stt:" + modelID,
		Kind:        "stt",
		DisplayName: displayName,
		Provider:    provider,
		ModelID:     modelID,
		Installed:   installed,
		Active:      active,
		Status:      status,
		Message:     message,
		InstallDir:  installDir,
		DownloadURL: downloadURL,
		CanDownload: supported,
	}
}

func handleSTTStatus(app *App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := getSTTModelStatus()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	}
}

type STTTranscribeRequest struct {
	Engine       string `json:"engine"`
	Fallback     bool   `json:"fallback"`
	Realtime     bool   `json:"realtime"`
	Audio        string `json:"audio"`      // Base64 encoded audio
	Format       string `json:"format"`     // "pcm" or "wav"
	SampleRate   int    `json:"sampleRate"` // e.g. 16000
	Language     string `json:"language"`   // e.g. "ko"
	SessionID    string `json:"sessionId"`
	SegmentID    string `json:"segmentId"`
	StartedAt    string `json:"startedAt"`
	EndedAt      string `json:"endedAt"`
	TriggerWord  string `json:"triggerWord"`
	SpeechFilter string `json:"speechFilter"`
	Revision     int    `json:"revision"`
}

type STTTranscribeResponse struct {
	StableText     string  `json:"stableText"`
	DraftText      string  `json:"draftText"`
	Engine         string  `json:"engine,omitempty"`
	Warning        string  `json:"warning,omitempty"`
	SessionID      string  `json:"sessionId,omitempty"`
	SegmentID      string  `json:"segmentId,omitempty"`
	Revision       int     `json:"revision,omitempty"`
	Stage          string  `json:"stage"`
	Rejected       bool    `json:"rejected"`
	Reason         string  `json:"reason,omitempty"`
	Text           string  `json:"text"`
	Language       string  `json:"language,omitempty"`
	Duration       float64 `json:"duration,omitempty"`
	Saved          bool    `json:"saved"`
	Triggered      bool    `json:"triggered"`
	Prompt         string  `json:"prompt,omitempty"`
	MatchedTrigger string  `json:"matchedTrigger,omitempty"`
}

func handleSTTTranscribe(app *App) http.HandlerFunc {
	return handleSTTAudio(app, false)
}

// Preview results never enter transcript storage or trigger evaluation.
func handleSTTPreview(app *App) http.HandlerFunc {
	return handleSTTAudio(app, true)
}

func handleSTTAudio(app *App, preview bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		var metadata STTTranscribeRequest
		contentType := r.Header.Get("Content-Type")
		var audioBytes []byte
		var format string = "pcm"
		var lang string = "ko"

		if strings.HasPrefix(contentType, "application/json") {
			var req STTTranscribeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid JSON body: "+err.Error(), http.StatusBadRequest)
				return
			}
			metadata = req
			if req.SampleRate != 0 && req.SampleRate != 16000 {
				http.Error(w, "STT requires 16000 Hz audio", 400)
				return
			}
			decoded, err := base64.StdEncoding.DecodeString(req.Audio)
			if err != nil {
				http.Error(w, "Invalid base64 audio data: "+err.Error(), http.StatusBadRequest)
				return
			}
			audioBytes = decoded
			if req.Format != "" {
				format = req.Format
			}
			if req.Language != "" {
				lang = req.Language
			}
		} else if strings.HasPrefix(contentType, "multipart/form-data") {
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				file, header, err = r.FormFile("audio")
			}
			if err != nil {
				http.Error(w, "Missing audio file in multipart form", http.StatusBadRequest)
				return
			}
			defer file.Close()

			data, err := io.ReadAll(file)
			if err != nil {
				http.Error(w, "Failed to read audio file: "+err.Error(), http.StatusInternalServerError)
				return
			}
			audioBytes = data
			if header != nil && strings.HasSuffix(strings.ToLower(header.Filename), ".wav") {
				format = "wav"
			}
			if formLang := r.FormValue("language"); formLang != "" {
				lang = formLang
			}
		} else {
			// Raw audio stream (e.g. audio/wav or raw PCM bytes)
			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read request body: "+err.Error(), http.StatusInternalServerError)
				return
			}
			audioBytes = data
			if strings.Contains(contentType, "wav") {
				format = "wav"
			}
			if queryLang := r.URL.Query().Get("lang"); queryLang != "" {
				lang = queryLang
			}
		}

		if len(audioBytes) == 0 || len(audioBytes) > 960128 || (format != "pcm" && format != "wav") {
			http.Error(w, "Audio must be pcm/wav and no longer than 30 seconds", http.StatusBadRequest)
			return
		}

		if format == "pcm" && (len(audioBytes)%2 != 0 || len(audioBytes) < 3200) {
			http.Error(w, "PCM audio must be 0.1–30 seconds of 16 kHz mono PCM16", 400)
			return
		}
		if len(metadata.TriggerWord) > 1024 || len(metadata.SessionID) > 100 || len(metadata.SegmentID) > 100 {
			http.Error(w, "Invalid transcript metadata", 400)
			return
		}
		aliases, err := parseSTTTriggers(metadata.TriggerWord)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		end := time.Now().UTC()
		if metadata.EndedAt != "" {
			parsed, err := time.Parse(time.RFC3339Nano, metadata.EndedAt)
			if err != nil {
				http.Error(w, "Invalid endedAt", 400)
				return
			}
			end = parsed.UTC()
		}
		start := end.Add(-time.Duration(float64(len(audioBytes)) / 32000 * float64(time.Second)))
		if metadata.StartedAt != "" {
			parsed, err := time.Parse(time.RFC3339Nano, metadata.StartedAt)
			if err != nil {
				http.Error(w, "Invalid startedAt", 400)
				return
			}
			start = parsed.UTC()
		}
		if start.After(end) || end.Sub(start) > 35*time.Second {
			http.Error(w, "Invalid transcript time range", 400)
			return
		}
		if preview && (metadata.SessionID == "" || metadata.SegmentID == "" || metadata.Revision < 1) {
			http.Error(w, "Preview requires sessionId, segmentId and revision", 400)
			return
		}
		if r.Context().Err() != nil {
			return
		}
		engine := metadata.Engine
		if engine == "" {
			engine = "whisper"
		}
		if engine != "whisper" && !chromeSTTEngine(engine) {
			http.Error(w, "unknown STT engine", 400)
			return
		}
		var inference sttWorkerReply
		warning := ""
		if chromeSTTEngine(engine) {
			metadata.Language = lang
			metadata.Format = format
			inference, err = runChromeSpeech(r.Context(), metadata, audioBytes, preview, r.Header.Get("X-User-ID"))
			if err != nil && err != errSTTBusy && metadata.Fallback && !preview && getSTTModelStatus().Installed {
				warning = "Chrome STT 오류로 Whisper 사용: " + err.Error()
				engine = "whisper"
				inference, err = runSTTInference(audioBytes, format, lang, metadata.SpeechFilter, metadata.Realtime)
			}
		} else {
			if !getSTTModelStatus().Installed {
				http.Error(w, "STT model is not installed. Please download it from Asset Manager.", 400)
				return
			}
			inference, err = runSTTInference(audioBytes, format, lang, metadata.SpeechFilter, preview || metadata.Realtime)
		}

		if err != nil {
			if err == errSTTBusy {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "STT engine busy", 429)
				return
			}
			http.Error(w, fmt.Sprintf("Transcription failed: %v", err), http.StatusInternalServerError)
			return
		}

		transcribedText := inference.Text
		if inference.Rejected {
			transcribedText = ""
		}
		resp := STTTranscribeResponse{Engine: engine, Warning: warning, StableText: inference.StableText, DraftText: inference.DraftText,
			SessionID: metadata.SessionID, SegmentID: metadata.SegmentID, Revision: metadata.Revision, Stage: "final",
			Rejected: inference.Rejected, Reason: inference.Reason,
			Text:     transcribedText,
			Language: lang,
		}
		if preview {
			resp.Stage = "draft"
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if strings.TrimSpace(transcribedText) != "" {
			word := strings.TrimSpace(metadata.TriggerWord)
			resp.Triggered, resp.Prompt, resp.MatchedTrigger = matchSTTTrigger(transcribedText, aliases)
			entry := mcp.TranscriptEntry{UserID: r.Header.Get("X-User-ID"), SessionID: metadata.SessionID, SegmentID: metadata.SegmentID, StartedAt: start.Format(time.RFC3339Nano), EndedAt: end.Format(time.RFC3339Nano), Text: transcribedText, TriggerWord: word, Triggered: resp.Triggered}
			if err := mcp.SaveTranscript(entry); err != nil {
				http.Error(w, "Transcript could not be saved: "+err.Error(), 500)
				return
			}
			resp.Saved = true
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
