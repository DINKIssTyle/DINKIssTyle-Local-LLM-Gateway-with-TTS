package promptkit

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	toolGuidelineMarker    = "### TOOL CALL GUIDELINES ###"
	toolGuidelineEndMarker = "### END TOOL CALL GUIDELINES ###"
)

type RuntimeInstructionsInput struct {
	EnvironmentInfo   string
	ModelID           string
	UseNativeTools    bool
	Tools             []ToolDefinition
	RecentContext     string
	MemorySnapshot    string
	ActiveContext     string
	RetrievalInjected bool
	UserProfileFacts  string
}

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

func ToolGuidelineMarker() string {
	return toolGuidelineMarker
}

// StripToolGuidelines removes the transient tool policy block before a
// tool-free final synthesis turn while preserving the base system prompt and
// any memory context appended after the block.
func StripToolGuidelines(reqMap map[string]interface{}) bool {
	if len(reqMap) == 0 {
		return false
	}
	changed := false
	strip := func(content string) string {
		start := strings.Index(content, toolGuidelineMarker)
		if start < 0 {
			return content
		}
		relativeEnd := strings.Index(content[start:], toolGuidelineEndMarker)
		if relativeEnd < 0 {
			return content
		}
		end := start + relativeEnd + len(toolGuidelineEndMarker)
		before := strings.TrimRight(content[:start], " \t\r\n")
		after := strings.TrimLeft(content[end:], " \t\r\n")
		changed = true
		if before == "" {
			return after
		}
		if after == "" {
			return before
		}
		return before + "\n\n" + after
	}
	if messages, ok := reqMap["messages"].([]interface{}); ok {
		for index, raw := range messages {
			message, _ := raw.(map[string]interface{})
			if role, _ := message["role"].(string); role != "system" {
				continue
			}
			if content, ok := message["content"].(string); ok {
				message["content"] = strip(content)
				messages[index] = message
			}
		}
		reqMap["messages"] = messages
	}
	if systemPrompt, ok := reqMap["system_prompt"].(string); ok {
		reqMap["system_prompt"] = strip(systemPrompt)
	}
	return changed
}

