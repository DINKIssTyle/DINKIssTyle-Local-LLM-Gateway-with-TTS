package evalharness

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"dinkisstyle-chat/internal/chatharness"
	"dinkisstyle-chat/internal/promptkit"
	"dinkisstyle-chat/internal/skillkit"
	"dinkisstyle-chat/internal/toolruntime"
)

const defaultMaxTurns = 6

type Config struct {
	Endpoint         string
	Model            string
	APIKey           string
	BuiltinSkillsDir string
	UserSkillsDir    string
}

type Expectations struct {
	RequireToolCall           bool     `json:"require_tool_call"`
	ForbidToolCall            bool     `json:"forbid_tool_call,omitempty"`
	RequireWebTool            bool     `json:"require_web_tool"`
	MaxLLMRounds              int      `json:"max_llm_rounds,omitempty"`
	MinAnswerChars            int      `json:"min_answer_chars,omitempty"`
	MaxAnswerChars            int      `json:"max_answer_chars,omitempty"`
	MinSearchAngles           int      `json:"min_search_angles"`
	MaxSearchAngles           int      `json:"max_search_angles"`
	MinSources                int      `json:"min_sources"`
	MinAuthoritativeSources   int      `json:"min_authoritative_sources"`
	RequireCitations          bool     `json:"require_citations"`
	AllowedTools              []string `json:"allowed_tools,omitempty"`
	ForbidDuplicates          bool     `json:"forbid_duplicates"`
	ForbidFutureDates         bool     `json:"forbid_future_dates"`
	RequiredURLSubstrings     []string `json:"required_url_substrings,omitempty"`
	RequiredAnswerSubstrings  []string `json:"required_answer_substrings,omitempty"`
	ForbiddenAnswerSubstrings []string `json:"forbidden_answer_substrings,omitempty"`
	ForbidToolArgumentErrors  bool     `json:"forbid_tool_argument_errors,omitempty"`
	RequiredSkills            []string `json:"required_skills,omitempty"`
}

type ScenarioMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Scenario struct {
	ID              string            `json:"id"`
	Category        string            `json:"category,omitempty"`
	Prompt          string            `json:"prompt"`
	SystemPrompt    string            `json:"system_prompt,omitempty"`
	History         []ScenarioMessage `json:"history,omitempty"`
	EnableTools     *bool             `json:"enable_tools,omitempty"`
	ToolNames       []string          `json:"tool_names,omitempty"`
	ReasoningEffort string            `json:"reasoning_effort,omitempty"`
	MaxTurns        int               `json:"max_turns,omitempty"`
	ContextStrategy string            `json:"context_strategy,omitempty"`
	RecentContext   string            `json:"recent_context,omitempty"`
	MemorySnapshot  string            `json:"memory_snapshot,omitempty"`
	ActiveContext   string            `json:"active_context,omitempty"`
	Expectations    Expectations      `json:"expectations"`
}

type ScenarioFile struct {
	Scenarios []Scenario `json:"scenarios"`
}

type ToolTrace struct {
	Round             int                             `json:"round"`
	Name              string                          `json:"name"`
	Arguments         json.RawMessage                 `json:"arguments"`
	RecoveredText     bool                            `json:"recovered_text,omitempty"`
	RepairedArguments bool                            `json:"repaired_arguments,omitempty"`
	DurationMS        int64                           `json:"duration_ms"`
	Duplicate         bool                            `json:"duplicate,omitempty"`
	Error             string                          `json:"error,omitempty"`
	ResultPreview     string                          `json:"result_preview,omitempty"`
	Sources           []chatharness.WebEvidenceSource `json:"sources,omitempty"`
}

type Check struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Details string `json:"details"`
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
	CachedTokens     int `json:"cached_tokens,omitempty"`
	ReasoningTokens  int `json:"reasoning_tokens,omitempty"`
}

type RoundMetric struct {
	Round           int        `json:"round"`
	DurationMS      int64      `json:"duration_ms"`
	RequestBytes    int        `json:"request_bytes"`
	SystemChars     int        `json:"system_chars"`
	ToolSchemaBytes int        `json:"tool_schema_bytes"`
	MessageCount    int        `json:"message_count"`
	FinishReason    string     `json:"finish_reason,omitempty"`
	Usage           TokenUsage `json:"usage"`
	Failure         string     `json:"failure,omitempty"`
}

