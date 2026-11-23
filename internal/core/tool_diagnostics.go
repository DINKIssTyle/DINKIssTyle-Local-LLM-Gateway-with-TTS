package core

import (
	"context"
	"encoding/json"
	"net"
	"regexp"
	"runtime"
	"strings"

	"dinkisstyle-chat/internal/chatharness"
	"dinkisstyle-chat/internal/toolruntime"
)

var bulkToolScope = regexp.MustCompile(`(?i)^(?:(?:모든|전체)\s*도구(?:를|요)?|도구\s*(?:모두|전부)|전부(?:요)?|모두(?:요)?|all(?: (?:of them|tools))?|every tool)[.!?\s]*$`)
var diagnosticInstructionQuestion = regexp.MustCompile(`(?i)(?:^\s*(?:how|what|why)\b|\b(?:do not|don't)\b|방법|어떻게|하지\s*마|테스트\s*안)`)
var pingRequest = regexp.MustCompile(`(?i)^(?:(\S+?)(?:에|로)?\s+)?(?:ping|핑)(?:\s*(?:test|테스트))?(?:\s*(?:해\s*(?:봐|줘|주세요)|실행))?(?:\s+(\S+))?$`)
var pingHostname = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

func isBulkDiagnostic(text, previousUserText string) bool {
	if diagnosticInstructionQuestion.MatchString(text) {
		return false
	}
	return chatharness.IsBulkToolTestRequest(text) ||
		(bulkToolScope.MatchString(strings.TrimSpace(text)) && !diagnosticInstructionQuestion.MatchString(previousUserText) &&
			chatharness.IsBulkToolTestRequest(previousUserText))
}

func previousDiagnosticUserText(req map[string]interface{}) string {
	for _, field := range []string{"messages", "input"} {
		messages, _ := req[field].([]interface{})
		foundCurrent := false
		for i := len(messages) - 1; i >= 0; i-- {
			message, _ := messages[i].(map[string]interface{})
			if message["role"] != "user" {
				continue
			}
			if foundCurrent {
				text, _ := message["content"].(string)
				return text
			}
			foundCurrent = true
		}
	}
	return ""
}

func diagnosticPingTarget(text string) string {
	match := pingRequest.FindStringSubmatch(strings.TrimRight(strings.TrimSpace(text), ".!?"))
	if match == nil || (match[1] != "" && match[2] != "") {
		return ""
	}
	target := strings.ToLower(match[1] + match[2])
	switch target {
	case "", "구글", "google":
		return "google.com"
	}
	if len(target) <= 253 && (pingHostname.MatchString(target) || net.ParseIP(target).To4() != nil) {
		return target
	}
	return ""
}

type diagnosticToolEvents func(kind, tool, callID string, payload map[string]interface{})

func callDiagnosticTool(ctx context.Context, registry *toolruntime.Registry, execCtx toolruntime.ExecutionContext, emit diagnosticToolEvents, name, args string) (toolruntime.Result, string) {
	callID := "diagnostic_" + name
	emit("tool_call.start", name, callID, map[string]interface{}{})
	emit("tool_call.arguments", name, callID, map[string]interface{}{"arguments": json.RawMessage(args)})
	result, err := registry.Call(ctx, execCtx, name, json.RawMessage(args))
	if err != nil || result.IsError || strings.TrimSpace(result.Content) == "" {
		reason := strings.TrimSpace(result.Content)
		if err != nil {
			reason = err.Error()
		}
		if reason == "" {
			reason = "도구가 빈 결과를 반환했습니다."
		}
		emit("tool_call.failure", name, callID, map[string]interface{}{"reason": compactText(reason, 240)})
		return result, reason
	}
	emit("tool_call.success", name, callID, map[string]interface{}{})
	return result, ""
}

func runPingDiagnostic(ctx context.Context, registry *toolruntime.Registry, execCtx toolruntime.ExecutionContext, emit diagnosticToolEvents, target string) string {
	count := "-c 3"
	switch runtime.GOOS {
	case "windows":
		count = "-n 3 -w 1000"
	case "darwin":
		count += " -W 1000"
	default:
		count += " -W 1 -w 5"
	}
	args, _ := json.Marshal(map[string]string{"command": "ping " + count + " " + target})
	result, reason := callDiagnosticTool(ctx, registry, execCtx, emit, "execute_command", string(args))
	answer := "Gateway 호스트에서 `" + target + "`로 보낸 핑 결과입니다."
	if reason != "" {
		answer = "`" + target + "` 핑 테스트 실패 · " + reason
	}
	if output := strings.TrimSpace(result.Content); output != "" {
		answer += "\n\n```text\n" + output + "\n```"
	}
	return answer
}

func runBulkToolDiagnostic(ctx context.Context, registry *toolruntime.Registry, execCtx toolruntime.ExecutionContext, emit diagnosticToolEvents) string {
	// Fixed probes exercise real handlers without relying on model-generated
	// arguments, account data identifiers, or desktop input.
	probes := map[string]string{
		"get_current_time":       `{}`,
		"get_current_location":   `{}`,
		"search_memory":          `{"query":"DKST_TOOL_DIAGNOSTIC"}`,
		"gmail_search":           `{"query":"subject:DKST_TOOL_DIAGNOSTIC","limit":1}`,
		"google_drive_search":    `{"query":"DKST_TOOL_DIAGNOSTIC","limit":1}`,
		"google_calendar_list":   `{}`,
		"google_calendar_search": `{"query":"DKST_TOOL_DIAGNOSTIC","limit":1}`,
		"read_help":              `{"query":"Gateway"}`,
		"search_web":             `{"query":"IANA example domains"}`,
		"search_web_multi":       `{"queries":["IANA example domains","RFC 2606 reserved domain names"]}`,
		"read_web_page":          `{"url":"https://example.com"}`,
		"naver_search":           `{"query":"훈민정음 국립한글박물관"}`,
		"namu_wiki":              `{"keyword":"훈민정음"}`,
		"computer_list_apps":     `{}`,
		"execute_command":        `{"command":"echo DKST_TOOL_DIAGNOSTIC"}`,
	}
	var lines []string
	for _, definition := range registry.List(execCtx) {
		if ctx.Err() != nil {
			break
		}
		prefix := "- `" + definition.Name + "` · "
		args, ok := probes[definition.Name]
		if !ok {
			reason := "전용 테스트 입력 필요"
			if definition.Metadata.SideEffecting {
				reason = "실제 데이터 또는 앱 상태를 변경할 수 있음"
			}
			lines = append(lines, prefix+"건너뜀 · "+reason)
			continue
		}
		_, reason := callDiagnosticTool(ctx, registry, execCtx, emit, definition.Name, args)
		if reason != "" {
			lines = append(lines, prefix+"실패 · "+compactText(reason, 180))
		} else {
			lines = append(lines, prefix+"응답 확인")
		}
	}
	if len(lines) == 0 {
		return "현재 계정에서 진단할 수 있는 도구가 없습니다."
	}
	return "Gateway 도구 진단 결과입니다.\n\n" + strings.Join(lines, "\n")
}
