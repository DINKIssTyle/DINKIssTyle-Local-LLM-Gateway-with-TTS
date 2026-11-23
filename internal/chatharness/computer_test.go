package chatharness

import (
	"dinkisstyle-chat/internal/promptkit"
	"encoding/json"
	"strings"
	"testing"
)

func TestComputerImagesUseProviderImageInputs(t *testing.T) {
	for _, mode := range []string{"standard", "stateful"} {
		t.Run(mode, func(t *testing.T) {
			req, body, err := PrepareToolFollowupRequest(ToolFollowupInput{
				LLMMode: mode, ModelID: "vision", ToolName: "computer_observe", ToolResult: `{"observation_id":"fresh","width":1440}`,
				ToolImageURLs: []string{"data:image/png;base64,AAAA"}, ToolCallID: "call1", ToolArguments: "{}",
				OriginalUserText: "사파리를 열어줘", ReqMap: map[string]interface{}{},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), "AAAA") || !strings.Contains(string(body), "observation_id") {
				t.Fatal("screenshot or coordinates lost")
			}
			if mode == "stateful" {
				parts := req["input"].([]interface{})
				image := parts[1].(map[string]interface{})
				if image["type"] != "image" || image["data_url"] != "data:image/png;base64,AAAA" {
					t.Fatalf("invalid native image: %#v", image)
				}
			} else {
				messages := req["messages"].([]interface{})
				if messages[1].(map[string]interface{})["tool_call_id"] != "call1" {
					t.Fatal("lost tool call linkage")
				}
				parts := messages[2].(map[string]interface{})["content"].([]interface{})
				if parts[1].(map[string]interface{})["type"] != "image_url" {
					t.Fatal("image sent as text")
				}
			}
		})
	}
}

func TestPlainReadToolRecoveryRequiresToolOnlyRegisteredScalarArguments(t *testing.T) {
	tools := []promptkit.ToolDefinition{{Name: "search_web", InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)}}
	raw := "search_web\nquery: 논산 내일 아침 날씨"
	name, args, ok := ParsePlainReadToolCall(raw, tools)
	if !ok || name != "search_web" || !strings.Contains(args, "논산 내일 아침 날씨") {
		t.Fatalf("tool-only fallback failed: %s %s %v", name, args, ok)
	}
	for _, bad := range []string{"search_web를 사용하세요\nquery: 날씨", "search_web\nunknown: 날씨", "search_web\nquery: 날씨\n확인하겠습니다", "computer_type\ntext: 계획 문장", "예시:\nsearch_web\nquery: 날씨", "search_web\nquery: a\nquery: b"} {
		if _, _, ok := ParsePlainReadToolCall(bad, tools); ok {
			t.Fatalf("prose or unrelated call executed: %q", bad)
		}
	}
	for _, prefix := range []string{"s", "search_web", "search_web\nq", "search_web\nquery: 날씨"} {
		if hold, _ := GemmaToolTextPrefix(prefix, tools); !hold {
			t.Fatalf("partial plain call leaked: %q", prefix)
		}
	}
}

func TestNewDesktopCaptureReplacesOnlyOldDesktopImage(t *testing.T) {
	req := map[string]interface{}{"messages": []interface{}{map[string]interface{}{"role": "user", "content": []interface{}{
		map[string]interface{}{"type": "text", "text": "User attachment"},
		map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "user-image"}},
	}}}}
	for _, url := range []string{"old-screen", "new-screen"} {
		var err error
		req, _, err = PrepareToolFollowupRequest(ToolFollowupInput{ToolName: "computer_observe", ToolResult: "screen", ToolImageURLs: []string{url}, ReqMap: req, ToolCallID: "call1"})
		if err != nil {
			t.Fatal(err)
		}
		// Exercise the actual request round trip used by compatible providers.
		raw, _ := json.Marshal(req)
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(req)
	if strings.Contains(string(raw), "old-screen") || !strings.Contains(string(raw), "new-screen") || !strings.Contains(string(raw), "user-image") {
		t.Fatalf("incorrect image retention: %s", raw)
	}
}

