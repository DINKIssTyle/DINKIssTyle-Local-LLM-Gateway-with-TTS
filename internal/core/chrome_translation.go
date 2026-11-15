package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

var errChromeTranslationBusy = errors.New("Chrome 번역 처리 중입니다")
var chromeTranslation = newChromeTranslator("")

type chromeTranslationRequest struct {
	RequestID string `json:"requestId,omitempty"`
	Text      string `json:"text"`
	Source    string `json:"source"`
	Target    string `json:"target"`
}
type chromeTranslationResult struct {
	Timings map[string]float64 `json:"timings,omitempty"`
	Text    string             `json:"text,omitempty"`
	Source  string             `json:"source"`
	Target  string             `json:"target"`
	Status  string             `json:"status"`
	Backend string             `json:"backend"`
	Error   string             `json:"error,omitempty"`
}
type chromeTranslator struct {
	mu              sync.RWMutex
	translationMu   sync.Mutex
	speechMu        sync.Mutex
	profile         string
	profileLock     *os.File
	tab             context.Context
	tabCancel       context.CancelFunc
	allocatorCancel context.CancelFunc
	server          *http.Server
}

func newChromeTranslator(profile string) *chromeTranslator {
	return &chromeTranslator{profile: profile}
}
func regularChromeExecutable() (string, error) {
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		candidates = []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", filepath.Join(home, "Applications/Google Chrome.app/Contents/MacOS/Google Chrome")}
	case "windows":
		for _, root := range []string{os.Getenv("PROGRAMFILES"), os.Getenv("PROGRAMFILES(X86)"), os.Getenv("LOCALAPPDATA")} {
			if root != "" {
				candidates = append(candidates, filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"))
			}
		}
	default:
		for _, name := range []string{"google-chrome", "google-chrome-stable"} {
			if path, err := exec.LookPath(name); err == nil {
				candidates = append(candidates, path)
			}
		}
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", errors.New("정식 Google Chrome이 필요합니다. Headless Shell은 사용하지 않습니다")
}
func (s *chromeTranslator) closeLocked() {
	if s.tab != nil && chromedp.FromContext(s.tab).Browser != nil && s.tab.Err() == nil {
		closeCtx, stop := context.WithTimeout(s.tab, 3*time.Second)
		_ = chromedp.Cancel(closeCtx)
		stop()
	}
	if s.tabCancel != nil {
		s.tabCancel()
	}
	if s.allocatorCancel != nil {
		s.allocatorCancel()
	}
	if s.server != nil {
		_ = s.server.Close()
	}
	s.tab = nil
	s.tabCancel = nil
	s.allocatorCancel = nil
	s.server = nil
	if s.profileLock != nil {
		_ = s.profileLock.Close()
		s.profileLock = nil
	}
}
func (s *chromeTranslator) Close() { s.mu.Lock(); defer s.mu.Unlock(); s.closeLocked() }
func (s *chromeTranslator) startLocked() error {
	if s.tab != nil && s.tab.Err() != nil {
		s.closeLocked()
	}
	if s.tab != nil && s.tab.Err() == nil {
		browser := chromedp.FromContext(s.tab).Browser
		if browser != nil {
			select {
			case <-browser.LostConnection:
				s.closeLocked()
			default:
				return nil
			}
		}
	}
	executable, err := regularChromeExecutable()
	if err != nil {
		return err
	}
	profile := s.profile
	if profile == "" {
		profile = joinAppDataPath("assets", "runtime", "chrome-translator-profile")
	}
	if err = os.MkdirAll(profile, 0700); err != nil {
		return err
	}
	profileLock, lockErr := acquireChromeProfileLock(profile)
	if lockErr != nil {
		return lockErr
	}
	s.profileLock = profileLock
	if err = recoverOrphanChrome(profile, executable); err != nil {
		s.closeLocked()
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		s.closeLocked()
		return err
	}
	s.server = &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/translator" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; connect-src 'none'")
		_, _ = w.Write([]byte(strings.Replace(chromeTranslationPage, "</script>", chromeSpeechScript+"</script>", 1)))
	})}
	server := s.server
	go func() { _ = server.Serve(listener) }()
	// Deliberately do not use chromedp's default automation flags: they disable
	// component updates/background networking and Translate itself.
	allocator, cancel := chromedp.NewExecAllocator(context.Background(), chromedp.ExecPath(executable), chromedp.UserDataDir(profile), chromedp.Headless, chromedp.NoFirstRun, chromedp.NoDefaultBrowserCheck, chromedp.Flag("disable-sync", true), chromedp.Flag("password-store", "basic"), chromedp.Flag("use-mock-keychain", true), chromedp.WSURLReadTimeout(12*time.Second))
	s.allocatorCancel = cancel
	s.tab, s.tabCancel = chromedp.NewContext(allocator)
	// Allocate with the persistent context so per-request deadlines never own
	// the browser process lifetime.
	if err = chromedp.Run(s.tab); err != nil {
		s.closeLocked()
		if strings.Contains(err.Error(), "SingletonLock") || strings.Contains(err.Error(), "ProcessSingleton") {
			return errChromeProfileInUse
		}
		return errors.New("Chrome 번역 엔진을 시작하지 못했습니다. 정식 Chrome 설치와 게이트웨이 중복 실행을 확인해 주세요")
	}
	setup, stop := context.WithTimeout(s.tab, 10*time.Second)
	defer stop()
	if err = chromedp.Run(setup, chromedp.Navigate("http://"+listener.Addr().String()+"/translator"), chromedp.WaitReady("body")); err != nil {
		s.closeLocked()
		return err
	}
	return nil
}
func validateChromeTranslation(req *chromeTranslationRequest, prepare bool) error {
	req.Source = strings.ToLower(strings.TrimSpace(req.Source))
	req.Target = strings.ToLower(strings.TrimSpace(req.Target))
	req.Text = strings.TrimSpace(req.Text)
	if req.Source == "zh-hant" {
		req.Source = "zh-Hant"
	}
	if req.Target == "zh-hant" {
		req.Target = "zh-Hant"
	}
	valid := func(lang string) bool {
		if lang == "zh-Hant" {
			return true
		}
		if len(lang) != 2 {
			return false
		}
		for _, c := range lang {
			if c < 'a' || c > 'z' {
				return false
			}
		}
		return true
	}
	if !valid(req.Source) || !valid(req.Target) {
		return errors.New("인식·번역 언어는 두 글자 언어 코드로 지정하세요")
	}
	if !prepare && (req.Text == "" || len([]rune(req.Text)) > 6000) {
		return errors.New("번역할 문장은 1~6000자여야 합니다")
	}
	return nil
}
func (s *chromeTranslator) preparedPath() string {
	profile := s.profile
	if profile == "" {
		profile = joinAppDataPath("assets", "runtime", "chrome-translator-profile")
	}
	return filepath.Join(profile, "dkst-translation-pairs.json")
}
func (s *chromeTranslator) preparedPairs() map[string]bool {
	pairs := map[string]bool{}
	data, err := os.ReadFile(s.preparedPath())
	if err == nil {
		_ = json.Unmarshal(data, &pairs)
	}
	if pairs == nil {
		pairs = map[string]bool{}
	}
	return pairs
}
func (s *chromeTranslator) rememberPair(key string) error {
	pairs := s.preparedPairs()
	if len(pairs) >= 16 {
		pairs = map[string]bool{}
	}
	pairs[key] = true
	data, _ := json.Marshal(pairs)
	return os.WriteFile(s.preparedPath(), data, 0600)
}

