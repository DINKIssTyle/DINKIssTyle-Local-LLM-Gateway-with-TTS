package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"dinkisstyle-chat/internal/mcp"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	wavcodec "github.com/go-audio/wav"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestChromeSpeechIntegration(t *testing.T) {
	if os.Getenv("DKST_CHROME_SPEECH_TEST_WAV") == "" {
		t.Skip("explicit speech fixture not requested")
	}
	engine := os.Getenv("DKST_CHROME_SPEECH_ENGINE")
	if engine == "" {
		engine = "chrome-online"
	}
	profile := os.Getenv("DKST_CHROME_TRANSLATION_PROFILE")
	if profile == "" {
		profile = t.TempDir()
	}
	s := newChromeTranslator(profile)
	defer s.Close()
	old := chromeTranslation
	chromeTranslation = s
	defer func() { chromeTranslation = old }()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	prepared, err := s.speech(ctx, chromeSpeechRequest{Language: "ko", Engine: engine}, "prepare")
	if err != nil || prepared.Status != "ready" {
		t.Fatalf("prepare %+v %v", prepared, err)
	}
	f, err := os.Open(os.Getenv("DKST_CHROME_SPEECH_TEST_WAV"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	buffer, err := wavcodec.NewDecoder(f).FullPCMBuffer()
	if err != nil {
		t.Fatal(err)
	}
	pcm := make([]byte, len(buffer.Data)*2)
	for i, sample := range buffer.Data {
		binary.LittleEndian.PutUint16(pcm[2*i:], uint16(int16(sample)))
	}
	translateDrafts := os.Getenv("DKST_CHROME_SPEECH_TRANSLATE") == "1"
	if translateDrafts {
		if _, err := s.run(ctx, chromeTranslationRequest{Source: "ko", Target: "en"}, true); err != nil {
			t.Fatal(err)
		}
	}
	translatedCount := 0
	lastTranslated := ""
	metadata := STTTranscribeRequest{Engine: engine, Format: "pcm", Language: "ko", SessionID: "test-stream", SegmentID: "test-phrase"}
	started := time.Now()
	draftSeen := false
	lastLog := time.Time{}
	for size := 8000; size < len(pcm); size += 8000 {
		result, err := runChromeSpeech(ctx, metadata, pcm[:size], true, "test-user")
		if err != nil {
			t.Fatal(err)
		}
		if translateDrafts && result.Text != "" && result.Text != lastTranslated {
			translated, err := s.run(ctx, chromeTranslationRequest{Text: result.Text, Source: "ko", Target: "en"}, false)
			if err != nil || translated.Text == "" {
				t.Fatalf("live translation: %+v %v", translated, err)
			}
			translatedCount++
			lastTranslated = result.Text
			if translatedCount == 1 {
				t.Logf("first translated draft: %s", translated.Text)
			}
		}
		if result.Text != "" {
			if !draftSeen || time.Since(lastLog) > 2*time.Second {
				t.Logf("draft %.2fs: %s", time.Since(started).Seconds(), result.Text)
				lastLog = time.Now()
			}
			draftSeen = true
		}
		time.Sleep(200 * time.Millisecond)
	}
	result, err := runChromeSpeech(ctx, metadata, pcm, false, "test-user")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("final %.2fs: %s; draft seen %v", time.Since(started).Seconds(), result.Text, draftSeen)
	if translateDrafts {
		translated, err := s.run(ctx, chromeTranslationRequest{Text: result.Text, Source: "ko", Target: "en"}, false)
		if err != nil || translatedCount < 2 || !strings.Contains(strings.ToLower(translated.Text), "glasses") {
			t.Fatalf("final/live translation: %+v %d %v", translated, translatedCount, err)
		}
		t.Logf("%d translated drafts; final: %s", translatedCount, translated.Text)
	}

	if !draftSeen || result.Text == "" {
		owner := fmt.Sprintf("%x", sha256.Sum256([]byte("test-user\x00test-stream")))
		key := fmt.Sprintf("%x", sha256.Sum256([]byte(owner+"\x00test-phrase")))
		debug, debugErr := s.speech(ctx, chromeSpeechRequest{ID: key, Owner: owner, Language: "ko", Engine: engine}, "read")
		t.Logf("debug=%+v err=%v", debug, debugErr)

	}
	if !strings.Contains(result.Text, "안녕") && !strings.Contains(result.Text, "날씨") {
		t.Fatalf("unexpected recognition: %+v", result)
	}
	if !draftSeen {
		t.Fatal("no streaming partial result")
	}
	if err := mcp.InitDB(filepath.Join(t.TempDir(), "chrome-transcripts.db")); err != nil {
		t.Fatal(err)
	}
	defer mcp.CloseDB()
	metadata.Audio = base64.StdEncoding.EncodeToString(pcm)
	metadata.SampleRate = 16000
	metadata.Revision = 99
	metadata.TriggerWord = "안녕하세요"
	body, _ := json.Marshal(metadata)
	previewRequest := httptest.NewRequest("POST", "/api/stt/preview", bytes.NewReader(body))
	previewRequest.Header.Set("Content-Type", "application/json")
	previewRequest.Header.Set("X-User-ID", "test-user")
	previewResponse := httptest.NewRecorder()
	handleSTTPreview(&App{})(previewResponse, previewRequest)
	var draft STTTranscribeResponse
	_ = json.Unmarshal(previewResponse.Body.Bytes(), &draft)
	if previewResponse.Code != 200 || draft.Saved || draft.Triggered || draft.Stage != "draft" {
		t.Fatalf("preview %d %s", previewResponse.Code, previewResponse.Body.String())
	}
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest("POST", "/api/stt/transcribe", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-User-ID", "test-user")
		response := httptest.NewRecorder()
		handleSTTTranscribe(&App{})(response, request)
		var final STTTranscribeResponse
		_ = json.Unmarshal(response.Body.Bytes(), &final)
		if response.Code != 200 || !final.Saved || !final.Triggered || final.Engine != "chrome-online" {
			t.Fatalf("final %d %s", response.Code, response.Body.String())
		}
	}
	entries, err := mcp.ListTranscripts("test-user", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano), time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano))
	if err != nil || len(entries) != 1 {
		t.Fatalf("duplicate final storage: %d %v", len(entries), err)
	}

}

func TestChromeSpeechEngineValidation(t *testing.T) {
	if chromeSTTEngine("chrome-local") || chromeSTTEngine("unknown") || !chromeSTTEngine("chrome-online") {
		t.Fatal("only verified Chrome online mode is exposed")
	}
	for _, path := range []string{"/api/stt/chrome/prepare", "/api/stt/chrome/stop"} {
		if !evenHubAPIPath(path) {
			t.Fatal("missing CORS", path)
		}
	}
	req := httptest.NewRequest("POST", "/api/stt/transcribe", strings.NewReader(`{"engine":"invalid","format":"pcm","sampleRate":16000,"audio":"`+base64.StdEncoding.EncodeToString(make([]byte, 3200))+`"}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handleSTTTranscribe(&App{})(response, req)
	if response.Code != 400 || !strings.Contains(response.Body.String(), "unknown STT engine") {
		t.Fatalf("%d %s", response.Code, response.Body.String())
	}
}
