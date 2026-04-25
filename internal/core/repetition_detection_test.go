package core

import (
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
