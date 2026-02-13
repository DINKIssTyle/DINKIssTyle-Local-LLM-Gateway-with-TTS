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

// SystemPromptMemoryTemplate returns the template for injecting user memory using the 3-layer model.
// SystemPromptMemoryTemplate: 채팅 컨텍스트에 3계층 메모리 구조를 주입합니다.
func SystemPromptMemoryTemplate(staticMemory string, userProfile string, activeContext string) string {
	return fmt.Sprintf(`
### MEMORY CONTEXT ###

#### STATIC MEMORY (Highest Authority)
%s

#### USER PROFILE (Persistent Long-Term Memory)
%s

#### ACTIVE CONTEXT (Recent, Temporary)
%s

MEMORY PRIORITY ORDER:
1. Static Memory
2. User Profile
3. Current Conversation

RULES:
- Static Memory overrides everything.
- Do NOT modify or reinterpret Static Memory.
- Do NOT generate any tool call to save memory.
- Memory persistence is handled outside the model.
- Any attempt to manually save memory is considered an error.
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

// MemoryValidationPromptTemplate creates a prompt to validate candidate memories.
// MemoryValidationPromptTemplate: 추출된 메모리 후보 중 장기 보관할 가치가 있는 항목만 검증합니다.
func MemoryValidationPromptTemplate(candidateMemory string) string {
	return fmt.Sprintf(`
You are a Memory Validation Agent.

Review the following candidate memory entries:

%s

Remove anything that is:
- Ephemeral
- Repeated
- Session-bound
- Assumption-based
- Speculative

Keep ONLY:
- Stable long-term traits
- Verified persistent user facts

If nothing remains, output:
NONE

Output bullet list only.
`, candidateMemory)
}

// MemoryConsolidationPromptTemplate creates a prompt to organize and dedup memory.
// MemoryConsolidationPromptTemplate: 메모리 파일을 정리하고 충돌을 해결합니다.
func MemoryConsolidationPromptTemplate(currentMemory string) string {
	return fmt.Sprintf(`
You are a Memory Optimization Agent.

Your task:

1. Remove duplicates.
2. If conflict:
   - Prefer more specific fact.
   - If timestamp exists → prefer newer.
   - If unsure → keep both and mark conflict.
   - NEVER invent new facts.
3. Preserve technical specificity.
4. Do NOT remove domain names, versions, technologies.
5. Remove:
   - Transient logs
   - CWD
   - IP
   - Debug context
   - Action descriptions
6. Remove empty sections.

OUTPUT:
- Markdown
- ## Section headers
- Bullet points only
- No filler text

CURRENT MEMORY:
%s
`, currentMemory)
}

// ChatSummaryPromptTemplate returns the prompt used to summarize a conversation session.
// ChatSummaryPromptTemplate: 대화 세션에서 새로운 장기 기억 항목을 추출합니다.
func ChatSummaryPromptTemplate(conversationText string) string {
	return fmt.Sprintf(`
You are a Long-term Memory Extraction Agent.

Extract ONLY new, stable, and long-term user facts.

STRICT RULES:

1. Extract atomic, stable traits only.
2. DO NOT extract temporary goals.
3. DO NOT extract debugging issues.
4. DO NOT extract one-time problems.
5. DO NOT extract session-specific context.
6. DO NOT extract information already stored.
7. DO NOT infer or speculate.
8. If unsure whether a fact is long-term → DO NOT include it.

Only include:
- Stable preferences
- Ongoing projects
- Long-term technical domains
- Repeated behavioral patterns

If nothing qualifies → output exactly:

NO_IMPORTANT_CONTENT

OUTPUT:
- Bullet list only
- No explanations

CONVERSATION:
%s
`, conversationText)
}

// SelfCorrectionPromptTemplate returns the prompt to ask the model to fix its tool call format.
// SelfCorrectionPromptTemplate: 도구 호출 형식 오류 시 즉각 수정을 요청합니다.
func SelfCorrectionPromptTemplate(badContent string) string {
	return fmt.Sprintf(`
SYSTEM ALERT: INVALID TOOL CALL FORMAT DETECTED.

You MUST correct this immediately.

If you output anything other than a single <tool_call> block, the request will fail.

❌ WRONG:
<tool_call>
name: search_web
query: test
</tool_call>

✅ CORRECT:
<tool_call>{"name":"search_web","arguments":{"query":"weather in Seoul"}}</tool_call>

Output ONLY the corrected <tool_call> block.
Do not apologize.

DETECTED CONTENT:
%s
`, badContent[:min(len(badContent), 100)])
}

// mcp/prompts.go 내부에서 에러 메시지 길이를 제한하기 위해 사용되는 유틸리티 함수입니다.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
