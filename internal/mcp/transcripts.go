package mcp

import "fmt"

type TranscriptEntry struct {
	ID          int64  `json:"id"`
	UserID      string `json:"-"`
	SessionID   string `json:"sessionId"`
	SegmentID   string `json:"segmentId"`
	StartedAt   string `json:"startedAt"`
	EndedAt     string `json:"endedAt"`
	Text        string `json:"text"`
	TriggerWord string `json:"triggerWord"`
	Triggered   bool   `json:"triggered"`
}

func SaveTranscript(entry TranscriptEntry) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	if entry.UserID == "" || entry.Text == "" {
		return fmt.Errorf("transcript user and text are required")
	}
	_, err := db.Exec(`INSERT INTO stt_transcripts(user_id,session_id,segment_id,started_at,ended_at,text,trigger_word,triggered)
	 VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(user_id,segment_id) WHERE segment_id != '' DO NOTHING`,
		entry.UserID, entry.SessionID, entry.SegmentID, entry.StartedAt, entry.EndedAt, entry.Text, entry.TriggerWord, entry.Triggered)
	return err
}

func ListTranscripts(userID, start, end string) ([]TranscriptEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := db.Query(`SELECT id,session_id,segment_id,started_at,ended_at,text,trigger_word,triggered FROM stt_transcripts WHERE user_id=? AND started_at>=? AND started_at<? ORDER BY started_at,id`, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []TranscriptEntry{}
	for rows.Next() {
		var entry TranscriptEntry
		if err := rows.Scan(&entry.ID, &entry.SessionID, &entry.SegmentID, &entry.StartedAt, &entry.EndedAt, &entry.Text, &entry.TriggerWord, &entry.Triggered); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
