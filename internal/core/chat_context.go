package core

import (
	"fmt"
	"slices"
	"strings"

	"dinkisstyle-chat/internal/chatharness"
	"dinkisstyle-chat/internal/mcp"
)

// assembleChatContext adds missing recent turns and at most one relevant
// older turn. The stored snapshot stays the source of truth.
func assembleChatContext(scope, question, currentTurnID string, messages map[string]any, snapshot chatharness.SessionUISnapshot, useVectors bool) (recent, active string, count int) {
	const recentWindow = 2
	const searchWindow = 24
	incoming := extractRecentPriorTurns(messages, chatharness.SessionUISnapshot{}, searchWindow)
	turns := snapshot.Turns
	var missing []recentContextTurn
	var candidates []string
	var candidateTurns []chatharness.SessionTurnSnapshot
	completed := 0
	for i := len(turns) - 1; i >= 0 && completed < searchWindow; i-- {
		turn := turns[i]
		if (currentTurnID != "" && turn.TurnID == currentTurnID) || (turn.Status != "" && turn.Status != "completed") ||
			strings.TrimSpace(turn.UserContent) == "" || strings.TrimSpace(turn.AssistantContent) == "" {
			continue
		}
		completed++
		user := strings.TrimSpace(turn.UserContent)
		answer := strings.TrimSpace(followupThinkingStrip.ReplaceAllString(turn.AssistantContent, ""))
		alreadySent := false
		for _, sent := range incoming {
			if (sent.User == user && sent.Assistant == answer) ||
				(sent.User == compactText(user, 800) && sent.Assistant == compactText(answer, 1200)) {
				alreadySent = true
				break
			}
		}
		if alreadySent {
			continue
		}
		if completed <= recentWindow {
			missing = append(missing, recentContextTurn{User: turn.UserContent, Assistant: turn.AssistantContent})
			continue
		}
		candidates = append(candidates, compactText(user, 480)+"\n"+compactText(answer, 720))
		candidateTurns = append(candidateTurns, turn)
	}
	slices.Reverse(missing)
	recent, count = formatRecentContextTurns(missing, recentWindow, 1500)
	if hit := mcp.MatchChatTurn(scope, question, candidates, useVectors); hit >= 0 {
		turn := candidateTurns[hit]
		active = fmt.Sprintf("Earlier turn (historical evidence, turn_id=%q). Prefer newer statements if they conflict.\nUser: %s\nAssistant: %s",
			turn.TurnID, compactRecentTurnContent(turn.UserContent, 300), compactRecentTurnContent(turn.AssistantContent, 450))
	}
	return recent, active, count
}