type Report struct {
	ScenarioID          string                          `json:"scenario_id"`
	Category            string                          `json:"category,omitempty"`
	Prompt              string                          `json:"prompt"`
	ToolsEnabled        bool                            `json:"tools_enabled"`
	Endpoint            string                          `json:"endpoint"`
	Model               string                          `json:"model"`
	StartedAt           string                          `json:"started_at"`
	DurationMS          int64                           `json:"duration_ms"`
	LLMRounds           int                             `json:"llm_rounds"`
	ProtocolCorrections int                             `json:"protocol_corrections"`
	SelectedSkills      []string                        `json:"selected_skills,omitempty"`
	SkillPromptChars    int                             `json:"skill_prompt_chars,omitempty"`
	SkillDiagnostics    []skillkit.Diagnostic           `json:"skill_diagnostics,omitempty"`
	RoundMetrics        []RoundMetric                   `json:"round_metrics"`
	Usage               TokenUsage                      `json:"usage"`
	ToolTrace           []ToolTrace                     `json:"tool_trace"`
	Sources             []chatharness.WebEvidenceSource `json:"sources,omitempty"`
	FinalAnswer         string                          `json:"final_answer"`
	Checks              []Check                         `json:"checks"`
	Passed              bool                            `json:"passed"`
	Failure             string                          `json:"failure,omitempty"`
}

type Runner struct {
	Config Config
	Client *http.Client
}

type chatResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		TotalTokens         int `json:"total_tokens"`
		InputTokens         int `json:"input_tokens"`
		OutputTokens        int `json:"output_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		InputTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"input_tokens_details"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
		OutputTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"output_tokens_details"`
	} `json:"usage"`
	Error json.RawMessage `json:"error,omitempty"`
}

func LoadConfig(envFile string) (Config, error) {
	values, err := loadEnvFile(envFile)
	if err != nil {
		return Config{}, err
	}
	config := Config{
		Endpoint:         firstValue(os.Getenv("DKST_EVAL_ENDPOINT"), values["DKST_EVAL_ENDPOINT"]),
		Model:            firstValue(os.Getenv("DKST_EVAL_MODEL"), values["DKST_EVAL_MODEL"]),
		APIKey:           firstValue(os.Getenv("DKST_EVAL_API_KEY"), values["DKST_EVAL_API_KEY"]),
		BuiltinSkillsDir: firstValue(os.Getenv("DKST_EVAL_BUILTIN_SKILLS_DIR"), values["DKST_EVAL_BUILTIN_SKILLS_DIR"], filepath.Join("bundle", "skills", "builtin")),
		UserSkillsDir:    firstValue(os.Getenv("DKST_EVAL_USER_SKILLS_DIR"), values["DKST_EVAL_USER_SKILLS_DIR"]),
	}
	if config.Endpoint == "" || config.Model == "" || config.APIKey == "" {
		return Config{}, errors.New("DKST_EVAL_ENDPOINT, DKST_EVAL_MODEL, and DKST_EVAL_API_KEY are required")
	}
	parsed, err := url.Parse(config.Endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return Config{}, fmt.Errorf("invalid DKST_EVAL_ENDPOINT")
	}
	return config, nil
}

func loadEnvFile(path string) (map[string]string, error) {
	values := make(map[string]string)
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open eval environment file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if len(value) >= 2 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')) {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read eval environment file: %w", err)
	}
	return values, nil
}

func firstValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func LoadScenarios(path string) ([]Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scenarios: %w", err)
	}
	var file ScenarioFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode scenarios: %w", err)
	}
	seen := make(map[string]bool)
	for index := range file.Scenarios {
		scenario := &file.Scenarios[index]
		scenario.ID = strings.TrimSpace(scenario.ID)
		scenario.Prompt = strings.TrimSpace(scenario.Prompt)
		if scenario.ID == "" || scenario.Prompt == "" || seen[scenario.ID] {
			return nil, fmt.Errorf("scenario %d has an empty or duplicate id/prompt", index+1)
		}
		seen[scenario.ID] = true
		if scenario.MaxTurns <= 0 {
			scenario.MaxTurns = defaultMaxTurns
		}
		scenario.ReasoningEffort = strings.ToLower(strings.TrimSpace(scenario.ReasoningEffort))
		for messageIndex := range scenario.History {
			message := &scenario.History[messageIndex]
			message.Role = strings.ToLower(strings.TrimSpace(message.Role))
			message.Content = strings.TrimSpace(message.Content)
			if (message.Role != "user" && message.Role != "assistant") || message.Content == "" {
				return nil, fmt.Errorf("scenario %q has invalid history message %d", scenario.ID, messageIndex+1)
			}
		}
	}
	return file.Scenarios, nil
}

func (s Scenario) toolsEnabled() bool {
	return s.EnableTools == nil || *s.EnableTools
}

func SafeToolDefinitions() ([]promptkit.ToolDefinition, []string) {
	allowed := map[string]bool{
		"search_web": true, "search_web_multi": true, "read_web_page": true,
		"read_buffered_source": true, "naver_search": true, "namu_wiki": true,
		"read_help": true, "get_current_time": true,
	}
	definitions := toolruntime.Default.List(toolruntime.ExecutionContext{EnableMemory: false})
	tools := make([]promptkit.ToolDefinition, 0, len(definitions))
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		if !allowed[definition.Name] {
			continue
		}
		tools = append(tools, promptkit.ToolDefinition{
			Name: definition.Name, Description: definition.Description, InputSchema: definition.InputSchema,
		})
		names = append(names, definition.Name)
	}
	return tools, names
}

