package chatharness

import (
	"strings"
	"testing"
)

func TestGlassesPromptProfileRejectsAutomaticContext(t *testing.T) {
	for _, mode := range []string{"standard", "stateful"} {
		body := []byte(`{"model":"test","messages":[{"role":"system","content":"CLIENT_CURRENT_TASK"},{"role":"user","content":"맞춤법 검사 피라미드"}],"previous_response_id":"resp_web","stream":true}`)
		if mode == "stateful" {
			body = []byte(`{"model":"test","input":"맞춤법 검사 피라미드","system_prompt":"CLIENT_CURRENT_TASK","previous_response_id":"resp_web","stream":true}`)
		}
		prepared, err := PrepareRequest(RequestInput{PromptProfile: "glasses", Body: body, LLMMode: mode, ContextStrategy: "retrieval", EnableTools: true, EnableMemory: true, Tools: testToolDefinitions(), RecentContext: "OLD_WEB_CHAT", ActiveContext: "OLD_TV_MEMORY", MemorySnapshot: "OLD_SNAPSHOT", UserProfileFacts: "OLD_PROFILE", SkillInstructions: "OLD_SKILL", RetrievalInjected: true})
		if err != nil {
			t.Fatal(err)
		}
		text := string(prepared.Body)
		for _, bad := range []string{"OLD_WEB_CHAT", "OLD_TV_MEMORY", "OLD_SNAPSHOT", "OLD_PROFILE", "OLD_SKILL", "BULK TOOL TEST RULE", "ENVIRONMENT INFO", "previous_response_id"} {
			if strings.Contains(text, bad) {
				t.Fatalf("%s leaked %s", mode, bad)
			}
		}
		if !strings.Contains(text, "CLIENT_CURRENT_TASK") || !strings.Contains(text, "GLASSES REQUEST CONTEXT") {
			t.Fatal(text)
		}
		if prepared.IsStatefulFollowup {
			t.Fatal("glasses request linked to web response")
		}
	}
}
func TestGeneralChatRetainsItsContext(t *testing.T) {
	prepared, err := PrepareRequest(RequestInput{Body: []byte(`{"messages":[{"role":"user","content":"피라미드"}]}`), LLMMode: "standard", ContextStrategy: "retrieval", RecentContext: "CURRENT_WEB_HISTORY", ActiveContext: "MATCHED_MEMORY", SkillInstructions: "SELECTED_SKILL"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"CURRENT_WEB_HISTORY", "MATCHED_MEMORY", "SELECTED_SKILL"} {
		if !strings.Contains(string(prepared.Body), want) {
			t.Fatalf("general chat lost %s", want)
		}
	}
}
func TestGlassesPlanDoesNotAdvertiseDisabledTools(t *testing.T) {
	prepared, err := PrepareRequest(RequestInput{PromptProfile: "glasses", Body: []byte(`{"input":"계획","system_prompt":"JSON only"}`), LLMMode: "stateful", Tools: testToolDefinitions(), EnableTools: false})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(prepared.Body), "get_current_time") {
		t.Fatal("disabled tool advertised in planning prompt")
	}
}

func TestGlassesChatUsesWebRuntimeWithHUDPresentation(t *testing.T) {
	for _, mode := range []string{"standard", "stateful"} {
		t.Run(mode, func(t *testing.T) {
			body := []byte(`{"model":"test","messages":[{"role":"user","content":"오늘 일정"}]}`)
			if mode == "stateful" {
				body = []byte(`{"model":"test","input":"오늘 일정","system_prompt":"Current task"}`)
			}
			prepared, err := PrepareRequest(RequestInput{HUDOutput: true, PromptProfile: "chat", Body: body, LLMMode: mode, ContextStrategy: "retrieval", EnableTools: true, EnableMemory: true, Tools: testToolDefinitions(), RecentContext: "CURRENT_CHAT_MEMORY", ActiveContext: "RETRIEVED_ACCOUNT_FACTS", SkillInstructions: "GOOGLE_CALENDAR_SKILL: search all subscribed calendars; cite links.", RetrievalInjected: true})
			if err != nil {
				t.Fatal(err)
			}
			text := string(prepared.Body)
			for _, want := range []string{"CURRENT_CHAT_MEMORY", "RETRIEVED_ACCOUNT_FACTS", "GOOGLE_CALENDAR_SKILL", "get_current_time", "SMART GLASSES OUTPUT FORMAT", "500 Unicode characters", "Omit source names"} {
				if !strings.Contains(text, want) {
					t.Fatalf("%s lost %s: %s", mode, want, text)
				}
			}
			if strings.Contains(text, "GLASSES REQUEST CONTEXT") {
				t.Fatal("interactive chat entered lightweight ambient runtime")
			}
		})
	}
}
