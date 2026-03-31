package core

import (
	"bytes"
	"io"
	"log"
	"strings"
	"sync"
)

type filteredLogWriter struct {
	mu     sync.Mutex
	target io.Writer
}

func (w *filteredLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if shouldWriteConsoleLog(string(p)) {
		_, err := w.target.Write(p)
		if err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func InitLoggingFilter() {
	log.SetOutput(&filteredLogWriter{target: log.Writer()})
}

func shouldWriteConsoleLog(msg string) bool {
	if isDebugTraceCollectorEnabled() {
		return true
	}

	msg = strings.TrimSpace(msg)
	if msg == "" {
		return false
	}

	lower := strings.ToLower(stripLogPrefix(msg))

	essentialPrefixes := []string{
		"[server]",
		"[https]",
		"[tts]",
		"tts ",
		"loaded and cached voice style",
		"tts initialized successfully",
		"attempting to load onnx runtime library",
		"onnx runtime initialized successfully",
		"llm request failed",
		"llm error response",
	}
	for _, prefix := range essentialPrefixes {
		if strings.HasPrefix(lower, strings.ToLower(prefix)) {
			return true
		}
	}

	essentialContains := []string{
		" error",
		" failed",
		"panic",
		"fatal",
		"warning",
		"context limit",
		"vision model",
		"authentication failed",
		"shutdown error",
		"network transfer",
	}
	for _, token := range essentialContains {
		if strings.Contains(lower, token) {
			return true
		}
	}

	noisyPrefixes := []string{
		"[mcp-debug]",
		"[mcp]",
		"[handlechat-debug]",
		"[handlechat]",
		"[http]",
		"[redirect]",
		"[hybrid]",
		"[db]",
		"[asyncmemory]",
		"[memoryworker]",
		"[memoryworker-llm]",
		"[self-evolution]",
		"[handleconfig]",
		"[fetchandcachemodels]",
	}
	for _, prefix := range noisyPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}

	return false
}

func stripLogPrefix(msg string) string {
	if idx := strings.Index(msg, ": "); idx >= 0 {
		prefix := msg[:idx]
		if bytes.Count([]byte(prefix), []byte("/")) >= 1 {
			return msg[idx+2:]
		}
	}
	return msg
}
