package core

import (
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func awaitIdleSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("idle cleanup did not run")
	}
}
func TestIdleModelWaitsForAllRequests(t *testing.T) {
	freed := make(chan struct{}, 1)
	m := newIdleModel("test", func() { freed <- struct{}{} })
	m.timeout = func() time.Duration { return 15 * time.Millisecond }
	defer m.stop()
	first, second := m.use(), m.use()
	first()
	first() // Releasing twice must not decrement another caller's lease.
	select {
	case <-freed:
		t.Fatal("unloaded while a request was active")
	case <-time.After(35 * time.Millisecond):
	}
	second()
	awaitIdleSignal(t, freed)
}
func TestIdleModelRequestRearmsTimer(t *testing.T) {
	freed := make(chan struct{}, 1)
	m := newIdleModel("test", func() { freed <- struct{}{} })
	m.timeout = func() time.Duration { return 15 * time.Millisecond }
	defer m.stop()
	m.use()()
	release := m.use()
	select {
	case <-freed:
		t.Fatal("stale timer unloaded active model")
	case <-time.After(35 * time.Millisecond):
	}
	release()
	awaitIdleSignal(t, freed)
}
func TestIdleModelDisabledAndPolicyChange(t *testing.T) {
	var duration atomic.Int64
	freed := make(chan struct{}, 1)
	m := newIdleModel("test", func() { freed <- struct{}{} })
	m.timeout = func() time.Duration { return time.Duration(duration.Load()) }
	defer m.stop()
	m.use()()
	m.mu.Lock()
	hasTimer := m.timer != nil
	m.mu.Unlock()
	if hasTimer {
		t.Fatal("zero must keep model loaded")
	}
	duration.Store(int64(time.Millisecond))
	m.reschedule()
	awaitIdleSignal(t, freed)
}
func TestIdleModelAcquisitionWaitsForCleanup(t *testing.T) {
	entered, finish, acquired := make(chan struct{}), make(chan struct{}), make(chan struct{})
	m := newIdleModel("test", func() { close(entered); <-finish })
	m.timeout = func() time.Duration { return time.Millisecond }
	m.use()()
	awaitIdleSignal(t, entered)
	go func() { release := m.use(); close(acquired); release() }()
	select {
	case <-acquired:
		t.Fatal("new inference entered during cleanup")
	case <-time.After(15 * time.Millisecond):
	}
	// Disable before allowing another timer to run the one-shot test callback.
	m.timeout = func() time.Duration { return 0 }
	close(finish)
	awaitIdleSignal(t, acquired)
	m.stop()
}
func TestIdleModelStatusDoesNotExtendIdleTime(t *testing.T) {
	m := newIdleModel("test", func() {})
	m.timeout = func() time.Duration { return time.Hour }
	defer m.stop()
	m.use()()
	m.mu.Lock()
	before := m.idleSince
	m.mu.Unlock()
	m.begin(false)()
	m.mu.Lock()
	after := m.idleSince
	m.mu.Unlock()
	if !before.Equal(after) {
		t.Fatal("status polling should not keep model resident")
	}
}

type idleTestEncoder struct {
	closes  int
	encodes int
}

func (e *idleTestEncoder) Encode(string, embeddingUsage) ([]float64, error) {
	e.encodes++
	return []float64{1}, nil
}
func (e *idleTestEncoder) Close() error { e.closes++; return nil }
func TestEmbeddingIdleRetainsProviderAndReloads(t *testing.T) {
	first, second := &idleTestEncoder{}, &idleTestEncoder{}
	reloads := 0
	rt := &embeddingRuntime{modelID: "test", backend: "test", encoder: first, reloadEncoder: func() (embeddingEncoder, error) { reloads++; return second, nil }}
	provider := embeddingProviderForRuntime(rt)
	rt.unloadForIdle()
	if first.closes != 1 || !rt.IsUsable() || rt.encoder != nil {
		t.Fatal("idle wrapper lost its provider capability")
	}
	vector, model, err := provider.BuildWithUsage("question", "query")
	if err != nil || len(vector) != 1 || model != "test:test" || reloads != 1 || second.encodes != 1 {
		t.Fatalf("lazy reload failed: %v %s %v", vector, model, err)
	}
	_, _, err = rt.Build("next", embeddingUsageQuery)
	if err != nil || reloads != 1 {
		t.Fatal("reloaded for every request")
	}
	rt.Close()
	if rt.IsUsable() {
		t.Fatal("explicitly disabled runtime reopened")
	}
	if _, _, err = rt.Build("disabled", embeddingUsageQuery); err == nil {
		t.Fatal("explicit close must stay closed")
	}
}
func TestIdleModelPolicyBounds(t *testing.T) {
	previous := int(modelIdleMinutes.Load())
	defer setModelIdleMinutes(previous)
	for _, value := range []int{-1, 121} {
		if err := setModelIdleMinutes(value); err == nil {
			t.Fatal("invalid timeout accepted")
		}
	}
	for _, value := range []int{0, 1, 5, 120} {
		if err := setModelIdleMinutes(value); err != nil {
			t.Fatal(err)
		}
	}
}