// Recreate failed translator objects once; retain the shared Chrome/STT process.
// NotSupportedError during model restoration keeps its existing startup grace.
func retryChromeTranslation(message string, attempt int, prepared bool) bool {
	if strings.HasPrefix(message, "UnknownError:") || strings.HasPrefix(message, "InvalidStateError:") {
		return attempt == 0
	}
	return prepared && attempt < 3 && strings.HasPrefix(message, "NotSupportedError:")
}

func (s *chromeTranslator) run(ctx context.Context, req chromeTranslationRequest, prepare bool) (chromeTranslationResult, error) {
	result := chromeTranslationResult{Source: req.Source, Target: req.Target, Backend: "chrome-translator"}
	if err := validateChromeTranslation(&req, prepare); err != nil {
		return result, err
	}
	result.Source = req.Source
	result.Target = req.Target
	if req.Source == req.Target {
		result.Text = req.Text
		result.Status = "ready"
		return result, nil
	}
	if !s.translationMu.TryLock() {
		return result, errChromeTranslationBusy
	}
	defer s.translationMu.Unlock()
	tab, release, err := s.leaseBrowser(ctx)
	if err != nil {
		return result, err
	}
	defer release()

	task, cancel := context.WithCancel(tab)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	req.RequestID = fmt.Sprintf("translate-%d", time.Now().UnixNano())
	payload, _ := json.Marshal(req)
	idJSON, _ := json.Marshal(req.RequestID)
	stopOperation := context.AfterFunc(ctx, func() {
		cancelCtx, stop := context.WithTimeout(tab, time.Second)
		defer stop()
		_ = chromedp.Run(cancelCtx, chromedp.Evaluate("window.chromeTranslation.cancel("+string(idJSON)+")", nil))
	})
	defer stopOperation()
	method := "translate"
	if prepare {
		method = "prepare"
	}
	restore := s.preparedPairs()[req.Source+">"+req.Target]
	expression := "window.chromeTranslation." + method + "(" + string(payload) + "," + fmt.Sprint(restore) + ")"
	for attempt := 0; attempt < 4; attempt++ {
		result = chromeTranslationResult{Source: req.Source, Target: req.Target, Backend: "chrome-translator"}
		err = chromedp.Run(task, chromedp.ActionFunc(func(ctx context.Context) error {
			object, exception, err := cdpruntime.Evaluate(expression).WithAwaitPromise(true).WithReturnByValue(true).WithUserGesture(prepare || restore).Do(ctx)
			if err != nil {
				return err
			}
			if exception != nil {
				return fmt.Errorf("Chrome 번역 실행 실패: %s", exception.Text)
			}
			if object == nil {
				return errors.New("Chrome 번역 응답 없음")
			}
			return json.Unmarshal(object.Value, &result)
		}))
		if err != nil || !retryChromeTranslation(result.Error, attempt, prepare || restore) {
			break
		}
		select {
		case <-task.Done():
			err = task.Err()
		case <-time.After(time.Duration(attempt+1) * 350 * time.Millisecond):
		}
		if err != nil {
			break
		}
	}

	if err != nil {
		// A failed translation must not tear down a live speech stream.
		// A lost browser is detected and restarted by the next lifecycle lease.
		return result, err
	}
	if result.Error != "" {
		return result, errors.New(result.Error)
	}
	if result.Status != "ready" {
		return result, errors.New("Chrome 번역 준비가 필요합니다")
	}
	if !prepare && strings.TrimSpace(result.Text) == "" {
		return result, errors.New("Chrome 번역문이 비어 있습니다")
	}
	if prepare {
		if err := s.rememberPair(req.Source + ">" + req.Target); err != nil {
			return result, err
		}
	}
	return result, nil
}
func handleChromeTranslation(service *chromeTranslator, prepare bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req chromeTranslationRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32000)).Decode(&req); err != nil {
			http.Error(w, "invalid translation request", 400)
			return
		}
		if err := validateChromeTranslation(&req, prepare); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		limit := 12 * time.Second
		if prepare {
			limit = 180 * time.Second
		}
		ctx, cancel := context.WithTimeout(r.Context(), limit)
		defer cancel()
		started := time.Now()
		result, err := service.run(ctx, req, prepare)
		result.Timings = map[string]float64{"server_ms": float64(time.Since(started).Microseconds()) / 1000}
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			result.Error = err.Error()
			if result.Status == "" {
				result.Status = "unavailable"
			}
			code := http.StatusServiceUnavailable
			if errors.Is(err, errChromeTranslationBusy) {
				code = http.StatusTooManyRequests
			}
			w.WriteHeader(code)
		}
		_ = json.NewEncoder(w).Encode(result)
	}
}

