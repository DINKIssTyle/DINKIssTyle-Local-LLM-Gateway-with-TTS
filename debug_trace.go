package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxDebugTraceEntries = 200

// DebugTraceEntry is the structured payload shown in the desktop debug panel.
type DebugTraceEntry struct {
	ID        int64             `json:"id"`
	Timestamp string            `json:"timestamp"`
	Source    string            `json:"source"`
	Stage     string            `json:"stage"`
	Message   string            `json:"message"`
	Details   map[string]string `json:"details,omitempty"`
	Payload   string            `json:"payload,omitempty"`
}

type debugTraceStore struct {
	mu      sync.RWMutex
	enabled bool
	entries []DebugTraceEntry
}

var (
	debugTraceSeq   int64
	debugTraceState = &debugTraceStore{
		entries: make([]DebugTraceEntry, 0, maxDebugTraceEntries),
	}
)

func setDebugTraceCollectorEnabled(enabled bool) {
	debugTraceState.mu.Lock()
	debugTraceState.enabled = enabled
	debugTraceState.mu.Unlock()
}

func isDebugTraceCollectorEnabled() bool {
	debugTraceState.mu.RLock()
	defer debugTraceState.mu.RUnlock()
	return debugTraceState.enabled
}

func getDebugTraceEntriesSnapshot() []DebugTraceEntry {
	debugTraceState.mu.RLock()
	defer debugTraceState.mu.RUnlock()

	out := make([]DebugTraceEntry, len(debugTraceState.entries))
	copy(out, debugTraceState.entries)
	return out
}

func clearDebugTraceEntries() {
	debugTraceState.mu.Lock()
	debugTraceState.entries = debugTraceState.entries[:0]
	debugTraceState.mu.Unlock()

	if globalApp != nil && globalApp.ctx != nil {
		wruntime.EventsEmit(globalApp.ctx, "debug-trace-cleared")
	}
}

func AddDebugTrace(source, stage, message string, details map[string]interface{}) {
	if !isDebugTraceCollectorEnabled() {
		return
	}

	payload := extractDebugPayload(details)
	entry := DebugTraceEntry{
		ID:        atomic.AddInt64(&debugTraceSeq, 1),
		Timestamp: time.Now().Format("15:04:05.000"),
		Source:    source,
		Stage:     stage,
		Message:   message,
		Details:   stringifyDebugTraceDetails(details),
		Payload:   payload,
	}

	debugTraceState.mu.Lock()
	debugTraceState.entries = append(debugTraceState.entries, entry)
	if len(debugTraceState.entries) > maxDebugTraceEntries {
		debugTraceState.entries = debugTraceState.entries[len(debugTraceState.entries)-maxDebugTraceEntries:]
	}
	debugTraceState.mu.Unlock()

	if globalApp != nil && globalApp.ctx != nil {
		wruntime.EventsEmit(globalApp.ctx, "debug-trace", entry)
	}
}

func extractDebugPayload(details map[string]interface{}) string {
	if len(details) == 0 {
		return ""
	}

	raw, ok := details["__payload"]
	if !ok {
		return ""
	}
	delete(details, "__payload")

	switch val := raw.(type) {
	case string:
		return val
	default:
		b, err := json.MarshalIndent(val, "", "  ")
		if err != nil {
			return fmt.Sprint(val)
		}
		return string(b)
	}
}

func stringifyDebugTraceDetails(details map[string]interface{}) map[string]string {
	if len(details) == 0 {
		return nil
	}

	keys := make([]string, 0, len(details))
	for k := range details {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make(map[string]string, len(details))
	for _, k := range keys {
		out[k] = compactDebugTraceValue(details[k])
	}
	return out
}

func compactDebugTraceValue(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return compactDebugTraceString(val, 220)
	case fmt.Stringer:
		return compactDebugTraceString(val.String(), 220)
	case error:
		return compactDebugTraceString(val.Error(), 220)
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return compactDebugTraceString(fmt.Sprint(val), 220)
		}
		return compactDebugTraceString(string(b), 220)
	}
}

func compactDebugTraceString(s string, limit int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len([]rune(s)) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit]) + "..."
}
