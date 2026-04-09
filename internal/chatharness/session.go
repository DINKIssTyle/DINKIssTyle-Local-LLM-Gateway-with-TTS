package chatharness

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"dinkisstyle-chat/internal/mcp"
)

type SessionToolHistorySnapshot struct {
	Tool   string `json:"tool"`
	Detail string `json:"detail"`
}

type SessionToolCardSnapshot struct {
	State    string                       `json:"state"`
	Summary  string                       `json:"summary"`
	Args     interface{}                  `json:"args,omitempty"`
	ToolName string                       `json:"tool_name"`
	History  []SessionToolHistorySnapshot `json:"history,omitempty"`
}

type SessionMessageSnapshot struct {
	TurnID                  string `json:"turn_id"`
	UserContent             string `json:"user_content,omitempty"`
	AssistantContent        string `json:"assistant_content,omitempty"`
	ReasoningContent        string `json:"reasoning_content,omitempty"`
	ReasoningDurationMS     int64  `json:"reasoning_duration_ms,omitempty"`
	ReasoningAccumulatedMS  int64  `json:"reasoning_accumulated_ms,omitempty"`
	ReasoningCurrentPhaseMS int64  `json:"reasoning_current_phase_ms,omitempty"`
}

type SessionUISnapshot struct {
	ToolCards    map[string]SessionToolCardSnapshot `json:"tool_cards"`
	Messages     []SessionMessageSnapshot           `json:"messages,omitempty"`
	LastEventSeq int                                `json:"last_event_seq,omitempty"`
}

type SessionPersistState struct {
	Status           string
	LLMMode          string
	ModelID          string
	LastResponseID   string
	SummaryText      string
	TurnCount        int
	EstimatedChars   int
	LastInputTokens  int
	LastOutputTokens int
	PeakInputTokens  int
	TokenBudget      int
	RiskScore        float64
	RiskLevel        string
	LastResetReason  string
	UIStateJSON      string
}

type SessionTracker struct {
	UserID       string
	ClientTurnID string
	Session      mcp.ChatSessionEntry
	SessionOK    bool
	Snapshot     SessionUISnapshot
	UIStateJSON  string
}

func ParseUISnapshot(raw string) SessionUISnapshot {
	snapshot := SessionUISnapshot{
		ToolCards: map[string]SessionToolCardSnapshot{},
		Messages:  []SessionMessageSnapshot{},
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return snapshot
	}
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return SessionUISnapshot{ToolCards: map[string]SessionToolCardSnapshot{}, Messages: []SessionMessageSnapshot{}}
	}
	if snapshot.ToolCards == nil {
		snapshot.ToolCards = map[string]SessionToolCardSnapshot{}
	}
	if snapshot.Messages == nil {
		snapshot.Messages = []SessionMessageSnapshot{}
	}
	return snapshot
}

