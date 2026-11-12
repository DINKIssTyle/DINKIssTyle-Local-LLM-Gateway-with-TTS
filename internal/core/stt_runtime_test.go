package core

import (
	"bytes"
	"dinkisstyle-chat/internal/mcp"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	wavcodec "github.com/go-audio/wav"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWhisperRuntimeIntegration(t *testing.T) {
	python, model, wav := os.Getenv("DKST_STT_TEST_PYTHON"), os.Getenv("DKST_STT_TEST_MODEL"), os.Getenv("DKST_STT_TEST_WAV")
	if python == "" || model == "" || wav == "" {
		t.Skip("local Whisper integration assets not specified")
	}
	worker, err := startSTTWorker(python, model, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer worker.close()
	audio, err := os.ReadFile(wav)
	if err != nil {
		t.Fatal(err)
	}

	rng := rand.New(rand.NewSource(10))
	noise, tone, hum, click := make([]byte, 64000), make([]byte, 64000), make([]byte, 64000), make([]byte, 64000)
	for i := 0; i < 32000; i++ {
		binary.LittleEndian.PutUint16(noise[i*2:], uint16(int16(rng.NormFloat64()*3000)))
		binary.LittleEndian.PutUint16(tone[i*2:], uint16(int16(5000*math.Sin(2*math.Pi*440*float64(i)/16000))))
		binary.LittleEndian.PutUint16(hum[i*2:], uint16(int16(5000*math.Sin(2*math.Pi*100*float64(i)/16000)+rng.NormFloat64()*180)))
		if i >= 8000 && i < 8100 {
			binary.LittleEndian.PutUint16(click[i*2:], uint16(29000))
		}
	}
	for _, test := range []struct {
		audio    []byte
		format   string
		wantText bool
	}{{audio, "wav", true}, {make([]byte, 32000), "pcm", false}, {noise, "pcm", false}, {tone, "pcm", false}, {hum, "pcm", false}, {click, "pcm", false}} {
		if err := json.NewEncoder(worker.input).Encode(STTTranscribeRequest{Audio: base64.StdEncoding.EncodeToString(test.audio), Format: test.format, Language: "ko"}); err != nil {
			t.Fatal(err)
		}
		reply, err := worker.read(90 * time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if (reply.Text != "") != test.wantText {
			t.Fatalf("unexpected transcript: %+v", reply)
		}
		t.Logf("Whisper %.2fs: %s", reply.Duration, reply.Text)
	}
	if err := mcp.InitDB(filepath.Join(t.TempDir(), "transcripts.db")); err != nil {
		t.Fatal(err)
	}
	defer mcp.CloseDB()
	activeSTTWorker = worker
	activeSTTModelID = filepath.Base(model)
	defer func() { activeSTTWorker = nil; activeSTTModelID = "" }()
	decoder := wavcodec.NewDecoder(bytes.NewReader(audio))
	buffer, err := decoder.FullPCMBuffer()
	if err != nil {
		t.Fatal(err)
	}
	pcm := make([]byte, len(buffer.Data)*2)
	for i, sample := range buffer.Data {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(int16(sample)))
	}
	previewTextCount := 0
	for revision, seconds := range []float64{1.3, 2.3, 3.3} {
		length := int(seconds * 32000)
		if length > len(pcm) {
			t.Fatal("speech fixture too short")
		}
		payload, _ := json.Marshal(STTTranscribeRequest{Audio: base64.StdEncoding.EncodeToString(pcm[:length]), Format: "pcm", SampleRate: 16000, Language: "ko", SessionID: "test-session", SegmentID: "test-segment", Revision: revision + 1, TriggerWord: "안녕, 안 녕"})
		previewReq := httptest.NewRequest(http.MethodPost, "/api/stt/preview", bytes.NewReader(payload))
		previewReq.Header.Set("Content-Type", "application/json")
		previewReq.Header.Set("X-User-ID", "test-user")
		previewResponse := httptest.NewRecorder()
		started := time.Now()
		handleSTTPreview(&App{})(previewResponse, previewReq)
		if previewResponse.Code != 200 {
			t.Fatalf("preview HTTP %d: %s", previewResponse.Code, previewResponse.Body.String())
		}
		var preview STTTranscribeResponse
		if err := json.Unmarshal(previewResponse.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		if preview.Stage != "draft" || preview.Saved || preview.Triggered || preview.Prompt != "" || preview.SessionID != "test-session" || preview.Revision != revision+1 {
			t.Fatalf("unsafe preview: %+v", preview)
		}
		if preview.Text != "" {
			previewTextCount++
		}
		rows, err := mcp.ListTranscripts("test-user", "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z")
		if err != nil || len(rows) != 0 {
			t.Fatalf("preview entered database: %+v %v", rows, err)
		}
		t.Logf("draft %.1fs audio in %s: %s", seconds, time.Since(started), preview.Text)
	}
	if previewTextCount == 0 {
		t.Fatal("no incremental captions before utterance endpoint")
	}
	body, _ := json.Marshal(STTTranscribeRequest{Audio: base64.StdEncoding.EncodeToString(audio), Format: "wav", Language: "ko", SessionID: "test-session", SegmentID: "test-segment", StartedAt: "2026-11-12T05:00:00Z", EndedAt: "2026-11-12T05:00:06Z", TriggerWord: "안녕"})
	req := httptest.NewRequest(http.MethodPost, "/api/stt/transcribe", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "test-user")
	w := httptest.NewRecorder()
	handleSTTTranscribe(&App{})(w, req)
	if w.Code != 200 {
		t.Fatalf("transcription HTTP %d: %s", w.Code, w.Body.String())
	}
	var reply STTTranscribeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	if !reply.Saved || !reply.Triggered || reply.Text == "" {
		t.Fatalf("transcription not saved/triggered: %+v", reply)
	}
	rows, err := mcp.ListTranscripts("test-user", "2026-11-12T00:00:00Z", "2026-11-13T00:00:00Z")
	if err != nil || len(rows) != 1 || rows[0].Text != reply.Text {
		t.Fatalf("transcript persistence: %+v %v", rows, err)
	}
}
