package chatharness

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	"dinkisstyle-chat/internal/promptkit"
)

const defaultToolTurnMaxTokens = 4096

type RequestInput struct {
	Body              []byte
	EndpointRaw       string
	TokenRaw          string
	LLMMode           string
	ContextStrategy   string
	EnableTools       bool
	EnableMemory      bool
	RecentContext     string
	MemorySnapshot    string
	ActiveContext     string
	RetrievalInjected bool
	UserProfileFacts  string
	SkillInstructions string
	Tools             []promptkit.ToolDefinition
}

type PreparedRequest struct {
	Body                 []byte
	ReqMap               map[string]interface{}
	Endpoint             string
	Token                string
	UpstreamURL          string
	ModelID              string
	InitialUserInputText string
	InjectedPrompt       bool
	IsStatefulFollowup   bool
}

func PrepareRequest(input RequestInput) (PreparedRequest, error) {
	prepared := PreparedRequest{
		Body:     input.Body,
		Endpoint: sanitizeEndpoint(input.EndpointRaw),
		Token:    sanitizeToken(input.TokenRaw),
	}
	contextStrategy := normalizeContextStrategy(input.LLMMode, input.ContextStrategy)

	var reqMap map[string]interface{}
	if err := json.Unmarshal(input.Body, &reqMap); err != nil {
		prepared.UpstreamURL = buildUpstreamURL(prepared.Endpoint, input.LLMMode)
		return prepared, nil
	}

	prepared.ReqMap = reqMap
	prepared.InitialUserInputText = extractChatInputText(reqMap)
	prepared.IsStatefulFollowup = isStatefulFollowup(input.LLMMode, contextStrategy, reqMap)
	removedLegacyIntegration := removeLegacyMCPIntegration(reqMap)
	removedProviderTools := removeProviderToolControls(reqMap)

	useNativeTools := input.EnableTools && strings.TrimSpace(strings.ToLower(input.LLMMode)) != "stateful"
	includeRetrievalMemory := contextStrategy == "retrieval"
	compactTaskInstructions := promptkit.BuildCompactTaskInstructions(prepared.InitialUserInputText)
	shouldInjectRuntime := input.EnableTools || includeRetrievalMemory || strings.TrimSpace(input.SkillInstructions) != "" || compactTaskInstructions != ""

	if shouldInjectRuntime {
		if useNativeTools {
			ensureChatCompletionTools(reqMap, input.Tools)
			applyToolTurnOutputBudget(reqMap)
		}
		extraInstr := compactTaskInstructions + input.SkillInstructions
		if !prepared.IsStatefulFollowup {
			extraInstr = promptkit.BuildRuntimeInstructions(promptkit.RuntimeInstructionsInput{
				EnvironmentInfo:   buildEnvironmentInfo(),
				ModelID:           extractModelID(reqMap),
				UseNativeTools:    useNativeTools,
				Tools:             input.Tools,
				RecentContext:     conditionalContextValue(includeRetrievalMemory, input.RecentContext),
				MemorySnapshot:    conditionalContextValue(includeRetrievalMemory, input.MemorySnapshot),
				ActiveContext:     conditionalContextValue(includeRetrievalMemory, input.ActiveContext),
				RetrievalInjected: includeRetrievalMemory && input.RetrievalInjected,
				UserProfileFacts:  input.UserProfileFacts,
			}) + extraInstr
		}
		prepared.InjectedPrompt = promptkit.InjectPrompt(reqMap, extraInstr)

		newBody, err := json.Marshal(reqMap)
		if err != nil {
			return prepared, fmt.Errorf("marshal prepared request: %w", err)
		}
		prepared.Body = newBody
	} else if removedLegacyIntegration || removedProviderTools {
		newBody, err := json.Marshal(reqMap)
		if err != nil {
			return prepared, fmt.Errorf("marshal request after sanitizing tool controls: %w", err)
		}
		prepared.Body = newBody
	}

	prepared.UpstreamURL = buildUpstreamURL(prepared.Endpoint, input.LLMMode)
	prepared.ModelID = extractModelID(reqMap)
	return prepared, nil
}

func applyToolTurnOutputBudget(reqMap map[string]interface{}) {
	if reqMap == nil {
		return
	}
	if _, exists := reqMap["max_tokens"]; exists {
		return
	}
	if _, exists := reqMap["max_completion_tokens"]; exists {
		return
	}
	reqMap["max_tokens"] = defaultToolTurnMaxTokens
}

func sanitizeEndpoint(endpointRaw string) string {
	endpoint := strings.TrimRight(endpointRaw, "/")
	return strings.TrimSuffix(endpoint, "/v1")
}

func sanitizeToken(tokenRaw string) string {
	token := strings.TrimSpace(tokenRaw)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return strings.TrimSpace(token[7:])
	}
	return token
}

func buildUpstreamURL(endpoint string, llmMode string) string {
	if llmMode == "stateful" {
		return endpoint + "/api/v1/chat"
	}
	return endpoint + "/v1/chat/completions"
}

