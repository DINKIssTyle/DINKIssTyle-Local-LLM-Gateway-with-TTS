package chatharness

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"dinkisstyle-chat/internal/promptkit"
)

var gemmaHistoryThinking = regexp.MustCompile(`(?is)<think>.*?(?:</think>|$)|<\|channel>\s*thought\b.*?(?:<channel\|>|$)`)
var gemmaSimpleAccountLookup = regexp.MustCompile(`^(?:(?:오늘|내일|이번 주|다음 주|최근|[월화수목금토일]요일)\s*)?(?:내\s*)?(일정|캘린더|이메일|메일함|받은 편지|드라이브)(?:을|를|은|는)?(?:\s*(?:체크|확인(?:해\s*(?:줘|주세요))?|알려\s*(?:줘|주세요)))?[ ?!.]*$`)

// ParseGlassesChatHistory decodes the completed pairs sent by released plugins.
// Ambient context uses a different format and is not parsed as conversation.
func ParseGlassesChatHistory(context string) []SessionTurnSnapshot {
	var turns []SessionTurnSnapshot
	question, found := strings.CutPrefix(strings.TrimSpace(context), "질문: ")
	for found {
		user, rest, ok := strings.Cut(question, "\n답변: ")
		if !ok {
			return nil
		}
		answer, next, more := strings.Cut(rest, "\n\n질문: ")
		turns = append(turns, SessionTurnSnapshot{UserContent: user, AssistantContent: answer})
		question, found = next, more
	}
	return turns
}

// PrepareGemma4History retains a bounded suffix of complete role pairs and
// the current user message. Incoming role pairs take precedence over fallback
// history, which is supplied only within the caller's conversation scope.
func PrepareGemma4History(body map[string]any, fallback []SessionTurnSnapshot) bool {
	messages, _ := body["messages"].([]any)
	lastUser := -1
	var system []string
	for i, raw := range messages {
		message, _ := raw.(map[string]any)
		switch message["role"] {
		case "system", "developer":
			if text, ok := message["content"].(string); ok && strings.TrimSpace(text) != "" {
				system = append(system, text)
			}
		case "user":
			lastUser = i
		}
	}
	if lastUser < 0 {
		return false
	}
	var turns []SessionTurnSnapshot
	var prior SessionTurnSnapshot
	for _, raw := range messages[:lastUser] {
		message, _ := raw.(map[string]any)
		content, _ := message["content"].(string)
		switch message["role"] {
		case "user":
			if prior.UserContent != "" && prior.AssistantContent != "" {
				turns = append(turns, prior)
			}
			prior = SessionTurnSnapshot{UserContent: content}
		case "assistant":
			if message["tool_calls"] == nil && content != "" {
				prior.AssistantContent = content
			}
		}
	}
	if prior.UserContent != "" && prior.AssistantContent != "" {
		turns = append(turns, prior)
	}
	if len(turns) == 0 {
		turns = fallback
	}
	var recent []SessionTurnSnapshot
	const historyBudget = 3600
	remaining := historyBudget
	for i := len(turns) - 1; i >= 0 && len(recent) < 4; i-- {
		t := turns[i]
		if t.Status != "" && t.Status != "completed" {
			continue
		}
		u := strings.TrimSpace(t.UserContent)
		a := strings.TrimSpace(gemmaHistoryThinking.ReplaceAllString(t.AssistantContent, ""))
		if u == "" || a == "" {
			continue
		}
		u, a = compactText(u, 800), compactText(a, 1200)
		size := utf8.RuneCountInString(u) + utf8.RuneCountInString(a)
		if size > remaining {
			break
		}
		remaining -= size
		recent = append(recent, SessionTurnSnapshot{UserContent: u, AssistantContent: a})
	}
	slices.Reverse(recent)
	out := make([]any, 0, 2*len(recent)+2)
	if len(system) > 0 {
		out = append(out, map[string]any{"role": "system", "content": strings.Join(system, "\n\n")})
	}
	for _, turn := range recent {
		out = append(out,
			map[string]any{"role": "user", "content": turn.UserContent},
			map[string]any{"role": "assistant", "content": turn.AssistantContent},
		)
	}
	out = append(out, messages[lastUser:]...)
	body["messages"] = out
	return len(recent) > 0
}

