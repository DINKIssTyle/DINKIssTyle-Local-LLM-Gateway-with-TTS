package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Diagnostic history only; never included in transcripts, embeddings or chat
// memory retrieval. Body text is stored only for generated/displayed cards.
type HintEvent struct {
	ID         int64  `json:"id,omitempty"`
	UserID     string `json:"-"`
	EventID    string `json:"eventId"`
	SessionID  string `json:"sessionId"`
	SegmentID  string `json:"segmentId"`
	JobID      string `json:"jobId"`
	Stage      string `json:"stage"`
	Reason     string `json:"reason,omitempty"`
	Topic      string `json:"topic,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Source     string `json:"source,omitempty"`
	Text       string `json:"text,omitempty"`
	Evidence   string `json:"evidence,omitempty"`
	Version    string `json:"version"`
	OccurredAt string `json:"occurredAt,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
}

const hintEventSchema = `CREATE TABLE IF NOT EXISTS glasses_hint_events (
 id INTEGER PRIMARY KEY AUTOINCREMENT,user_id TEXT NOT NULL,event_id TEXT NOT NULL,session_id TEXT NOT NULL,segment_id TEXT NOT NULL,job_id TEXT NOT NULL,
 stage TEXT NOT NULL,reason TEXT NOT NULL,topic TEXT NOT NULL,kind TEXT NOT NULL,source TEXT NOT NULL,text TEXT NOT NULL,evidence TEXT NOT NULL,version TEXT NOT NULL,
 occurred_at TEXT NOT NULL,created_at TEXT NOT NULL,UNIQUE(user_id,event_id));
 CREATE INDEX IF NOT EXISTS glasses_hint_events_user_time ON glasses_hint_events(user_id,created_at,id);
CREATE INDEX IF NOT EXISTS glasses_hint_events_retention ON glasses_hint_events(created_at);`

func saveHintEvents(ctx context.Context, conn *sql.DB, user string, events []HintEvent) error {
	if conn == nil {
		return fmt.Errorf("database not initialized")
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z")
	for _, e := range events {
		_, err = tx.ExecContext(ctx, `INSERT INTO glasses_hint_events(user_id,event_id,session_id,segment_id,job_id,stage,reason,topic,kind,source,text,evidence,version,occurred_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id,event_id) DO NOTHING`, user, e.EventID, e.SessionID, e.SegmentID, e.JobID, e.Stage, e.Reason, e.Topic, e.Kind, e.Source, e.Text, e.Evidence, e.Version, e.OccurredAt, now)
		if err != nil {
			return err
		}
	}
	// All stages and body text expire together; bound each user's storage.
	_, err = tx.ExecContext(ctx, `DELETE FROM glasses_hint_events WHERE created_at<? OR (user_id=? AND id NOT IN (SELECT id FROM glasses_hint_events WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT 5000))`, time.Now().UTC().Add(-14*24*time.Hour).Format("2006-01-02T15:04:05.000000000Z"), user, user)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func SaveHintEvents(ctx context.Context, user string, events []HintEvent) error {
	if user == "" || len(events) < 1 || len(events) > 16 {
		return fmt.Errorf("invalid hint event batch")
	}
	return saveHintEvents(ctx, db, user, events)
}
func listHintEvents(ctx context.Context, conn *sql.DB, user, after string, limit int) ([]HintEvent, error) {
	if conn == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := conn.QueryContext(ctx, `SELECT id,event_id,session_id,segment_id,job_id,stage,reason,topic,kind,source,text,evidence,version,occurred_at,created_at FROM glasses_hint_events WHERE user_id=? AND created_at>? AND created_at>=? ORDER BY created_at DESC,id DESC LIMIT ?`, user, after, time.Now().UTC().Add(-14*24*time.Hour).Format("2006-01-02T15:04:05.000000000Z"), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []HintEvent{}
	for rows.Next() {
		var e HintEvent
		if err = rows.Scan(&e.ID, &e.EventID, &e.SessionID, &e.SegmentID, &e.JobID, &e.Stage, &e.Reason, &e.Topic, &e.Kind, &e.Source, &e.Text, &e.Evidence, &e.Version, &e.OccurredAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
func ListHintEvents(ctx context.Context, user, after string, limit int) ([]HintEvent, error) {
	return listHintEvents(ctx, db, user, after, limit)
}
