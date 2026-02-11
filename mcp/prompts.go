package mcp

import (
	"fmt"
	"time"
)

// SystemPromptToolUsage returns the guidelines for tool usage to be injected into the system prompt.
// SystemPromptToolUsage: server.go에서 모델에게 도구 사용 가이드라인(TOOL CALL GUIDELINES)을 제공할 때 사용됩니다.
func SystemPromptToolUsage(envInfo string) string {
	prompt := fmt.Sprintf("\n\n### TOOL CALL GUIDELINES ###\n"+
		"1. Use a SINGLE valid <tool_call> block for tool requests.\n"+
		"2. DO NOT use search_web or read_web_page for person identification or image description unless explicitly asked.\n"+
		"3. CURRENT_TIME: %s", time.Now().Format("2006-01-02 15:04:05 Monday"))

	if envInfo != "" {
		prompt += fmt.Sprintf("\n4. ENVIRONMENT INFO:\n%s", envInfo)
	}

	return prompt
}

// SystemPromptMemoryTemplate returns the template for injecting user memory.
// It expects the preloaded memory string as an argument.
// SystemPromptMemoryTemplate: server.go에서 사용자의 기존 메모리(Personal/Static Memory)를 채팅 컨텍스트에 주입할 때 사용됩니다.
func SystemPromptMemoryTemplate(preloadedMemory string) string {
	if preloadedMemory != "" {
		return fmt.Sprintf("\n\n### USER'S SAVED MEMORY ###\n%s\n### END OF MEMORY ###\n"+
			"NOTE: Memory is saved AUTOMATICALLY after the session. Do NOT attempt to save facts manually via tools.\n"+
			"CRITICAL: [Static Memory] contains verified, immutable facts. If [Personal Memory] conflicts with [Static Memory], ALWAYS trust [Static Memory] as the absolute truth.", preloadedMemory)
	}
	return "\n- USER_MEMORY: Empty. Memory is saved AUTOMATICALLY after the session. Do NOT attempt to save facts manually."
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

// MemoryConsolidationPromptTemplate creates a prompt to organize and dedup memory.
// MemoryConsolidationPromptTemplate: memory_agent.go에서 비대해진 메모리 파일을 정리하고 중복을 제거할 때 사용됩니다.
func MemoryConsolidationPromptTemplate(currentMemory string) string {
	return fmt.Sprintf(`You are a Memory Optimization Agent.
The user's long-term memory file has grown and needs consolidation.
Your task is to:
1. Remove duplicate facts and redundant information.
2. Resolve conflicting information (keep the most specific or likely recent one).
3. Group facts into logical sections (e.g., ## Personal, ## Preferences, ## Domain Knowledge).
4. Summarize verbose descriptions into concise atomic facts.
5. CLEANUP: Remove transient context (CWD, IPs), mundane action logs ("User asked for..."), and any entries that do not contain meaningful long-term facts about the user.
6. REMOVE empty sections or sections with only transient/junk data.

CURRENT MEMORY CONTENT:
%s

OUTPUT FORMAT:
- Output valid Markdown with headers (## Section Name).
- Use bullet points for facts.
- Do not include any conversational filler (e.g., "Here is the consolidated memory").`, currentMemory)
}

// ChatSummaryPromptTemplate returns the prompt used to summarize a conversation session.
// ChatSummaryPromptTemplate: memory_agent.go에서 대화 세션을 요약하여 핵심 내용을 추출할 때 사용됩니다.
func ChatSummaryPromptTemplate(conversationText string) string {
	return fmt.Sprintf(`You are a Long-term Memory Extraction Agent.
Analyze the following conversation and extract ONLY NEW and SIGNIFICANT facts about the user for long-term storage.

STRICT GUIDELINES:
1. Extract atomic facts (e.g., "User likes Python", "User lives in Seoul").
2. DO NOT include transient session context (Current working directory, IP, process IDs).
3. DO NOT include mundane action logs (e.g., "User asked for a profile", "Assistant provided a path").
4. DO NOT include information that is already common knowledge (e.g., facts about public figures found in searches).
5. If NO new personal facts, preferences, or meaningful context are found, output exactly: "NO_IMPORTANT_CONTENT"

Conversation:
%s

OUTPUT FORMAT:
- A simple list of bullet points for NEW facts only.
- No headers, no intro/outro filler.`, conversationText)
}

// SelfCorrectionPromptTemplate returns the prompt to ask the model to fix its tool call format.
// SelfCorrectionPromptTemplate: server.go에서 모델이 잘못된 도구 호출 형식을 사용했을 때, 즉시 교정 요청을 보낼 때 사용됩니다.
func SelfCorrectionPromptTemplate(badContent string) string {
	return fmt.Sprintf(`SYSTEM ALERT: INVALID TOOL CALL FORMAT DETECTED.
Your previous response tried to use a tool but failed the syntax check.
We detected: "%s..."

You MUST correct this immediately.

❌ WRONG FORMAT (DO NOT USE):
<tool_call>
name: search_web
query: something
</tool_call>
(No Markdown code blocks like `+"`"+``+"`"+`json or `+"`"+``+"`"+`xml)

✅ CORRECT FORMAT (XML + ONE LINER JSON):
<tool_call>{"name": "search_web", "arguments": {"query": "weather in Seoul"}}</tool_call>

INSTRUCTIONS:
1. Output ONLY the corrected <tool_call> block.
2. Do not apologize or explain.
3. Ensure the JSON inside is valid and minified (no distinct newlines inside JSON if possible).

REWRITE THE TOOL CALL NOW:`, badContent[:min(len(badContent), 100)])
}

// mcp/prompts.go 내부에서 에러 메시지 길이를 제한하기 위해 사용되는 유틸리티 함수입니다.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
