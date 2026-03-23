package mcp

import (
	"fmt"
	"sync"
	"time"
)

// TraceEvent is a lightweight debug event emitted by the MCP package.
type TraceEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"`
	Stage     string                 `json:"stage"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

var (
	traceHookMu sync.RWMutex
	traceHook   func(TraceEvent)
)

// SetTraceHook registers a callback for MCP debug traces.
func SetTraceHook(fn func(TraceEvent)) {
	traceHookMu.Lock()
	defer traceHookMu.Unlock()
	traceHook = fn
}

// EmitTrace emits a debug trace if a hook is registered.
func EmitTrace(source, stage, message string, details map[string]interface{}) {
	traceHookMu.RLock()
	hook := traceHook
	traceHookMu.RUnlock()

	if hook == nil {
		return
	}

	hook(TraceEvent{
		Timestamp: time.Now(),
		Source:    source,
		Stage:     stage,
		Message:   message,
		Details:   details,
	})
}

func errorDetail(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func durationMs(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func traceDetails(args ...interface{}) map[string]interface{} {
	if len(args)%2 != 0 {
		args = append(args, "(missing)")
	}

	details := make(map[string]interface{}, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		key := fmt.Sprint(args[i])
		details[key] = args[i+1]
	}
	return details
}
