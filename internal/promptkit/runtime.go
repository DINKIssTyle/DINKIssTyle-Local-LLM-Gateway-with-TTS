package promptkit

import (
	"fmt"
	"strings"
	"time"
)

const toolGuidelineMarker = "### TOOL CALL GUIDELINES ###"

type RuntimeInstructionsInput struct {
	EnvironmentInfo       string
	ModelID               string
	UseNativeIntegrations bool
	RecentContext         string
	MemorySnapshot        string
	ActiveContext         string
	RetrievalInjected     bool
	UserProfileFacts      string
}

func ToolGuidelineMarker() string {
	return toolGuidelineMarker
}

func BuildRuntimeInstructions(input RuntimeInstructionsInput) string {
	extraInstr := ""
	if input.UseNativeIntegrations {
		extraInstr = buildToolUsage(input.EnvironmentInfo, input.ModelID, input.UseNativeIntegrations)
	}
	if input.RecentContext != "" || input.MemorySnapshot != "" || input.ActiveContext != "" || input.UserProfileFacts != "" {
		if input.UseNativeIntegrations {
			extraInstr += buildMemoryTemplate("", input.RecentContext, input.MemorySnapshot, input.ActiveContext, input.RetrievalInjected, input.UserProfileFacts)
		} else {
			extraInstr += buildPassiveMemoryTemplate(input.RecentContext, input.MemorySnapshot, input.ActiveContext, input.UserProfileFacts)
		}
	}
	return extraInstr
}

func InjectPrompt(reqMap map[string]interface{}, extraInstr string) bool {
	if len(reqMap) == 0 || strings.TrimSpace(extraInstr) == "" {
		return false
	}

	foundSystem := false
	if messages, ok := reqMap["messages"].([]interface{}); ok {
		messages = truncateMessages(messages)
		reqMap["messages"] = messages

		for i, msg := range messages {
			m, ok := msg.(map[string]interface{})
			if !ok {
				continue
			}
			role, _ := m["role"].(string)
			if role != "system" {
				continue
			}
			content, _ := m["content"].(string)
			if !strings.Contains(content, toolGuidelineMarker) {
				m["content"] = content + extraInstr
				messages[i] = m
			}
			foundSystem = true
			break
		}
		if !foundSystem {
			newMsg := map[string]interface{}{
				"role":    "system",
				"content": "You are a helpful assistant." + extraInstr,
			}
			reqMap["messages"] = append([]interface{}{newMsg}, messages...)
			foundSystem = true
		}
	}

	if sp, ok := reqMap["system_prompt"].(string); ok {
		if !strings.Contains(sp, toolGuidelineMarker) {
			reqMap["system_prompt"] = sp + extraInstr
		}
		foundSystem = true
	}

	return foundSystem
}

func truncateMessages(messages []interface{}) []interface{} {
	const maxIndividualLen = 10000
	const maxTotalChars = 15000
	const maxCount = 10

	for i, msg := range messages {
		m, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		content, ok := m["content"].(string)
		if !ok || len(content) <= maxIndividualLen {
			continue
		}
		m["content"] = content[:maxIndividualLen] + "\n... (content truncated for context optimization)"
		messages[i] = m
	}

	currentTotal := 0
	var truncated []interface{}
	systemIndex := -1

	if len(messages) > 0 {
		if m, ok := messages[0].(map[string]interface{}); ok {
			if role, ok := m["role"].(string); ok && role == "system" {
				systemIndex = 0
				if content, ok := m["content"].(string); ok {
					currentTotal += len(content)
				}
			}
		}
	}

	for i := len(messages) - 1; i >= 0; i-- {
		if i == systemIndex {
			continue
		}
		msg := messages[i]
		m, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		content, ok := m["content"].(string)
		if !ok {
			continue
		}
		if currentTotal+len(content) > maxTotalChars || len(truncated) >= maxCount {
			break
		}
		currentTotal += len(content)
		truncated = append([]interface{}{msg}, truncated...)
	}

	if systemIndex >= 0 {
		truncated = append([]interface{}{messages[systemIndex]}, truncated...)
	}

	return truncated
}

