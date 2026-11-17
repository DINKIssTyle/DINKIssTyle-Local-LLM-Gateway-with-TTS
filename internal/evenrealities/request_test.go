package evenrealities

import (
	"encoding/json"
	"strings"
	"testing"
)

func assemble(t *testing.T, req map[string]any) map[string]any {
	t.Helper()
	req["version"] = 1
	raw, _ := json.Marshal(map[string]any{"model": "@llm/remote/encoded", "stream": true, "glasses_request": req, "system_prompt": "CLIENT_SYSTEM", "input": "CLIENT_INPUT", "messages": []any{map[string]any{"role": "system", "content": "CLIENT_SYSTEM"}}, "response_format": map[string]any{"bad": true}, "previous_response_id": "OLD_CHAT"})
	out, handled, err := AssembleRequest(raw)
	if err != nil || !handled {
		t.Fatalf("%t %v", handled, err)
	}
	var result map[string]any
	_ = json.Unmarshal(out, &result)
	for _, key := range []string{"glasses_request", "input", "system_prompt", "previous_response_id"} {
		if _, ok := result[key]; ok {
			t.Fatalf("leaked %s", key)
		}
	}
	if strings.Contains(string(out), "CLIENT_SYSTEM") || strings.Contains(string(out), "CLIENT_INPUT") {
		t.Fatal("client instructions were trusted")
	}
	if result["model"] != "@llm/remote/encoded" || result["store"] != false {
		t.Fatal("model or stateless boundary changed")
	}
	return result
}
func prompt(result map[string]any, index int) string {
	return result["messages"].([]any)[index].(map[string]any)["content"].(string)
}
func TestTasksAreAssembledFromData(t *testing.T) {
	for _, lang := range []string{"ko", "en", "ja", "zh", "es", "fr", "de", "vi", "th"} {
		for _, task := range []string{"chat", "judge", "plan", "answer", "translation", "translation_only", "translation_replies", "translation_stream", "translation_preview"} {
			t.Run(task+"_"+lang, func(t *testing.T) {
				req := map[string]any{"task": task, "text": "새 질문 {{language}}?", "context": "이전 대화", "target": lang, "source": "auto", "translated": "완료된 번역", "chatSession": true, "judge": map[string]any{"latest": "피라미드 구조를 보고 있어요", "recent": "이집트의 건축 이야기", "collected": true}, "plan": map[string]any{"taskOriented": true, "topic": "피라미드의 구조 원리", "question": "피라미드 구조의 안정성을 설명", "kind": "key", "source": "internal", "knownFacts": []string{"피라미드 구조를 보고 있어요"}}, "observed": "피라미드 구조를 보고 있어요", "anchor": "피라미드"}
				result := assemble(t, req)
				sys, user := prompt(result, 0), prompt(result, 1)
				if strings.Contains(sys, "{{") {
					t.Fatalf("unresolved server placeholder: %s", sys)
				}
				if task == "judge" {
					if !strings.Contains(sys, "Cards are written in "+glassesLanguage(lang)) || !strings.Contains(user, "피라미드") {
						t.Fatal("judge lost language or speech")
					}
					if result["glasses_hint_phase"] != "judge" || result["response_format"] == nil {
						t.Fatal("judge contract missing")
					}
				}
				if task == "chat" {
					if !strings.Contains(user, "Previous explicit AI chat") || !strings.HasSuffix(user, "새 질문 {{language}}?") {
						t.Fatal("raw input was expanded recursively or explicit history lost")
					}
					if result["glasses_client"] != nil || result["glasses_hint"] != nil {
						t.Fatal("explicit chat became ambient")
					}
				}
				if task == "answer" {
					proof := result["hint_research"].(map[string]any)
					if proof["require_grounding"] != true || proof["anchor"] != "피라미드" || proof["task_oriented"] != true {
						t.Fatal("grounding contract changed")
					}
				}
				if strings.HasPrefix(task, "translation") {
					if result["glasses_client"] != true || result["glasses_task"] == nil {
						t.Fatal("translation isolation missing")
					}
					if task == "translation_replies" && (!strings.Contains(user, "완료된 번역") || !strings.Contains(sys, "do not generate or change translation")) {
						t.Fatal("completed translation must only generate replies")
					}
				}
			})
		}
	}
}
func TestJudgeRepairAndPolicyAreServerOwned(t *testing.T) {
	base := map[string]any{"task": "judge", "target": "ko", "preferWeb": false, "sensitivity": "active", "repair": 1, "judge": map[string]any{"latest": "문 손잡이의 원리는 무엇일까요"}}
	result := assemble(t, base)
	sys := prompt(result, 0)
	for _, part := range []string{"Use certain stable internal knowledge", "A single clear mention is enough", "not just a category"} {
		if !strings.Contains(sys, part) {
			t.Fatalf("missing %s", part)
		}
	}
	schema := result["response_format"].(map[string]any)["json_schema"].(map[string]any)["schema"].(map[string]any)
	if strings.Contains(string(mustJSON(schema)), "recap") {
		t.Fatal("recap returned to selection schema")
	}
}
func mustJSON(value any) []byte { raw, _ := json.Marshal(value); return raw }
func TestRequestValidationAndUnrelatedChat(t *testing.T) {
	for _, raw := range []string{`{"glasses_request":null}`, `{"glasses_request":{"version":99,"task":"chat"}}`, `{"glasses_request":{"version":1,"task":"unknown"}}`, `{"glasses_request":{"version":1,"task":"answer","plan":{"source":"execute"}}}`} {
		if _, handled, err := AssembleRequest([]byte(raw)); !handled || err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	raw := []byte(`{"messages":[{"role":"user","content":"ordinary web chat"}]}`)
	result, handled, err := AssembleRequest(raw)
	if handled || err != nil || string(raw) != string(result) {
		t.Fatal("ordinary web chat was changed")
	}
	req := map[string]any{"version": 1, "task": "chat", "text": strings.Repeat("가", 32001)}
	_, handled, err = AssembleRequest(mustJSON(map[string]any{"glasses_request": req}))
	if !handled || err == nil {
		t.Fatal("unbounded input accepted")
	}
}

func BenchmarkAssembleJudge(b *testing.B) {
	raw := []byte(`{"model":"small","stream":true,"glasses_request":{"version":1,"task":"judge","target":"ko","judge":{"recent":"영화와 주말 계획을 이야기합니다.","latest":"극장에서 영화를 보려고 합니다.","collected":true}}}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, err := AssembleRequest(raw); err != nil {
			b.Fatal(err)
		}
	}
}
