package chatharness

import (
	"encoding/json"
	"strings"
	"time"
)

// Calendar results are structured evidence. Cutting the serialized JSON at a
// character boundary can remove every event after the coverage metadata, or
// split an event before its title/time. Preserve all returned event identities
// and schedule fields; shorten only the independently marked descriptions.
func calendarEvidence(tool, result string) (string, bool) {
	switch strings.TrimSpace(tool) {
	case "google_calendar_search", "google_calendar_read", "google_calendar_list":
	default:
		return "", false
	}
	var root map[string]json.RawMessage
	if json.Unmarshal([]byte(result), &root) != nil || root == nil {
		return "", false
	}
	var out map[string]json.RawMessage
	if tool == "google_calendar_read" {
		if _, ok := root["id"]; !ok {
			return "", false
		}
		out = calendarEventEvidence(root)
	} else {
		raw, ok := root["items"]
		if !ok {
			return "", false
		}
		var items []map[string]json.RawMessage
		if json.Unmarshal(raw, &items) != nil {
			return "", false
		}
		kept := make([]map[string]json.RawMessage, 0, len(items))
		for _, item := range items {
			if tool == "google_calendar_list" {
				kept = append(kept, calendarFields(item, []string{"id", "summary", "summaryOverride", "primary", "timeZone", "accessRole", "hidden", "selected"}))
			} else {
				kept = append(kept, calendarEventEvidence(item))
			}
		}
		out = calendarFields(root, []string{"scope", "summary", "timeZone", "effective_range", "calendars_checked", "calendars_failed", "next_pages", "nextPageToken", "partial", "complete"})
		out["items"], _ = json.Marshal(kept)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", false
	}
	return string(b), true
}
func calendarFields(input map[string]json.RawMessage, keys []string) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(keys))
	for _, key := range keys {
		if value, ok := input[key]; ok {
			out[key] = value
		}
	}
	return out
}
func calendarEventEvidence(event map[string]json.RawMessage) map[string]json.RawMessage {
	out := calendarFields(event, []string{"id", "summary", "start", "end", "location", "htmlLink", "status", "calendar_id", "calendar_name", "description", "organizer", "recurringEventId", "originalStartTime", "eventType", "transparency", "visibility"})
	var start, end struct {
		Date string `json:"date"`
	}
	if json.Unmarshal(out["start"], &start) == nil && json.Unmarshal(out["end"], &end) == nil && start.Date != "" && end.Date != "" {
		first, firstErr := time.Parse(time.DateOnly, start.Date)
		until, untilErr := time.Parse(time.DateOnly, end.Date)
		if firstErr == nil && untilErr == nil && until.After(first) {
			out["all_day_start_date"], _ = json.Marshal(start.Date)
			out["all_day_last_date_inclusive"], _ = json.Marshal(until.AddDate(0, 0, -1).Format(time.DateOnly))
			delete(out, "start")
			delete(out, "end")
		}
	}
	if raw, ok := out["description"]; ok {
		var description string
		if json.Unmarshal(raw, &description) == nil {
			runes := []rune(description)
			if len(runes) > 2000 {
				out["description"], _ = json.Marshal(string(runes[:2000]))
				out["description_truncated"] = json.RawMessage(`true`)
			}
		}
	}
	return out
}
