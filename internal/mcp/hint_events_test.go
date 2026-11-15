package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
)

func TestHintEventsIsolationRetentionAndRetry(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetMaxOpenConns(1)
	if _, err = conn.Exec(hintEventSchema); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	e := HintEvent{EventID: "one", JobID: "job", Stage: "generated", Text: "synthetic hint", Version: "test"}
	if err = saveHintEvents(ctx, conn, "alice", []HintEvent{e, e}); err != nil {
		t.Fatal(err)
	}
	if err = saveHintEvents(ctx, conn, "bob", []HintEvent{e}); err != nil {
		t.Fatal(err)
	}
	a, err := listHintEvents(ctx, conn, "alice", "", 100)
	if err != nil || len(a) != 1 || a[0].Text != e.Text {
		t.Fatalf("retry/isolation %+v %v", a, err)
	}
	if _, err = conn.Exec(`UPDATE glasses_hint_events SET created_at=? WHERE user_id='alice'`, time.Now().UTC().Add(-15*24*time.Hour).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	a, err = listHintEvents(ctx, conn, "alice", "", 100)
	if err != nil || len(a) != 0 {
		t.Fatal("expired events visible", a, err)
	}
	events := make([]HintEvent, 5001)
	for i := range events {
		events[i] = HintEvent{EventID: fmt.Sprint(i), JobID: "job", Stage: "decision"}
	}
	if err = saveHintEvents(ctx, conn, "alice", events); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = conn.QueryRow(`SELECT count(*) FROM glasses_hint_events WHERE user_id='alice'`).Scan(&n); err != nil || n != 5000 {
		t.Fatal("retention bound", n, err)
	}
	b, err := listHintEvents(ctx, conn, "bob", "", 200)
	if err != nil || len(b) != 1 {
		t.Fatal("other user deleted", b, err)
	}
	// No transcript or memory table is created or populated by hint history.
	if err = conn.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name IN ('stt_transcripts','memories','chat_events')`).Scan(&n); err != nil || n != 0 {
		t.Fatal("hint history contaminated memory", n, err)
	}
}
