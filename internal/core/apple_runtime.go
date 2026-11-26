package core

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type appleRuntimeWorker struct {
	mu      sync.Mutex
	writeMu sync.Mutex
	cmd     *exec.Cmd
	input   io.WriteCloser
	pending map[uint64]chan []byte
	next    uint64
	log     limitedWorkerLog
}

var appleNative = &appleRuntimeWorker{}
var appleSpeechMu sync.Mutex
var appleTranslationMu sync.Mutex

var (
	macOSMajorVersionOnce sync.Once
	macOSMajorVersionVal  int
	macOSMajorVersionErr  error
)

func detectMacOSMajorVersion() (int, error) {
	macOSMajorVersionOnce.Do(func() {
		version, err := exec.Command("/usr/bin/sw_vers", "-productVersion").Output()
		if err != nil {
			macOSMajorVersionErr = err
			return
		}
		macOSMajorVersionVal, macOSMajorVersionErr = strconv.Atoi(strings.Split(strings.TrimSpace(string(version)), ".")[0])
	})
	return macOSMajorVersionVal, macOSMajorVersionErr
}

func tryLockWithTimeout(ctx context.Context, mu *sync.Mutex, wait time.Duration) bool {
	if mu.TryLock() {
		return true
	}
	if wait <= 0 {
		return false
	}
	deadline := time.Now().Add(wait)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if mu.TryLock() {
				return true
			}
		}
	}
	return mu.TryLock()
}

const maxTranslationResultCache = 128

var translationResultCache = struct {
	sync.Mutex
	entries map[string]string
	order   []string
}{entries: make(map[string]string)}

func translationCacheKey(backend, source, target, text string) string {
	return backend + "\x00" + source + "\x00" + target + "\x00" + text
}

func getCachedTranslation(backend, source, target, text string) (string, bool) {
	if text == "" {
		return "", false
	}
	key := translationCacheKey(backend, source, target, text)
	translationResultCache.Lock()
	defer translationResultCache.Unlock()
	val, ok := translationResultCache.entries[key]
	return val, ok
}

func putCachedTranslation(backend, source, target, text, translated string) {
	if text == "" || strings.TrimSpace(translated) == "" {
		return
	}
	key := translationCacheKey(backend, source, target, text)
	translationResultCache.Lock()
	defer translationResultCache.Unlock()
	if _, exists := translationResultCache.entries[key]; !exists {
		if len(translationResultCache.order) >= maxTranslationResultCache {
			evict := translationResultCache.order[0]
			translationResultCache.order = translationResultCache.order[1:]
			delete(translationResultCache.entries, evict)
		}
		translationResultCache.order = append(translationResultCache.order, key)
	}
	translationResultCache.entries[key] = translated
}