// Opt-in GPU/ONNX tests exercise real installed assets without changing the
// user's configuration, inference sessions, or transcript database.
func TestIdleInstalledSTTRuntimes(t *testing.T) {
	if os.Getenv("DKST_IDLE_TEST_INSTALLED") != "1" {
		t.Skip("installed model memory test not requested")
	}
	for _, kind := range []string{"qwen", "whisper"} {
		t.Run(kind, func(t *testing.T) {
			memory := qwenModelMemory
			run := func() error {
				_, err := runQwenSTTInference(make([]byte, 3200), "pcm", "auto", "strict", false)
				return err
			}
			closeRuntime := closeQwenSTTRuntime
			worker := func() *sttWorker { qwenSTTLock.Lock(); defer qwenSTTLock.Unlock(); return activeQwenSTTWorker }
			if kind == "whisper" {
				memory = whisperModelMemory
				closeRuntime = closeWhisperSTTRuntime
				run = func() error { _, err := runSTTInference(make([]byte, 3200), "pcm", "auto", "strict"); return err }
				worker = func() *sttWorker { sttLock.Lock(); defer sttLock.Unlock(); return activeSTTWorker }
			}
			oldTimeout := memory.timeout
			memory.timeout = func() time.Duration { return 50 * time.Millisecond }
			defer func() { memory.stop(); memory.timeout = oldTimeout; closeRuntime() }()
			if err := run(); err != nil {
				t.Fatal(err)
			}
			original := worker()
			if original == nil {
				t.Fatal("runtime did not load")
			}
			hold := memory.use()
			time.Sleep(100 * time.Millisecond)
			if worker() != original {
				t.Fatal("unloaded an active runtime")
			}
			hold()
			deadline := time.Now().Add(5 * time.Second)
			for worker() != nil && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			if worker() != nil {
				t.Fatal("idle runtime remained resident")
			}
			if original.cmd.ProcessState == nil {
				t.Fatal("old worker was not reaped")
			}
			if err := run(); err != nil {
				t.Fatal(err)
			}
			if worker() == nil || worker() == original {
				t.Fatal("runtime did not reload on next request")
			}
			t.Log("loaded, protected while active, unloaded, and reloaded")
		})
	}
}
func TestIdleInstalledTTSReloadsOnRequest(t *testing.T) {
	if os.Getenv("DKST_IDLE_TEST_INSTALLED") != "1" {
		t.Skip("installed model memory test not requested")
	}
	oldTimeout := ttsModelMemory.timeout
	ttsModelMemory.timeout = func() time.Duration { return 50 * time.Millisecond }
	defer func() { ttsModelMemory.stop(); ttsModelMemory.timeout = oldTimeout; unloadIdleTTS() }()
	if err := InitTTS(getTTSAssetsDir(), ttsConfig.Threads); err != nil {
		t.Fatal(err)
	}
	globalTTSMutex.RLock()
	original := globalTTS
	globalTTSMutex.RUnlock()
	if original == nil {
		t.Fatal("TTS did not load")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		globalTTSMutex.RLock()
		unloaded := globalTTS == nil
		globalTTSMutex.RUnlock()
		if unloaded {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	globalTTSMutex.RLock()
	unloaded := globalTTS == nil
	globalTTSMutex.RUnlock()
	if !unloaded {
		t.Fatal("TTS was not unloaded")
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/tts", strings.NewReader(`{"text":"안녕하세요.","lang":"ko","format":"wav","steps":5}`))
	handleTTS(w, req)
	if w.Code != 200 || w.Body.Len() < 44 {
		t.Fatalf("lazy TTS request failed: %d %s", w.Code, w.Body.String())
	}
	globalTTSMutex.RLock()
	reloaded := globalTTS
	globalTTSMutex.RUnlock()
	if reloaded == nil || reloaded == original {
		t.Fatal("TTS instance was not recreated")
	}
	t.Logf("TTS unloaded and next request synthesized %d bytes", w.Body.Len())
}

func TestIdleModelShutdownCannotRearmTimer(t *testing.T) {
	m := newIdleModel("test", func() { t.Error("cleanup timer ran after shutdown") })
	m.timeout = func() time.Duration { return time.Hour }
	finish := m.use()
	m.stop()
	finish()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.timer != nil {
		t.Fatal("finishing request restarted a shutdown timer")
	}
}
