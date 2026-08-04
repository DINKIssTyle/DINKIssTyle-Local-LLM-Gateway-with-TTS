package codetestbed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type RunRequest struct {
	Endpoint      string `json:"endpoint"`
	APIKey        string `json:"api_key"`
	Model         string `json:"model"`
	Workspace     string `json:"workspace"`
	TestPrompt    string `json:"test_prompt"`
	AnalysisTask  string `json:"analysis_task"`
	Task          string `json:"task,omitempty"`
	Mode          string `json:"mode"`
	AllowWrites   bool   `json:"allow_writes"`
	AllowCommands bool   `json:"allow_commands"`
	MaxSteps      int    `json:"max_steps"`
}

type RunResult struct {
	Answer       string                  `json:"answer"`
	Trace        []TraceStep             `json:"trace"`
	ChangedFiles []string                `json:"changed_files,omitempty"`
	Rounds       int                     `json:"rounds"`
	DurationMS   int64                   `json:"duration_ms"`
	Failure      string                  `json:"failure,omitempty"`
	TestReport   *ConversationTestReport `json:"test_report,omitempty"`
}

type ConversationTestReport struct {
	Prompt          string          `json:"prompt"`
	FinalAnswer     string          `json:"final_answer"`
	Failure         string          `json:"failure,omitempty"`
	SelectedSkills  []string        `json:"selected_skills,omitempty"`
	ToolTrace       []TestToolTrace `json:"tool_trace,omitempty"`
	LLMRounds       int             `json:"llm_rounds"`
	DurationMS      int64           `json:"duration_ms"`
	TotalTokens     int             `json:"total_tokens,omitempty"`
	ProtocolRepairs int             `json:"protocol_corrections,omitempty"`
}

type TestToolTrace struct {
	Round         int             `json:"round"`
	Name          string          `json:"name"`
	Arguments     json.RawMessage `json:"arguments"`
	Error         string          `json:"error,omitempty"`
	ResultPreview string          `json:"result_preview,omitempty"`
	Sources       []string        `json:"sources,omitempty"`
}