func BuildRuntimeInstructions(input RuntimeInstructionsInput) string {
	extraInstr := ""
	if len(input.Tools) > 0 {
		extraInstr = buildToolUsage(input.EnvironmentInfo, input.ModelID, input.UseNativeTools, input.Tools)
	}
	if input.RecentContext != "" || input.MemorySnapshot != "" || input.ActiveContext != "" || input.UserProfileFacts != "" {
		if len(input.Tools) > 0 {
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

func buildToolUsage(envInfo string, modelID string, useNativeTools bool, tools []ToolDefinition) string {
	lowerModelID := strings.ToLower(strings.TrimSpace(modelID))
	lines := []string{"", "", toolGuidelineMarker}
	has := func(names ...string) bool { return toolDefinitionsContain(tools, names...) }

	if useNativeTools {
		lines = append(lines, nativeToolGuidelines(tools)...)
	} else {
		sampleTool := "tool_name"
		sampleArgs := "{...}"
		if len(tools) > 0 {
			sampleTool = tools[0].Name
			sampleArgs = "{}"
		}
		lines = append(lines,
			fmt.Sprintf("1. For any tool use, output exactly one tool-specific XML element whose body is the JSON arguments object. Example: <%s>%s</%s>. General form: <tool_name>{...}</tool_name>.", sampleTool, sampleArgs, sampleTool),
			"2. Output the tool call only. Do not add prose before or after it. Do not use Python/function syntax such as tool_name(key=\"value\"); use the XML form above. Wait for the app to return the result.",
			"3. If no tool is needed, answer normally.",
			"3a. TOOL DECISION DEADLINE: Keep private reasoning brief. Once you have selected a tool and its arguments, invoke it immediately. Never rehearse, restate, or repeat the planned tool call; if you notice yourself repeating the plan, stop reasoning and emit the tool element now.",
			"4. RESPONSE LANGUAGE RULE: Always answer in the same language as the user's current request unless the user explicitly asks for another language. Tool names, tool arguments, and tool results must never change the response language.",
			"5. CURRENT REQUEST BOUNDARY RULE: Treat the current user message as the authoritative intent. Use earlier turns only to resolve a genuinely omitted or ambiguous referent. When the current message explicitly names a new subject, never prepend words from a completed prior request to tool arguments. For namu_wiki, pass only the exact page title/keyword, excluding the site name and command phrases such as search, find, or 검색.",
			"6. BULK TOOL TEST RULE: If the user explicitly requests every/all tools to be tested, continue automatically with one remaining safe tool per turn instead of asking which tool to test next. Finish with a pass/fail/skipped summary and never delete real user data merely to satisfy a diagnostic.",
		)

		if has("search_web", "search_web_multi", "naver_search", "read_web_page", "read_buffered_source") {
			lines = append(lines,
				"7. FRESHNESS SOURCE QUALITY RULE: For current news or rapidly changing claims, use search_web_multi with two distinct complementary queries (discovery + primary-source/established-news verification). Prioritize official sources and reputable newsrooms. Never present SEO blog claims, blog-only rumors, or unverified claims as verified facts; cite returned source links.",
				"8. Web tools return compact buffered evidence handles; use 1~3 calls max. Answer directly from search evidence when sufficient; call read_web_page only for specific high-value URLs or read_buffered_source for focused excerpts. Do not invent missing dates or retry failed pages/queries repeatedly.",
			)
		}
		if has("search_memory", "read_memory", "read_memory_context", "save_user_fact") {
			lines = append(lines,
				"9. MEMORY-THEN-WEB RULE: If the user asks about prior chats, personal facts, preferences, or earlier reasons, search memory first. If memory is insufficient and the question is still a factual/public information question, then search the web.",
			)
		}
		if has("execute_command") {
			lines = append(lines,
				"10. COMMAND RECOVERY RULE: For execute_command, use ENVIRONMENT INFO for OS-appropriate commands. Never imitate another built-in tool. If a command fails, inspect the error and try a safe OS-appropriate alternative yourself; after success, answer directly without repeating the command.",
			)
		}
		if has("read_help") {
			lines = append(lines,
				"11. For app usage, setup, certificates, endpoints, LM Studio, or app-tool configuration questions, prefer read_help before searching the web.",
			)
		}
	}

	if toolDefinitionsContain(tools, "execute_command") {
		if guidance := platformCommandGuidance(envInfo); guidance != "" {
			lines = append(lines, guidance)
		}
	}

	if len(tools) > 0 && !useNativeTools {
		lines = append(lines, "AVAILABLE APP TOOLS:")
		for _, tool := range tools {
			schemaStr := compactSchemaJSON(tool.InputSchema)
			lines = append(lines, fmt.Sprintf("- %s: %s Input JSON Schema: %s", tool.Name, strings.TrimSpace(tool.Description), schemaStr))
		}
	}

	if strings.Contains(lowerModelID, "gemma-4") {
		lines = append(lines,
			"GEMMA-4 RULE: Prefer the smallest useful number of tool calls. Once a tool result already answers the user's request well enough, stop calling tools and answer immediately.",
		)
	}

	if toolDefinitionsContain(tools, "search_web", "search_web_multi", "naver_search", "read_web_page", "read_buffered_source", "get_current_time") {
		lines = append(lines, fmt.Sprintf("CURRENT_TIME: %s", time.Now().Format("2006-01-02 15:04:05 Monday")))
	}

	if envInfo != "" && toolDefinitionsContain(tools, "execute_command") {
		lines = append(lines, "ENVIRONMENT INFO:", strings.TrimRight(envInfo, "\n"))
	}
	lines = append(lines, toolGuidelineEndMarker)

	return strings.Join(lines, "\n")
}

func compactSchemaJSON(raw json.RawMessage) string {
	schema := strings.TrimSpace(string(raw))
	if schema == "" {
		return `{"type":"object","properties":{}}`
	}
	var obj interface{}
	if err := json.Unmarshal([]byte(schema), &obj); err == nil {
		if b, err := json.Marshal(obj); err == nil {
			return string(b)
		}
	}
	return schema
}

func nativeToolGuidelines(tools []ToolDefinition) []string {
	has := func(names ...string) bool { return toolDefinitionsContain(tools, names...) }
	lines := []string{
		"1. Use provider-native tool calls only; never print tool XML, pseudo-schemas, function syntax, or hidden reasoning.",
		"2. If no tool is needed, answer normally. Otherwise call one tool at a time and invoke it as soon as its arguments are ready.",
		"3. Treat the current message as authoritative. Use earlier turns only for an omitted referent, and never contaminate a new subject with a completed request.",
		"4. Answer in the language of the current request unless the user asks otherwise.",
		"5. After a sufficient tool result, answer directly. Do not repeat equivalent calls.",
		"6. If the user explicitly requests a safe bulk tool test, continue one remaining tool at a time and finish with a pass/fail/skipped summary.",
	}
	if has("search_web", "search_web_multi", "naver_search", "read_web_page", "read_buffered_source", "namu_wiki") {
		lines = append(lines,
			"WEB: For fresh news, current events, or genuine comparison, use search_web_multi once with exactly two complementary queries: broad discovery plus official/primary or established-news verification, using CURRENT_TIME's year.",
			"WEB: Usually use one search call and at most three evidence calls. Cite returned links, answer when evidence is sufficient, and read a page/buffer only for missing high-value detail.",
			"WEB: Do not present weak, conflicting, blog-only, or off-topic evidence as verified. Do not invent missing dates or retry the same failed page/query.",
			"WEB: For relatives, verify and distinguish biological, adopted, and step relationships; never infer birth from wording such as 'children with'.",
		)
	}
	if has("search_memory", "read_memory", "read_memory_context", "save_user_fact") {
		lines = append(lines,
			"MEMORY: For prior chats or personal facts, use provided context first, then search_memory if needed; inspect the best candidate with read_memory_context before guessing.",
			"MEMORY: Save newly stated durable personal facts with save_user_fact, and search the web only if unresolved public facts remain.",
		)
	}
	if has("read_help") {
		lines = append(lines, "HELP: For app usage, setup, certificates, endpoints, LM Studio, or app-tool configuration, prefer read_help over web search.")
	}
	if has("execute_command") {
		lines = append(lines,
			"COMMAND: Use ENVIRONMENT INFO for OS-appropriate commands. Never imitate another built-in tool with execute_command.",
			"COMMAND: On failure, inspect the error and try one safe alternative when available; after success, answer without repeating the command.",
		)
	}
	if has("namu_wiki") {
		lines = append(lines, "NAMUWIKI: Pass only the exact page title or keyword, excluding the site name and command phrases.")
	}
	return lines
}

func toolDefinitionsContain(tools []ToolDefinition, names ...string) bool {
	for _, tool := range tools {
		for _, name := range names {
			if tool.Name == name {
				return true
			}
		}
	}
	return false
}

func platformCommandGuidance(envInfo string) string {
	lower := strings.ToLower(envInfo)
	switch {
	case strings.Contains(lower, "operating system: darwin"):
		return "PLATFORM COMMAND GUIDE (Darwin/macOS, BSD userland): Do not use GNU-only flags such as ps --sort. For processes by memory, use `ps axo pid,comm,%mem,rss | sort -k3 -nr | head -n 11`. For boot time, use `sysctl -n kern.boottime`."
	case strings.Contains(lower, "operating system: linux"):
		return "PLATFORM COMMAND GUIDE (Linux, GNU userland): For processes by memory, use `ps -eo pid,comm,%mem,rss --sort=-%mem | head -n 11`. For boot time, use `uptime -s` (or an available equivalent if unsupported)."
	case strings.Contains(lower, "operating system: windows"):
		return "PLATFORM COMMAND GUIDE (Windows): Use Windows-native commands. For processes by memory, use `powershell -NoProfile -Command \"Get-Process | Sort-Object WorkingSet64 -Descending | Select-Object -First 10 Id,ProcessName,WorkingSet64\"`. For boot time, use `powershell -NoProfile -Command \"(Get-CimInstance Win32_OperatingSystem).LastBootUpTime\"`."
	default:
		return ""
	}
}

func buildMemoryTemplate(staticMemory string, recentContext string, userProfile string, activeContext string, retrievalInjected bool, userProfileFacts string) string {
	combinedProfile := ""
	if strings.TrimSpace(userProfileFacts) != "" {
		combinedProfile = "## Known Facts (always available, no search needed):\n" + userProfileFacts
		if strings.TrimSpace(userProfile) != "" {
			combinedProfile += "\n\n## Recent Memory Snapshot:\n" + userProfile
		}
	} else {
		combinedProfile = userProfile
	}

	var sections []string
	sections = append(sections, "### MEMORY CONTEXT ###")
	if s := strings.TrimSpace(staticMemory); s != "" {
		sections = append(sections, "#### STATIC MEMORY\n"+s)
	}
	if s := strings.TrimSpace(recentContext); s != "" {
		sections = append(sections, "#### RECENT CONTEXT\n"+s)
	}
	if s := strings.TrimSpace(combinedProfile); s != "" {
		sections = append(sections, "#### USER PROFILE\n"+s)
	}
	if s := strings.TrimSpace(activeContext); s != "" {
		sections = append(sections, "#### ACTIVE CONTEXT\n"+s)
	}

	rules := []string{
		"1. Current user message is authoritative; use RECENT CONTEXT to resolve omitted subjects/referents (especially in Korean).",
		"2. USER PROFILE Known Facts are ground truth for personal details; they never require search_memory.",
		"3. For missing/uncertain past chats or facts, use 'search_memory' (then 'read_memory_context'). Do not guess.",
		"4. Proactively use 'save_user_fact' to permanently save newly stated user facts (name, birthday, preferences, etc.).",
		"5. Mention only relevant profile facts. If memory is insufficient for public facts, search the web.",
	}
	if retrievalInjected && strings.TrimSpace(activeContext) != "" {
		rules = append([]string{
			"0. ACTIVE CONTEXT was already retrieved for this turn; prefer answering from RECENT CONTEXT plus ACTIVE CONTEXT when sufficient.",
		}, rules...)
	}

	sections = append(sections, "MEMORY & SEARCH RULES:\n"+strings.Join(rules, "\n"))
	return "\n\n" + strings.Join(sections, "\n\n") + "\n"
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

	var sections []string
	sections = append(sections, "### MEMORY CONTEXT ###")
	if s := strings.TrimSpace(recentContext); s != "" {
		sections = append(sections, "#### RECENT CONTEXT\n"+s)
	}
	if s := strings.TrimSpace(combinedProfile); s != "" {
		sections = append(sections, "#### USER PROFILE\n"+s)
	}
	if s := strings.TrimSpace(activeContext); s != "" {
		sections = append(sections, "#### ACTIVE CONTEXT\n"+s)
	}

	sections = append(sections, "MEMORY USAGE RULES:\n"+strings.Join([]string{
		"1. Current message is authoritative; use RECENT CONTEXT, USER PROFILE, and ACTIVE CONTEXT as reference.",
		"2. Resolve omitted subjects/referents from RECENT CONTEXT without asking unnecessary clarifying questions.",
		"3. USER PROFILE Known Facts are ground truth for personal details; use them when directly relevant without listing unrelated facts.",
		"4. Do not mention tools, integrations, or hidden retrieval steps.",
		"5. If provided memory context is insufficient, answer normally from model knowledge without guessing.",
	}, "\n"))

	return "\n\n" + strings.Join(sections, "\n\n") + "\n"
}
