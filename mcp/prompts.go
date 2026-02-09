package mcp

import (
	"fmt"
	"time"
)

// SystemPromptToolUsage returns the guidelines for tool usage to be injected into the system prompt.
func SystemPromptToolUsage() string {
	return fmt.Sprintf("\n\n### TOOL CALL GUIDELINES ###\n"+
		"1. Use a SINGLE valid <tool_call> block for tool requests.\n"+
		"2. DO NOT use search_web or read_web_page for person identification or image description unless explicitly asked.\n"+
		"3. CURRENT_TIME: %s", time.Now().Format("2006-01-02 15:04:05 Monday"))
}

// SystemPromptMemoryTemplate returns the template for injecting user memory.
// It expects the preloaded memory string as an argument.
func SystemPromptMemoryTemplate(preloadedMemory string) string {
	if preloadedMemory != "" {
		return fmt.Sprintf("\n\n=== USER'S SAVED MEMORY ===\n%s\n=== END OF MEMORY ===\n"+
			"NOTE: Memory is saved AUTOMATICALLY after the session. Do NOT attempt to save facts manually via tools.", preloadedMemory)
	}
	return "\n- USER_MEMORY: Empty. Memory is saved AUTOMATICALLY after the session. Do NOT attempt to save facts manually."
}

// EvolutionPromptTemplate returns the prompt used for self-evolution (regex generation).
// It expects the sample line that failed parsing.
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
func MemoryConsolidationPromptTemplate(currentMemory string) string {
	return fmt.Sprintf(`You are a Memory Optimization Agent.
The user's long-term memory file has grown and needs consolidation.
Your task is to:
1. Remove duplicate facts.
2. Resolve conflicting information (keep the most specific or likely recent one).
3. Group facts into logical sections (e.g., ## Personal, ## Preferences, ## History).
4. Summarize verbose descriptions into concise facts.

CURRENT MEMORY CONTENT:
%s

OUTPUT FORMAT:
- Output valid Markdown with headers (## Section Name).
- Use bullet points for facts.
- Do not include any conversational filler (e.g., "Here is the consolidated memory").`, currentMemory)
}

// ChatSummaryPromptTemplate returns the prompt used to summarize a conversation session.
func ChatSummaryPromptTemplate(conversationText string) string {
	return fmt.Sprintf(`You are a memory assistant.
Summarize the following conversation session into a concise list of key points for future reference.
Focus on:
- Main topics discussed
- Decisions made or tasks completed
- Specific user preferences or constraints mentioned
- Any important context that should be remembered

Conversation:
%s

OUTPUT FORMAT:
- Output a simple list of bullet points.
- Do not include headers like "Here is the summary".
- If nothing significant happened (just greetings), output: "NO_IMPORTANT_CONTENT"`, conversationText)
}
