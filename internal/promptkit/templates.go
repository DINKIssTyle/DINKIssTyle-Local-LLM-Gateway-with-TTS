package promptkit

import "fmt"

// SelfCorrectionPromptTemplate returns the prompt to ask the model to fix its tool call format.
func SelfCorrectionPromptTemplate(badContent string) string {
	return fmt.Sprintf(`
Return only one valid tool-specific XML element.
Do not explain anything.
Do not include markdown.
Use the registered tool name for both tags and strict JSON for the body.
General form: <tool_name>{...}</tool_name>
Do not use Python/function syntax such as tool_name(key="value").

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
<search_web>{"query":"weather in Seoul"}</search_web>
`, badContent[:min(len(badContent), 100)])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