type TraceStep struct {
	Round     int             `json:"round"`
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	Result    string          `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	Duration  int64           `json:"duration_ms"`
	Recovered bool            `json:"recovered_text,omitempty"`
}

type Agent struct {
	Client *http.Client
}

type chatResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []toolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Error json.RawMessage `json:"error,omitempty"`
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (a Agent) Run(ctx context.Context, request RunRequest) (result RunResult) {
	started := time.Now()
	defer func() { result.DurationMS = time.Since(started).Milliseconds() }()

	request = normalizeRunRequest(request)
	if err := validateRunRequest(request); err != nil {
		result.Failure = err.Error()
		return result
	}
	executor, err := NewToolExecutor(request.Workspace, request.AllowWrites && request.Mode == "fix", request.AllowCommands)
	if err != nil {
		result.Failure = err.Error()
		return result
	}

	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt(request)},
		{"role": "user", "content": request.Task},
	}
	definitions := executor.Definitions()
	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 4 * time.Minute}
	}
	changed := make(map[string]bool)
	signatures := make(map[string]int)

	for round := 1; round <= request.MaxSteps; round++ {
		result.Rounds = round
		response, err := callLocalLLM(ctx, client, request, messages, definitions)
		if err != nil {
			result.Failure = err.Error()
			return result
		}
		if len(response.Choices) == 0 {
			result.Failure = "local LLM returned no choices"
			return result
		}
		message := response.Choices[0].Message
		calls := message.ToolCalls
		recoveredText := false
		if len(calls) == 0 {
			if recovered, ok := parseTextToolCall(message.Content, definitions); ok {
				calls = []toolCall{recovered}
				recoveredText = true
			}
		}
		if len(calls) == 0 {
			result.Answer = strings.TrimSpace(message.Content)
			if result.Answer == "" {
				result.Failure = "local LLM returned neither an answer nor a tool call"
			}
			result.ChangedFiles = sortedKeys(changed)
			return result
		}
		for index := range calls {
			if strings.TrimSpace(calls[index].ID) == "" {
				calls[index].ID = fmt.Sprintf("call_%d_%d", round, index)
			}
		}

		if !recoveredText {
			messages = append(messages, map[string]interface{}{
				"role": "assistant", "content": message.Content, "tool_calls": calls,
			})
		} else {
			messages = append(messages, map[string]interface{}{"role": "assistant", "content": message.Content})
		}

		for index, call := range calls {
			if index >= 3 {
				break
			}
			name := strings.TrimSpace(call.Function.Name)
			arguments := strings.TrimSpace(call.Function.Arguments)
			if arguments == "" {
				arguments = "{}"
			}
			signature := name + ":" + arguments
			signatures[signature]++
			step := TraceStep{Round: round, Tool: name, Arguments: json.RawMessage(arguments), Recovered: recoveredText}
			toolStarted := time.Now()
			var output string
			var toolErr error
			if signatures[signature] > 2 {
				toolErr = errors.New("duplicate tool call stopped after two identical attempts")
			} else {
				output, toolErr = executor.Execute(ctx, name, json.RawMessage(arguments))
			}
			step.Duration = time.Since(toolStarted).Milliseconds()
			step.Result = compactResult(output, 6000)
			if toolErr != nil {
				step.Error = toolErr.Error()
			}
			result.Trace = append(result.Trace, step)
			if toolErr == nil && (name == "replace_text" || name == "write_new_file") {
				var pathArg struct {
					Path string `json:"path"`
				}
				if json.Unmarshal([]byte(arguments), &pathArg) == nil && strings.TrimSpace(pathArg.Path) != "" {
					changed[pathArg.Path] = true
				}
			}
			toolContent := output
			if toolErr != nil {
				toolContent = "ERROR: " + toolErr.Error()
				if strings.TrimSpace(output) != "" {
					toolContent += "\n\n" + output
				}
			}
			if recoveredText {
				messages = append(messages, map[string]interface{}{
					"role":    "user",
					"content": fmt.Sprintf("Tool result for %s with arguments %s:\n%s\nContinue the task. Use another tool if needed, otherwise give the final answer.", name, arguments, toolContent),
				})
			} else {
				messages = append(messages, map[string]interface{}{
					"role": "tool", "tool_call_id": call.ID, "name": name, "content": toolContent,
				})
			}
		}
	}

	result.ChangedFiles = sortedKeys(changed)
	result.Failure = fmt.Sprintf("maximum agent steps reached (%d)", request.MaxSteps)
	return result
}

func normalizeRunRequest(request RunRequest) RunRequest {
	request.Endpoint = strings.TrimRight(strings.TrimSpace(request.Endpoint), "/")
	request.Model = strings.TrimSpace(request.Model)
	request.Workspace = strings.TrimSpace(request.Workspace)
	request.TestPrompt = strings.TrimSpace(request.TestPrompt)
	request.AnalysisTask = strings.TrimSpace(request.AnalysisTask)
	request.Task = strings.TrimSpace(request.Task)
	request.Mode = strings.ToLower(strings.TrimSpace(request.Mode))
	if request.Mode == "" {
		request.Mode = "diagnose"
	}
	if request.MaxSteps <= 0 || request.MaxSteps > 20 {
		request.MaxSteps = 8
	}
	return request
}

func validateRunRequest(request RunRequest) error {
	if request.Endpoint == "" || request.Model == "" || request.Workspace == "" || request.Task == "" {
		return errors.New("endpoint, model, workspace, and analysis task are required")
	}
	if request.Mode != "diagnose" && request.Mode != "fix" {
		return errors.New("mode must be diagnose or fix")
	}
	parsed, err := url.Parse(request.Endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return errors.New("endpoint must be a valid HTTP(S) URL")
	}
	if !isLoopbackHost(parsed.Hostname()) {
		return errors.New("only local LLM endpoints are allowed")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func systemPrompt(request RunRequest) string {
	permission := "This is a diagnosis-only run. Do not modify files."
	if request.Mode == "fix" && request.AllowWrites {
		permission = "You may modify files with replace_text or write_new_file after inspecting the relevant code. Keep changes minimal and in scope."
	}
	commandRule := "Commands are disabled."
	if request.AllowCommands {
		commandRule = "You may run only the commands offered by run_command. After a code change, run the narrowest relevant test, then broader tests when practical."
	}
	return strings.Join([]string{
		"You are a careful coding testbed agent operating on one local workspace.",
		"You will receive an ACTUAL CONVERSATION TEST REPORT produced before this analysis. Treat that report as the primary evidence.",
		"Scenario files are only regression definitions. A missing named scenario is never the root cause of a failed runtime response unless the user explicitly asked to add that regression.",
		"Inspect evidence with tools before deciding the cause. Never invent file contents, command output, or test results.",
		permission,
		commandRule,
		"All paths must be workspace-relative. Never attempt to escape the workspace or access secrets.",
		"Prefer search_text, then read_file with focused ranges. Avoid reading generated dependencies and large files.",
		"When using tools, use native function calls. If unavailable, emit exactly one JSON object inside <tool_call> with name and arguments.",
		`Fallback example: <tool_call>{"name":"read_file","arguments":{"path":"main.go","start_line":1,"end_line":120}}</tool_call>`,
		"Finish with a concise report: root cause, files changed, validation performed, and any remaining risk. Answer in the user's language.",
	}, "\n")
}

func callLocalLLM(ctx context.Context, client *http.Client, request RunRequest, messages []map[string]interface{}, tools []ToolDefinition) (chatResponse, error) {
	payload := map[string]interface{}{
		"model":       request.Model,
		"messages":    messages,
		"tools":       tools,
		"tool_choice": "auto",
		"temperature": 0.1,
		"stream":      false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return chatResponse{}, err
	}
	chatURL := request.Endpoint
	if !strings.HasSuffix(chatURL, "/chat/completions") {
		chatURL += "/chat/completions"
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(body))
	if err != nil {
		return chatResponse{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(request.APIKey) != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+strings.TrimSpace(request.APIKey))
	}
	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return chatResponse{}, fmt.Errorf("call local LLM: %w", err)
	}
	defer httpResponse.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, 4*1024*1024))
	if err != nil {
		return chatResponse{}, err
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return chatResponse{}, fmt.Errorf("local LLM returned HTTP %d: %s", httpResponse.StatusCode, compactResult(string(responseBody), 1000))
	}
	var response chatResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return chatResponse{}, fmt.Errorf("decode local LLM response: %w", err)
	}
	if len(response.Error) > 0 && string(response.Error) != "null" {
		return chatResponse{}, fmt.Errorf("local LLM error: %s", compactResult(string(response.Error), 1000))
	}
	return response, nil
}

func parseTextToolCall(content string, definitions []ToolDefinition) (toolCall, bool) {
	trimmed := strings.TrimSpace(content)
	if start := strings.Index(trimmed, "<tool_call>"); start >= 0 {
		start += len("<tool_call>")
		if end := strings.Index(trimmed[start:], "</tool_call>"); end >= 0 {
			trimmed = strings.TrimSpace(trimmed[start : start+end])
		}
	}
	first := strings.Index(trimmed, "{")
	last := strings.LastIndex(trimmed, "}")
	if first < 0 || last <= first {
		return toolCall{}, false
	}
	var wrapper struct {
		Name      string          `json:"name"`
		Tool      string          `json:"tool"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal([]byte(trimmed[first:last+1]), &wrapper) != nil {
		return toolCall{}, false
	}
	name := strings.TrimSpace(wrapper.Name)
	if name == "" {
		name = strings.TrimSpace(wrapper.Tool)
	}
	if !knownTool(name, definitions) {
		return toolCall{}, false
	}
	arguments := strings.TrimSpace(string(wrapper.Arguments))
	if arguments == "" || arguments == "null" {
		arguments = "{}"
	}
	var call toolCall
	call.ID = "text_call"
	call.Type = "function"
	call.Function.Name = name
	call.Function.Arguments = arguments
	return call, true
}

func knownTool(name string, definitions []ToolDefinition) bool {
	for _, definition := range definitions {
		if definition.Function.Name == name {
			return true
		}
	}
	return false
}

func compactResult(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "\n… truncated"
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
