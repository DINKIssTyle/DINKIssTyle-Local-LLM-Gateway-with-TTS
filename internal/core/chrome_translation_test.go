package core

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestChromeTranslationValidationAndIdentity(t *testing.T) {
	s := newChromeTranslator(t.TempDir())
	defer s.Close()
	for _, req := range []chromeTranslationRequest{{Text: "hello", Source: "auto", Target: "ko"}, {Text: "", Source: "en", Target: "ko"}, {Text: strings.Repeat("가", 6001), Source: "ko", Target: "en"}} {
		if _, err := s.run(context.Background(), req, false); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	result, err := s.run(context.Background(), chromeTranslationRequest{Text: "안녕하세요", Source: "ko", Target: "ko"}, false)
	if err != nil || result.Text != "안녕하세요" || s.tab != nil {
		t.Fatalf("same-language request should not start Chrome: %+v %v", result, err)
	}
	s.mu.Lock()
	_, err = s.run(context.Background(), chromeTranslationRequest{Text: "hello", Source: "en", Target: "ko"}, false)
	s.mu.Unlock()
	if !errors.Is(err, errChromeTranslationBusy) {
		t.Fatalf("no unbounded translation queue: %v", err)
	}
}
func TestChromeTranslationHTTPValidation(t *testing.T) {
	s := newChromeTranslator(t.TempDir())
	defer s.Close()
	for _, tc := range []struct {
		method, body string
		status       int
	}{{"GET", "", 405}, {"POST", "{", 400}, {"POST", `{"source":"auto","target":"en","text":"test"}`, 400}, {"POST", `{"source":"ko","target":"ko","text":"테스트"}`, 200}} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, "/api/translate/chrome", strings.NewReader(tc.body))
		handleChromeTranslation(s, false)(w, r)
		if w.Code != tc.status {
			t.Fatalf("%+v got %d %s", tc, w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"/api/translate/chrome", "/api/translate/chrome/prepare"} {
		if !evenHubAPIPath(path) {
			t.Fatal("missing Even Hub CORS", path)
		}
	}
}
func TestChromeTranslationLocal(t *testing.T) {
	if os.Getenv("DKST_CHROME_TRANSLATION_INTEGRATION") != "1" {
		t.Skip("explicit local model download test not requested")
	}
	profile := os.Getenv("DKST_CHROME_TRANSLATION_PROFILE")
	if profile == "" {
		profile = t.TempDir()
	}
	s := newChromeTranslator(profile)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	req := chromeTranslationRequest{Text: "커피 드시겠어요?", Source: "ko", Target: "en"}
	started := time.Now()
	prepared, err := s.run(ctx, req, true)
	if err != nil {
		t.Fatalf("prepare: %+v %v", prepared, err)
	}
	t.Logf("prepared %s after %v", prepared.Status, time.Since(started))
	result, err := s.run(ctx, req, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(result.Text), "coffee") {
		t.Fatalf("unexpected translation: %+v", result)
	}
	t.Logf("translated: %s", result.Text)
	// Verify cache/model usability after a worker restart with a fresh localhost
	// origin; this catches availability masking and user-activation issues.
	s.Close()
	result, err = s.run(ctx, req, false)
	if err != nil {
		t.Fatalf("restart must retain readiness: %+v %v", result, err)
	}
	t.Logf("after restart: %s", result.Text)
	reverse := chromeTranslationRequest{Text: "Would you like some coffee?", Source: "en", Target: "ko"}
	if _, err = s.run(ctx, reverse, true); err != nil {
		t.Fatal(err)
	}
	result, err = s.run(ctx, reverse, false)
	if err != nil || !strings.Contains(result.Text, "커피") {
		t.Fatalf("reverse translation: %+v %v", result, err)
	}
	t.Logf("English to Korean: %s", result.Text)

}
