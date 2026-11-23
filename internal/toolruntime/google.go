package toolruntime

import (
	"context"
	"encoding/json"
	"fmt"

	"dinkisstyle-chat/internal/googleintegration"
)

func registerGoogleTools(r *Registry) {
	type spec struct {
		name, description string
		required          []string
		properties        map[string]any
	}
	str := func(description string) any { return map[string]any{"type": "string", "description": description} }
	common := func() map[string]any {
		return map[string]any{"page_token": str("Continuation token from the previous result"), "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 10}}
	}
	gmail := common()
	gmail["query"] = str("Optional Gmail filter. Omit for the latest inbox messages; translate natural-language conditions yourself. Never ask the user to supply Gmail syntax.")
	drive := common()
	drive["query"] = str("Optional keyword for file names and indexed contents. Omit to list the 10 most recently modified files. Translate natural language yourself; do not require search syntax.")
	calendar := common()
	calendar["calendar_id"] = str("Omit or use all to search every connected/subscribed calendar (default). Set a specific calendar ID, including primary, only when the user requests that calendar.")
	calendar["query"] = str("Optional event keyword")
	calendar["time_min"] = str("Optional inclusive range start, RFC3339 with timezone offset. Omit for the selected period.")
	calendar["time_max"] = str("Optional exclusive range end, RFC3339 with timezone offset. Defaults to 7 days after start.")
	calendar["time_zone"] = str("Optional IANA time zone, e.g. Asia/Seoul. Defaults to the primary calendar time zone for all-calendar searches, or the selected calendar time zone.")
	calendar["period"] = map[string]any{"type": "string", "enum": []string{"upcoming", "today", "tomorrow", "this_week", "next_week", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}, "description": "Relative date range resolved by the server. Default upcoming: next 7 days. Use today/tomorrow for 오늘/내일 and a weekday (e.g. wednesday for 수요일) for the next occurrence of that day, including today. Weeks start Monday. For a specific date or a weekday in a specified week, use explicit timestamps."}
	specs := []spec{
		{"gmail_search", "Get recent email or search this account's connected Gmail. Call with {} for the latest 10 inbox messages. Returns sender, subject, date and snippets; use gmail_read only for full body details.", nil, gmail},
		{"gmail_read", "Read a Gmail message body by ID returned by gmail_search. Attachments are not read.", []string{"id"}, map[string]any{"id": str("Message ID from gmail_search")}},
		{"google_drive_search", "List recent files or search connected Google Drive. Call with {} for the 10 most recently modified files. File lists need no body reads; use google_drive_read for contents.", nil, drive},
		{"google_drive_read", "Read Drive text files or export Google Docs/Slides as text and Sheets as first-sheet CSV. PDF/Office/image extraction is not supported.", []string{"id"}, map[string]any{"id": str("File ID from google_drive_search")}},
		{"google_calendar_list", "List calendars available to this account's connected Google account, including IDs and time zones.", nil, map[string]any{"page_token": str("Continuation token")}},
		{"google_calendar_search", "Check connected Google Calendar. Call with {} for upcoming events in the next 7 days, or period:today/tomorrow/this_week/next_week. No manually formatted dates are needed for these requests. Searches ALL connected/subscribed calendars by default, including shared calendars. Do not set primary for a general schedule question. Each result identifies its calendar. limit is per calendar; next_pages gives per-calendar continuation. Expands recurring events.", nil, calendar},
		{"google_calendar_read", "Read one Google Calendar event using its returned calendar_id and event ID. Preserve the source calendar for shared/subscribed events.", []string{"id", "calendar_id"}, map[string]any{"id": str("Event ID from google_calendar_search"), "calendar_id": str("Source calendar_id returned with the event")}},
	}
	for _, s := range specs {
		schema := map[string]any{"type": "object", "properties": s.properties, "additionalProperties": false}
		if len(s.required) > 0 {
			schema["required"] = s.required
		}
		raw, _ := json.Marshal(schema)
		name := s.name
		if err := r.Register(Definition{Name: name, Description: s.description, InputSchema: raw, Metadata: Metadata{Category: "google", ReadOnly: true}}, func(ctx context.Context, exec ExecutionContext, args json.RawMessage) (Result, error) {
			if exec.Google == nil {
				return Result{IsError: true}, fmt.Errorf("Google is not configured")
			}
			content, err := exec.Google.Execute(ctx, exec.UserID, name, args)
			return Result{Content: content, IsError: err != nil}, err
		}); err != nil {
			panic(err)
		}
	}
}
func googleToolAllowed(exec ExecutionContext, name string) bool {
	service := googleintegration.ServiceForTool(name)
	return service == "" || (exec.Google != nil && exec.Google.Allowed(exec.UserID, service))
}