func EncodeUISnapshot(snapshot SessionUISnapshot) string {
	if snapshot.ToolCards == nil {
		snapshot.ToolCards = map[string]SessionToolCardSnapshot{}
	}
	if snapshot.Messages == nil {
		snapshot.Messages = []SessionMessageSnapshot{}
	}
	bytes, err := json.Marshal(snapshot)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func NewSessionTracker(userID, clientTurnID string, entry mcp.ChatSessionEntry, ok bool, snapshot SessionUISnapshot, uiStateJSON string) *SessionTracker {
	return &SessionTracker{
		UserID:       userID,
		ClientTurnID: clientTurnID,
		Session:      entry,
		SessionOK:    ok,
		Snapshot:     snapshot,
		UIStateJSON:  uiStateJSON,
	}
}

func (t *SessionTracker) AppendEvent(state SessionPersistState, role, eventType string, payload interface{}) {
	if t == nil || !t.SessionOK {
		return
	}
	jsonPayload := "{}"
	if payload != nil {
		if bytes, err := json.Marshal(payload); err == nil {
			jsonPayload = string(bytes)
		}
	}
	if shouldPersistChatEvent(eventType) {
		eventEntry, err := mcp.AppendChatEvent(t.UserID, t.Session.ID, role, eventType, "", t.ClientTurnID, jsonPayload)
		if err != nil {
			log.Printf("[chat-session] failed to append %s event for %s: %v", eventType, t.UserID, err)
		} else if eventEntry.EventSeq > t.Snapshot.LastEventSeq {
			t.Snapshot.LastEventSeq = eventEntry.EventSeq
		}
	}
	if shouldUpdateSessionMessageSnapshot(eventType) {
		updateMessageSnapshot(&t.Snapshot, t.ClientTurnID, role, eventType, payload)
	}
	if shouldUpdateSessionToolSnapshot(eventType) {
		updateToolSnapshot(&t.Snapshot, t.ClientTurnID, eventType, payload)
	}
	if shouldPersistSessionUISnapshot(eventType) {
		t.UIStateJSON = EncodeUISnapshot(t.Snapshot)
		state.UIStateJSON = t.UIStateJSON
		t.Session.UIStateJSON = t.UIStateJSON
		if _, err := mcp.UpsertChatSession(toChatSessionEntry(t.Session.UserID, t.Session.SessionKey, state)); err != nil {
			log.Printf("[chat-session] failed to persist ui state for %s: %v", t.UserID, err)
		}
	}
}

func shouldUpdateSessionMessageSnapshot(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "message.created", "chat.end", "request.complete", "request.cancelled", "session.cleared":
		return true
	default:
		return false
	}
}

func shouldPersistChatEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "message.created", "chat.end", "request.complete", "generation.started", "generation.first_token", "generation.phase", "generation.finished":
		return true
	default:
		return false
	}
}

func shouldUpdateSessionToolSnapshot(eventType string) bool {
	eventType = strings.TrimSpace(eventType)
	return eventType == "tool_call.success" || eventType == "tool_call.failure" || eventType == "chat.end" || eventType == "request.complete"
}

func shouldPersistSessionUISnapshot(eventType string) bool {
	return shouldUpdateSessionMessageSnapshot(eventType) || shouldUpdateSessionToolSnapshot(eventType)
}

func (t *SessionTracker) Finalize(state SessionPersistState) {
	if t == nil || !t.SessionOK {
		return
	}
	state.UIStateJSON = t.UIStateJSON
	if _, err := mcp.UpsertChatSession(toChatSessionEntry(t.Session.UserID, t.Session.SessionKey, state)); err != nil {
		log.Printf("[chat-session] failed to finalize current session for %s: %v", t.UserID, err)
	}
}

func toChatSessionEntry(userID, sessionKey string, state SessionPersistState) mcp.ChatSessionEntry {
	return mcp.ChatSessionEntry{
		UserID:           userID,
		SessionKey:       sessionKey,
		Status:           state.Status,
		LLMMode:          state.LLMMode,
		ModelID:          state.ModelID,
		LastResponseID:   state.LastResponseID,
		SummaryText:      state.SummaryText,
		TurnCount:        state.TurnCount,
		EstimatedChars:   state.EstimatedChars,
		LastInputTokens:  state.LastInputTokens,
		LastOutputTokens: state.LastOutputTokens,
		PeakInputTokens:  state.PeakInputTokens,
		TokenBudget:      state.TokenBudget,
		RiskScore:        state.RiskScore,
		RiskLevel:        state.RiskLevel,
		LastResetReason:  state.LastResetReason,
		UIStateJSON:      state.UIStateJSON,
	}
}

