package promptkit

import (
	"fmt"
	"strings"
)

func truncateRunes(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// SelfCorrectionPromptTemplate returns the prompt to ask the model to fix its
// stateful XML tool call format. Prefer SelfCorrectionPromptForMode when the
// transport mode is known.
func SelfCorrectionPromptTemplate(badContent string) string {
	return SelfCorrectionPromptForMode(badContent, "stateful", "")
}

// SelfCorrectionPromptForMode tailors tool-call repair instructions to the active
// LLM transport mode (OpenAI-compatible native function calling vs LM Studio stateful XML).
func SelfCorrectionPromptForMode(badContent, llmMode, modelFamily string) string {
	snippet := truncateRunes(badContent, 400)
	if strings.EqualFold(strings.TrimSpace(llmMode), "standard") {
		extra := ""
		if modelFamily == "gemma4" || modelFamily == "gemma3" {
			extra = "\nDo not output Python-style calls, key: value lines, or <tool_name> XML tags in assistant text."
		}
		return fmt.Sprintf(`Your previous tool call could not be parsed.
Invoke the registered tool using the provider's native function-calling mechanism (tool_calls) with a valid JSON object matching its schema.%s
Do not print tool syntax, XML tags, or markdown code blocks in assistant content.
If no tool is needed to answer the user, reply directly in plain text.

Tool reminders:
- read_buffered_source uses {"source_id":"...","query":"..."}.
- search_memory uses {"query":"..."}.
- read_memory uses {"memory_id":123}.
- read_memory_context uses {"memory_id":123,"chunk_index":0}.
- Never use source_id, query, or question with read_memory or read_memory_context.

Malformed output:
%s`, extra, snippet)
	}

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
<search_web>{"query":"current weather for the requested location"}</search_web>
`, snippet)
}

// StructuredJSONRepairPrompt asks the model to repair a malformed or truncated
// structured JSON response for out-of-chat control/auxiliary phases.
func StructuredJSONRepairPrompt(phase, parseErr, rawOutput string, schemaJSON []byte) string {
	return fmt.Sprintf(`Your previous response for phase %q failed JSON parsing or schema validation.
Error: %s

Return ONLY one complete, valid JSON object matching the required schema below.
Do not include explanations, markdown fences, or extra text before or after the JSON object.

Required JSON schema:
%s

Previous invalid output:
%s`, strings.TrimSpace(phase), truncateRunes(parseErr, 300), truncateRunes(string(schemaJSON), 4000), truncateRunes(rawOutput, 1200))
}
