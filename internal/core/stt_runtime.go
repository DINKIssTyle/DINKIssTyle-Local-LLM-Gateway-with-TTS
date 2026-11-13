package core

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	bundledata "dinkisstyle-chat/bundle"
)

func sttRuntimeDir() string { return joinAppDataPath(assetsDirName, runtimeDirName, "whisper") }
func sttPythonPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(sttRuntimeDir(), "venv", "Scripts", "python.exe")
	}
	return filepath.Join(sttRuntimeDir(), "venv", "bin", "python")
}

func installSTTDependencies() error {
	uv, err := exec.LookPath("uv")
	if err != nil {
		home, _ := os.UserHomeDir()
		uv = filepath.Join(home, ".local", "bin", "uv")
		if !fileExists(uv) {
			return fmt.Errorf("Whisper runtime setup requires uv; install uv, then download STT again")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	steps := [][]string{}
	if !fileExists(sttPythonPath()) {
		steps = append(steps, []string{"venv", "--python", "3.12", filepath.Join(sttRuntimeDir(), "venv")})
	}
	steps = append(steps, []string{"pip", "install", "--python", sttPythonPath(), "optimum-onnx[onnxruntime]==0.1.0", "transformers==4.57.6", "torch>=2.2,<3", "numpy>=1.26,<3"})
	for _, args := range steps {
		cmd := exec.CommandContext(ctx, uv, args...)
		var output limitedWorkerLog
		cmd.Stdout, cmd.Stderr = &output, &output
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Whisper runtime installation failed: %w: %s", err, output.String())
		}
	}
	return os.WriteFile(filepath.Join(sttRuntimeDir(), "installed"), []byte("optimum-onnx-0.1.0/transformers-4.57.6"), 0644)
}

func installMLXSTTDependencies() error {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return fmt.Errorf("MLX Whisper requires Apple Silicon macOS")
	}
	if err := installSTTDependencies(); err != nil {
		return err
	}
	uv, err := exec.LookPath("uv")
	if err != nil {
		home, _ := os.UserHomeDir()
		uv = filepath.Join(home, ".local", "bin", "uv")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, uv, "pip", "install", "--python", sttPythonPath(), "mlx-whisper==0.4.3")
	var output limitedWorkerLog
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("MLX Whisper installation failed: %w: %s", err, output.String())
	}
	return os.WriteFile(filepath.Join(sttRuntimeDir(), "installed-mlx"), []byte("mlx-whisper-0.4.3"), 0644)
}

type sttWorker struct {
	cmd    *exec.Cmd
	input  io.WriteCloser
	output *bufio.Reader
	log    limitedWorkerLog
}
type sttWorkerReply struct {
	StableText string  `json:"stableText,omitempty"`
	DraftText  string  `json:"draftText,omitempty"`
	Ready      bool    `json:"ready"`
	Text       string  `json:"text"`
	Error      string  `json:"error"`
	Duration   float64 `json:"duration"`
	Rejected   bool    `json:"rejected"`
	Reason     string  `json:"reason"`
}

var activeSTTWorker *sttWorker // protected by sttLock
var activeSTTModelID string

func (w *sttWorker) read(timeout time.Duration) (sttWorkerReply, error) {
	type response struct {
		reply sttWorkerReply
		err   error
	}
	ch := make(chan response, 1)
	go func() {
		line, err := w.output.ReadBytes('\n')
		var reply sttWorkerReply
		if err == nil {
			err = json.Unmarshal(line, &reply)
		}
		ch <- response{reply, err}
	}()
	select {
	case result := <-ch:
		if result.err != nil {
			return result.reply, fmt.Errorf("Whisper worker: %w: %s", result.err, w.log.String())
		}
		if result.reply.Error != "" {
			return result.reply, fmt.Errorf("Whisper: %s", result.reply.Error)
		}
		return result.reply, nil
	case <-time.After(timeout):
		return sttWorkerReply{}, fmt.Errorf("Whisper inference timed out")
	}
}
func (w *sttWorker) close() {
	_ = w.input.Close()
	_ = w.cmd.Process.Kill()
	_ = w.cmd.Wait()
}
func closeSTTRuntime() {
	sttLock.Lock()
	defer sttLock.Unlock()
	if activeSTTWorker != nil {
		activeSTTWorker.close()
		activeSTTWorker = nil
	}
}
func startSTTWorker(python, model, root string) (*sttWorker, error) {
	path := filepath.Join(root, "whisper_worker.py")
	if err := bundledata.MaterializeWhisperWorker(path); err != nil {
		return nil, err
	}
	worker := &sttWorker{}
	args := []string{"-u", path, "--model", model}
	if filepath.Base(model) == mlxSTTModelID {
		args = append(args, "--backend", "mlx")
	}
	worker.cmd = exec.Command(python, args...)
	worker.cmd.Env = append(os.Environ(), "HF_HUB_OFFLINE=1", "TRANSFORMERS_OFFLINE=1", "TOKENIZERS_PARALLELISM=false")
	worker.cmd.Stderr = &worker.log
	input, err := worker.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	worker.input = input
	output, err := worker.cmd.StdoutPipe()
	if err != nil {
		_ = input.Close()
		return nil, err
	}
	worker.output = bufio.NewReader(output)
	if err := worker.cmd.Start(); err != nil {
		_ = input.Close()
		return nil, err
	}
	reply, err := worker.read(120 * time.Second)
	if err != nil || !reply.Ready {
		worker.close()
		return nil, fmt.Errorf("Whisper startup failed: %v", err)
	}
	return worker, nil
}

var errSTTBusy = errors.New("STT engine busy")

func runSTTInference(audio []byte, format, lang, speechFilter string, preview ...bool) (sttWorkerReply, error) {
	if len(preview) > 0 && preview[0] {
		if !sttLock.TryLock() {
			return sttWorkerReply{}, errSTTBusy
		}
	} else {
		sttLock.Lock()
	}
	defer sttLock.Unlock()
	modelID := selectedSTTModelID()
	if activeSTTWorker != nil && activeSTTModelID != "" && activeSTTModelID != modelID {
		activeSTTWorker.close()
		activeSTTWorker = nil
	}
	if !isSTTModelInstalled(modelID) || !fileExists(sttPythonPath()) {
		return sttWorkerReply{}, fmt.Errorf("Whisper model/runtime is missing; download STT in Asset Manager")
	}
	if activeSTTWorker == nil {
		worker, err := startSTTWorker(sttPythonPath(), getWritableSTTAssetsDir(modelID), sttRuntimeDir())
		if err != nil {
			return sttWorkerReply{}, err
		}
		activeSTTWorker = worker
		activeSTTModelID = modelID
	}
	request := STTTranscribeRequest{Audio: base64.StdEncoding.EncodeToString(audio), Format: format, Language: lang, SpeechFilter: speechFilter}
	if err := json.NewEncoder(activeSTTWorker.input).Encode(request); err != nil {
		activeSTTWorker.close()
		activeSTTWorker = nil
		return sttWorkerReply{}, err
	}
	reply, err := activeSTTWorker.read(90 * time.Second)
	if err != nil {
		if reply.Error == "" {
			activeSTTWorker.close()
			activeSTTWorker = nil
		}
		return sttWorkerReply{}, err
	}
	return reply, nil
}