func compactToolSnapshotDetail(toolName string, args interface{}, summary string) string {
	argsMap, _ := args.(map[string]interface{})
	if argsMap != nil {
		switch strings.TrimSpace(toolName) {
		case "search_web", "naver_search", "read_buffered_source", "search_memory":
			if query := extractStringValue(argsMap, []string{"query", "keyword", "text"}); query != "" {
				return compactText(query, 220)
			}
		case "read_web_page":
			if url := extractStringValue(argsMap, []string{"url"}); url != "" {
				return compactText(url, 220)
			}
		case "read_memory":
			if memoryID, ok := argsMap["memory_id"]; ok {
				return compactText("memory_id="+strings.TrimSpace(strings.ReplaceAll(strings.TrimSpace(strings.Trim(strings.ReplaceAll(strings.TrimSpace(compactText(toJSONString(memoryID), 220)), "\"", ""), "\"")), "\n", " ")), 220)
			}
		case "execute_command":
			if command := extractStringValue(argsMap, []string{"command"}); command != "" {
				return compactText(command, 220)
			}
		case "get_current_location":
			return "사용자 위치를 확인했습니다."
		case "get_current_time":
			return "현재 시간을 확인했습니다."
		}

		for _, key := range []string{"query", "url", "text", "prompt", "input", "title"} {
			if value := extractStringValue(argsMap, []string{key}); value != "" {
				return compactText(value, 220)
			}
		}
		if bytes, err := json.Marshal(argsMap); err == nil {
			return compactText(strings.TrimSpace(string(bytes)), 220)
		}
	}
	if args != nil {
		if bytes, err := json.Marshal(args); err == nil {
			return compactText(strings.TrimSpace(string(bytes)), 220)
		}
	}
	return compactText(strings.TrimSpace(summary), 220)
}

func extractStringValue(obj map[string]interface{}, keys []string) string {
	for _, key := range keys {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func updateToolSnapshot(snapshot *SessionUISnapshot, turnID, eventType string, payload interface{}) {
	if snapshot == nil || strings.TrimSpace(turnID) == "" {
		return
	}
	if snapshot.ToolCards == nil {
		snapshot.ToolCards = map[string]SessionToolCardSnapshot{}
	}
	card := snapshot.ToolCards[turnID]
	payloadMap, _ := payload.(map[string]interface{})
	if payloadMap == nil {
		payloadMap = map[string]interface{}{}
	}
	toolName, _ := payloadMap["tool"].(string)
	summary, _ := payloadMap["reason"].(string)
	args := payloadMap["arguments"]

	switch eventType {
	case "tool_call.start":
		card.State = "running"
		card.ToolName = strings.TrimSpace(toolName)
		card.Summary = ""
	case "tool_call.arguments":
		card.State = "running"
		if strings.TrimSpace(toolName) != "" {
			card.ToolName = strings.TrimSpace(toolName)
		}
		card.Args = args
		detail := compactToolSnapshotDetail(card.ToolName, args, summary)
		if detail != "" {
			entry := SessionToolHistorySnapshot{Tool: strings.TrimSpace(card.ToolName), Detail: detail}
			if entry.Tool == "" {
				entry.Tool = "Tool"
			}
			last := SessionToolHistorySnapshot{}
			if len(card.History) > 0 {
				last = card.History[len(card.History)-1]
			}
			if last.Tool != entry.Tool || last.Detail != entry.Detail {
				card.History = append(card.History, entry)
			}
		}
	case "tool_call.success":
		card.State = "success"
		if strings.TrimSpace(toolName) != "" {
			card.ToolName = strings.TrimSpace(toolName)
		}
		card.Summary = ""
	case "tool_call.failure":
		card.State = "failure"
		if strings.TrimSpace(toolName) != "" {
			card.ToolName = strings.TrimSpace(toolName)
		}
		card.Summary = strings.TrimSpace(summary)
	case "chat.end", "request.complete":
		if extracted := extractToolCardSnapshotFromPayload(payloadMap); extracted != nil {
			card = *extracted
		}
	}

	snapshot.ToolCards[turnID] = card
}

func ensureMessageSnapshot(snapshot *SessionUISnapshot, turnID string) *SessionMessageSnapshot {
	if snapshot == nil || strings.TrimSpace(turnID) == "" {
		return nil
	}
	if snapshot.Messages == nil {
		snapshot.Messages = []SessionMessageSnapshot{}
	}
	for i := range snapshot.Messages {
		if snapshot.Messages[i].TurnID == turnID {
			return &snapshot.Messages[i]
		}
	}
	snapshot.Messages = append(snapshot.Messages, SessionMessageSnapshot{TurnID: turnID})
	return &snapshot.Messages[len(snapshot.Messages)-1]
}

func payloadInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case float32:
		return int64(v), true
	case float64:
		return int64(v), true
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return n, true
		}
		f, ferr := v.Float64()
		if ferr == nil {
			return int64(f), true
		}
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, false
		}
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return n, true
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return int64(f), true
		}
	}
	return 0, false
}

