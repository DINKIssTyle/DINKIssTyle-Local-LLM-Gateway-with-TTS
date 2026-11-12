package mcp

import (
	"path/filepath"
	"testing"
)

func TestTranscriptsIsolationAndRetry(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "transcripts.db")); err != nil {
		t.Fatal(err)
	}
	defer CloseDB()
	entry := TranscriptEntry{UserID: "alice", SessionID: "session", SegmentID: "segment", StartedAt: "2026-11-11T15:01:00Z", EndedAt: "2026-11-11T15:01:02Z", Text: "이븐 오늘 일정 알려줘", TriggerWord: "이븐", Triggered: true}
	if err := SaveTranscript(entry); err != nil {
		t.Fatal(err)
	}
	if err := SaveTranscript(entry); err != nil {
		t.Fatal(err)
	}
	rows, err := ListTranscripts("alice", "2026-11-11T15:00:00Z", "2026-11-12T15:00:00Z")
	if err != nil || len(rows) != 1 || !rows[0].Triggered || rows[0].Text != entry.Text {
		t.Fatalf("retry/date query: %+v %v", rows, err)
	}
	rows, err = ListTranscripts("bob", "2026-11-11T15:00:00Z", "2026-11-12T15:00:00Z")
	if err != nil || len(rows) != 0 {
		t.Fatalf("cross-user transcript leak: %+v %v", rows, err)
	}
	rows, err = ListTranscripts("alice", "2026-11-12T15:00:00Z", "2026-11-13T15:00:00Z")
	if err != nil || len(rows) != 0 {
		t.Fatalf("wrong day: %+v %v", rows, err)
	}
}
