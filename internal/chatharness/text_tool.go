package chatharness

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	functionParameterWrapperPattern = regexp.MustCompile(`(?is)^\s*<tool_call>\s*<function\s*=\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*>\s*([\s\S]*?)\s*</function>\s*</tool_call>\s*$`)
	functionParameterArgPattern     = regexp.MustCompile(`(?is)<parameter\s*=\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*>\s*([\s\S]*?)\s*</parameter>`)
)

// ParseFunctionParameterToolCall recognizes the Qwen-style textual fallback:
// <tool_call><function=name><parameter=key>value</parameter></function></tool_call>.
func ParseFunctionParameterToolCall(raw string) (ProviderToolCall, bool) {
	match := functionParameterWrapperPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if len(match) < 3 {
		return ProviderToolCall{}, false
	}
	name := strings.TrimSpace(match[1])
	args := make(map[string]interface{})
	for _, parameter := range functionParameterArgPattern.FindAllStringSubmatch(match[2], -1) {
		if len(parameter) < 3 {
			continue
		}
		key := strings.TrimSpace(parameter[1])
		value := strings.TrimSpace(parameter[2])
		if key != "" {
			args[key] = parseFunctionParameterValue(value)
		}
	}
	if name == "" || len(args) == 0 {
		return ProviderToolCall{}, false
	}
	arguments, err := json.Marshal(args)
	if err != nil {
		return ProviderToolCall{}, false
	}
	return ProviderToolCall{Name: name, Arguments: string(arguments)}, true
}

func parseFunctionParameterValue(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var decoded interface{}
	if json.Unmarshal([]byte(value), &decoded) == nil {
		switch decoded.(type) {
		case []interface{}, map[string]interface{}, bool, float64, nil:
			return decoded
		}
	}
	return value
}
