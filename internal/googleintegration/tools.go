package googleintegration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
)

func ServiceForTool(name string) string {
	switch name {
	case "gmail_search", "gmail_read":
		return "gmail"
	case "google_drive_search", "google_drive_read":
		return "drive"
	case "google_calendar_search", "google_calendar_read", "google_calendar_list":
		return "calendar"
	}
	return ""
}

type Arguments struct {
	Query      string `json:"query"`
	ID         string `json:"id"`
	CalendarID string `json:"calendar_id"`
	TimeMin    string `json:"time_min"`
	TimeMax    string `json:"time_max"`
	TimeZone   string `json:"time_zone"`
	Period     string `json:"period"`
	PageToken  string `json:"page_token"`
	Limit      int    `json:"limit"`
}

func textLimit(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "\n[Truncated]"
	}
	return s
}
func textHTML(s string) string {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			href := ""
			for _, attr := range n.Attr {
				if strings.EqualFold(attr.Key, "href") {
					href = strings.TrimSpace(attr.Val)
					break
				}
			}
			if isPreservedHTTPLink(href) {
				var label strings.Builder
				var collect func(*html.Node)
				collect = func(cur *html.Node) {
					if cur.Type == html.ElementNode && (cur.Data == "script" || cur.Data == "style") {
						return
					}
					if cur.Type == html.TextNode {
						label.WriteString(cur.Data)
						label.WriteByte(' ')
					}
					for c := cur.FirstChild; c != nil; c = c.NextSibling {
						collect(c)
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					collect(c)
				}
				text := strings.Join(strings.Fields(label.String()), " ")
				if text == "" {
					b.WriteString(href)
				} else if strings.Contains(text, href) {
					b.WriteString(text)
				} else {
					b.WriteString("[" + text + "](" + href + ")")
				}
				b.WriteByte(' ')
				return
			}
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return strings.Join(strings.Fields(b.String()), " ")
}

func isPreservedHTTPLink(href string) bool {
	if len(href) == 0 || len(href) > 400 {
		return false
	}
	lower := strings.ToLower(href)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}
func resultJSON(v any) (string, error) { b, err := json.Marshal(v); return string(b), err }
func (m *Manager) Execute(ctx context.Context, user, name string, args json.RawMessage) (string, error) {
	service := ServiceForTool(name)
	if service == "" {
		return "", fmt.Errorf("unknown Google tool")
	}
	var a Arguments
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Limit <= 0 {
		a.Limit = 10
	}
	if a.Limit > 10 {
		a.Limit = 10
	}
	if !m.Allowed(user, service) {
		return "", fmt.Errorf("Google %s is not connected and enabled for this gateway account", service)
	}
	get := func(path string, q url.Values) ([]byte, error) { return m.get(ctx, user, service, path, q) }
	switch name {
	case "gmail_search":
		if strings.TrimSpace(a.Query) == "" {
			a.Query = "in:inbox"
		}
		q := url.Values{"q": {a.Query}, "maxResults": {strconv.Itoa(a.Limit)}}
		if a.PageToken != "" {
			q.Set("pageToken", a.PageToken)
		}
		b, err := get("/gmail/v1/users/me/messages", q)
		if err != nil {
			return "", err
		}
		var list struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
			Next string `json:"nextPageToken"`
		}
		if err = json.Unmarshal(b, &list); err != nil {
			return "", err
		}
		out := []any{}
		for _, item := range list.Messages {
			b, err = get("/gmail/v1/users/me/messages/"+url.PathEscape(item.ID), url.Values{"format": {"metadata"}, "metadataHeaders": {"Subject", "From", "To", "Date"}})
			if err != nil {
				return "", err
			}
			var msg gmailMessage
			if err = json.Unmarshal(b, &msg); err != nil {
				return "", err
			}
			out = append(out, msg.summary(false))
		}
		return resultJSON(map[string]any{"query": a.Query, "messages": out, "next_page_token": list.Next, "evidence_type": "message metadata and snippets; read the message for full content"})
	case "gmail_read":
		if a.ID == "" {
			return "", fmt.Errorf("id is required")
		}
		b, err := get("/gmail/v1/users/me/messages/"+url.PathEscape(a.ID), url.Values{"format": {"full"}})
		if err != nil {
			return "", err
		}
		var msg gmailMessage
		if err = json.Unmarshal(b, &msg); err != nil {
			return "", err
		}
		return resultJSON(msg.summary(true))
	case "google_drive_search":
		escaped := strings.ReplaceAll(strings.ReplaceAll(a.Query, "\\", "\\\\"), "'", "\\'")
		filter := "trashed = false and mimeType != 'application/vnd.google-apps.folder'"
		if strings.TrimSpace(a.Query) != "" {
			filter = "trashed = false and (fullText contains '" + escaped + "')"
		}
		q := url.Values{"q": {filter}, "orderBy": {"modifiedTime desc"}, "pageSize": {strconv.Itoa(a.Limit)}, "fields": {"nextPageToken,files(id,name,mimeType,description,webViewLink,modifiedTime)"}, "includeItemsFromAllDrives": {"true"}, "supportsAllDrives": {"true"}}
		if a.PageToken != "" {
			q.Set("pageToken", a.PageToken)
		}
		b, err := get("/drive/v3/files", q)
		if err != nil {
			return "", err
		}
		return resultJSON(map[string]any{"query": a.Query, "order_by": "modifiedTime desc", "evidence_type": "file metadata; read the file for document content", "result": json.RawMessage(b)})
	case "google_drive_read":
		if a.ID == "" {
			return "", fmt.Errorf("id is required")
		}
		path := "/drive/v3/files/" + url.PathEscape(a.ID)
		b, err := get(path, url.Values{"fields": {"id,name,mimeType,webViewLink,description"}, "supportsAllDrives": {"true"}})
		if err != nil {
			return "", err
		}
		var file struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			MIME string `json:"mimeType"`
			Link string `json:"webViewLink"`
		}
		if err = json.Unmarshal(b, &file); err != nil {
			return "", err
		}
		mime := ""
		switch file.MIME {
		case "application/vnd.google-apps.document", "application/vnd.google-apps.presentation":
			mime = "text/plain"
		case "application/vnd.google-apps.spreadsheet":
			mime = "text/csv"
		}
		note := ""
		if file.MIME == "application/vnd.google-apps.spreadsheet" {
			note = "CSV export covers the first sheet only."
		}
		if mime != "" {
			b, err = get(path+"/export", url.Values{"mimeType": {mime}})
		} else if strings.HasPrefix(file.MIME, "text/") || file.MIME == "application/json" || file.MIME == "application/xml" {
			b, err = get(path, url.Values{"alt": {"media"}, "supportsAllDrives": {"true"}})
		} else {
			return resultJSON(map[string]any{"file": file, "content_available": false, "reason": "Text extraction is currently supported for Google Docs, Slides, the first sheet of Sheets, and text files. PDF, Office, images and other binary files require a separate extractor."})
		}
		if err != nil {
			return "", err
		}
		if !utf8.Valid(b) {
			return "", fmt.Errorf("document is not UTF-8 text")
		}
		content := string(b)
		if file.MIME == "text/html" {
			content = textHTML(content)
		}
		return resultJSON(map[string]any{"file": file, "content": textLimit(content, 24000), "truncated": utf8.RuneCountInString(content) > 24000, "note": note})
	case "google_calendar_list":
		q := url.Values{"maxResults": {"100"}, "showHidden": {"true"}, "fields": {"nextPageToken,items(id,summary,summaryOverride,primary,timeZone,accessRole,hidden,selected)"}}
		if a.PageToken != "" {
			q.Set("pageToken", a.PageToken)
		}
		b, err := get("/calendar/v3/users/me/calendarList", q)
		return string(b), err
	case "google_calendar_search", "google_calendar_read":
		if name == "google_calendar_search" && (a.CalendarID == "" || a.CalendarID == "all") {
			return m.searchAllCalendars(ctx, user, a)
		}
		id := a.CalendarID
		if id == "" {
			id = "primary"
		}
		path := "/calendar/v3/calendars/" + url.PathEscape(id) + "/events"
		q := url.Values{}
		if name == "google_calendar_read" {
			if a.ID == "" || a.CalendarID == "" {
				return "", fmt.Errorf("id and the event's source calendar_id are required")
			}
			path += "/" + url.PathEscape(a.ID)
		} else {
			if _, weekday := weekdayPeriod(a.Period); !weekday && a.Period != "" && a.Period != "upcoming" && a.Period != "today" && a.Period != "tomorrow" && a.Period != "this_week" && a.Period != "next_week" {
				return "", fmt.Errorf("unknown calendar period; use upcoming, today, tomorrow, this_week, next_week or a weekday")
			}
			if a.TimeMin == "" || a.TimeMax == "" {
				if a.TimeZone == "" {
					metadata, err := get("/calendar/v3/users/me/calendarList/"+url.PathEscape(id), url.Values{"fields": {"timeZone"}})
					if err != nil {
						return "", err
					}
					var calendar struct {
						TimeZone string `json:"timeZone"`
					}
					if err = json.Unmarshal(metadata, &calendar); err != nil {
						return "", err
					}
					if calendar.TimeZone == "" {
						return "", fmt.Errorf("Google calendar returned no time zone")
					}
					a.TimeZone = calendar.TimeZone
				}
			}
			var err error
			a.TimeMin, a.TimeMax, err = calendarRange(a, time.Now())
			if err != nil {
				return "", err
			}

			q = url.Values{"timeMin": {a.TimeMin}, "timeMax": {a.TimeMax}, "singleEvents": {"true"}, "orderBy": {"startTime"}, "maxResults": {strconv.Itoa(a.Limit)}}
			if a.Query != "" {
				q.Set("q", a.Query)
			}
			if a.PageToken != "" {
				q.Set("pageToken", a.PageToken)
			}
		}
		if a.TimeZone != "" {
			q.Set("timeZone", a.TimeZone)
		}
		b, err := get(path, q)
		if err != nil {
			return "", err
		}
		if name == "google_calendar_search" {
			var result map[string]any
			if err = json.Unmarshal(b, &result); err != nil {
				return "", err
			}
			filterAllDayEvents(result, a)
			result["effective_range"] = map[string]string{"time_min": a.TimeMin, "time_max": a.TimeMax, "time_zone": a.TimeZone, "calendar_id": id}
			return resultJSON(result)
		}
		return string(b), nil
	}
	return "", fmt.Errorf("unknown Google tool")
}

