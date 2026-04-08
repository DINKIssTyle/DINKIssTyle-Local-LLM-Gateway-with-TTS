package promptkit

import "fmt"

// EvolutionPromptTemplate returns the prompt used for self-evolution (regex generation).
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
func SelfCorrectionPromptTemplate(badContent string) string {
	return fmt.Sprintf(`
Return only one valid <tool_call> block.
Do not explain anything.
Do not include markdown.
Use strict JSON for arguments.

Tool reminders:
- read_buffered_source uses {"source_id":"...","query":"..."}.
- search_memory uses {"query":"..."}.
- read_memory uses {"memory_id":123}.
- read_memory_context uses {"memory_id":123,"chunk_index":0}.
- Never use source_id, query, or question with read_memory or read_memory_context.
- Never call execute_command to simulate search_memory, search_web, read_memory, read_memory_context, read_web_page, or read_buffered_source.
- If the user explicitly asked to search memory, do not skip search_memory.
- If search_memory is not enough and the remaining question is factual/public, search the web next.

Malformed output:
%s

Valid example:
<tool_call>{"name":"search_web","arguments":{"query":"weather in Seoul"}}</tool_call>
`, badContent[:min(len(badContent), 100)])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