func isStatefulFollowup(llmMode string, contextStrategy string, reqMap map[string]interface{}) bool {
	if llmMode != "stateful" || contextStrategy != "stateful" || reqMap == nil {
		return false
	}
	pid, _ := reqMap["previous_response_id"].(string)
	return IsValidResponseID(pid)
}

func normalizeContextStrategy(llmMode string, raw string) string {
	mode := strings.TrimSpace(strings.ToLower(llmMode))
	strategy := strings.TrimSpace(strings.ToLower(raw))
	if mode == "stateful" {
		switch strategy {
		case "retrieval", "stateful", "none":
			return strategy
		default:
			return "stateful"
		}
	}
	switch strategy {
	case "retrieval", "history", "none":
		return strategy
	default:
		return "history"
	}
}

func conditionalContextValue(enabled bool, value string) string {
	if !enabled {
		return ""
	}
	return value
}

func removeLegacyMCPIntegration(reqMap map[string]interface{}) bool {
	const targetMCP = "mcp/dinkisstyle-gateway"
	var integrations []interface{}
	removed := false
	if existing, ok := reqMap["integrations"].([]interface{}); ok {
		for _, v := range existing {
			if str, ok := v.(string); ok && str == targetMCP {
				removed = true
				continue
			}
			integrations = append(integrations, v)
		}
	}
	if len(integrations) == 0 {
		if _, exists := reqMap["integrations"]; exists {
			delete(reqMap, "integrations")
		}
		return removed
	}
	reqMap["integrations"] = integrations
	return removed
}

func removeProviderToolControls(reqMap map[string]interface{}) bool {
	removed := false
	for _, key := range []string{"tools", "tool_choice", "parallel_tool_calls", "functions", "function_call"} {
		if _, exists := reqMap[key]; exists {
			delete(reqMap, key)
			removed = true
		}
	}
	return removed
}

func ensureChatCompletionTools(reqMap map[string]interface{}, definitions []promptkit.ToolDefinition) {
	tools := make([]interface{}, 0, len(definitions))
	for _, definition := range definitions {
		if strings.TrimSpace(definition.Name) == "" {
			continue
		}
		var parameters interface{}
		if err := json.Unmarshal(definition.InputSchema, &parameters); err != nil {
			continue
		}
		tools = append(tools, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        definition.Name,
				"description": definition.Description,
				"parameters":  parameters,
			},
		})
	}
	if len(tools) == 0 {
		delete(reqMap, "tools")
		return
	}
	reqMap["tools"] = tools
	reqMap["tool_choice"] = "auto"
	reqMap["parallel_tool_calls"] = false
}

func buildEnvironmentInfo() string {
	var envLines []string
	envLines = append(envLines, fmt.Sprintf("- Operating System: %s", runtime.GOOS))
	envLines = append(envLines, fmt.Sprintf("- Architecture: %s", runtime.GOARCH))
	if runtime.GOOS == "windows" {
		if shell := strings.TrimSpace(os.Getenv("ComSpec")); shell != "" {
			envLines = append(envLines, fmt.Sprintf("- Preferred Shell: %s", shell))
		}
	} else {
		if shell := strings.TrimSpace(os.Getenv("SHELL")); shell != "" {
			envLines = append(envLines, fmt.Sprintf("- Preferred Shell: %s", shell))
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		envLines = append(envLines, fmt.Sprintf("- Current Working Directory: %s", cwd))
	}
	if len(envLines) == 0 {
		return ""
	}
	return strings.Join(envLines, "\n") + "\n"
}

func extractModelID(reqMap map[string]interface{}) string {
	if reqMap == nil {
		return ""
	}
	modelID, _ := reqMap["model"].(string)
	return strings.TrimSpace(modelID)
}

func extractChatInputText(reqMap map[string]interface{}) string {
	if reqMap == nil {
		return ""
	}
	if input, ok := reqMap["input"].(string); ok {
		return strings.TrimSpace(input)
	}
	if items, ok := reqMap["input"].([]interface{}); ok {
		var parts []string
		for _, item := range items {
			obj, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			itemType, _ := obj["type"].(string)
			switch itemType {
			case "text":
				if content, ok := obj["content"].(string); ok && strings.TrimSpace(content) != "" {
					parts = append(parts, strings.TrimSpace(content))
				}
			case "input_text":
				if text, ok := obj["text"].(string); ok && strings.TrimSpace(text) != "" {
					parts = append(parts, strings.TrimSpace(text))
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	if messages, ok := reqMap["messages"].([]interface{}); ok {
		for i := len(messages) - 1; i >= 0; i-- {
			msg, ok := messages[i].(map[string]interface{})
			if !ok {
				continue
			}
			role, _ := msg["role"].(string)
			if role != "user" {
				continue
			}
			if content, ok := msg["content"].(string); ok {
				return strings.TrimSpace(content)
			}
		}
	}
	return ""
}
