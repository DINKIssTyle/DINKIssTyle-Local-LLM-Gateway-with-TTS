package chatharness

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestCalendarEvidencePreservesLateEventsAndCoverage(t *testing.T) {
	items := []any{}
	for i := 0; i < 20; i++ {
		items = append(items, map[string]any{"id": fmt.Sprint(i), "summary": fmt.Sprintf("정확한 제목 %d", i), "location": "회의실 서울", "start": map[string]string{"dateTime": "2026-11-19T17:30:00+09:00"}, "end": map[string]string{"dateTime": "2026-11-19T18:45:00+09:00"}, "calendar_id": "family", "calendar_name": "공유 캘린더", "description": strings.Repeat("긴 설명", 1000), "attendees": []string{strings.Repeat("unneeded", 1000)}})
	}
	raw, _ := json.Marshal(map[string]any{"items": items, "complete": false, "calendars_failed": []any{map[string]string{"calendar_id": "blocked", "error": "permission denied"}}, "next_pages": []any{map[string]string{"calendar_id": "family", "page_token": "more"}}})
	compact, ok := calendarEvidence("google_calendar_search", string(raw))
	if !ok {
		t.Fatal("calendar evidence not recognized")
	}
	var output struct {
		Items []struct {
			Summary              string            `json:"summary"`
			Location             string            `json:"location"`
			Start                map[string]string `json:"start"`
			End                  map[string]string `json:"end"`
			DescriptionTruncated bool              `json:"description_truncated"`
			CalendarID           string            `json:"calendar_id"`
		} `json:"items"`
		Failed []any `json:"calendars_failed"`
		Next   []any `json:"next_pages"`
	}
	if err := json.Unmarshal([]byte(compact), &output); err != nil {
		t.Fatal("evidence is not complete JSON", err)
	}
	if len(output.Items) != 20 || len(output.Failed) != 1 || len(output.Next) != 1 {
		t.Fatal("returned events or coverage were lost")
	}
	last := output.Items[19]
	if last.Summary != "정확한 제목 19" || last.Location != "회의실 서울" || last.Start["dateTime"] != "2026-11-19T17:30:00+09:00" || last.End["dateTime"] != "2026-11-19T18:45:00+09:00" || last.CalendarID != "family" || !last.DescriptionTruncated {
		t.Fatalf("event fields lost: %+v", last)
	}
	for _, mode := range []string{"standard", "stateful"} {
		t.Run(mode, func(t *testing.T) {
			_, body, err := PrepareToolFollowupRequest(ToolFollowupInput{LLMMode: mode, ToolName: "google_calendar_search", ToolResult: string(raw), ToolCallID: "call1", ReqMap: map[string]any{}, OriginalUserText: "오늘 일정 상세 알려줘"})
			if err != nil {
				t.Fatal(err)
			}
			var request map[string]any
			if err = json.Unmarshal(body, &request); err != nil {
				t.Fatal(err)
			}
			var content string
			if mode == "stateful" {
				content = request["input"].(string)
			} else {
				messages := request["messages"].([]any)
				content = messages[len(messages)-1].(map[string]any)["content"].(string)
			}
			if !strings.Contains(content, compact) || !strings.Contains(content, "정확한 제목 19") {
				t.Fatal("model follow-up cut calendar evidence")
			}
		})
	}
}
func TestCalendarDetailAndListPreserveCompleteFields(t *testing.T) {
	title := strings.Repeat("정확한 일정 제목", 300)
	raw, _ := json.Marshal(map[string]any{"id": "event", "summary": title, "location": "장소", "start": map[string]string{"date": "2026-11-19"}, "end": map[string]string{"date": "2026-11-20"}})
	out := CompactToolResult("google_calendar_read", string(raw), "일정 내용")
	if !strings.Contains(out, title) || !strings.Contains(out, "2026-11-20") {
		t.Fatal("long title or all-day date truncated")
	}
	list, _ := json.Marshal(map[string]any{"items": []any{map[string]any{"id": "calendar", "summary": title, "timeZone": "Asia/Seoul"}}, "nextPageToken": "next"})
	if out = CompactToolResult("google_calendar_list", string(list), "캘린더 목록"); !strings.Contains(out, title) || !strings.Contains(out, "nextPageToken") {
		t.Fatal("calendar list truncated")
	}
	if _, ok := calendarEvidence("google_calendar_search", "not JSON"); ok {
		t.Fatal("invalid response accepted")
	}
	if _, ok := calendarEvidence("search_web", string(raw)); ok {
		t.Fatal("unrelated tool treated as calendar")
	}
}

func TestHUDToolFollowupRetainsEvidenceAndConciseFormat(t *testing.T) {
	for _, mode := range []string{"standard", "stateful"} {
		_, body, err := PrepareToolFollowupRequest(ToolFollowupInput{HUDOutput: true, LLMMode: mode, ToolName: "google_calendar_search", ToolResult: `{"items":[{"id":"event","calendar_id":"family","summary":"병원 예약","start":{"dateTime":"2026-11-19T10:00:00+09:00"},"location":"서울"}],"complete":true}`, ToolCallID: "call1", ReqMap: map[string]any{}, OriginalUserText: "오늘 일정"})
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, want := range []string{"병원 예약", "2026-11-19T10:00:00+09:00", "서울", "SMART GLASSES OUTPUT FORMAT", "500 Unicode characters", "Omit source names"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s lost %s", mode, want)
			}
		}
		if strings.Contains(text, "cite its source links") || strings.Contains(text, "with citations when supported") {
			t.Fatal("web citation requirements overrode HUD format")
		}
	}
	plain := CompactToolResult("search_web", "evidence", "오늘 소식")
	if !strings.Contains(plain, "cite its source links") || strings.Contains(plain, "SMART GLASSES OUTPUT FORMAT") {
		t.Fatal("web chat output changed")
	}
}
