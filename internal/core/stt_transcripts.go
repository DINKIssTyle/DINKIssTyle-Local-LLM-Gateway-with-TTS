package core

import (
	"dinkisstyle-chat/internal/mcp"
	"encoding/json"
	"net/http"
	"time"
)

func handleSTTTranscripts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", 405)
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
		if date == "" {
			date = time.Now().In(location).Format("2006-01-02")
		}
		start, err := time.ParseInLocation("2006-01-02", date, location)
		if err != nil {
			http.Error(w, "Invalid date", 400)
			return
		}
		entries, err := mcp.ListTranscripts(r.Header.Get("X-User-ID"), start.UTC().Format(time.RFC3339Nano), start.AddDate(0, 0, 1).UTC().Format(time.RFC3339Nano))
		if err != nil {
			http.Error(w, "Transcript query failed", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"date": date, "timezone": zone, "entries": entries})
	}
}
