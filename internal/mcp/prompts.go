package mcp

import (
	"fmt"
	"strings"
	"time"
)

// SystemPromptToolUsage returns the guidelines for tool usage to be injected into the system prompt.
// SystemPromptToolUsage: server.go에서 모델에게 도구 사용 가이드라인(TOOL CALL GUIDELINES)을 제공할 때 사용됩니다.
func SystemPromptToolUsage(envInfo string, modelID string, useNativeIntegrations bool) string {
	lowerModelID := strings.ToLower(strings.TrimSpace(modelID))
	lines := []string{"", "", "### TOOL CALL GUIDELINES ###"}

	if useNativeIntegrations {
		lines = append(lines,
			"1. If a tool is needed, use the provider's native tool-calling integration instead of printing a textual tool call.",
			"2. Do not output XML-like wrappers or pseudo-schemas such as <tool_call>, </tool_call>, <remark>, </remark>, <think>, or JSON meant only for the tool parser.",
			"3. If no tool is needed, answer normally in plain text.",
			"4. Call at most one tool at a time. After each tool result, either answer the user directly or make one clearly necessary next tool call.",
			"5. Avoid search_web or read_web_page for person identification or image description unless explicitly asked.",
			"6. Web-reading tools may return a buffered source handle instead of the full text to save context.",
			"7. After search_web, read_web_page, naver_search, or namu_wiki, call read_buffered_source with source_id and the user's actual question when you need detailed evidence.",
			"8. If read_buffered_source omits source_id, it will use the most recent buffered source for this user.",
			"9. Avoid repeating the same search_web or read_web_page call with near-identical inputs in one answer, but one refined follow-up search is acceptable if it materially improves evidence quality.",
			"10. If read_web_page fails or times out, do not retry the exact same page immediately. Prefer answering from the buffered search evidence, or read a different relevant source if that would clearly improve quality.",
			"11. For execute_command, use the provided ENVIRONMENT INFO to choose OS-appropriate commands. Do not call execute_command only to discover the OS or shell when ENVIRONMENT INFO already tells you.",
			"12. After execute_command returns enough information, answer the user directly. Do not repeat the same or near-identical command in the same answer unless the user explicitly asked to re-run or refresh it.",
		)
	} else {
		lines = append(lines,
			"1. For any tool use, output exactly one valid <tool_call> block.",
			"2. If no tool is needed, answer normally.",
			"3. Avoid search_web or read_web_page for person identification or image description unless explicitly asked.",
			"4. Web-reading tools may return a buffered source handle instead of the full text to save context.",
			"5. After search_web, read_web_page, naver_search, or namu_wiki, call read_buffered_source with source_id and the user's actual question when you need detailed evidence.",
			"6. If read_buffered_source omits source_id, it will use the most recent buffered source for this user.",
			"7. Avoid repeating the same search_web or read_web_page call with near-identical inputs in one answer, but one refined follow-up search is acceptable if it materially improves evidence quality.",
			"8. If read_web_page fails or times out, do not retry the exact same page immediately. Prefer answering from the buffered search evidence, or read a different relevant source if that would clearly improve quality.",
			"9. For execute_command, use the provided ENVIRONMENT INFO to choose OS-appropriate commands. Do not call execute_command only to discover the OS or shell when ENVIRONMENT INFO already tells you.",
			"10. After execute_command returns enough information, answer the user directly. Do not repeat the same or near-identical command in the same answer unless the user explicitly asked to re-run or refresh it.",
		)
	}

	if strings.Contains(lowerModelID, "gemma-4") {
		lines = append(lines,
			"13. GEMMA-4 RULE: Prefer the smallest useful number of tool calls.",
			"14. GEMMA-4 RULE: For memory, path, time, or other simple system checks, make one tool call first and wait for the result before deciding on any second tool call.",
			"15. GEMMA-4 RULE: Once a tool result already answers the user's request well enough, stop calling tools and answer immediately.",
		)
	}

	lines = append(lines, fmt.Sprintf("16. CURRENT_TIME: %s", time.Now().Format("2006-01-02 15:04:05 Monday")))

	if envInfo != "" {
		lines = append(lines, "17. ENVIRONMENT INFO:", strings.TrimRight(envInfo, "\n"))
	}

	return strings.Join(lines, "\n")
}

// SystemPromptMemoryTemplate returns the template for injecting user memory using the 3-layer model.
// SystemPromptMemoryTemplate: 채팅 컨텍스트에 3계층 메모리 구조를 주입합니다.
func SystemPromptMemoryTemplate(staticMemory string, userProfile string, activeContext string) string {
	return fmt.Sprintf(`
### MEMORY CONTEXT ###

#### STATIC MEMORY
%s

#### USER PROFILE
%s

#### ACTIVE CONTEXT
%s

MEMORY & SEARCH RULES:
1. Treat USER PROFILE as summary only.
2. If past details are missing or uncertain, use 'search_memory'.
3. After 'search_memory', call 'read_memory' for the most relevant result before relying on it.
4. Try alternative names, relationships, or synonyms if the first search fails.
5. Do not guess past details.
6. Do not create tool calls to save memory; saving happens automatically.
`, staticMemory, userProfile, activeContext)
}

// EvolutionPromptTemplate returns the prompt used for self-evolution (regex generation).
// It expects the sample line that failed parsing.
// EvolutionPromptTemplate: evolution.go에서 새로운 도구 호출 패턴을 학습하기 위한 정규식 생성용 프롬프트로 사용됩니다.
func EvolutionPromptTemplate(sampleLine string) string {
	return fmt.Sprintf(`You are an expert at Go Regular Expressions and LLM Tool Calling patterns.
I have a log from an LLM that appears to be a tool call, but my current parser missed it.
The sample content is: "%s"

Please generate a single Go-compatible Regular Expression (regexp) to capture:
- Group 1: The Tool Name (e.g., search_web, personal_memory)
- Group 2: The JSON Arguments or parameters block.

REQUIREMENTS:
1. Return ONLY the regex string. Do not wrap in markdown or code blocks.
2. The regex must be robust (use (?s) if it spans multiple lines).
3. If no tool call found, return "NONE".`, sampleLine)
}

// SelfCorrectionPromptTemplate returns the prompt to ask the model to fix its tool call format.
// SelfCorrectionPromptTemplate: 도구 호출 형식 오류 시 즉각 수정을 요청합니다.
func SelfCorrectionPromptTemplate(badContent string) string {
	return fmt.Sprintf(`
Return only one valid <tool_call> block.
Do not explain anything.
Do not include markdown.

Malformed output:
%s

Valid example:
<tool_call>{"name":"search_web","arguments":{"query":"weather in Seoul"}}</tool_call>
`, badContent[:min(len(badContent), 100)])
}

// mcp/prompts.go 내부에서 에러 메시지 길이를 제한하기 위해 사용되는 유틸리티 함수입니다.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