func filterToolDefinitions(tools []promptkit.ToolDefinition, names []string) ([]promptkit.ToolDefinition, []string) {
	if len(names) == 0 {
		allNames := make([]string, 0, len(tools))
		for _, tool := range tools {
			allNames = append(allNames, tool.Name)
		}
		return tools, allNames
	}
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			wanted[name] = true
		}
	}
	filtered := make([]promptkit.ToolDefinition, 0, len(wanted))
	filteredNames := make([]string, 0, len(wanted))
	for _, tool := range tools {
		if wanted[tool.Name] {
			filtered = append(filtered, tool)
			filteredNames = append(filteredNames, tool.Name)
			delete(wanted, tool.Name)
		}
	}
	if len(wanted) > 0 {
		missing := make([]string, 0, len(wanted))
		for name := range wanted {
			missing = append(missing, name)
		}
		sort.Strings(missing)
		return nil, missing
	}
	return filtered, filteredNames
}

func (r Runner) Run(ctx context.Context, scenario Scenario) Report {
	started := time.Now()
	toolsEnabled := scenario.toolsEnabled()
	report := Report{
		ScenarioID:   scenario.ID,
		Category:     scenario.Category,
		Prompt:       scenario.Prompt,
		ToolsEnabled: toolsEnabled,
		Endpoint:     sanitizeEndpointForReport(r.Config.Endpoint),
		Model:        r.Config.Model,
		StartedAt:    started.Format(time.RFC3339),
	}
	finish := func() Report {
		report.DurationMS = time.Since(started).Milliseconds()
		return finalizeReport(report, scenario)
	}

	tools, _ := SafeToolDefinitions()
	var availableToolNames []string
	if toolsEnabled {
		var missing []string
		tools, availableToolNames = filterToolDefinitions(tools, scenario.ToolNames)
		if tools == nil && len(missing) == 0 {
			missing = availableToolNames
		}
		if len(missing) > 0 {
			report.Failure = "unknown or unsafe scenario tools: " + strings.Join(missing, ", ")
			return finish()
		}
	} else {
		tools = nil
	}
	systemPrompt := strings.TrimSpace(scenario.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = "You are a rigorous evaluation assistant. Use the app tools for current or externally verifiable information."
		if !toolsEnabled {
			systemPrompt = "You are a helpful assistant. Follow the user's instructions accurately and answer in the user's language."
		}
	}
	messages := []any{map[string]any{"role": "system", "content": systemPrompt}}
	for _, historyMessage := range scenario.History {
		messages = append(messages, map[string]any{"role": historyMessage.Role, "content": historyMessage.Content})
	}
	messages = append(messages, map[string]any{"role": "user", "content": scenario.Prompt})
	skillCompilation := skillkit.LoadAndCompile(skillkit.Config{
		BuiltinDir: r.Config.BuiltinSkillsDir,
		UserDir:    r.Config.UserSkillsDir,
	}, scenario.Prompt)
	for _, skill := range skillCompilation.Selected {
		report.SelectedSkills = append(report.SelectedSkills, skill.Namespace)
	}
	report.SkillPromptChars = len([]rune(skillCompilation.Prompt))
	report.SkillDiagnostics = skillCompilation.Diagnostics
	requestPayload := map[string]any{
		"model":    r.Config.Model,
		"messages": messages,
		"stream":   false,
	}
	if scenario.ReasoningEffort != "" {
		requestPayload["reasoning_effort"] = scenario.ReasoningEffort
	}
	requestBody, _ := json.Marshal(requestPayload)
	contextStrategy := strings.TrimSpace(scenario.ContextStrategy)
	if contextStrategy == "" {
		contextStrategy = "none"
	}
	memorySnapshot := scenario.MemorySnapshot
	activeContext := scenario.ActiveContext
	if strings.TrimSpace(scenario.RecentContext) != "" && chatharness.IsLikelyContextualFollowup(scenario.Prompt) {
		memorySnapshot = ""
		activeContext = ""
	}
	prepared, err := chatharness.PrepareRequest(chatharness.RequestInput{
		Body: requestBody, EndpointRaw: r.Config.Endpoint, TokenRaw: r.Config.APIKey,
		LLMMode: "standard", ContextStrategy: contextStrategy, EnableTools: toolsEnabled, Tools: tools,
		RecentContext: scenario.RecentContext, MemorySnapshot: memorySnapshot, ActiveContext: activeContext,
		RetrievalInjected: strings.TrimSpace(scenario.RecentContext) != "" || strings.TrimSpace(memorySnapshot) != "" || strings.TrimSpace(activeContext) != "",
		SkillInstructions: skillCompilation.Prompt,
	})
	if err != nil {
		report.Failure = err.Error()
		return finish()
	}
	reqMap := prepared.ReqMap
	providerTools, _ := reqMap["tools"].([]interface{})
	endpoint := prepared.UpstreamURL
	execContext := toolruntime.ExecutionContext{RequestID: "eval-" + scenario.ID, UserID: "eval-" + scenario.ID, EnableMemory: false}
	seenSignatures := make(map[string]bool)
	completedTools := make(map[string]bool)
	sourceURLs := make(map[string]bool)
	webSearchAttempts := 0
	mustAnswer := false
	finalCorrectionUsed := false
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	for round := 1; round <= scenario.MaxTurns+1; round++ {
		report.LLMRounds = round
		metric := inspectRoundRequest(round, reqMap)
		llmStarted := time.Now()
		response, callErr := r.callChat(ctx, client, endpoint, reqMap)
		metric.DurationMS = time.Since(llmStarted).Milliseconds()
		if callErr != nil {
			metric.Failure = callErr.Error()
			report.RoundMetrics = append(report.RoundMetrics, metric)
			report.Failure = callErr.Error()
			break
		}
		metric.Usage = usageFromResponse(response)
		if len(response.Choices) > 0 {
			metric.FinishReason = response.Choices[0].FinishReason
		}
		report.RoundMetrics = append(report.RoundMetrics, metric)
		addUsage(&report.Usage, metric.Usage)
		if len(response.Choices) == 0 {
			report.Failure = "LLM response contained no choices"
			break
		}
		message := response.Choices[0].Message
		if mustAnswer {
			_, textualToolIntent := chatharness.ParseFunctionParameterToolCall(message.Content)
			if len(message.ToolCalls) > 0 || textualToolIntent {
				if finalCorrectionUsed {
					report.Failure = "LLM repeated a tool call after the final-answer protocol correction"
					break
				}
				messages, _ := reqMap["messages"].([]interface{})
				messages = append(messages, map[string]interface{}{
					"role":    "user",
					"content": "[APP PROTOCOL CORRECTION — NOT A USER REQUEST]\nThe tool phase is closed and all needed evidence is already present. Do not emit or describe a tool call. Produce the final answer to the original request now, in the user's language, with the strongest retrieved source links.",
				})
				reqMap["messages"] = messages
				reqMap["stream"] = false
				finalCorrectionUsed = true
				report.ProtocolCorrections++
				continue
			}
		}
		callID := ""
		name := ""
		arguments := ""
		recoveredTextCall := false
		if len(message.ToolCalls) > 0 {
			call := message.ToolCalls[0]
			callID = call.ID
			name = strings.TrimSpace(call.Function.Name)
			arguments = strings.TrimSpace(call.Function.Arguments)
		} else if call, ok := chatharness.ParseFunctionParameterToolCall(message.Content); ok {
			name = strings.TrimSpace(call.Name)
			arguments = strings.TrimSpace(call.Arguments)
			recoveredTextCall = true
		} else {
			report.FinalAnswer = strings.TrimSpace(message.Content)
			break
		}
		if !toolsEnabled {
			report.Failure = fmt.Sprintf("model attempted disabled tool %q", name)
			break
		}

		if arguments == "" {
			arguments = "{}"
		}
		repairedArguments := false
		if repaired, ok := chatharness.RepairMissingSearchToolArguments(name, arguments, scenario.Prompt, scenario.RecentContext); ok {
			arguments = repaired
			repairedArguments = true
		}
		if refined, ok := chatharness.RefineFamilySearchToolArguments(name, arguments, scenario.Prompt); ok {
			arguments = refined
			repairedArguments = true
		}
		if upgradedName, upgradedArguments, upgraded := chatharness.UpgradeFreshnessSearchToolCall(name, arguments, scenario.Prompt); upgraded {
			name = upgradedName
			arguments = upgradedArguments
			repairedArguments = true
		}
		signature := canonicalToolSignature(name, arguments)
		duplicate := seenSignatures[signature]
		seenSignatures[signature] = true
		trace := ToolTrace{
			Round: round, Name: name, Arguments: safeRawJSON(arguments),
			RecoveredText: recoveredTextCall, RepairedArguments: repairedArguments, Duplicate: duplicate,
		}

		result := ""
		if duplicate {
			result = "Duplicate tool call prevented. Use the evidence already returned and answer now."
		} else {
			toolStarted := time.Now()
			toolResult, toolErr := toolruntime.Default.Call(ctx, execContext, name, json.RawMessage(arguments))
			trace.DurationMS = time.Since(toolStarted).Milliseconds()
			result = toolResult.Content
			if toolErr != nil {
				trace.Error = toolErr.Error()
				if strings.TrimSpace(result) == "" {
					result = "Error executing tool: " + toolErr.Error()
				}
			}
		}
		completedTools[name] = true
		trace.ResultPreview = compact(result, 900)
		trace.Sources = chatharness.ExtractWebEvidenceSources(result, 6)
		for _, source := range trace.Sources {
			if !sourceURLs[source.URL] {
				sourceURLs[source.URL] = true
				report.Sources = append(report.Sources, source)
			}
		}
		webSearchAttempts += searchAngles(name, arguments, duplicate)
		report.ToolTrace = append(report.ToolTrace, trace)

		freshnessCrossCheck := chatharness.IsFreshnessSensitiveWebRequest(scenario.Prompt) &&
			isWebSearchTool(name) && webSearchAttempts < 2 && !duplicate
		finalAnswerOnly := chatharness.ShouldFinalizeAfterWebSearch(name, result, false, false)
		singleSearchRefinement := strings.Contains(strings.ToLower(result), "evidence quality warning: no_authoritative_or_reputable_source")
		if isWebSearchTool(name) && webSearchAttempts >= 3 {
			finalAnswerOnly = true
			singleSearchRefinement = false
		}
		if freshnessCrossCheck {
			finalAnswerOnly = false
		}
		if duplicate {
			finalAnswerOnly = true
		}
		completed := make([]string, 0, len(completedTools))
		for _, toolName := range availableToolNames {
			if completedTools[toolName] {
				completed = append(completed, toolName)
			}
		}
		reqMap, _, err = chatharness.PrepareToolFollowupRequest(chatharness.ToolFollowupInput{
			LLMMode: "standard", ModelID: r.Config.Model, ToolName: name, ToolResult: result,
			ReqMap: reqMap, ToolCallID: callID, ToolArguments: arguments, OriginalUserText: scenario.Prompt,
			CompletedTools: completed, AvailableTools: availableToolNames, FinalAnswerOnly: finalAnswerOnly,
			RequireFreshnessCrossCheck: freshnessCrossCheck, SingleSearchRefinement: singleSearchRefinement,
			ProviderTools: providerTools,
		})
		if err != nil {
			report.Failure = err.Error()
			break
		}
		reqMap["stream"] = false
		mustAnswer = finalAnswerOnly
	}

	if report.FinalAnswer == "" && report.Failure == "" {
		report.Failure = fmt.Sprintf("no final answer after %d LLM rounds", scenario.MaxTurns)
	}
	report.FinalAnswer = chatharness.AppendMissingWebEvidenceSources(report.FinalAnswer, scenario.Prompt, report.Sources)
	return finish()
}