func appleRuntimeExecutable() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", errors.New("Apple 엔진은 macOS 26 이상에서만 지원합니다")
	}
	major, err := detectMacOSMajorVersion()
	if err != nil || major < 26 {
		return "", errors.New("Apple 엔진은 macOS 26 이상이 필요합니다")
	}
	if override := os.Getenv("DKST_APPLE_RUNTIME"); override != "" {
		return override, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	candidates := []string{filepath.Join(filepath.Dir(executable), "..", "Helpers", "DKSTAppleRuntime.app", "Contents", "MacOS", "DKSTAppleRuntime"), filepath.Join("build", "bin", "DKST LLM Chat Server.app", "Contents", "Helpers", "DKSTAppleRuntime.app", "Contents", "MacOS", "DKSTAppleRuntime")}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return filepath.Abs(path)
		}
	}
	return "", errors.New("Apple 보조 엔진이 앱에 없습니다. 게이트웨이 앱을 업데이트하세요")
}
func (w *appleRuntimeWorker) startLocked() error {
	if w.cmd != nil {
		return nil
	}
	path, err := appleRuntimeExecutable()
	if err != nil {
		return err
	}
	cmd := exec.Command(path)
	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return err
	}
	cmd.Stderr = &w.log
	if err = cmd.Start(); err != nil {
		input.Close()
		return err
	}
	w.cmd = cmd
	w.input = input
	w.pending = map[uint64]chan []byte{}
	go func() {
		scanner := bufio.NewScanner(output)
		scanner.Buffer(make([]byte, 4096), 8*1024*1024)
		for scanner.Scan() {
			data := append([]byte(nil), scanner.Bytes()...)
			var header struct {
				ID uint64 `json:"id"`
			}
			if json.Unmarshal(data, &header) != nil {
				continue
			}
			w.mu.Lock()
			if w.cmd == cmd {
				if ch := w.pending[header.ID]; ch != nil {
					delete(w.pending, header.ID)
					ch <- data
				}
			}
			w.mu.Unlock()
		}
		_ = cmd.Wait()
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.cmd == cmd {
			w.cmd = nil
			w.input = nil
			for id, ch := range w.pending {
				delete(w.pending, id)
				close(ch)
			}
		}
	}()
	return nil
}
func (w *appleRuntimeWorker) request(ctx context.Context, p map[string]any) (map[string]any, error) {
	releaseMemory := appleModelMemory.begin(p["op"] != "status")
	defer releaseMemory()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	w.mu.Lock()
	if err := w.startLocked(); err != nil {
		w.mu.Unlock()
		return nil, err
	}
	w.next++
	id := w.next
	p["id"] = id
	ch := make(chan []byte, 1)
	w.pending[id] = ch
	input := w.input
	w.mu.Unlock()
	defer func() { w.mu.Lock(); delete(w.pending, id); w.mu.Unlock() }()
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	w.writeMu.Lock()
	_, err = input.Write(append(data, '\n'))
	w.writeMu.Unlock()
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		// Cancel the native request as well as its Go wait. Otherwise a timed-out
		// STT finalization can keep the native speech queue occupied indefinitely.
		// Keep shared model installation alive and never kill the shared runtime.
		if p["op"] == "translate" || p["op"] == "sttPush" || p["op"] == "sttRead" {
			cancelData, _ := json.Marshal(map[string]any{"op": "cancel", "cancelID": id})
			w.writeMu.Lock()
			_, _ = input.Write(append(cancelData, '\n'))
			w.writeMu.Unlock()
		}
		return nil, ctx.Err()
	case data, ok := <-ch:
		if !ok {
			return nil, errors.New("Apple 엔진이 종료되었습니다: " + w.log.String())
		}
		var result map[string]any
		if err = json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		if message, _ := result["error"].(string); message != "" {
			return result, errors.New(message)
		}
		return result, nil
	}
}
func (w *appleRuntimeWorker) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cmd != nil {
		_ = w.input.Close()
		_ = w.cmd.Process.Kill()
		w.cmd = nil
		w.input = nil
		for id, ch := range w.pending {
			delete(w.pending, id)
			close(ch)
		}
	}
}
func appleSpeechIdentity(user, session, segment string) (string, string) {
	owner := fmt.Sprintf("%x", sha256.Sum256([]byte(user+"\x00"+session)))
	return owner, fmt.Sprintf("%x", sha256.Sum256([]byte(owner+"\x00"+segment)))
}
func runAppleSpeech(ctx context.Context, metadata STTTranscribeRequest, audio []byte, preview bool, user string) (sttWorkerReply, error) {
	var output sttWorkerReply
	started := time.Now()
	if metadata.Format != "" && metadata.Format != "pcm" {
		return output, errors.New("Apple STT는 PCM16 입력이 필요합니다")
	}
	if preview {
		if !appleSpeechMu.TryLock() {
			return output, errSTTBusy
		}
	} else {
		if !tryLockWithTimeout(ctx, &appleSpeechMu, 2*time.Second) {
			if ctx.Err() != nil {
				return output, ctx.Err()
			}
			return output, errSTTBusy
		}
	}
	defer appleSpeechMu.Unlock()
	output.LockMS = float64(time.Since(started).Microseconds()) / 1000
	engineStarted := time.Now()
	segment := metadata.SegmentID
	if segment == "" {
		segment = fmt.Sprintf("legacy-%d", time.Now().UnixNano())
	}
	owner, key := appleSpeechIdentity(user, metadata.SessionID, segment)
	language := metadata.Language
	if language == "" {
		language = "ko"
	}
	result, err := appleNative.request(ctx, map[string]any{"op": "sttPush", "session": key, "owner": owner, "language": language, "audio": base64.StdEncoding.EncodeToString(audio), "final": !preview})
	if err != nil {
		return output, err
	}
	output.EngineMS = float64(time.Since(engineStarted).Microseconds()) / 1000
	output.Text, _ = result["text"].(string)
	output.StableText, _ = result["stableText"].(string)
	output.DraftText, _ = result["draftText"].(string)
	output.Text = strings.TrimSpace(output.Text)
	if !preview && output.Text == "" {
		output.Rejected = true
		output.Reason = "no_speech"
	}
	return output, nil
}
func handleAppleStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", 405)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		result, err := appleNative.request(ctx, map[string]any{"op": "status"})
		if err != nil {
			result = map[string]any{"available": false, "error": err.Error()}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}
}
func handleAppleOperation(op string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		var req struct {
			Language  string `json:"language"`
			Source    string `json:"source"`
			Target    string `json:"target"`
			Text      string `json:"text"`
			SessionID string `json:"sessionId"`
			SegmentID string `json:"segmentId"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32000)).Decode(&req); err != nil {
			http.Error(w, "invalid Apple request", 400)
			return
		}
		var validatedTranslation chromeTranslationRequest
		if op == "translate" || op == "translationPrepare" {
			validatedTranslation = chromeTranslationRequest{Source: req.Source, Target: req.Target, Text: req.Text}
			if err := validateChromeTranslation(&validatedTranslation, op == "translationPrepare"); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if op == "translate" {
				if validatedTranslation.Source == validatedTranslation.Target {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"status": "ready", "text": validatedTranslation.Text})
					return
				}
				if cached, ok := getCachedTranslation("apple", validatedTranslation.Source, validatedTranslation.Target, validatedTranslation.Text); ok {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"status": "ready", "text": cached, "cached": true})
					return
				}
			}
			wait := 250 * time.Millisecond
			if op == "translationPrepare" {
				wait = 0
			}
			if !tryLockWithTimeout(r.Context(), &appleTranslationMu, wait) {
				if r.Context().Err() != nil {
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(429)
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "busy", "error": "Apple 번역 처리 중입니다"})
				return
			}
			defer appleTranslationMu.Unlock()
			if op == "translate" {
				if cached, ok := getCachedTranslation("apple", validatedTranslation.Source, validatedTranslation.Target, validatedTranslation.Text); ok {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"status": "ready", "text": cached, "cached": true})
					return
				}
			}
		}
		p := map[string]any{"op": op}
		switch op {
		case "sttPrepare":
			if len(req.Language) != 2 {
				http.Error(w, "invalid language", 400)
				return
			}
			p["language"] = req.Language
		case "sttStop":
			if req.SessionID == "" || req.SegmentID == "" || len(req.SessionID) > 100 || len(req.SegmentID) > 100 {
				http.Error(w, "invalid speech session", 400)
				return
			}
			owner, key := appleSpeechIdentity(r.Header.Get("X-User-ID"), req.SessionID, req.SegmentID)
			p["owner"] = owner
			p["session"] = key
		default:
			p["source"] = validatedTranslation.Source
			p["target"] = validatedTranslation.Target
			p["text"] = validatedTranslation.Text
		}
		timeout := 12 * time.Second
		if op == "sttPrepare" || op == "translationPrepare" {
			timeout = 180 * time.Second
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		started := time.Now()
		result, err := appleNative.request(ctx, p)
		w.Header().Set("Content-Type", "application/json")
		if result == nil {
			result = map[string]any{}
		}
		result["timings"] = map[string]float64{"server_ms": float64(time.Since(started).Microseconds()) / 1000}
		if err != nil {
			result["error"] = err.Error()
			result["status"] = "error"
			w.WriteHeader(503)
		} else if op == "translate" {
			if translated, ok := result["text"].(string); ok && strings.TrimSpace(translated) != "" {
				putCachedTranslation("apple", validatedTranslation.Source, validatedTranslation.Target, validatedTranslation.Text, translated)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}
}
