package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type TranscriptDay struct {
	Date      string `json:"date"`
	Count     int    `json:"count"`
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt"`
	Preview   string `json:"preview"`
}

// TranscriptDateRange is shared by library endpoints and read-only chat tools.
// Calendar days are resolved in the caller's IANA zone, including DST changes.
func TranscriptDateRange(date, zone string) (time.Time, time.Time, error) {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid timezone")
	}
	start, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date")
	}
	return start.UTC(), start.AddDate(0, 0, 1).UTC(), nil
}

func ListTranscriptDays(ctx context.Context, user, zone, query, before string, limit int) ([]TranscriptDay, bool, error) {
	if db == nil || user == "" {
		return nil, false, fmt.Errorf("authenticated transcript database required")
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, false, fmt.Errorf("invalid timezone")
	}
	if limit < 1 || limit > 100 {
		limit = 30
	}
	bound := "9999-12-31T00:00:00"
	if before != "" {
		start, _, err := TranscriptDateRange(before, zone)
		if err != nil {
			return nil, false, err
		}
		bound = start.Format("2006-01-02T15:04:05")
	}
	// Only bounded previews cross the SQL boundary. Matching uses full source text;
	// counts and time ranges always describe the whole day, not only matched rows.
	query = strings.TrimSpace(query)
	rows, err := db.QueryContext(ctx, `SELECT started_at,ended_at,substr(text,1,181),CASE WHEN ?='' THEN 1 ELSE instr(lower(text),lower(?))>0 END FROM stt_transcripts WHERE user_id=? AND started_at<? ORDER BY started_at DESC,id DESC`, query, query, user, bound)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	days := map[string]*TranscriptDay{}
	matches := map[string]bool{}
	needle := strings.ToLower(strings.TrimSpace(query))
	for rows.Next() {
		var start, end, preview string
		var match bool
		if err := rows.Scan(&start, &end, &preview, &match); err != nil {
			return nil, false, err
		}
		at, err := time.Parse(time.RFC3339Nano, start)
		if err != nil {
			return nil, false, fmt.Errorf("invalid stored transcript time")
		}
		date := at.In(loc).Format("2006-01-02")
		day := days[date]
		if day == nil {
			// Browsing needs just one extra complete day to establish pagination.
			if needle == "" && len(days) > limit {
				break
			}
			day = &TranscriptDay{Date: date, EndedAt: end}
			days[date] = day
		}
		day.Count++
		day.StartedAt = start
		// Prepend earlier speech so the preview stays chronological even though
		// the index is traversed newest-first for efficient date pagination.
		text := strings.Join(strings.Fields(preview), " ")
		combined := strings.TrimSpace(text + " " + day.Preview)
		chars := []rune(combined)
		if len(chars) > 180 {
			combined = string(chars[:180]) + "…"
		}
		day.Preview = combined
		matches[date] = matches[date] || match || strings.Contains(date, needle)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	items := []TranscriptDay{}
	for date, day := range days {
		if matches[date] {
			items = append(items, *day)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Date > items[j].Date })
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, nil
}

func DeleteTranscriptDay(ctx context.Context, user, date, zone string) (int64, error) {
	if db == nil || user == "" {
		return 0, fmt.Errorf("authenticated transcript database required")
	}
	start, end, err := TranscriptDateRange(date, zone)
	if err != nil {
		return 0, err
	}
	result, err := db.ExecContext(ctx, `DELETE FROM stt_transcripts WHERE user_id=? AND started_at>=? AND started_at<?`, user, start.Format("2006-01-02T15:04:05"), end.Format("2006-01-02T15:04:05"))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ReadTranscriptPage bounds tool results without loading an entire day's text.
func ReadTranscriptPage(ctx context.Context, user, date, zone string, offset, limit int) ([]TranscriptEntry, bool, error) {
	if db == nil || user == "" {
		return nil, false, fmt.Errorf("authenticated transcript database required")
	}
	if offset < 0 || limit < 1 || limit > 100 {
		return nil, false, fmt.Errorf("invalid transcript page")
	}
	start, end, err := TranscriptDateRange(date, zone)
	if err != nil {
		return nil, false, err
	}
	rows, err := db.QueryContext(ctx, `SELECT id,session_id,segment_id,started_at,ended_at,text,trigger_word,triggered FROM stt_transcripts WHERE user_id=? AND started_at>=? AND started_at<? ORDER BY started_at,id LIMIT ? OFFSET ?`, user, start.Format("2006-01-02T15:04:05"), end.Format("2006-01-02T15:04:05"), limit+1, offset)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	entries := []TranscriptEntry{}
	for rows.Next() {
		var entry TranscriptEntry
		if err := rows.Scan(&entry.ID, &entry.SessionID, &entry.SegmentID, &entry.StartedAt, &entry.EndedAt, &entry.Text, &entry.TriggerWord, &entry.Triggered); err != nil {
			return nil, false, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(entries) > limit
	if more {
		entries = entries[:limit]
	}
	return entries, more, nil
}
