package googleintegration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"time"
)

type calendarEntry struct {
	ID              string `json:"id"`
	Summary         string `json:"summary"`
	SummaryOverride string `json:"summaryOverride"`
	Primary         bool   `json:"primary"`
	TimeZone        string `json:"timeZone"`
	AccessRole      string `json:"accessRole"`
}

func (c calendarEntry) name() string {
	if c.SummaryOverride != "" {
		return c.SummaryOverride
	}
	if c.Summary != "" {
		return c.Summary
	}
	return c.ID
}

// A general schedule request is a server-enforced search across every calendar
// in the account's list, rather than a model-dependent series of tool calls.
func (m *Manager) searchAllCalendars(ctx context.Context, user string, a Arguments) (string, error) {
	if a.PageToken != "" {
		return "", fmt.Errorf("for pagination use next_pages: provide its calendar_id, page_token and effective_range")
	}
	calendars := []calendarEntry{}
	seenIDs := map[string]bool{}
	seenPages := map[string]bool{}
	page := ""
	for {
		q := url.Values{"maxResults": {"250"}, "showHidden": {"true"}, "showDeleted": {"false"}, "fields": {"nextPageToken,items(id,summary,summaryOverride,primary,timeZone,accessRole)"}}
		if page != "" {
			q.Set("pageToken", page)
		}
		b, err := m.get(ctx, user, "calendar", "/calendar/v3/users/me/calendarList", q)
		if err != nil {
			return "", err
		}
		var list struct {
			Items []calendarEntry `json:"items"`
			Next  string          `json:"nextPageToken"`
		}
		if err = json.Unmarshal(b, &list); err != nil {
			return "", err
		}
		for _, calendar := range list.Items {
			if calendar.ID != "" && !seenIDs[calendar.ID] {
				seenIDs[calendar.ID] = true
				calendars = append(calendars, calendar)
			}
		}
		if list.Next == "" {
			break
		}
		if seenPages[list.Next] {
			return "", fmt.Errorf("Google returned a repeated calendar-list page token; calendar coverage could not be verified")
		}
		seenPages[list.Next] = true
		page = list.Next
	}
	// Resolve local dates once. A calendar in another zone must not shift the
	// user's requested day when querying that calendar's events.
	if (a.TimeMin == "" || a.TimeMax == "") && a.TimeZone == "" {
		for _, calendar := range calendars {
			if calendar.Primary {
				a.TimeZone = calendar.TimeZone
				break
			}
		}
		if a.TimeZone == "" {
			b, err := m.get(ctx, user, "calendar", "/calendar/v3/users/me/calendarList/primary", url.Values{"fields": {"timeZone"}})
			if err != nil {
				return "", err
			}
			var primary calendarEntry
			if err = json.Unmarshal(b, &primary); err != nil {
				return "", err
			}
			a.TimeZone = primary.TimeZone
		}
	}
	var err error
	a.TimeMin, a.TimeMax, err = calendarRange(a, time.Now())
	if err != nil {
		return "", err
	}
	items := []map[string]any{}
	checked := []map[string]any{}
	failed := []map[string]string{}
	nextPages := []map[string]string{}
	for _, calendar := range calendars {
		if err = ctx.Err(); err != nil {
			return "", err
		}
		if calendar.AccessRole == "freeBusyReader" || calendar.AccessRole == "none" {
			failed = append(failed, map[string]string{"calendar_id": calendar.ID, "calendar_name": calendar.name(), "error": "Only free/busy access is available; event details could not be read."})
			continue
		}
		request := a
		request.CalendarID = calendar.ID
		raw, _ := json.Marshal(request)
		out, callErr := m.Execute(ctx, user, "google_calendar_search", raw)
		if callErr != nil {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			// A permission change or disconnect stops the entire search immediately.
			if !m.Allowed(user, "calendar") {
				return "", callErr
			}
			failed = append(failed, map[string]string{"calendar_id": calendar.ID, "calendar_name": calendar.name(), "error": callErr.Error()})
			continue
		}
		var result struct {
			Items []map[string]any `json:"items"`
			Next  string           `json:"nextPageToken"`
		}
		if err = json.Unmarshal([]byte(out), &result); err != nil {
			return "", err
		}
		for _, event := range result.Items {
			event["calendar_id"] = calendar.ID
			event["calendar_name"] = calendar.name()
			items = append(items, event)
		}
		checked = append(checked, map[string]any{"calendar_id": calendar.ID, "calendar_name": calendar.name(), "events_returned": len(result.Items)})
		if result.Next != "" {
			nextPages = append(nextPages, map[string]string{"calendar_id": calendar.ID, "calendar_name": calendar.name(), "page_token": result.Next, "time_min": a.TimeMin, "time_max": a.TimeMax, "time_zone": a.TimeZone})
		}
	}
	loc := time.UTC
	if start, e := time.Parse(time.RFC3339, a.TimeMin); e == nil {
		loc = start.Location()
	}
	if a.TimeZone != "" {
		if zone, e := time.LoadLocation(a.TimeZone); e == nil {
			loc = zone
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return eventStart(items[i], loc).Before(eventStart(items[j], loc)) })
	return resultJSON(map[string]any{"scope": "all_connected_calendars", "items": items, "calendars_checked": checked, "calendars_failed": failed, "next_pages": nextPages, "partial": len(failed) > 0, "complete": len(failed) == 0 && len(nextPages) == 0, "effective_range": map[string]string{"time_min": a.TimeMin, "time_max": a.TimeMax, "time_zone": a.TimeZone}})
}
func eventStart(event map[string]any, loc *time.Location) time.Time {
	start, _ := event["start"].(map[string]any)
	if value, ok := start["dateTime"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			return parsed
		}
	}
	if value, ok := start["date"].(string); ok {
		if parsed, err := time.ParseInLocation("2006-01-02", value, loc); err == nil {
			return parsed
		}
	}
	return time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
}