func extractFinalAssistantContent(payloadMap map[string]interface{}) string {
	if payloadMap == nil {
		return ""
	}
	resultMap, _ := payloadMap["result"].(map[string]interface{})
	if resultMap != nil {
		outputList, _ := resultMap["output"].([]interface{})
		if len(outputList) > 0 {
			var builder strings.Builder
			for _, raw := range outputList {
				item, _ := raw.(map[string]interface{})
				if item == nil {
					continue
				}
				if itemType, _ := item["type"].(string); itemType != "message" {
					continue
				}
				content, _ := item["content"].(string)
				if strings.TrimSpace(content) == "" {
					continue
				}
				builder.WriteString(content)
			}
			if builder.Len() > 0 {
				return builder.String()
			}
		}
	}
	if finalContent, ok := payloadMap["final_assistant_content"].(string); ok && finalContent != "" {
		return finalContent
	}
	return ""
}

func extractReasoningContent(payloadMap map[string]interface{}) string {
	if payloadMap == nil {
		return ""
	}
	resultMap, _ := payloadMap["result"].(map[string]interface{})
	if resultMap == nil {
		return ""
	}
	outputList, _ := resultMap["output"].([]interface{})
	if len(outputList) == 0 {
		return ""
	}
	parts := make([]string, 0, len(outputList))
	for _, raw := range outputList {
		item, _ := raw.(map[string]interface{})
		if item == nil {
			continue
		}
		if itemType, _ := item["type"].(string); itemType != "reasoning" {
			continue
		}
		content, _ := item["content"].(string)
		if strings.TrimSpace(content) == "" {
			continue
		}
		parts = append(parts, content)
	}
	return strings.Join(parts, "\n\n")
}

func extractToolCardSnapshotFromPayload(payloadMap map[string]interface{}) *SessionToolCardSnapshot {
	if payloadMap == nil {
		return nil
	}
	resultMap, _ := payloadMap["result"].(map[string]interface{})
	if resultMap == nil {
		return nil
	}
	outputList, _ := resultMap["output"].([]interface{})
	if len(outputList) == 0 {
		return nil
	}

	card := SessionToolCardSnapshot{
		State:   "success",
		History: []SessionToolHistorySnapshot{},
	}
	for _, raw := range outputList {
		item, _ := raw.(map[string]interface{})
		if item == nil {
			continue
		}
		if itemType, _ := item["type"].(string); itemType != "tool_call" {
			continue
		}
		toolName, _ := item["tool"].(string)
		if strings.TrimSpace(toolName) != "" {
			card.ToolName = strings.TrimSpace(toolName)
		}
		if args := item["arguments"]; args != nil {
			card.Args = args
			detail := compactToolSnapshotDetail(card.ToolName, args, "")
			if detail != "" {
				entry := SessionToolHistorySnapshot{
					Tool:   strings.TrimSpace(card.ToolName),
					Detail: detail,
				}
				if entry.Tool == "" {
					entry.Tool = "Tool"
				}
				last := SessionToolHistorySnapshot{}
				if len(card.History) > 0 {
					last = card.History[len(card.History)-1]
				}
				if last.Tool != entry.Tool || last.Detail != entry.Detail {
					card.History = append(card.History, entry)
				}
			}
		}
		if output, ok := item["output"].(string); ok && strings.TrimSpace(output) == "" {
			card.State = "failure"
		}
	}

	if strings.TrimSpace(card.ToolName) == "" && len(card.History) == 0 {
		return nil
	}
	if card.State == "" {
		card.State = "success"
	}
	return &card
}

