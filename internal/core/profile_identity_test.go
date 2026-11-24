package core

import (
	"testing"

	"dinkisstyle-chat/internal/mcp"
)

func TestCurrentAssistantIdentityUsesStoredNickname(t *testing.T) {
	facts := []mcp.UserProfileFact{{FactKey: "ai_nickname", FactValue: "이디스"}, {FactKey: "name", FactValue: "테스트 사용자"}}
	for question, want := range map[string]string{
		"니 이름이 뭐야?":             "제 이름은 이디스입니다.",
		"안녕 니 이름은 뭐고 내 이름은 뭐지?": "제 이름은 이디스입니다. 사용자님의 이름은 테스트 사용자입니다.",
		"이전 이름이 뭐였어?":           "",
		"남산타워 이름 변천사":           "",
		"너의 이름을 바꿔줘":            "",
	} {
		if got := storedIdentityAnswer(question, facts); got != want {
			t.Errorf("storedIdentityAnswer(%q) = %q, want %q", question, got, want)
		}
	}
}
