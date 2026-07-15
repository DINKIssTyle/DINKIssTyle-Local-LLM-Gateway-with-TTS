package mcp

import (
	"strings"
	"sync"
)

var browserRuntimeConfig struct {
	sync.RWMutex
	executablePath string
}

// SetBrowserExecutablePath configures the dedicated browser used by MCP page
// reads. An empty path leaves chromedp's normal browser discovery in place.
func SetBrowserExecutablePath(path string) {
	browserRuntimeConfig.Lock()
	browserRuntimeConfig.executablePath = strings.TrimSpace(path)
	browserRuntimeConfig.Unlock()
}

func getBrowserExecutablePath() string {
	browserRuntimeConfig.RLock()
	defer browserRuntimeConfig.RUnlock()
	return browserRuntimeConfig.executablePath
}
