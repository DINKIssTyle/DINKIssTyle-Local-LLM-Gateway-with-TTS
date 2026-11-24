package evenrealities

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSourceSelectionContractAndProtocol(t *testing.T) {
	data := map[string]any{"task": "가상 기준일의 관객 수 순위", "choices": []map[string]any{{"selection": 1, "record": 1, "sentence": 1, "title": "자료 A", "url": "https://example.com/a", "text": "가상 기준일의 관객 수 자료입니다."}, {"selection": 2, "record": 1, "sentence": 2, "title": "자료 A", "url": "https://example.com/a", "text": "작품 A가 집계 첫 번째입니다."}, {"selection": 3, "record": 2, "sentence": 1, "title": "자료 B", "url": "https://example.com/b", "text": "다른 집계의 결과입니다."}}}
	for _, native := range []bool{false, true} {
		body := map[string]any{"model": "role", "stream": true}
		if native {
			body["input"] = "old"
		} else {
			body["messages"] = []any{}
		}
		raw, err := PrepareSourceSelection(mustJSON(body), data, true)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		_ = json.Unmarshal(raw, &result)
		if result["temperature"] != float64(0) {
			t.Fatal("source selection must be deterministic")
		}
		var input string
		if native {
			input = result["input"].(string)
			if result["response_format"] != nil || result["messages"] != nil {
				t.Fatal("unsupported native grammar fields")
			}
			if !strings.Contains(result["system_prompt"].(string), "local sentence numbers") {
				t.Fatal("native contract missing")
			}
		} else {
			input = prompt(result, 1)
			format := result["response_format"].(map[string]any)["json_schema"].(map[string]any)
			if format["name"] != "hint_source_sentences" {
				t.Fatal("missing machine-readable contract")
			}
			schema := format["schema"].(map[string]any)
			if schema["additionalProperties"] != false {
				t.Fatal("unbounded fields")
			}
		}
		var source map[string]any
		_ = json.Unmarshal([]byte(input), &source)
		records := source["records"].([]any)
		if len(records) != 2 || source["choices"] != nil || len(records[0].(map[string]any)["sentences"].([]any)) != 2 {
			t.Fatal("record grouping lost")
		}
		for i, url := range []string{"https://example.com/a", "https://example.com/b"} {
			if records[i].(map[string]any)["url"] != url {
				t.Fatal("source URL lost before model quality review")
			}
		}
	}
}
func TestUnresolvedTasksAndInternalFactContract(t *testing.T) {
	for _, task := range []string{"해당 물체가 무엇인지 알아내려면 추가 정보가 필요합니다.", "Ask for more context to identify this object", "Additional details are required to identify the person"} {
		_, _, err := AssembleRequest(mustJSON(map[string]any{"glasses_request": map[string]any{"version": 1, "task": "answer", "plan": map[string]any{"source": "internal", "question": task}}}))
		if !errors.Is(err, ErrUnresolvedTask) {
			t.Fatalf("unanswerable task accepted: %s %v", task, err)
		}
	}
	for _, task := range []string{"문 손잡이가 힘을 줄이는 지렛대 원리", "공식 접수 기한과 신청 방법에 대한 추가 정보를 제공", "공식 발표 자료에서 일정과 접수 마감일을 확인", "어떤 자료가 인과관계를 입증하는지 과학적 조건 설명"} {
		if !actionableHintTask(task) {
			t.Fatalf("clear general knowledge was blocked: %s", task)
		}
	}
	result := assemble(t, map[string]any{"task": "answer", "plan": map[string]any{"taskOriented": true, "source": "internal", "question": "문 손잡이의 원리", "topic": "손잡이", "kind": "key"}})
	format := result["response_format"].(map[string]any)["json_schema"].(map[string]any)
	if format["name"] != "hint_fact" || result["max_tokens"] != float64(1024) {
		t.Fatal("internal JSON contract missing")
	}
}

func TestHintResearchSeparatesTaskAndSearchQuery(t *testing.T) {
	task, query := "장치가 지원하는 최대 입력 전력", "장치 제조사 사용 설명서 입력 전력"
	result := assemble(t, map[string]any{"task": "answer", "plan": map[string]any{"taskOriented": true, "source": "web", "question": task, "searchQuery": query}})
	research := result["hint_research"].(map[string]any)
	if research["query"] != task || research["search_query"] != query {
		t.Fatalf("lost task/search separation: %+v", research)
	}
	judge := assemble(t, map[string]any{"task": "judge", "judge": map[string]any{"latest": "장치 입력 전력이 궁금해"}})
	schema := judge["response_format"].(map[string]any)["json_schema"].(map[string]any)["schema"].(map[string]any)
	if schema["properties"].(map[string]any)["search_query"] == nil {
		t.Fatal("planner cannot return a source-focused search query")
	}
}

func TestHintMemoryRequiresPersonalEvidenceRatherThanModelIntent(t *testing.T) {
	for _, tc := range []struct {
		task, speech string
		want         bool
	}{
		{"내 메모의 치과 예약 확인", "내 메모에서 치과 예약을 찾아줘", true},
		{"제 일정 확인", "제 일정에 다음 약속이 언제죠", true},
		{"Find my saved appointment", "When is my appointment", true},
		{"What did we decide last time", "What did we agree last time", true},
		{"국내 일정 확인", "국내 일정은 어떻게 되죠", false},
		{"우리나라 일정", "우리나라 일정에 대해 알려줘", false},
		{"내일 예약 확인", "내일 예약이 필요합니다", false},
		{"내 메모 확인", "What is the context of this utterance?", false},
		{"원자의 기원", "내 메모를 확인해줘", false},
	} {
		if got := AllowsHintMemory(tc.task, tc.speech); got != tc.want {
			t.Errorf("task=%q speech=%q: got %v want %v", tc.task, tc.speech, got, tc.want)
		}
	}
}
