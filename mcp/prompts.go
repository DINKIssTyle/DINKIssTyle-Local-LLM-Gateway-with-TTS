package mcp

import (
	"fmt"
	"time"
)

// SystemPromptToolUsage returns the guidelines for tool usage to be injected into the system prompt.
// SystemPromptToolUsage: server.go에서 모델에게 도구 사용 가이드라인(TOOL CALL GUIDELINES)을 제공할 때 사용됩니다.
func SystemPromptToolUsage(envInfo string) string {
	prompt := fmt.Sprintf("\n\n### TOOL CALL GUIDELINES ###\n"+
		"1. For any tool use, output exactly one valid <tool_call> block.\n"+
		"2. If no tool is needed, answer normally.\n"+
		"3. Avoid search_web or read_web_page for person identification or image description unless explicitly asked.\n"+
		"4. Web-reading tools may return a buffered source handle instead of the full text to save context.\n"+
		"5. After search_web, read_web_page, naver_search, or namu_wiki, call read_buffered_source with source_id and the user's actual question when you need detailed evidence.\n"+
		"6. If read_buffered_source omits source_id, it will use the most recent buffered source for this user.\n"+
		"7. Avoid repeating the same search_web or read_web_page call with near-identical inputs in one answer, but one refined follow-up search is acceptable if it materially improves evidence quality.\n"+
		"8. If read_web_page fails or times out, do not retry the exact same page immediately. Prefer answering from the buffered search evidence, or read a different relevant source if that would clearly improve quality.\n"+
		"9. CURRENT_TIME: %s", time.Now().Format("2006-01-02 15:04:05 Monday"))

	if envInfo != "" {
		prompt += fmt.Sprintf("\n10. ENVIRONMENT INFO:\n%s", envInfo)
	}

	return prompt
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