const chromeTranslationPage = `<!doctype html><meta charset="utf-8"><title>DKST Local Translator</title><body><script>
(() => {
const controllers=new Map();const cache=new Map();
const result=(p,extra)=>({source:p.source,target:p.target,backend:'chrome-translator',...extra});
function discard(p){const key=p.source+'>'+p.target;const translator=cache.get(key);cache.delete(key);try{translator?.destroy();}catch{}}
async function get(p,prepare){
 if(!self.Translator)return result(p,{status:'unsupported',error:'Chrome Translator API를 지원하지 않습니다'});
 const key=p.source+'>'+p.target;
 if(cache.has(key))return result(p,{status:'ready'});
 if(!prepare){
  const availability=await Translator.availability({sourceLanguage:p.source,targetLanguage:p.target});
  if(availability==='unavailable')return result(p,{status:'unsupported',error:'Chrome이 지원하지 않는 언어 쌍입니다'});
  if(availability!=='available')return result(p,{status:'needs_prepare',error:'Chrome 번역 준비 버튼으로 언어팩을 먼저 준비하세요'});
 }
 // Call create before any async availability probe consumes user activation.
 const translator=await Translator.create({sourceLanguage:p.source,targetLanguage:p.target});
 if(cache.size>=4){const oldest=cache.keys().next().value;const old=cache.get(oldest);cache.delete(oldest);try{old.destroy();}catch{}}
 cache.set(key,translator);return result(p,{status:'ready'});
}
window.chromeTranslation={
 async prepare(p){try{return await get(p,true);}catch(e){discard(p);return result(p,{status:'unavailable',error:e.name+': '+e.message});}},
 cancel(id){controllers.get(id)?.abort();},
 async translate(p,restore){const controller=new AbortController();controllers.set(p.requestId,controller);try{const state=await get(p,restore===true);if(state.status!=='ready')return state;return result(p,{status:'ready',text:await cache.get(p.source+'>'+p.target).translate(p.text,{signal:controller.signal})});}catch(e){if(e.name!=='AbortError')discard(p);return result(p,{status:e.name==='AbortError'?'cancelled':'unavailable',error:e.name+': '+e.message});}finally{controllers.delete(p.requestId);}}
};
})();
</script>`

func (s *chromeTranslator) leaseBrowser(ctx context.Context) (context.Context, func(), error) {
	deadline := time.Now().Add(250 * time.Millisecond)
	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if s.mu.TryRLock() {
			healthy := s.tab != nil && s.tab.Err() == nil
			if healthy {
				browser := chromedp.FromContext(s.tab).Browser
				healthy = browser != nil
				if healthy {
					select {
					case <-browser.LostConnection:
						healthy = false
					default:
					}
				}
			}
			if healthy {
				return s.tab, s.mu.RUnlock, nil
			}
			s.mu.RUnlock()
			if s.mu.TryLock() {
				err := s.startLocked()
				s.mu.Unlock()
				if err != nil {
					return nil, nil, err
				}
				continue
			}
		}
		if time.Now().After(deadline) {
			return nil, nil, errChromeTranslationBusy
		}
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
