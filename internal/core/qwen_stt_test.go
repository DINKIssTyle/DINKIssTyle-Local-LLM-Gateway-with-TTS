package core

import (
	"bytes"
	"dinkisstyle-chat/internal/mcp"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQwenSTTModelPlanAndStatus(t *testing.T) {
	status := getSTTModelStatusFor(qwenSTTModelID)
	if status.ModelID != qwenSTTModelID || status.Provider != "mlx" || status.CanDownload != qwenSTTSupported() {
		t.Fatalf("unexpected Qwen status: %+v", status)
	}
	if qwenSTTPythonPath() == sttPythonPath() {
		t.Fatal("Qwen and Whisper must have separate environments")
	}
	app := &App{}
	found := false
	for _, model := range app.GetManagedModels() {
		if model.Key == "stt:"+qwenSTTModelID {
			found = true
		}
	}
	if !found {
		t.Fatal("Qwen missing from asset manager")
	}
	_, specs, install, err := app.getManagedModelDownloadPlan("stt:" + qwenSTTModelID)
	if !qwenSTTSupported() {
		if err == nil {
			t.Fatal("unsupported download allowed")
		}
		return
	}
	if err != nil || install == nil || len(specs) != len(qwenSTTFiles()) {
		t.Fatalf("invalid plan: %v %+v", err, specs)
	}
	for i, spec := range specs {
		if filepath.Base(spec.DestPath) != qwenSTTFiles()[i] {
			t.Fatalf("missing asset: %s", spec.DestPath)
		}
		if !strings.HasSuffix(spec.DestPath, "silero_vad.onnx") && !strings.Contains(spec.URL, qwenSTTModelRevision) {
			t.Fatal("model revision must be pinned")
		}
	}
}

func TestQwenSTTStatusAndEngineValidation(t *testing.T) {
	for _, engine := range []string{"qwen3-asr", "not-an-engine"} {
		w := httptest.NewRecorder()
		handleSTTStatus(&App{})(w, httptest.NewRequest(http.MethodGet, "/api/stt/status?engine="+engine, nil))
		if engine == "not-an-engine" {
			if w.Code != 400 {
				t.Fatal(w.Code)
			}
			continue
		}
		var status ManagedModelStatus
		if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil || status.ModelID != qwenSTTModelID {
			t.Fatalf("Qwen query resolved to another engine: %s", w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	handleSTTWarmup()(w, httptest.NewRequest(http.MethodPost, "/api/stt/warmup?engine=invalid", nil))
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}

func TestQwenPreviewBusyNeverFallsBackOrQueues(t *testing.T) {
	qwenSTTLock.Lock()
	defer qwenSTTLock.Unlock()
	if _, err := runQwenSTTInference(nil, "pcm", "auto", "strict", true); err != errSTTBusy {
		t.Fatal(err)
	}
	body, _ := json.Marshal(STTTranscribeRequest{Engine: "qwen3-asr", Fallback: true, Audio: base64.StdEncoding.EncodeToString(make([]byte, 32000)), Format: "pcm", SampleRate: 16000, Language: "auto", SessionID: "qwen-test", SegmentID: "busy", Revision: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/stt/preview", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleSTTPreview(&App{})(w, req)
	if w.Code != 429 || w.Header().Get("Retry-After") != "1" {
		t.Fatalf("busy Qwen preview should retry independently: %d %s", w.Code, w.Body.String())
	}
}

// Optional real GPU test. Does not download models or mutate the user's assets.
func TestQwenSTTWorkerIntegration(t *testing.T) {
	python, model, fixtures := os.Getenv("DKST_QWEN_TEST_PYTHON"), os.Getenv("DKST_QWEN_TEST_MODEL"), os.Getenv("DKST_QWEN_TEST_AUDIO_DIR")
	if python == "" || model == "" || fixtures == "" {
		t.Skip("explicit local Qwen integration assets not specified")
	}
	started := time.Now()
	worker, err := startQwenSTTWorker(python, model, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer worker.close()
	t.Logf("Qwen cold load: %s", time.Since(started))
	for _, lang := range []string{"ko", "en", "ja"} {
		audio, err := os.ReadFile(filepath.Join(fixtures, lang+".wav"))
		if err != nil {
			t.Fatal(err)
		}
		request := STTTranscribeRequest{Audio: base64.StdEncoding.EncodeToString(audio), Format: "wav", Language: "auto", SpeechFilter: "standard"}
		started := time.Now()
		if err := json.NewEncoder(worker.input).Encode(request); err != nil {
			t.Fatal(err)
		}
		reply, err := worker.read(90 * time.Second)
		if err != nil || reply.Text == "" || reply.Rejected || reply.Language != lang {
			t.Fatalf("%s: %+v %v", lang, reply, err)
		}
		t.Logf("%s %.2fs audio in %s: %s", lang, reply.Duration, time.Since(started), reply.Text)
	}
	request := STTTranscribeRequest{Audio: base64.StdEncoding.EncodeToString(make([]byte, 64000)), Format: "pcm", Language: "auto"}
	if err := json.NewEncoder(worker.input).Encode(request); err != nil {
		t.Fatal(err)
	}
	reply, err := worker.read(30 * time.Second)
	if err != nil || reply.Text != "" {
		t.Fatalf("silence produced text: %+v %v", reply, err)
	}
}

// Verify the installed asset/runtime plus HTTP persistence, in an isolated test DB.
func TestQwenInstalledGatewayIntegration(t *testing.T) {
	if os.Getenv("DKST_QWEN_TEST_INSTALLED") != "1" {
		t.Skip("installed Qwen end-to-end test not requested")
	}
	if !getSTTModelStatusFor(qwenSTTModelID).Active {
		t.Fatal("Qwen assets/runtime not installed")
	}
	if err := mcp.InitDB(filepath.Join(t.TempDir(), "transcripts.db")); err != nil {
		t.Fatal(err)
	}
	defer mcp.CloseDB()
	defer closeQwenSTTRuntime()
	w := httptest.NewRecorder()
	handleSTTWarmup()(w, httptest.NewRequest(http.MethodPost, "/api/stt/warmup?engine=qwen3-asr", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), qwenSTTModelID) {
		t.Fatalf("warmup %d: %s", w.Code, w.Body.String())
	}
	audio, err := os.ReadFile(filepath.Join(os.Getenv("DKST_QWEN_TEST_AUDIO_DIR"), "ko.wav"))
	if err != nil {
		t.Fatal(err)
	}
	metadata := STTTranscribeRequest{Engine: "qwen3-asr", Format: "wav", Audio: base64.StdEncoding.EncodeToString(audio), Language: "auto", SessionID: "qwen-http", SegmentID: "question", Revision: 1, TriggerWord: "안녕하세요", SpeechFilter: "standard", StartedAt: "2026-11-23T06:00:00Z", EndedAt: "2026-11-23T06:00:06Z"}
	for _, preview := range []bool{true, false} {
		body, _ := json.Marshal(metadata)
		req := httptest.NewRequest(http.MethodPost, "/api/stt/transcribe", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", "qwen-test-user")
		w := httptest.NewRecorder()
		handleSTTAudio(&App{}, preview)(w, req)
		var response STTTranscribeResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || w.Code != 200 {
			t.Fatalf("HTTP %d %s", w.Code, w.Body.String())
		}
		if response.Engine != "qwen3-asr" || response.Language != "ko" || response.Text == "" {
			t.Fatalf("lost engine/language/text: %+v", response)
		}
		rows, err := mcp.ListTranscripts("qwen-test-user", "2026-11-23T00:00:00Z", "2026-11-24T00:00:00Z")
		if err != nil {
			t.Fatal(err)
		}
		if preview {
			if response.Saved || response.Triggered || response.Stage != "draft" || len(rows) != 0 {
				t.Fatalf("preview leaked into storage/trigger: %+v", response)
			}
		} else if !response.Saved || !response.Triggered || response.Stage != "final" || len(rows) != 1 {
			t.Fatalf("final not saved/triggered: %+v rows=%d", response, len(rows))
		}
		t.Logf("preview=%v: %+v", preview, response)
	}
}