func TestComputerEvidenceKeepsObservationAndAppList(t *testing.T) {
	evidence := strings.Repeat("window metadata ", 110) + `{"observation_id":"latest"}`
	got, _ := formatToolEvidence("computer_observe", evidence)
	if !strings.Contains(got, "latest") {
		t.Fatal("observation ID truncated")
	}
}

func TestComputerDiscoveryFollowupKeepsTaskThroughVerificationInstructions(t *testing.T) {
	const question = "다운로드 중인 작업이 얼마나 진행되었는지 확인해봐"
	for _, tc := range []struct {
		mode  string
		gemma bool
	}{{"standard", false}, {"standard", true}, {"stateful", false}} {
		t.Run(tc.mode+map[bool]string{false: "", true: "_gemma"}[tc.gemma], func(t *testing.T) {
			req, body, err := PrepareToolFollowupRequest(ToolFollowupInput{
				LLMMode: tc.mode, Gemma4Chat: tc.gemma, ToolName: "computer_list_apps", ToolResult: `{"applications":[{"name":"Example App","pid":42}]}`,
				ToolCallID: "call_discovery", ToolArguments: "{}", OriginalUserText: question, ToolReasoning: "Find the requested status in the app.", ReqMap: map[string]interface{}{},
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{question, "computer_switch_app", "computer_observe", "original request already authorizes", "only to list apps", "requested value or status is visible"} {
				if !strings.Contains(string(body), want) {
					t.Fatalf("desktop continuation missing %q: %s", want, body)
				}
			}
			if strings.Contains(string(body), "Output only the user-facing answer") {
				t.Fatal("answer-only instruction can stop desktop work after discovery")
			}
			if tc.gemma {
				messages := req["messages"].([]interface{})
				call := messages[0].(map[string]interface{})
				if call["reasoning_content"] != "Find the requested status in the app." {
					t.Fatal("Gemma tool-call reasoning was dropped")
				}
			}
		})
	}
}

func TestComputerFinalizationDoesNotPretendDiscoveryCompletedTheTask(t *testing.T) {
	_, body, err := PrepareToolFollowupRequest(ToolFollowupInput{Gemma4Chat: true, ToolName: "computer_list_apps", ToolResult: `{"applications":[]}`, ToolCallID: "call", ReqMap: map[string]interface{}{}, FinalAnswerOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "specific unfinished part") || strings.Contains(string(body), "call the next tool now") {
		t.Fatalf("incorrect desktop budget finalization: %s", body)
	}
}

func TestWeatherLookupPromiseDoesNotMatchClarification(t *testing.T) {
	const question = "내일 아침 논산 날씨"
	if !IsUnfulfilledWeatherLookup(question, "내일 아침 논산의 날씨를 확인해 드리겠습니다. 잠시만 기다려 주세요.") {
		t.Fatal("lookup promise should trigger recovery")
	}
	for _, answer := range []string{"어느 지역의 날씨인가요?", "논산은 맑겠습니다."} {
		if IsUnfulfilledWeatherLookup(question, answer) {
			t.Fatalf("unexpected recovery for %q", answer)
		}
	}
}

func TestGemmaSearchCallAfterPreamble(t *testing.T) {
	tools := []promptkit.ToolDefinition{{Name: "search_web", InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)}}
	var visible, pending string
	probe, definite, overflow := true, false, false
	for _, chunk := range []string{"오늘의 일본 뉴스", "를 검색해 드리겠습니다.\n\nsea", "rch_web{query:", "오늘의 일본 뉴스}"} {
		var output string
		output, pending, probe, definite, overflow, _ = ProbeGemmaToolDelta(chunk, pending, probe, definite, overflow, tools)
		visible += output
	}
	if strings.TrimSpace(visible) != "오늘의 일본 뉴스를 검색해 드리겠습니다." {
		t.Fatalf("unexpected visible content: %q", visible)
	}
	name, args, ok := ParsePlainReadToolCall(pending, tools)
	if !ok || name != "search_web" || args != `{"query":"오늘의 일본 뉴스"}` {
		t.Fatalf("unexpected recovered call: %q %q %t", name, args, ok)
	}
}
