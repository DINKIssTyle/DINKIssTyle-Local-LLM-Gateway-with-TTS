package mcp

import (
	"strings"
	"sync"
)

const toolConfigText = `
search_web = enable
read_web_page = enable
read_buffered_source = enable
get_current_time = enable
search_memory = enable
read_memory = enable
delete_memory = enable
naver_search = enable
namu_wiki = enable
get_current_location = enable
send_keys = disable
read_terminal_tail = disable
execute_command = enable
`

var (
	toolConfigOnce sync.Once
	toolConfigMap  map[string]string
)

func isToolConfiguredEnabled(name string) bool {
	toolConfigOnce.Do(loadToolConfig)
	return toolConfigMap[strings.TrimSpace(name)] == "enable"
}

func loadToolConfig() {
	toolConfigMap = make(map[string]string)
	for _, line := range strings.Split(toolConfigText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.ToLower(strings.TrimSpace(value))
		switch value {
		case "enable", "disable":
			toolConfigMap[key] = value
		}
	}
}
