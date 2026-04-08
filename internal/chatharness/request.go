package chatharness

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	"dinkisstyle-chat/internal/promptkit"
)

type RequestInput struct {
	Body              []byte
	EndpointRaw       string
	TokenRaw          string
	LLMMode           string
	ContextStrategy   string
	EnableMCP         bool
	EnableMemory      bool
	ProceduralHint    string
	RecentContext     string
	MemorySnapshot    string
	ActiveContext     string
	RetrievalInjected bool
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

	useNativeIntegrations := input.EnableMCP && strings.TrimSpace(strings.ToLower(input.LLMMode)) == "stateful"
	includeRetrievalMemory := contextStrategy == "retrieval"
	shouldInjectRuntime := useNativeIntegrations || includeRetrievalMemory || strings.TrimSpace(input.ProceduralHint) != ""

	if shouldInjectRuntime {
		if useNativeIntegrations {
			ensureMCPIntegration(reqMap)
		}
		if !prepared.IsStatefulFollowup {
			extraInstr := promptkit.BuildRuntimeInstructions(promptkit.RuntimeInstructionsInput{
				EnvironmentInfo:       buildEnvironmentInfo(),
				ModelID:               extractModelID(reqMap),
				UseNativeIntegrations: useNativeIntegrations,
				ProceduralHint:        input.ProceduralHint,
				RecentContext:         conditionalContextValue(includeRetrievalMemory, input.RecentContext),
				MemorySnapshot:        conditionalContextValue(includeRetrievalMemory, input.MemorySnapshot),
				ActiveContext:         conditionalContextValue(includeRetrievalMemory, input.ActiveContext),
				RetrievalInjected:     includeRetrievalMemory && input.RetrievalInjected,
			})
			prepared.InjectedPrompt = promptkit.InjectPrompt(reqMap, extraInstr)
		}

		newBody, err := json.Marshal(reqMap)
		if err != nil {
			return prepared, fmt.Errorf("marshal prepared request: %w", err)
		}
		prepared.Body = newBody
	}

	prepared.UpstreamURL = buildUpstreamURL(prepared.Endpoint, input.LLMMode)
	prepared.ModelID = extractModelID(reqMap)
	return prepared, nil
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
	return strings.TrimSpace(pid) != ""
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

func ensureMCPIntegration(reqMap map[string]interface{}) {
	const targetMCP = "mcp/dinkisstyle-gateway"

	var integrations []string
	if existing, ok := reqMap["integrations"].([]interface{}); ok {
		for _, v := range existing {
			if str, ok := v.(string); ok {
				integrations = append(integrations, str)
			}
		}
	}

	for _, integration := range integrations {
		if integration == targetMCP {
			return
		}
	}

	reqMap["integrations"] = append(integrations, targetMCP)
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