func (r Runner) callChat(ctx context.Context, client *http.Client, endpoint string, payload map[string]any) (chatResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return chatResponse{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return chatResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+r.Config.APIKey)
	response, err := client.Do(request)
	if err != nil {
		return chatResponse{}, fmt.Errorf("LLM request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return chatResponse{}, fmt.Errorf("read LLM response: %w", err)
	}
	var decoded chatResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return chatResponse{}, fmt.Errorf("decode LLM response (status %d): %w", response.StatusCode, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || len(decoded.Error) > 0 {
		message := strings.TrimSpace(http.StatusText(response.StatusCode))
		if len(decoded.Error) > 0 {
			var textError string
			var objectError struct {
				Message string `json:"message"`
			}
			if json.Unmarshal(decoded.Error, &textError) == nil && strings.TrimSpace(textError) != "" {
				message = strings.TrimSpace(textError)
			} else if json.Unmarshal(decoded.Error, &objectError) == nil && strings.TrimSpace(objectError.Message) != "" {
				message = strings.TrimSpace(objectError.Message)
			}
		}
		message = strings.ReplaceAll(message, r.Config.APIKey, "[REDACTED]")
		return chatResponse{}, fmt.Errorf("LLM API returned status %d: %s", response.StatusCode, compact(message, 400))
	}
	return decoded, nil
}

func inspectRoundRequest(round int, payload map[string]any) RoundMetric {
	metric := RoundMetric{Round: round}
	if body, err := json.Marshal(payload); err == nil {
		metric.RequestBytes = len(body)
	}
	if tools, exists := payload["tools"]; exists {
		if encoded, err := json.Marshal(tools); err == nil {
			metric.ToolSchemaBytes = len(encoded)
		}
	}
	if messages, ok := payload["messages"].([]interface{}); ok {
		metric.MessageCount = len(messages)
		for _, raw := range messages {
			message, _ := raw.(map[string]interface{})
			if role, _ := message["role"].(string); role != "system" {
				continue
			}
			if content, _ := message["content"].(string); content != "" {
				metric.SystemChars += len([]rune(content))
			}
		}
	}
	if input, ok := payload["input"].(string); ok && metric.MessageCount == 0 {
		metric.MessageCount = 1
		_ = input
	}
	return metric
}

func usageFromResponse(response chatResponse) TokenUsage {
	promptTokens := response.Usage.PromptTokens
	if promptTokens == 0 {
		promptTokens = response.Usage.InputTokens
	}
	completionTokens := response.Usage.CompletionTokens
	if completionTokens == 0 {
		completionTokens = response.Usage.OutputTokens
	}
	totalTokens := response.Usage.TotalTokens
	if totalTokens == 0 && (promptTokens > 0 || completionTokens > 0) {
		totalTokens = promptTokens + completionTokens
	}
	cachedTokens := response.Usage.PromptTokensDetails.CachedTokens
	if cachedTokens == 0 {
		cachedTokens = response.Usage.InputTokensDetails.CachedTokens
	}
	reasoningTokens := response.Usage.CompletionTokensDetails.ReasoningTokens
	if reasoningTokens == 0 {
		reasoningTokens = response.Usage.OutputTokensDetails.ReasoningTokens
	}
	return TokenUsage{
		PromptTokens: promptTokens, CompletionTokens: completionTokens, TotalTokens: totalTokens,
		CachedTokens: cachedTokens, ReasoningTokens: reasoningTokens,
	}
}

func addUsage(total *TokenUsage, usage TokenUsage) {
	if total == nil {
		return
	}
	total.PromptTokens += usage.PromptTokens
	total.CompletionTokens += usage.CompletionTokens
	total.TotalTokens += usage.TotalTokens
	total.CachedTokens += usage.CachedTokens
	total.ReasoningTokens += usage.ReasoningTokens
}

func finalizeReport(report Report, scenario Scenario) Report {
	checks := []Check{{Name: "final_answer", Passed: strings.TrimSpace(report.FinalAnswer) != "", Details: "final answer must be non-empty"}}
	lowerAnswer := normalizeEvaluationText(report.FinalAnswer)
	rawToolMarkup := strings.Contains(lowerAnswer, "<tool_call>") || strings.Contains(lowerAnswer, "<function=")
	checks = append(checks, Check{
		Name: "no_raw_tool_markup", Passed: !rawToolMarkup,
		Details: "final answer must not contain textual tool-call markup",
	})
	if scenario.Expectations.RequireToolCall {
		checks = append(checks, Check{Name: "tool_call", Passed: len(report.ToolTrace) > 0, Details: fmt.Sprintf("tool calls=%d", len(report.ToolTrace))})
	}
	if scenario.Expectations.ForbidToolCall {
		checks = append(checks, Check{Name: "no_tool_call", Passed: len(report.ToolTrace) == 0, Details: fmt.Sprintf("tool calls=%d", len(report.ToolTrace))})
	}
	if scenario.Expectations.MaxLLMRounds > 0 {
		checks = append(checks, Check{
			Name: "llm_round_budget", Passed: report.LLMRounds <= scenario.Expectations.MaxLLMRounds,
			Details: fmt.Sprintf("rounds=%d, maximum=%d", report.LLMRounds, scenario.Expectations.MaxLLMRounds),
		})
	}
	answerChars := len([]rune(strings.TrimSpace(report.FinalAnswer)))
	if scenario.Expectations.MinAnswerChars > 0 {
		checks = append(checks, Check{
			Name: "answer_min_length", Passed: answerChars >= scenario.Expectations.MinAnswerChars,
			Details: fmt.Sprintf("answer chars=%d, minimum=%d", answerChars, scenario.Expectations.MinAnswerChars),
		})
	}
	if scenario.Expectations.MaxAnswerChars > 0 {
		checks = append(checks, Check{
			Name: "answer_max_length", Passed: answerChars <= scenario.Expectations.MaxAnswerChars,
			Details: fmt.Sprintf("answer chars=%d, maximum=%d", answerChars, scenario.Expectations.MaxAnswerChars),
		})
	}
	if scenario.Expectations.RequireWebTool {
		found := false
		for _, trace := range report.ToolTrace {
			if isWebTool(trace.Name) {
				found = true
				break
			}
		}
		checks = append(checks, Check{Name: "web_tool", Passed: found, Details: "at least one real web tool must run"})
	}
	angles := distinctSearchAngles(report.ToolTrace)
	if scenario.Expectations.MinSearchAngles > 0 {
		checks = append(checks, Check{Name: "search_angles", Passed: angles >= scenario.Expectations.MinSearchAngles, Details: fmt.Sprintf("distinct angles=%d, required=%d", angles, scenario.Expectations.MinSearchAngles)})
	}
	if scenario.Expectations.MaxSearchAngles > 0 {
		checks = append(checks, Check{Name: "search_angle_budget", Passed: angles <= scenario.Expectations.MaxSearchAngles, Details: fmt.Sprintf("distinct angles=%d, maximum=%d", angles, scenario.Expectations.MaxSearchAngles)})
	}
	if scenario.Expectations.MinSources > 0 {
		checks = append(checks, Check{Name: "sources", Passed: len(report.Sources) >= scenario.Expectations.MinSources, Details: fmt.Sprintf("sources=%d, required=%d", len(report.Sources), scenario.Expectations.MinSources)})
	}
	if scenario.Expectations.MinAuthoritativeSources > 0 {
		authoritative := 0
		for _, source := range report.Sources {
			if isHighConfidenceQuality(source.Quality) || isAuthoritativeSource(source.URL) {
				authoritative++
			}
		}
		checks = append(checks, Check{
			Name: "authoritative_sources", Passed: authoritative >= scenario.Expectations.MinAuthoritativeSources,
			Details: fmt.Sprintf("authoritative sources=%d, required=%d", authoritative, scenario.Expectations.MinAuthoritativeSources),
		})
	}
	if scenario.Expectations.RequireCitations {
		cited := false
		for _, source := range report.Sources {
			if strings.Contains(report.FinalAnswer, source.URL) {
				cited = true
				break
			}
		}
		checks = append(checks, Check{Name: "citations", Passed: cited, Details: "final answer must contain a retrieved source URL"})
	}
	if len(scenario.Expectations.RequiredAnswerSubstrings) > 0 {
		var missing []string
		for _, required := range scenario.Expectations.RequiredAnswerSubstrings {
			if !strings.Contains(lowerAnswer, normalizeEvaluationText(required)) {
				missing = append(missing, required)
			}
		}
		checks = append(checks, Check{
			Name: "required_answer_content", Passed: len(missing) == 0,
			Details: "missing=" + strings.Join(missing, ","),
		})
	}
	if len(scenario.Expectations.ForbiddenAnswerSubstrings) > 0 {
		var found []string
		for _, forbidden := range scenario.Expectations.ForbiddenAnswerSubstrings {
			if strings.Contains(lowerAnswer, normalizeEvaluationText(forbidden)) {
				found = append(found, forbidden)
			}
		}
		checks = append(checks, Check{
			Name: "forbidden_answer_content", Passed: len(found) == 0,
			Details: "found=" + strings.Join(found, ","),
		})
	}
	if scenario.Expectations.ForbidToolArgumentErrors {
		var failures []string
		for _, trace := range report.ToolTrace {
			normalizedError := strings.ToLower(strings.TrimSpace(trace.Error))
			if strings.Contains(normalizedError, "invalid arguments") ||
				strings.Contains(normalizedError, "argument \"query\" is required") ||
				strings.Contains(normalizedError, "argument \"queries\" is required") {
				failures = append(failures, trace.Name+": "+trace.Error)
			}
		}
		checks = append(checks, Check{
			Name: "tool_arguments", Passed: len(failures) == 0,
			Details: strings.Join(failures, "; "),
		})
	}
	if len(scenario.Expectations.RequiredSkills) > 0 {
		selected := make(map[string]bool, len(report.SelectedSkills))
		for _, namespace := range report.SelectedSkills {
			selected[namespace] = true
		}
		var missing []string
		for _, required := range scenario.Expectations.RequiredSkills {
			if !selected[required] {
				missing = append(missing, required)
			}
		}
		checks = append(checks, Check{
			Name: "required_skills", Passed: len(missing) == 0,
			Details: "selected=" + strings.Join(report.SelectedSkills, ",") + "; missing=" + strings.Join(missing, ","),
		})
	}
	if len(scenario.Expectations.RequiredURLSubstrings) > 0 {
		var missing []string
		for _, required := range scenario.Expectations.RequiredURLSubstrings {
			found := false
			for _, source := range report.Sources {
				if strings.Contains(source.URL, required) {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, required)
			}
		}
		checks = append(checks, Check{Name: "required_urls", Passed: len(missing) == 0, Details: "missing=" + strings.Join(missing, ",")})
	}
	if scenario.Expectations.ForbidDuplicates {
		duplicates := 0
		for _, trace := range report.ToolTrace {
			if trace.Duplicate {
				duplicates++
			}
		}
		checks = append(checks, Check{Name: "duplicate_calls", Passed: duplicates == 0, Details: fmt.Sprintf("duplicates=%d", duplicates)})
	}
	if scenario.Expectations.ForbidFutureDates {
		futureDates := futureDateMentions(report.FinalAnswer, time.Now())
		checks = append(checks, Check{
			Name: "future_dates", Passed: len(futureDates) == 0,
			Details: "future date mentions=" + strings.Join(futureDates, ","),
		})
	}
	if len(scenario.Expectations.AllowedTools) > 0 {
		allowed := make(map[string]bool)
		for _, name := range scenario.Expectations.AllowedTools {
			allowed[name] = true
		}
		var unexpected []string
		for _, trace := range report.ToolTrace {
			if !allowed[trace.Name] {
				unexpected = append(unexpected, trace.Name)
			}
		}
		sort.Strings(unexpected)
		checks = append(checks, Check{Name: "allowed_tools", Passed: len(unexpected) == 0, Details: "unexpected=" + strings.Join(unexpected, ",")})
	}
	if report.Failure != "" {
		checks = append(checks, Check{Name: "runtime", Passed: false, Details: report.Failure})
	}
	report.Checks = checks
	report.Passed = true
	for _, check := range checks {
		if !check.Passed {
			report.Passed = false
			break
		}
	}
	return report
}

func normalizeEvaluationText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.NewReplacer(
		"₀", "0", "₁", "1", "₂", "2", "₃", "3", "₄", "4",
		"₅", "5", "₆", "6", "₇", "7", "₈", "8", "₉", "9",
		"⁰", "0", "¹", "1", "²", "2", "³", "3", "⁴", "4",
		"⁵", "5", "⁶", "6", "⁷", "7", "⁸", "8", "⁹", "9",
	).Replace(value)
}

func isHighConfidenceQuality(quality string) bool {
	switch strings.ToLower(strings.TrimSpace(quality)) {
	case "authoritative", "reputable_news", "primary_repository":
		return true
	default:
		return false
	}
}

func canonicalToolSignature(name, arguments string) string {
	var value any
	if json.Unmarshal([]byte(arguments), &value) == nil {
		if encoded, err := json.Marshal(value); err == nil {
			arguments = string(encoded)
		}
	}
	return strings.TrimSpace(name) + ":" + strings.TrimSpace(arguments)
}

func searchAngles(name, arguments string, duplicate bool) int {
	if duplicate {
		return 0
	}
	switch name {
	case "search_web", "naver_search", "namu_wiki":
		return 1
	case "search_web_multi":
		var payload struct {
			Queries []string `json:"queries"`
		}
		if json.Unmarshal([]byte(arguments), &payload) == nil {
			return len(payload.Queries)
		}
	}
	return 0
}

func distinctSearchAngles(traces []ToolTrace) int {
	seen := make(map[string]bool)
	for _, trace := range traces {
		if trace.Duplicate {
			continue
		}
		var payload map[string]any
		_ = json.Unmarshal(trace.Arguments, &payload)
		if trace.Name == "search_web_multi" {
			if queries, ok := payload["queries"].([]any); ok {
				for _, raw := range queries {
					if query, ok := raw.(string); ok {
						seen[strings.ToLower(strings.Join(strings.Fields(query), " "))] = true
					}
				}
			}
			continue
		}
		if isWebSearchTool(trace.Name) {
			for _, key := range []string{"query", "keyword"} {
				if query, ok := payload[key].(string); ok && strings.TrimSpace(query) != "" {
					seen[strings.ToLower(strings.Join(strings.Fields(query), " "))] = true
				}
			}
		}
	}
	return len(seen)
}

func isWebSearchTool(name string) bool {
	switch name {
	case "search_web", "search_web_multi", "naver_search", "namu_wiki":
		return true
	default:
		return false
	}
}

func isWebTool(name string) bool {
	return isWebSearchTool(name) || name == "read_web_page" || name == "read_buffered_source"
}

func sanitizeEndpointForReport(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "invalid-endpoint"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func isAuthoritativeSource(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" {
		return false
	}
	for _, suffix := range []string{".gov", ".gov.uk", ".go.kr", ".edu", ".ac.kr", ".int"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	for _, domain := range []string{
		"go.dev", "golang.org", "openai.com", "anthropic.com", "deepmind.google", "ai.google.dev",
		"blog.google", "microsoft.com", "meta.com", "nvidia.com", "huggingface.co", "github.com",
		"reuters.com", "apnews.com", "bbc.com", "bbc.co.uk", "nytimes.com", "ft.com",
		"theguardian.com", "bloomberg.com", "wsj.com", "cnn.com", "aljazeera.com",
		"washingtonpost.com", "foxnews.com", "techcrunch.com", "theverge.com", "wired.com", "arstechnica.com",
		"yna.co.kr", "yonhapnews.co.kr", "chosun.com", "joongang.co.kr", "donga.com", "hani.co.kr", "khan.co.kr", "mk.co.kr", "sedaily.com", "etnews.com", "zdnet.co.kr", "aitimes.com", "lawtimes.co.kr",
		"aa.com.tr", "elpais.com", "euronews.com",
		"un.org", "unhcr.org", "iom.int", "europa.eu", "nia.or.kr",
	} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func compact(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if limit <= 0 || len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit])) + "... (truncated)"
}

var localizedDatePattern = regexp.MustCompile(`(?:(\d{4})년\s*)?(\d{1,2})월\s*(\d{1,2})일`)

func futureDateMentions(text string, now time.Time) []string {
	seen := make(map[string]bool)
	var mentions []string
	for _, match := range localizedDatePattern.FindAllStringSubmatch(text, -1) {
		year := now.Year()
		if match[1] != "" {
			year, _ = strconv.Atoi(match[1])
		}
		month, _ := strconv.Atoi(match[2])
		day, _ := strconv.Atoi(match[3])
		candidate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, now.Location())
		if candidate.Year() != year || int(candidate.Month()) != month || candidate.Day() != day {
			continue
		}
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		if candidate.After(today) && !seen[match[0]] {
			seen[match[0]] = true
			mentions = append(mentions, match[0])
		}
	}
	return mentions
}

func WriteReport(path string, report Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func safeRawJSON(value string) json.RawMessage {
	raw := json.RawMessage(strings.TrimSpace(value))
	if json.Valid(raw) {
		return raw
	}
	encoded, _ := json.Marshal(value)
	return json.RawMessage(encoded)
}
