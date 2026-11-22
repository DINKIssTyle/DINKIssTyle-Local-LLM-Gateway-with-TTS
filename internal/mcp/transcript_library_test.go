package mcp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTranscriptLibraryDateSearchPaginationAndDeletion(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "library.db")); err != nil {
		t.Fatal(err)
	}
	defer CloseDB()
	for i, entry := range []TranscriptEntry{
		{UserID: "alice", StartedAt: "2026-11-20T14:59:59Z", Text: "전날 기록"},
		{UserID: "alice", StartedAt: "2026-11-20T15:00:00.001Z", Text: strings.Repeat("첫 기록 ", 100) + "후반 검색어"},
		{UserID: "alice", StartedAt: "2026-11-21T05:00:00Z", Text: "회의 일정"},
		{UserID: "alice", StartedAt: "2026-11-21T15:00:00Z", Text: "다음날 기록"},
		{UserID: "bob", StartedAt: "2026-11-20T15:00:00Z", Text: "다른 사용자 비밀"},
	} {
		entry.EndedAt = entry.StartedAt
		entry.SegmentID = string(rune('a' + i))
		if err := SaveTranscript(entry); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	days, more, err := ListTranscriptDays(ctx, "alice", "Asia/Seoul", "", "", 1)
	if err != nil || !more || len(days) != 1 || days[0].Date != "2026-11-22" {
		t.Fatalf("first page: %+v %t %v", days, more, err)
	}
	days, _, err = ListTranscriptDays(ctx, "alice", "Asia/Seoul", "", "2026-11-22", 30)
	if err != nil || len(days) != 2 || days[0].Date != "2026-11-21" || days[0].Count != 2 || len([]rune(days[0].Preview)) > 181 {
		t.Fatalf("day grouping: %+v %v", days, err)
	}
	for _, q := range []string{"후반 검색어", "2026-11-21"} {
		found, _, err := ListTranscriptDays(ctx, "alice", "Asia/Seoul", q, "", 30)
		if err != nil || len(found) != 1 || found[0].Count != 2 {
			t.Fatalf("full-day search %s: %+v %v", q, found, err)
		}
	}
	if !strings.HasPrefix(days[0].Preview, "첫 기록") || days[0].EndedAt != "2026-11-21T05:00:00Z" {
		t.Fatalf("chronological preview/time range lost: %+v", days[0])
	}
	found, _, _ := ListTranscriptDays(ctx, "bob", "Asia/Seoul", "회의", "", 30)
	if len(found) != 0 {
		t.Fatal("cross-user search leak")
	}
	count, err := DeleteTranscriptDay(ctx, "alice", "2026-11-21", "Asia/Seoul")
	if err != nil || count != 2 {
		t.Fatalf("deleted %d: %v", count, err)
	}
	found, _, _ = ListTranscriptDays(ctx, "alice", "Asia/Seoul", "", "", 30)
	if len(found) != 2 {
		t.Fatalf("neighboring day deleted: %+v", found)
	}
	found, _, _ = ListTranscriptDays(ctx, "bob", "Asia/Seoul", "", "", 30)
	if len(found) != 1 {
		t.Fatal("other user's day deleted")
	}
	if _, err := DeleteTranscriptDay(ctx, "alice", "", "Asia/Seoul"); err == nil {
		t.Fatal("empty deletion date accepted")
	}
	start, end, err := TranscriptDateRange("2026-03-08", "America/New_York")
	if err != nil || end.Sub(start) != 23*time.Hour {
		t.Fatalf("DST calendar day: %s %v", end.Sub(start), err)
	}
}
