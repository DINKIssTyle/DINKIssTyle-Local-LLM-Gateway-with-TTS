package googleintegration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCalendarSearchIncludesSubscriptionsAndReportsPartialResults(t *testing.T) {
	requests := map[string]int{}
	var rangeMin, rangeMax string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests[r.URL.Path]++
		q := r.URL.Query()
		switch r.URL.Path {
		case "/calendar/v3/users/me/calendarList":
			if q.Get("showHidden") != "true" {
				t.Error("hidden subscriptions omitted")
			}
			if q.Get("pageToken") == "" {
				io.WriteString(w, `{"items":[{"id":"owner","summary":"Personal","primary":true,"timeZone":"Asia/Seoul","accessRole":"owner"},{"id":"shared","summary":"Shared","summaryOverride":"Family","timeZone":"America/New_York","accessRole":"reader","hidden":true,"selected":false}],"nextPageToken":"more-calendars"}`)
			} else if q.Get("pageToken") == "more-calendars" {
				io.WriteString(w, `{"items":[{"id":"holidays","summary":"Holidays","accessRole":"reader"},{"id":"forbidden","summary":"Blocked","accessRole":"reader"},{"id":"busy","summary":"Busy only","accessRole":"freeBusyReader"}]}`)
			} else {
				t.Error("wrong calendar page")
			}
		case "/calendar/v3/calendars/owner/events", "/calendar/v3/calendars/shared/events", "/calendar/v3/calendars/holidays/events", "/calendar/v3/calendars/forbidden/events":
			if rangeMin == "" {
				rangeMin = q.Get("timeMin")
				rangeMax = q.Get("timeMax")
			}
			if q.Get("timeMin") != rangeMin || q.Get("timeMax") != rangeMax || q.Get("timeZone") != "Asia/Seoul" {
				t.Error("calendars were searched using different date ranges")
			}
			if q.Get("q") != "meeting" {
				t.Error("keyword lost")
			}
			switch r.URL.Path {
			case "/calendar/v3/calendars/owner/events":
				io.WriteString(w, `{"items":[{"id":"personal-event","start":{"dateTime":"2026-11-19T12:00:00+09:00"}}]}`)
			case "/calendar/v3/calendars/shared/events":
				io.WriteString(w, `{"items":[{"id":"shared-event","start":{"dateTime":"2026-11-18T20:00:00-05:00"}}],"nextPageToken":"family-page-2"}`)
			case "/calendar/v3/calendars/holidays/events":
				io.WriteString(w, `{"items":[{"id":"holiday","start":{"date":"2026-11-19"}}]}`)
			default:
				w.WriteHeader(403)
				io.WriteString(w, `{"error":{"status":"PERMISSION_DENIED"}}`)
			}
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	m := New(t.TempDir())
	m.apiBase = server.URL
	m.records = map[string]record{"alice": {Settings: Settings{Services: []string{"calendar"}}, AccessToken: "token", RefreshToken: "refresh", Expiry: time.Now().Add(time.Hour), Scopes: strings.Join(serviceScopes["calendar"], " ")}}
	out, err := m.Execute(context.Background(), "alice", "google_calendar_search", json.RawMessage(`{"period":"today","query":"meeting"}`))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Scope string `json:"scope"`
		Items []struct {
			ID           string `json:"id"`
			CalendarID   string `json:"calendar_id"`
			CalendarName string `json:"calendar_name"`
		} `json:"items"`
		Checked  []any               `json:"calendars_checked"`
		Failed   []map[string]string `json:"calendars_failed"`
		Next     []map[string]string `json:"next_pages"`
		Partial  bool                `json:"partial"`
		Complete bool                `json:"complete"`
	}
	if err = json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result.Scope != "all_connected_calendars" || len(result.Checked) != 3 || len(result.Failed) != 2 || !result.Partial || result.Complete {
		t.Fatalf("wrong coverage: %s", out)
	}
	if len(result.Items) != 3 || result.Items[0].ID != "holiday" || result.Items[1].ID != "shared-event" || result.Items[2].ID != "personal-event" {
		t.Fatalf("subscriptions missing or incorrect chronological order: %s", out)
	}
	if result.Items[1].CalendarID != "shared" || result.Items[1].CalendarName != "Family" {
		t.Fatal("event calendar identity lost")
	}
	if len(result.Next) != 1 || result.Next[0]["calendar_id"] != "shared" || result.Next[0]["page_token"] != "family-page-2" || result.Next[0]["time_min"] != rangeMin {
		t.Fatal("per-calendar continuation lost")
	}
	if requests["/calendar/v3/users/me/calendarList"] != 2 || requests["/calendar/v3/calendars/busy/events"] != 0 {
		t.Fatal("list pagination or free/busy handling failed")
	}
}

func TestExplicitCalendarSearchStaysScopedAndPaginates(t *testing.T) {
	for _, calendar := range []string{"primary", "shared"} {
		t.Run(calendar, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/calendar/v3/calendars/"+calendar+"/events" || r.URL.Query().Get("pageToken") != "page2" {
					t.Error("explicit calendar or pagination changed")
				}
				io.WriteString(w, `{"items":[]}`)
			}))
			defer server.Close()
			m := New(t.TempDir())
			m.apiBase = server.URL
			m.records = map[string]record{"alice": {Settings: Settings{Services: []string{"calendar"}}, AccessToken: "token", RefreshToken: "refresh", Expiry: time.Now().Add(time.Hour), Scopes: strings.Join(serviceScopes["calendar"], " ")}}
			raw, _ := json.Marshal(Arguments{CalendarID: calendar, TimeMin: "2026-11-19T00:00:00+09:00", TimeMax: "2026-11-20T00:00:00+09:00", PageToken: "page2"})
			if _, err := m.Execute(context.Background(), "alice", "google_calendar_search", raw); err != nil {
				t.Fatal(err)
			}
		})
	}
}