// Gemma4Tools narrows straightforward account lookups. Keep the full
// catalog for mixed, ambiguous, and explicitly routed requests.
func Gemma4Tools(question string, tools []promptkit.ToolDefinition) []promptkit.ToolDefinition {
	match := gemmaSimpleAccountLookup.FindStringSubmatch(strings.TrimSpace(question))
	if match == nil {
		return tools
	}
	var prefix string
	switch match[1] {
	case "일정", "캘린더":
		prefix = "google_calendar_"
	case "이메일", "메일함", "받은 편지":
		prefix = "gmail_"
	case "드라이브":
		prefix = "google_drive_"
	}
	var relevant []promptkit.ToolDefinition
	for _, tool := range tools {
		if strings.HasPrefix(tool.Name, prefix) {
			relevant = append(relevant, tool)
		}
	}
	if len(relevant) == 0 {
		return tools
	}
	return relevant
}

const gemmaToolOpen = "<|tool_call>"
const gemmaToolClose = "<tool_call|>"
const gemmaToolQuote = `<|"|>`

// GemmaToolTextPrefix identifies only a prospective call at the start of an
// Gemma 4 reply. Holding a partial prefix keeps a leaked call out of the UI.
func GemmaToolTextPrefix(text string, tools []promptkit.ToolDefinition) (hold, definite bool) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, gemmaToolOpen) {
		return true, true
	}
	if strings.HasPrefix(gemmaToolOpen, text) {
		return true, false
	}
	for _, tool := range tools {
		prefix := tool.Name + "{"
		if strings.HasPrefix(text, prefix) {
			return true, true
		}
		if strings.HasPrefix(prefix, text) {
			return true, false
		}
	}
	return false, false
}

var gemmaArgumentKey = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z_0-9]*$`)

// ParseGemmaToolText accepts a single complete call in Gemma's wire syntax.
// Unwrapped text requires Gemma's special string delimiter to distinguish it
// from ordinary prose and examples. The normal registry validates arguments.
func ParseGemmaToolText(text string) (name, args string, ok bool) {
	text = strings.TrimSpace(text)
	wrapped := strings.HasPrefix(text, gemmaToolOpen)
	if wrapped {
		if !strings.HasSuffix(text, gemmaToolClose) {
			return "", "", false
		}
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, gemmaToolOpen), gemmaToolClose))
		text = strings.TrimPrefix(text, "call:")
	} else if !strings.Contains(text, gemmaToolQuote) {
		return "", "", false
	}
	name, rest, found := strings.Cut(text, "{")
	if !found || !gemmaArgumentKey.MatchString(name) || !strings.HasSuffix(rest, "}") {
		return "", "", false
	}
	rest = strings.TrimSpace(strings.TrimSuffix(rest, "}"))
	values := map[string]any{}
	if wrapped && json.Unmarshal([]byte("{"+rest+"}"), &values) == nil {
		b, _ := json.Marshal(values)
		return name, string(b), true
	}
	for rest != "" {
		key, valueText, found := strings.Cut(rest, ":")
		key = strings.TrimSpace(key)
		if _, duplicate := values[key]; !found || duplicate || !gemmaArgumentKey.MatchString(key) {
			return "", "", false
		}
		valueText = strings.TrimSpace(valueText)
		var value any
		if quoted, ok := strings.CutPrefix(valueText, gemmaToolQuote); ok {
			var closed bool
			value, rest, closed = strings.Cut(quoted, gemmaToolQuote)
			if !closed {
				return "", "", false
			}
		} else {
			decoder := json.NewDecoder(strings.NewReader(valueText))
			decoder.UseNumber()
			if err := decoder.Decode(&value); err != nil {
				return "", "", false
			}
			rest = valueText[decoder.InputOffset():]
		}
		values[key] = value
		rest = strings.TrimSpace(rest)
		if rest != "" {
			next, comma := strings.CutPrefix(rest, ",")
			rest = strings.TrimSpace(next)
			if !comma || rest == "" {
				return "", "", false
			}
		}
	}
	b, err := json.Marshal(values)
	return name, string(b), err == nil
}