type gmailPart struct {
	MIME     string `json:"mimeType"`
	Filename string `json:"filename"`
	Headers  []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"headers"`
	Body struct {
		Data         string `json:"data"`
		AttachmentID string `json:"attachmentId"`
	} `json:"body"`
	Parts []gmailPart `json:"parts"`
}
type gmailMessage struct {
	ID       string    `json:"id"`
	ThreadID string    `json:"threadId"`
	Snippet  string    `json:"snippet"`
	Payload  gmailPart `json:"payload"`
}

func (msg gmailMessage) summary(full bool) map[string]any {
	out := map[string]any{"id": msg.ID, "thread_id": msg.ThreadID, "snippet": msg.Snippet, "url": "https://mail.google.com/mail/u/0/#all/" + url.PathEscape(msg.ThreadID)}
	headers := map[string]string{}
	for _, h := range msg.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "subject", "from", "to", "date":
			headers[strings.ToLower(h.Name)] = h.Value
		}
	}
	out["headers"] = headers
	if full {
		plain, rich := []string{}, []string{}
		attachments := []string{}
		var walk func(gmailPart)
		walk = func(p gmailPart) {
			if p.Filename != "" {
				attachments = append(attachments, p.Filename)
				return
			}
			data, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(p.Body.Data, "="))
			if err == nil && len(data) > 0 {
				if p.MIME == "text/plain" {
					plain = append(plain, string(data))
				} else if p.MIME == "text/html" {
					rich = append(rich, textHTML(string(data)))
				}
			}
			for _, child := range p.Parts {
				walk(child)
			}
		}
		walk(msg.Payload)
		if len(plain) == 0 {
			plain = rich
		}
		body := strings.Join(plain, "\n")
		out["content"] = textLimit(body, 24000)
		out["truncated"] = utf8.RuneCountInString(body) > 24000
		out["attachments_not_read"] = attachments
	}
	return out
}