func buildToolUsage(envInfo string, modelID string, useNativeIntegrations bool) string {
	lowerModelID := strings.ToLower(strings.TrimSpace(modelID))
	lines := []string{"", "", toolGuidelineMarker}

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
			"12. Never use execute_command to imitate built-in tools such as search_memory, search_web, read_memory, read_memory_context, read_web_page, or read_buffered_source. Call the real tool directly.",
			"13. After execute_command returns enough information, answer the user directly. Do not repeat the same or near-identical command in the same answer unless the user explicitly asked to re-run or refresh it.",
			"14. MEMORY-THEN-WEB RULE: If the user asks about prior chats, personal facts, preferences, or earlier reasons, search memory first. If memory is insufficient and the question is still a factual/public information question, then search the web.",
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
			"10. Never use execute_command to imitate built-in tools such as search_memory, search_web, read_memory, read_memory_context, read_web_page, or read_buffered_source. Call the real tool directly.",
			"11. After execute_command returns enough information, answer the user directly. Do not repeat the same or near-identical command in the same answer unless the user explicitly asked to re-run or refresh it.",
			"12. MEMORY-THEN-WEB RULE: If the user asks about prior chats, personal facts, preferences, or earlier reasons, search memory first. If memory is insufficient and the question is still a factual/public information question, then search the web.",
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

func buildMemoryTemplate(staticMemory string, recentContext string, userProfile string, activeContext string, retrievalInjected bool, userProfileFacts string) string {
	// If we have structured profile facts, prepend them to the USER PROFILE section
	combinedProfile := ""
	if strings.TrimSpace(userProfileFacts) != "" {
		combinedProfile = "## Known Facts (always available, no search needed):\n" + userProfileFacts
		if strings.TrimSpace(userProfile) != "" {
			combinedProfile += "\n\n## Recent Memory Snapshot:\n" + userProfile
		}
	} else {
		combinedProfile = userProfile
	}

	rules := []string{
		"1. Treat RECENT CONTEXT as the primary source for continuity about the latest few turns.",
		"2. Treat USER PROFILE Known Facts as ground truth for personal details about the user. These never require search_memory.",
		"3. If the user explicitly asks you to search memory, recall prior chats, or find what was said before, you MUST use 'search_memory' before answering.",
		"4. If past details are missing or uncertain, use 'search_memory' instead of saying you do not know.",
		"5. After 'search_memory', prefer 'read_memory_context' for the best candidate before relying on it. Use 'read_memory' only when you need the full original text.",
		"6. 'read_memory' and 'read_memory_context' require 'memory_id'. Never use 'source_id', 'query', or 'question' with memory tools.",
		"7. If memory search still does not answer the question and the remaining question is about factual/public knowledge, use web search next instead of stopping at 'I do not know'.",
		"8. Only 'read_buffered_source' uses 'source_id' for web evidence.",
		"9. Try alternative names, relationships, or synonyms if the first search fails.",
		"10. Do not guess past details.",
		"11. When the user tells you personal facts (name, birthday, preferences, etc.), proactively use 'save_user_fact' to save them to their permanent profile.",
	}
	if retrievalInjected && strings.TrimSpace(activeContext) != "" {
		rules = []string{
			"1. Treat RECENT CONTEXT as the primary source for continuity about the latest few turns.",
			"2. ACTIVE CONTEXT was already retrieved for this turn. Prefer answering from RECENT CONTEXT plus ACTIVE CONTEXT when they are sufficient.",
			"3. Treat USER PROFILE Known Facts as ground truth for personal details about the user. These never require search_memory.",
			"4. If the user explicitly asks you to search memory, recall prior chats, or find what was said before, you MUST use 'search_memory' before answering.",
			"5. Use 'search_memory' whenever RECENT CONTEXT and ACTIVE CONTEXT are clearly insufficient or contradictory. Do not simply say you do not know without trying memory search first.",
			"6. If you must inspect memory further, prefer 'read_memory_context' after 'search_memory'. Use 'read_memory' only for the full original text.",
			"7. 'read_memory' and 'read_memory_context' require 'memory_id'. Never use 'source_id', 'query', or 'question' with memory tools.",
			"8. If memory search still does not answer the question and the remaining question is about factual/public knowledge, use web search next instead of stopping at 'I do not know'.",
			"9. Only 'read_buffered_source' uses 'source_id' for web evidence.",
			"10. Do not guess past details.",
			"11. When the user tells you personal facts (name, birthday, preferences, etc.), proactively use 'save_user_fact' to save them to their permanent profile.",
		}
	}
	return fmt.Sprintf(`
### MEMORY CONTEXT ###

#### STATIC MEMORY
%s

#### RECENT CONTEXT
%s

#### USER PROFILE
%s

#### ACTIVE CONTEXT
%s

MEMORY & SEARCH RULES:
%s
`, staticMemory, recentContext, combinedProfile, activeContext, strings.Join(rules, "\n"))
}

func buildPassiveMemoryTemplate(recentContext string, userProfile string, activeContext string, userProfileFacts string) string {
	combinedProfile := ""
	if strings.TrimSpace(userProfileFacts) != "" {
		combinedProfile = "## Known Facts:\n" + userProfileFacts
		if strings.TrimSpace(userProfile) != "" {
			combinedProfile += "\n\n## Recent Memory Snapshot:\n" + userProfile
		}
	} else {
		combinedProfile = userProfile
	}

	return fmt.Sprintf(`
### MEMORY CONTEXT ###

#### RECENT CONTEXT
%s

#### USER PROFILE
%s

#### ACTIVE CONTEXT
%s

MEMORY USAGE RULES:
1. Use RECENT CONTEXT, USER PROFILE, and ACTIVE CONTEXT only as provided reference context for this answer.
2. Do not mention tools, integrations, MCP, or hidden retrieval steps.
3. If the provided memory context is insufficient, answer normally from the visible conversation and your model knowledge.
4. Do not invent or claim to have searched additional memory when no tool access is available.
`, recentContext, combinedProfile, activeContext)
}
