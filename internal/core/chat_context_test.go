package core

import (
	"strings"
	"testing"

	"dinkisstyle-chat/internal/chatharness"
)

func TestAssembleChatContextUsesMissingRecentAndRelevantOldTurn(t *testing.T) {
	turn := func(id, user, reply string) chatharness.SessionTurnSnapshot {
		return chatharness.SessionTurnSnapshot{TurnID: id, Status: "completed", UserContent: user, AssistantContent: reply}
	}
	snapshot := chatharness.SessionUISnapshot{Turns: []chatharness.SessionTurnSnapshot{
		turn("source", "배포 대상은 동백서버입니다", "확인했습니다. 배포 시점은 금요일입니다."),
		turn("unrelated", "사진 분류", strings.Repeat("이미지 설명 ", 160)),
		turn("old", "다른 채팅 주제", strings.Repeat("관련 없는 내용 ", 160)),
		turn("recent", "문서 초안", "작성했습니다."),
		turn("correction", "배포는 토요일로 정정합니다", "토요일로 확인했습니다."),
		turn("current", "동백서버 배포 일정은?", "재시도 이전 답변"),
	}}
	messages := map[string]any{"messages": []any{
		map[string]any{"role": "user", "content": "배포는 토요일로 정정합니다"},
		map[string]any{"role": "assistant", "content": "토요일로 확인했습니다."},
		map[string]any{"role": "user", "content": "동백서버 배포 일정은?"},
	}}
	recent, active, count := assembleChatContext("test/session", "동백서버 배포 일정은?", "current", messages, snapshot, false)
	if count != 1 || !strings.Contains(recent, "문서 초안") || strings.Contains(recent, "토요일") {
		t.Fatalf("unexpected recent context (%d): %s", count, recent)
	}
	if !strings.Contains(active, `turn_id="source"`) || !strings.Contains(active, "동백서버") ||
		strings.Contains(recent+active, "관련 없는 내용") || strings.Contains(recent+active, "재시도 이전 답변") {
		t.Fatalf("unexpected active context: %s", active)
	}
	legacy, _ := formatRecentContextTurns([]recentContextTurn{
		{User: snapshot.Turns[2].UserContent, Assistant: snapshot.Turns[2].AssistantContent},
		{User: snapshot.Turns[3].UserContent, Assistant: snapshot.Turns[3].AssistantContent},
		{User: snapshot.Turns[4].UserContent, Assistant: snapshot.Turns[4].AssistantContent},
	}, 4, 2200)
	if len(recent)+len(active) >= len(legacy) {
		t.Fatalf("expected fewer history bytes, got %d versus %d", len(recent)+len(active), len(legacy))
	}
	t.Logf("history bytes: %d -> %d", len(legacy), len(recent)+len(active))
}
