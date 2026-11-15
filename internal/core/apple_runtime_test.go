package core

import (
	"bytes"
	"context"
	"dinkisstyle-chat/internal/mcp"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	wavcodec "github.com/go-audio/wav"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppleRoutesAndIdentity(t *testing.T) {
	for _, path := range []string{"/api/apple/status", "/api/stt/apple/prepare", "/api/stt/apple/stop", "/api/translate/apple", "/api/translate/apple/prepare"} {
		if !evenHubAPIPath(path) {
			t.Fatal(path)
		}
	}
	a, b := appleSpeechIdentity("one", "session", "segment")
	c, d := appleSpeechIdentity("two", "session", "segment")
	if a == c || b == d {
		t.Fatal("cross-user identity collision")
	}
}
func TestAppleRuntimeIntegration(t *testing.T) {
	if os.Getenv("DKST_APPLE_INTEGRATION") != "1" {
		t.Skip("explicit native integration not requested")
	}
	worker := &appleRuntimeWorker{}
	defer worker.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	result, err := worker.request(ctx, map[string]any{"op": "status"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("status: %+v", result)
	prepared, err := worker.request(ctx, map[string]any{"op": "sttPrepare", "language": "ko"})
	if err != nil {
		t.Fatalf("speech prepare: %v", err)
	}
	t.Logf("prepared: %+v", prepared)
	f, err := os.Open("/private/tmp/even-chrome-stt-ko.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	audio, err := wavcodec.NewDecoder(f).FullPCMBuffer()
	if err != nil {
		t.Fatal(err)
	}
	pcm := make([]byte, len(audio.Data)*2)
	for i, sample := range audio.Data {
		binary.LittleEndian.PutUint16(pcm[2*i:], uint16(int16(sample)))
	}
	started := time.Now()
	seen := false
	for size := 8000; size < len(pcm); size += 8000 {
		result, err = worker.request(ctx, map[string]any{"op": "sttPush", "session": "fixture", "owner": "synthetic", "language": "ko", "audio": base64.StdEncoding.EncodeToString(pcm[:size])})
		if err != nil {
			t.Fatal(err)
		}
		if text, _ := result["text"].(string); text != "" && !seen {
			t.Logf("first interim %.2fs: %s", time.Since(started).Seconds(), text)
			seen = true
		}
		time.Sleep(200 * time.Millisecond)
	}
	result, err = worker.request(ctx, map[string]any{"op": "sttPush", "session": "fixture", "owner": "synthetic", "language": "ko", "audio": base64.StdEncoding.EncodeToString(pcm), "final": true})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("final: %+v", result)
	text, _ := result["text"].(string)
	if !strings.Contains(text, "안녕") || !seen || result["finished"] != true {
		t.Fatalf("unexpected streaming result: %+v", result)
	}
	// A new live segment can start while older audio awaits persistence. Native
	// sessions interrupted by such switches must rebuild from full PCM snapshots.
	for _, id := range []string{"archive", "live"} {
		_, err = worker.request(ctx, map[string]any{"op": "sttPush", "session": id, "owner": "synthetic", "language": "ko", "audio": base64.StdEncoding.EncodeToString(pcm[:8000])})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"archive", "live"} {
		result, err = worker.request(ctx, map[string]any{"op": "sttPush", "session": id, "owner": "synthetic", "language": "ko", "audio": base64.StdEncoding.EncodeToString(pcm), "final": true})
		if err != nil {
			t.Fatal(err)
		}
		if text, _ := result["text"].(string); !strings.Contains(text, "안녕") || result["finished"] != true {
			t.Fatalf("interleaved %s lost full audio: %+v", id, result)
		}
		t.Logf("interleaved %s recovered: %v", id, result["text"])
	}

	// Exercise the actual HTTP save/trigger route, including a final-only upload.
	old := appleNative
	appleNative = worker
	defer func() { appleNative = old }()
	if err = mcp.InitDB(filepath.Join(t.TempDir(), "apple-transcripts.db")); err != nil {
		t.Fatal(err)
	}
	defer mcp.CloseDB()
	metadata := STTTranscribeRequest{Engine: "apple", Format: "pcm", SampleRate: 16000, Language: "ko", SessionID: "apple-gateway", SegmentID: "final-only", TriggerWord: "안녕하세요", Audio: base64.StdEncoding.EncodeToString(pcm)}
	body, _ := json.Marshal(metadata)
	for attempt := 0; attempt < 2; attempt++ {
		req := httptest.NewRequest("POST", "/api/stt/transcribe", bytes.NewReader(body))
		req.Header.Set("X-User-ID", "apple-fixture")
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handleSTTTranscribe(&App{})(recorder, req)
		var response STTTranscribeResponse
		_ = json.Unmarshal(recorder.Body.Bytes(), &response)
		if recorder.Code != 200 || response.Engine != "apple" || !response.Saved || !response.Triggered || !strings.Contains(response.Text, "안녕") {
			t.Fatalf("gateway final-only: %d %s", recorder.Code, recorder.Body.String())
		}
	}
	entries, err := mcp.ListTranscripts("apple-fixture", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano), time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano))
	if err != nil || len(entries) != 1 {
		t.Fatalf("duplicate transcript: %d %v", len(entries), err)
	}
	metadata.SegmentID = "preview-only"
	metadata.Revision = 1
	metadata.Audio = base64.StdEncoding.EncodeToString(pcm[:16000])
	previewBody, _ := json.Marshal(metadata)
	previewRequest := httptest.NewRequest("POST", "/api/stt/preview", bytes.NewReader(previewBody))
	previewRequest.Header.Set("Content-Type", "application/json")
	previewRequest.Header.Set("X-User-ID", "apple-fixture")
	previewResponse := httptest.NewRecorder()
	handleSTTPreview(&App{})(previewResponse, previewRequest)
	var draft STTTranscribeResponse
	_ = json.Unmarshal(previewResponse.Body.Bytes(), &draft)
	if previewResponse.Code != 200 || draft.Stage != "draft" || draft.Saved || draft.Triggered {
		t.Fatalf("preview side effects: %d %s", previewResponse.Code, previewResponse.Body.String())
	}
	if os.Getenv("DKST_APPLE_TRANSLATION_TEST") == "1" {
		for _, op := range []string{"translationPrepare", "translate"} {
			result, err = worker.request(ctx, map[string]any{"op": op, "source": "ko", "target": "en", "text": "커피 드시겠어요?"})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: %+v", op, result)
		}
		if !strings.Contains(strings.ToLower(result["text"].(string)), "coffee") {
			t.Fatal(result)
		}
		worker.Close()
		fresh := &appleRuntimeWorker{}
		defer fresh.Close()
		result, err = fresh.request(ctx, map[string]any{"op": "translate", "source": "ko", "target": "en", "text": "커피 드시겠어요?"})
		if err != nil || !strings.Contains(strings.ToLower(fmt.Sprint(result["text"])), "coffee") {
			t.Fatalf("translation restart without prepare: %+v %v", result, err)
		}
		t.Logf("translation after worker restart: %+v", result)

	}
}

func TestAppleHTTPValidation(t *testing.T) {
	for _, tc := range []struct{ op, body string }{
		{"sttPrepare", `{"language":"auto"}`}, {"sttStop", `{"sessionId":""}`},
		{"translate", `{"source":"auto","target":"ko","text":"hello"}`},
		{"translationPrepare", `{"source":"ko","target":""}`},
	} {
		req := httptest.NewRequest("POST", "/api/apple", strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		handleAppleOperation(tc.op)(w, req)
		if w.Code != 400 {
			t.Fatalf("%+v: %d %s", tc, w.Code, w.Body.String())
		}
	}
	appleTranslationMu.Lock()
	w := httptest.NewRecorder()
	handleAppleOperation("translate")(w, httptest.NewRequest("POST", "/api/translate/apple", strings.NewReader(`{"source":"ko","target":"en","text":"테스트"}`)))
	appleTranslationMu.Unlock()
	if w.Code != 429 {
		t.Fatal("translation contention must not create an unbounded queue", w.Code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker := &appleRuntimeWorker{}
	if _, err := worker.request(ctx, map[string]any{"op": "status"}); err == nil || worker.cmd != nil {
		t.Fatal("cancelled request started native engine")
	}
}

func TestAppleTranslationRequestCancellation(t *testing.T) {
	if os.Getenv("DKST_APPLE_CANCEL_TEST") != "1" {
		t.Skip("explicit installed native translation test")
	}
	worker := &appleRuntimeWorker{}
	defer worker.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := worker.request(ctx, map[string]any{"op": "translationPrepare", "source": "en", "target": "ko"}); err != nil {
		t.Fatal(err)
	}
	small, stop := context.WithTimeout(ctx, 25*time.Millisecond)
	_, err := worker.request(small, map[string]any{"op": "translate", "source": "en", "target": "ko", "text": strings.Repeat("The meeting is on Friday. ", 200)})
	stop()
	if err == nil {
		t.Log("translation completed before cancellation")
	}
	result, err := worker.request(ctx, map[string]any{"op": "translate", "source": "en", "target": "ko", "text": "Please bring your laptop."})
	if err != nil || result["status"] != "ready" {
		t.Fatalf("cancel killed shared runtime: %+v %v", result, err)
	}
	t.Log("cancelled old translation; next translation succeeded without runtime restart")
}

// An in-memory JSON-lines input verifies cancellation reaches the native task,
// without opening a microphone, launching a helper or installing language packs.
type appleCancelInput struct{ writes chan []byte }

func (w *appleCancelInput) Write(p []byte) (int, error) {
	w.writes <- append([]byte(nil), p...)
	return len(p), nil
}
func (w *appleCancelInput) Close() error { return nil }
func TestAppleSpeechCancellationProtocol(t *testing.T) {
	for _, op := range []string{"sttPush", "sttRead", "translate"} {
		t.Run(op, func(t *testing.T) {
			input := &appleCancelInput{writes: make(chan []byte, 2)}
			worker := &appleRuntimeWorker{cmd: &exec.Cmd{}, input: input, pending: make(map[uint64]chan []byte)}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := worker.request(ctx, map[string]any{"op": op}); done <- err }()
			var request map[string]any
			select {
			case raw := <-input.writes:
				if err := json.Unmarshal(raw, &request); err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("missing request")
			}
			cancel()
			select {
			case err := <-done:
				if err != context.Canceled {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("request did not cancel")
			}
			select {
			case raw := <-input.writes:
				var control map[string]any
				if err := json.Unmarshal(raw, &control); err != nil {
					t.Fatal(err)
				}
				if control["op"] != "cancel" || control["cancelID"] != request["id"] {
					t.Fatalf("wrong native cancellation: %s", raw)
				}
			case <-time.After(time.Second):
				t.Fatal("timed-out native work was left running")
			}
			if len(worker.pending) != 0 {
				t.Fatal("leaked canceled waiter")
			}
		})
	}
}

func TestAppleSpeechNativeCancellationRecovery(t *testing.T) {
	if os.Getenv("DKST_APPLE_SPEECH_CANCEL_TEST") != "1" {
		t.Skip("explicit installed native speech test")
	}
	// Use only installed Korean assets and synthetic file PCM. No preparation,
	// downloads, microphone capture or connection to the user's running gateway.
	f, err := os.Open("/private/tmp/even-chrome-stt-ko.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	audio, err := wavcodec.NewDecoder(f).FullPCMBuffer()
	if err != nil {
		t.Fatal(err)
	}
	pcm := make([]byte, len(audio.Data)*2)
	for i, sample := range audio.Data {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(int16(sample)))
	}
	worker := &appleRuntimeWorker{}
	defer worker.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("cancel-%d", i)
		first, err := worker.request(ctx, map[string]any{"op": "sttPush", "session": id, "owner": "isolated-cancel-test", "language": "ko", "audio": base64.StdEncoding.EncodeToString(pcm[:8000])})
		if err != nil || first["status"] != "ready" {
			t.Fatalf("installed speech startup: %v %v", first, err)
		}
		process := worker.cmd
		small, stop := context.WithTimeout(ctx, 25*time.Millisecond)
		_, err = worker.request(small, map[string]any{"op": "sttPush", "session": id, "owner": "isolated-cancel-test", "language": "ko", "audio": base64.StdEncoding.EncodeToString(pcm), "final": true})
		stop()
		if err == nil {
			t.Log("finalization completed before cancellation")
		}
		started := time.Now()
		next, err := worker.request(ctx, map[string]any{"op": "sttPush", "session": id + "-next", "owner": "isolated-cancel-test", "language": "ko", "audio": base64.StdEncoding.EncodeToString(pcm), "final": true})
		if err != nil || next["status"] != "ready" || strings.TrimSpace(fmt.Sprint(next["text"])) == "" {
			t.Fatalf("speech did not recover: %v %v", next, err)
		}
		if worker.cmd != process {
			t.Fatal("cancellation restarted the shared helper")
		}
		t.Logf("canceled finalization; next synthetic segment completed in %s", time.Since(started))
	}
}
