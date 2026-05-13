package core

import (
	"strings"
	"testing"
)

func TestCompactRecentTurnContentPreservesSignalLines(t *testing.T) {
	long := strings.Repeat("intro text ", 40) +
		"\nerror: failed to open /tmp/demo.txt: permission denied" +
		"\n$ go test ./..." +
		"\n" + strings.Repeat("tail text ", 30)

	got := compactRecentTurnContent(long, 220)

	if !strings.Contains(got, "error: failed to open /tmp/demo.txt") {
		t.Fatalf("expected error line to survive compaction, got %q", got)
	}
	if !strings.Contains(got, "$ go test ./...") {
		t.Fatalf("expected command line to survive compaction, got %q", got)
	}
	if len([]rune(got)) > 260 {
		t.Fatalf("expected compacted text to stay reasonably close to budget, got %d chars", len([]rune(got)))
	}
}

func TestBuildRecentContextFromSnapshotUsesLatestTurnPriority(t *testing.T) {
	snapshot := chatSessionUISnapshot{
		Messages: []chatSessionMessageSnapshot{
			{
				TurnID:           "turn-1",
				UserContent:      "Earlier user question that should be compacted because it is older.",
				AssistantContent: "Earlier assistant answer that should also be compacted.",
			},
			{
				TurnID:           "turn-2",
				UserContent:      strings.Repeat("latest user context ", 60) + "\n/path/to/file.go:42",
				AssistantContent: strings.Repeat("latest assistant context ", 50) + "\nerror: panic recovered",
			},
		},
	}

	got, turns := buildRecentContextFromSnapshot(snapshot, 4)

	if turns != 2 {
		t.Fatalf("expected 2 turns, got %d", turns)
	}
	if !strings.Contains(got, "Turn -2") || !strings.Contains(got, "Turn -1") {
		t.Fatalf("expected relative turn labels in context, got %q", got)
	}
	if !strings.Contains(got, "/path/to/file.go:42") {
		t.Fatalf("expected latest turn file path to survive, got %q", got)
	}
	if !strings.Contains(got, "error: panic recovered") {
		t.Fatalf("expected latest assistant signal to survive, got %q", got)
	}
}

func TestWebEvidenceToolClassificationAndBudgetMessage(t *testing.T) {
	counts := map[string]int{
		"namu_wiki":            1,
		"read_buffered_source": 1,
		"search_web":           1,
		"execute_command":      3,
	}

	if !isWebEvidenceTool("namu_wiki") || !isWebEvidenceTool("read_web_page") || !isWebEvidenceTool("read_buffered_source") {
		t.Fatalf("expected Korean/search/page/buffer tools to count as web evidence")
	}
	if isWebEvidenceTool("execute_command") || isWebEvidenceTool("search_memory") {
		t.Fatalf("expected non-web tools to stay out of web evidence budget")
	}
	if got := totalToolUsageFor(counts, isWebEvidenceTool); got != 3 {
		t.Fatalf("expected 3 web evidence tool calls, got %d", got)
	}
	if got := totalToolUsageFor(counts, isWebSearchProviderTool); got != 2 {
		t.Fatalf("expected 2 web search provider calls, got %d", got)
	}

	message := webEvidenceBudgetMessage(3)
	if !strings.Contains(message, "Web evidence budget reached after 3 tool calls") {
		t.Fatalf("expected budget message to include call count, got %q", message)
	}
	if !strings.Contains(message, "If the evidence already returned is strong enough") {
		t.Fatalf("expected budget message to require evidence quality judgment, got %q", message)
	}
	if !strings.Contains(message, "ask whether to continue with deeper research") {
		t.Fatalf("expected budget message to offer deeper research when evidence is weak, got %q", message)
	}
}

func TestIsDeepWebResearchRequest(t *testing.T) {
	if !isDeepWebResearchRequest("여러 출처를 교차검증해서 깊게 조사해줘") {
		t.Fatalf("expected Korean deep research phrasing to expand web budget")
	}
	if !isDeepWebResearchRequest("please compare sources and dig deeper") {
		t.Fatalf("expected English deep research phrasing to expand web budget")
	}
	if isDeepWebResearchRequest("드라마 다모 개요 및 줄거리") {
		t.Fatalf("expected ordinary lookup to keep the normal web budget")
	}
}
