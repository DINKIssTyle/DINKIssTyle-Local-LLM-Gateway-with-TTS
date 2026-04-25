package core

import (
	"fmt"
	"strings"
	"testing"
)

func TestDetectReasoningRunawayRepetitionChunkLoop(t *testing.T) {
	repeated := strings.Repeat("검토 중입니다. ", 12)
	snippet, source, ok := detectReasoningRunawayRepetition(strings.Repeat("prefix ", 30) + repeated)

	if !ok {
		t.Fatal("expected repeated reasoning chunk to be detected")
	}
	if source != "chunk-loop" {
		t.Fatalf("expected chunk-loop source, got %q", source)
	}
	if !strings.Contains(snippet, "검토") {
		t.Fatalf("expected snippet to include repeated text, got %q", snippet)
	}
}

func TestDetectReasoningRunawayRepetitionWordLoop(t *testing.T) {
	text := strings.Repeat("ordinary reasoning context ", 12) + strings.Repeat("again ", 12)
	snippet, source, ok := detectReasoningRunawayRepetition(text)

	if !ok {
		t.Fatal("expected repeated word reasoning to be detected")
	}
	if source != "word-loop" && source != "chunk-loop" {
		t.Fatalf("expected word or chunk loop source, got %q", source)
	}
	if !strings.Contains(snippet, "again") {
		t.Fatalf("expected repeated word snippet, got %q", snippet)
	}
}

func TestDetectReasoningRunawayRepetitionIgnoresNormalText(t *testing.T) {
	parts := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		parts = append(parts, "This reasoning step advances with varied detail number "+string(rune('A'+i))+".")
	}
	text := strings.Join(parts, " ")
	if snippet, source, ok := detectReasoningRunawayRepetition(text); ok {
		t.Fatalf("expected normal text to pass, got snippet=%q source=%q", snippet, source)
	}
}

func TestDetectReasoningRunawayRepetitionRepeatedSection(t *testing.T) {
	repeatedSentence := `사용자가 "간단히 다시 알려주세요"라고 한 것은, 아마도 이전에 요청했던 복잡한 정보를 더 간결하게 요약해달라는 의미일 것입니다.`
	parts := []string{"초기 reasoning 문맥입니다."}
	for i := 0; i < 5; i++ {
		parts = append(parts,
			fmt.Sprintf("고유한 중간 검토 %d입니다.", i),
			repeatedSentence,
			fmt.Sprintf("다음 후보를 %d번째로 비교합니다.", i),
		)
	}

	snippet, source, ok := detectReasoningRunawayRepetition(strings.Join(parts, "\n\n"))
	if !ok {
		t.Fatal("expected repeated reasoning section to be detected")
	}
	if source != "repeated-section" {
		t.Fatalf("expected repeated-section source, got %q", source)
	}
	if !strings.Contains(snippet, "간단히 다시 알려주세요") {
		t.Fatalf("expected snippet to include repeated section, got %q", snippet)
	}
}

func TestDetectReasoningPayloadRunawayRepetitionFromChatEnd(t *testing.T) {
	repeatedBlock := strings.Join([]string{
		`사용자가 "간단히 다시 알려주세요"라고 한 것은, 아마도 이전에 요청했던 복잡한 정보를 더 간결하게 요약해달라는 의미일 것입니다.`,
		`하지만 사용자가 정확히 무엇을 원하는지 명확하지 않으므로, 어떤 내용을 간단히 정리해드릴지 확인하는 것이 안전할 것 같습니다.`,
		`가장 최근의 주제는 "마이 페이버릿 쓰리츠"였으니, 이를 간단히 요약해드리겠습니다.`,
		`My Favorite Things는 사운드 오브 뮤직의 대표곡으로, 불안한 상황을 위로하는 메시지가 담겨 있습니다.`,
	}, "\n\n")
	payload := map[string]interface{}{
		"type": "chat.end",
		"result": map[string]interface{}{
			"output": []interface{}{
				map[string]interface{}{
					"type":    "reasoning",
					"content": strings.Repeat(repeatedBlock+"\n\n", 4),
				},
			},
		},
	}

	snippet, source, ok := detectReasoningPayloadRunawayRepetition(payload)
	if !ok {
		t.Fatal("expected chat.end reasoning payload repetition to be detected")
	}
	if source != "section-loop" && source != "repeated-section" {
		t.Fatalf("expected section-based source, got %q", source)
	}
	if !strings.Contains(snippet, "간단히 다시 알려주세요") {
		t.Fatalf("expected snippet to include repeated reasoning, got %q", snippet)
	}
}
