package chatharness

import (
	"encoding/json"
	"fmt"
	"strings"
)

func CompactToolResult(toolName, result string) string {
	result = strings.TrimSpace(result)
	if result == "" {
		return fmt.Sprintf("Tool Result (%s): [empty]\nUse this result to answer the user directly. Do not repeat the same tool call unless the user explicitly asked for a refresh.", toolName)
	}

	return fmt.Sprintf("Tool Result (%s):\n%s\n\nUse this result to answer the user directly. Do not repeat the same or near-identical tool call unless the user explicitly asked for a refresh.", toolName, compactText(result, 1200))
}

func ExtractExecuteCommandFromArgsJSON(argsJSON string) string {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &payload); err != nil {
		return ""
	}
	command, _ := payload["command"].(string)
	return strings.TrimSpace(command)
}

func ExecuteCommandBudgetFamily(command string) string {
	normalized := strings.ToLower(strings.TrimSpace(command))
	if normalized == "" {
		return ""
	}

	switch {
	case strings.Contains(normalized, "physmem"), strings.Contains(normalized, "vm_stat"), strings.Contains(normalized, "pages free"), strings.Contains(normalized, "pages active"), strings.Contains(normalized, "pages inactive"), strings.Contains(normalized, "rss"), strings.Contains(normalized, "memory_usage"):
		return "memory"
	case strings.Contains(normalized, "pwd"), strings.Contains(normalized, "cwd"), strings.Contains(normalized, "current directory"), strings.Contains(normalized, "current working directory"):
		return "path"
	case strings.Contains(normalized, "whoami"), strings.Contains(normalized, "id"):
		return "identity"
	case strings.Contains(normalized, "date"), strings.Contains(normalized, "time"):
		return "time"
	}

	fields := strings.Fields(normalized)
	if len(fields) == 0 {
		return normalized
	}
	return fields[0]
}

type ToolFollowupInput struct {
	LLMMode             string
	ModelID             string
	LastResponseID      string
	ToolName            string
	ToolResult          string
	LastAssistantBuffer string
	ReqMap              map[string]interface{}
	EnableMCP           bool
}

func PrepareToolFollowupRequest(input ToolFollowupInput) (map[string]interface{}, []byte, error) {
	var reqMap map[string]interface{}
	if input.LLMMode == "stateful" {
		reqMap = map[string]interface{}{
			"model":                input.ModelID,
			"input":                CompactToolResult(input.ToolName, input.ToolResult),
			"previous_response_id": input.LastResponseID,
			"stream":               true,
		}
	} else {
		reqMap = input.ReqMap
		msgs, _ := reqMap["messages"].([]interface{})
		msgs = append(msgs, map[string]interface{}{
			"role":    "assistant",
			"content": compactText(input.LastAssistantBuffer, 400),
		})
		msgs = append(msgs, map[string]interface{}{
			"role":    "user",
			"content": CompactToolResult(input.ToolName, input.ToolResult),
		})
		reqMap["messages"] = msgs
	}

	if input.EnableMCP {
		reqMap["integrations"] = []string{"mcp/dinkisstyle-gateway"}
	}

	body, err := json.Marshal(reqMap)
	return reqMap, body, err
}

func compactText(input string, limit int) string {
	input = strings.TrimSpace(input)
	if limit <= 0 || len([]rune(input)) <= limit {
		return input
	}
	runes := []rune(input)
	return strings.TrimSpace(string(runes[:limit])) + "... (truncated)"
}
