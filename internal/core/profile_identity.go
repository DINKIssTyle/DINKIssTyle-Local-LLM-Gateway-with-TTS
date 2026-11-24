package core

import (
	"regexp"
	"strings"

	"dinkisstyle-chat/internal/mcp"
)

// Only direct current-identity questions use the stored value. Requests about
// old aliases, other subjects, and instructions to rename stay in normal chat.
var directAssistantNameQuestion = regexp.MustCompile(`(?i)^(?:안녕[,.!]?\s*)?(?:(?:너의|너|니|네|당신의|당신)\s*이름(?:은|이)?\s*(?:뭐야|뭐지|뭐니|뭐예요|뭔가요|뭡니까|뭐고\s*(?:내|제)\s*이름(?:은|이)?\s*(?:뭐야|뭐지|뭐니|뭐예요|뭔가요|뭡니까))|what(?: is|'s) your name)[?!.]*$`)

func storedIdentityAnswer(question string, facts []mcp.UserProfileFact) string {
	question = strings.Join(strings.Fields(question), " ")
	if !directAssistantNameQuestion.MatchString(question) {
		return ""
	}
	var assistantName, userName string
	for _, fact := range facts {
		switch fact.FactKey {
		case "ai_nickname":
			assistantName = strings.TrimSpace(fact.FactValue)
		case "name":
			userName = strings.TrimSpace(fact.FactValue)
		}
	}
	if assistantName == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(question), "what") {
		return "My name is " + assistantName + "."
	}
	answer := "제 이름은 " + assistantName + "입니다."
	if strings.Contains(question, "뭐고") {
		if userName == "" {
			return ""
		}
		answer += " 사용자님의 이름은 " + userName + "입니다."
	}
	return answer
}
