package core

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Only gateway-owned runtimes participate. LLM connections are never unloaded.
const defaultModelIdleMinutes = 5

var modelIdleMinutes atomic.Int64

func init() { modelIdleMinutes.Store(defaultModelIdleMinutes) }

// Acquire before taking any runtime lock. A timer holds this lock throughout
// cleanup so a new request cannot enter a runtime halfway through unloading.
type idleModel struct {
	mu         sync.Mutex
	name       string
	unload     func()
	users      int
	stopped    bool
	idleSince  time.Time
	timer      *time.Timer
	generation uint64
	timeout    func() time.Duration
}

func newIdleModel(name string, unload func()) *idleModel {
	return &idleModel{name: name, unload: unload, timeout: func() time.Duration { return time.Duration(modelIdleMinutes.Load()) * time.Minute }}
}
func (m *idleModel) use() func() { return m.begin(true) }
func (m *idleModel) begin(touch bool) func() {
	m.mu.Lock()
	m.cancelLocked()
	m.users++
	m.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.users--
			if touch || m.idleSince.IsZero() {
				m.idleSince = time.Now()
			}
			m.scheduleLocked()
		})
	}
}
func (m *idleModel) cancelLocked() {
	m.generation++
	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}
}
func (m *idleModel) scheduleLocked() {
	if m.stopped || m.users != 0 || m.idleSince.IsZero() || m.timeout() <= 0 {
		return
	}
	delay := time.Until(m.idleSince.Add(m.timeout()))
	if delay < 0 {
		delay = 0
	}
	generation := m.generation
	m.timer = time.AfterFunc(delay, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.stopped || m.generation != generation || m.users != 0 || m.timeout() <= 0 {
			return
		}
		if time.Since(m.idleSince) < m.timeout() {
			m.scheduleLocked()
			return
		}
		m.timer = nil
		m.unload()
		m.idleSince = time.Time{}
		log.Printf("[model-memory] Released idle %s runtime", m.name)
	})
}
func (m *idleModel) reschedule() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = false
	m.cancelLocked()
	m.scheduleLocked()
}
func (m *idleModel) stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
	m.cancelLocked()
	m.idleSince = time.Time{}
}

var whisperModelMemory = newIdleModel("Whisper", closeWhisperSTTRuntime)
var qwenModelMemory = newIdleModel("Qwen3-ASR", closeQwenSTTRuntime)
var ttsModelMemory = newIdleModel("Supertonic TTS", unloadIdleTTS)
var embeddingModelMemory = newIdleModel("Embedding / MediaPipe", unloadIdleEmbedding)
var appleModelMemory = newIdleModel("Apple speech / translation", func() { appleNative.Close() })
var chromeModelMemory = newIdleModel("Chrome speech / translation", func() { chromeTranslation.Close() })
var localModelMemories = []*idleModel{whisperModelMemory, qwenModelMemory, ttsModelMemory, embeddingModelMemory, appleModelMemory, chromeModelMemory}

func setModelIdleMinutes(minutes int) error {
	if minutes < 0 || minutes > 120 {
		return fmt.Errorf("model idle timeout must be between 0 and 120 minutes")
	}
	modelIdleMinutes.Store(int64(minutes))
	for _, memory := range localModelMemories {
		memory.reschedule()
	}
	return nil
}
func (a *App) GetModelIdleMinutes() int { return int(modelIdleMinutes.Load()) }
func (a *App) SetModelIdleMinutes(minutes int) error {
	if err := setModelIdleMinutes(minutes); err != nil {
		return err
	}
	a.saveConfig()
	return nil
}
func stopModelIdleTimers() {
	for _, memory := range localModelMemories {
		memory.stop()
	}
}

func unloadIdleTTS() {
	destroyTTSRuntime()
	styleMutex.Lock()
	defer styleMutex.Unlock()
	for key, style := range styleCache {
		if style != nil {
			style.Destroy()
		}
		delete(styleCache, key)
	}
}
func unloadIdleEmbedding() {
	if rt := getEmbeddingRuntime(); rt != nil {
		rt.unloadForIdle()
	}
}
