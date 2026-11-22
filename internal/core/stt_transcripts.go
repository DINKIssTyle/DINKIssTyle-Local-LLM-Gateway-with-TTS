package core

import (
	"dinkisstyle-chat/internal/mcp"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func handleSTTTranscriptDays() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", 405)
			return
		}
		user := r.Header.Get("X-User-ID")
		if user == "" {
			http.Error(w, "authentication required", 401)
			return
		}
		zone := r.URL.Query().Get("tz")
		if zone == "" {
			zone = "Asia/Seoul"
		}
		if _, err := time.LoadLocation(zone); err != nil {
			http.Error(w, "Invalid timezone", 400)
			return
		}
		before := r.URL.Query().Get("before")
		if before != "" {
			if _, _, err := mcp.TranscriptDateRange(before, zone); err != nil {
				http.Error(w, "Invalid date", 400)
				return
			}
		}
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if len([]rune(query)) > 300 {
			http.Error(w, "Query too long", 400)
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		items, more, err := mcp.ListTranscriptDays(r.Context(), user, zone, query, before, limit)
		if err != nil {
			http.Error(w, "Transcript query failed", 500)
			return
		}
		next := ""
		if more && len(items) > 0 {
			next = items[len(items)-1].Date
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "next": next, "timezone": zone})
	}
}

func handleSTTTranscripts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", 405)
			return
		}
		user := r.Header.Get("X-User-ID")
		if user == "" {
			http.Error(w, "authentication required", 401)
			return
		}
		zone := r.URL.Query().Get("tz")
		if zone == "" {
			zone = "Asia/Seoul"
		}
		location, err := time.LoadLocation(zone)
		if err != nil {
			http.Error(w, "Invalid timezone", 400)
			return
		}
		date := r.URL.Query().Get("date")
		if date == "" && r.Method == http.MethodDelete {
			http.Error(w, "Date required", 400)
			return
		}
		if date == "" {
			date = time.Now().In(location).Format("2006-01-02")
		}
		start, end, err := mcp.TranscriptDateRange(date, zone)
		if err != nil {
			http.Error(w, "Invalid date", 400)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodDelete {
			count, err := mcp.DeleteTranscriptDay(r.Context(), user, date, zone)
			if err != nil {
				http.Error(w, "Transcript deletion failed", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"date": date, "deleted": count})
			return
		}
		entries, err := mcp.ListTranscripts(user, start.Format("2006-01-02T15:04:05"), end.Format("2006-01-02T15:04:05"))
		if err != nil {
			http.Error(w, "Transcript query failed", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"date": date, "timezone": zone, "entries": entries})
	}
}