func updateMessageSnapshot(snapshot *SessionUISnapshot, turnID, role, eventType string, payload interface{}) {
	if snapshot == nil || strings.TrimSpace(turnID) == "" {
		return
	}
	msg := ensureMessageSnapshot(snapshot, turnID)
	if msg == nil {
		return
	}
	payloadMap, _ := payload.(map[string]interface{})
	if payloadMap == nil {
		payloadMap = map[string]interface{}{}
	}

	switch eventType {
	case "reasoning.start":
		msg.ReasoningCurrentPhaseMS = 0
		msg.ReasoningDurationMS = msg.ReasoningAccumulatedMS
	case "message.created":
		if role == "user" {
			if content, ok := payloadMap["content"].(string); ok {
				msg.UserContent = content
			}
		}
	case "message.delta":
		if fullContent, ok := payloadMap["full_content"].(string); ok {
			msg.AssistantContent = fullContent
		} else if content, ok := payloadMap["content"].(string); ok && content != "" {
			msg.AssistantContent += content
		}
	case "chat.end", "request.complete":
		if finalContent := extractFinalAssistantContent(payloadMap); finalContent != "" {
			msg.AssistantContent = finalContent
		}
		if reasoningContent := extractReasoningContent(payloadMap); reasoningContent != "" {
			msg.ReasoningContent = reasoningContent
		}
	case "reasoning.delta":
		if content, ok := payloadMap["content"].(string); ok && content != "" {
			msg.ReasoningContent += content
		} else if content, ok := payloadMap["reasoning_content"].(string); ok && content != "" {
			msg.ReasoningContent += content
		} else if text, ok := payloadMap["text"].(string); ok && text != "" {
			msg.ReasoningContent += text
		}
		if totalMS, ok := payloadInt64(payloadMap["total_elapsed_ms"]); ok && totalMS > 0 {
			msg.ReasoningDurationMS = totalMS
			if totalMS >= msg.ReasoningAccumulatedMS {
				msg.ReasoningCurrentPhaseMS = totalMS - msg.ReasoningAccumulatedMS
			}
			break
		}
		if elapsedMS, ok := payloadInt64(payloadMap["elapsed_ms"]); ok && elapsedMS > 0 {
			if elapsedMS > msg.ReasoningCurrentPhaseMS {
				msg.ReasoningCurrentPhaseMS = elapsedMS
			}
			msg.ReasoningDurationMS = msg.ReasoningAccumulatedMS + msg.ReasoningCurrentPhaseMS
		}
	case "reasoning.end":
		if totalMS, ok := payloadInt64(payloadMap["total_elapsed_ms"]); ok && totalMS > 0 {
			msg.ReasoningDurationMS = totalMS
			msg.ReasoningAccumulatedMS = totalMS
			msg.ReasoningCurrentPhaseMS = 0
			break
		}
		if elapsedMS, ok := payloadInt64(payloadMap["elapsed_ms"]); ok && elapsedMS > 0 {
			if elapsedMS > msg.ReasoningCurrentPhaseMS {
				msg.ReasoningCurrentPhaseMS = elapsedMS
			}
		}
		msg.ReasoningAccumulatedMS += msg.ReasoningCurrentPhaseMS
		msg.ReasoningCurrentPhaseMS = 0
		msg.ReasoningDurationMS = msg.ReasoningAccumulatedMS
	}
}

func toJSONString(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(bytes)
}
