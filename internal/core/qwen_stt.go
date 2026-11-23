package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	bundledata "dinkisstyle-chat/bundle"
)

const qwenSTTModelID = "qwen3-asr-0.6b-mlx"
const qwenSTTModelURL = "https://huggingface.co/mlx-community/Qwen3-ASR-0.6B-8bit"
const qwenSTTModelRevision = "89e96d92ba34aca20b3e29fb10cc284097d1219f"
const qwenSTTRuntimeVersion = "mlx-audio-0.6.7"

func qwenSTTSupported() bool    { return runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" }
func qwenSTTRuntimeDir() string { return joinAppDataPath(assetsDirName, runtimeDirName, "qwen3-asr") }
func qwenSTTPythonPath() string { return filepath.Join(qwenSTTRuntimeDir(), "venv", "bin", "python") }
func qwenSTTFiles() []string {
	return []string{"config.json", "model.safetensors", "model.safetensors.index.json", "tokenizer_config.json", "vocab.json", "merges.txt", "preprocessor_config.json", "generation_config.json", "chat_template.json", "silero_vad.onnx"}
}

// Qwen uses Transformers 5; do not upgrade the existing Whisper environment.
func installQwenSTTDependencies() error {
	if !qwenSTTSupported() {
		return fmt.Errorf("Qwen3-ASR MLX requires Apple Silicon macOS")
	}
	uv, err := exec.LookPath("uv")
	if err != nil {
		home, _ := os.UserHomeDir()
		uv = filepath.Join(home, ".local", "bin", "uv")
		if !fileExists(uv) {
			return fmt.Errorf("Qwen3-ASR setup requires uv; install uv then download the model again")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	steps := [][]string{}
	if !fileExists(qwenSTTPythonPath()) {
		steps = append(steps, []string{"venv", "--python", "3.12", filepath.Join(qwenSTTRuntimeDir(), "venv")})
	}
	steps = append(steps, []string{"pip", "install", "--python", qwenSTTPythonPath(), "mlx-audio[stt]==0.6.7", "onnxruntime>=1.20,<2"})
	for _, args := range steps {
		cmd := exec.CommandContext(ctx, uv, args...)
		var output limitedWorkerLog
		cmd.Stdout, cmd.Stderr = &output, &output
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Qwen3-ASR runtime installation failed: %w: %s", err, output.String())
		}
	}
	return os.WriteFile(filepath.Join(qwenSTTRuntimeDir(), "installed"), []byte(qwenSTTRuntimeVersion), 0644)
}

var qwenSTTLock sync.Mutex
var activeQwenSTTWorker *sttWorker // protected by qwenSTTLock

func closeQwenSTTRuntime() {
	qwenSTTLock.Lock()
	defer qwenSTTLock.Unlock()
	if activeQwenSTTWorker != nil {
		activeQwenSTTWorker.close()
		activeQwenSTTWorker = nil
	}
}

func startQwenSTTWorker(python, model, root string) (*sttWorker, error) {
	path := filepath.Join(root, "qwen_asr_worker.py")
	if err := bundledata.MaterializeQwenSTTWorker(path); err != nil {
		return nil, err
	}
	return launchSTTWorker(python, "Qwen3-ASR", []string{"-u", path, "--model", model})
}

func runQwenSTTInference(audio []byte, format, lang, speechFilter string, preview bool) (sttWorkerReply, error) {
	started := time.Now()
	if preview {
		if !qwenSTTLock.TryLock() {
			return sttWorkerReply{}, errSTTBusy
		}
	} else {
		qwenSTTLock.Lock()
	}
	defer qwenSTTLock.Unlock()
	lockMS := float64(time.Since(started).Microseconds()) / 1000
	engineStarted := time.Now()
	if status := getSTTModelStatusFor(qwenSTTModelID); !status.Active {
		return sttWorkerReply{}, fmt.Errorf("Qwen3-ASR: %s", status.Message)
	}
	if activeQwenSTTWorker == nil {
		worker, err := startQwenSTTWorker(qwenSTTPythonPath(), getWritableSTTAssetsDir(qwenSTTModelID), qwenSTTRuntimeDir())
		if err != nil {
			return sttWorkerReply{}, err
		}
		activeQwenSTTWorker = worker
	}
	request := STTTranscribeRequest{Audio: base64.StdEncoding.EncodeToString(audio), Format: format, Language: lang, SpeechFilter: speechFilter}
	if err := json.NewEncoder(activeQwenSTTWorker.input).Encode(request); err != nil {
		activeQwenSTTWorker.close()
		activeQwenSTTWorker = nil
		return sttWorkerReply{}, err
	}
	reply, err := activeQwenSTTWorker.read(90 * time.Second)
	reply.LockMS = lockMS
	reply.EngineMS = float64(time.Since(engineStarted).Microseconds()) / 1000
	if err != nil && reply.Error == "" {
		activeQwenSTTWorker.close()
		activeQwenSTTWorker = nil
	}
	return reply, err
}
