package core

import (
	"context"
	"errors"
	"github.com/chromedp/chromedp"
	"net/http/httptest"
	"os"
	"os/exec"
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
	// Inject the reported browser exception once, then exercise the real model.
	// Recovery must keep the same browser so active Chrome STT is not aborted.
	browser := chromedp.FromContext(s.tab).Browser
	if err = chromedp.Run(s.tab, chromedp.Evaluate(`(() => { const original=Translator.prototype.translate; Translator.prototype.translate=function(...args){Translator.prototype.translate=original;return Promise.reject(new DOMException('Other generic failures occurred.','UnknownError'));}; })()`, nil)); err != nil {
		t.Fatal(err)
	}
	recovered, recoverErr := s.run(ctx, req, false)
	if recoverErr != nil || !strings.Contains(strings.ToLower(recovered.Text), "coffee") || chromedp.FromContext(s.tab).Browser != browser {
		t.Fatalf("translator-only recovery: %+v %v", recovered, recoverErr)
	}
	t.Logf("recovered UnknownError without restarting Chrome: %s", recovered.Text)

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

func TestChromeTranslationRecoveryBounds(t *testing.T) {
	for _, tc := range []struct {
		message        string
		attempt        int
		prepared, want bool
	}{
		{"UnknownError: Other generic failures occurred.", 0, true, true},
		{"UnknownError: Other generic failures occurred.", 1, true, false},
		{"UnknownError: failed", 0, false, true},
		{"InvalidStateError: destroyed", 0, true, true},
		{"InvalidStateError: destroyed", 1, true, false},
		{"NotSupportedError: starting", 2, true, true},
		{"NotSupportedError: unsupported", 0, false, false},
		{"NotSupportedError: starting", 3, true, false},
		{"NotAllowedError: activation", 0, true, false},
		{"", 0, true, false},
	} {
		if got := retryChromeTranslation(tc.message, tc.attempt, tc.prepared); got != tc.want {
			t.Errorf("%+v got %v", tc, got)
		}
	}
}

func TestChromeTranslationFailedObjectEvicted(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable")
	}
	script := strings.TrimSuffix(strings.SplitN(chromeTranslationPage, "<script>", 2)[1], "</script>")
	fixture := `const assert=require('node:assert/strict');global.self=global;global.window=global;
 let creates=0,destroys=0;
 global.Translator={availability:async()=> 'available',create:async()=>{const id=++creates;return{destroy(){destroys++;if(id===1)throw Error('already closed')},async translate(text){if(id===1)throw Object.assign(Error('Other generic failures occurred.'),{name:'UnknownError'});return 'translated '+text;}}}};
 ` + script + `
 (async()=>{
 const p={source:'ko',target:'en',text:'test'};
 await chromeTranslation.prepare(p);
 const other={source:'ja',target:'en',text:'other'};await chromeTranslation.prepare(other);
 const failed=await chromeTranslation.translate(p,true);assert.match(failed.error,/^UnknownError:/);
 assert.equal(destroys,1);assert.equal(creates,2);
 const recovered=await chromeTranslation.translate(p,true);assert.equal(recovered.text,'translated test');assert.equal(creates,3);
 const kept=await chromeTranslation.translate(other,true);assert.equal(kept.text,'translated other');assert.equal(creates,3,'healthy language pairs must stay cached');
 })().catch(error=>{console.error(error);process.exitCode=1;});`
	cmd := exec.Command(node)
	cmd.Stdin = strings.NewReader(fixture)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cache recovery: %v %s", err, out)
	}
}

// Opt-in browser test: synthetic JS only, no microphone or model download.
func TestChromeTranslationSpeechConcurrency(t *testing.T) {
	if os.Getenv("DKST_CHROME_CONCURRENCY") != "1" {
		t.Skip("explicit isolated browser test")
	}
	s := newChromeTranslator(t.TempDir())
	defer s.Close()
	s.mu.Lock()
	err := s.startLocked()
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err = chromedp.Run(s.tab, chromedp.Evaluate(`window.concurrentStarted=false;window.cancelObserved=false;window.chromeTranslation.translate=async(p)=>{window.concurrentStarted=true;return await new Promise(resolve=>{window.completeTranslation=()=>resolve({status:'ready',text:'done'});});};window.chromeSpeech.read=()=>({status:'ready',text:'speech',finished:true});window.chromeTranslation.cancel=()=>{window.cancelObserved=true;window.completeTranslation();};`, nil))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	translationCtx, stop := context.WithCancel(ctx)
	go func() {
		_, err := s.run(translationCtx, chromeTranslationRequest{Source: "en", Target: "ko", Text: "hello"}, false)
		done <- err
	}()
	started := false
	for i := 0; i < 100; i++ {
		if err = chromedp.Run(s.tab, chromedp.Evaluate("window.concurrentStarted", &started)); err != nil {
			t.Fatal(err)
		}
		if started {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !started {
		t.Fatal("translation did not start")
	}
	speechStarted := time.Now()
	r, err := s.speech(ctx, chromeSpeechRequest{Language: "en", Engine: "chrome-online"}, "read")
	if err != nil || r.Text != "speech" || time.Since(speechStarted) > 500*time.Millisecond {
		t.Fatalf("speech blocked by translator %+v %v", r, err)
	}
	stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancel blocked")
	}
	observed := false
	for i := 0; i < 100; i++ {
		_ = chromedp.Run(s.tab, chromedp.Evaluate("window.cancelObserved", &observed))
		if observed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !observed {
		t.Fatal("native translation cancellation not propagated")
	}
}
