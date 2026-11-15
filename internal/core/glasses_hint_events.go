package core

import (
	"context"
	"dinkisstyle-chat/internal/mcp"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"
)

func validHintEvent(e mcp.HintEvent) bool {
	switch e.Stage {
	case "decision", "selected", "generation", "generated", "validated", "skipped", "phone_display", "device_display", "device_error", "canceled":
	default:
		return false
	}
	for _, field := range []struct {
		text  string
		limit int
	}{{e.EventID, 120}, {e.SessionID, 100}, {e.SegmentID, 100}, {e.JobID, 120}, {e.Reason, 240}, {e.Topic, 80}, {e.Kind, 16}, {e.Source, 24}, {e.Text, 2000}, {e.Evidence, 160}, {e.Version, 24}} {
		if !utf8.ValidString(field.text) || utf8.RuneCountInString(field.text) > field.limit {
			return false
		}
	}

	if e.EventID == "" || e.JobID == "" {
		return false
	}
	if e.Text != "" && e.Stage != "generated" && e.Stage != "phone_display" {
		return false
	}
	if e.OccurredAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, e.OccurredAt); err != nil {
			return false
		}
	}
	return true
}
func handleGlassesHintEvents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-User-ID")
		if user == "" {
			http.Error(w, "authentication required", 401)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			var req struct {
				Events []mcp.HintEvent `json:"events"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req); err != nil || len(req.Events) < 1 || len(req.Events) > 16 {
				http.Error(w, "invalid hint event batch", 400)
				return
			}
			for _, event := range req.Events {
				if !validHintEvent(event) {
					http.Error(w, "invalid hint event", 400)
					return
				}
			}
			if err := mcp.SaveHintEvents(ctx, user, req.Events); err != nil {
				http.Error(w, "hint history unavailable", 503)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]bool{"saved": true})
		case http.MethodGet:
			after := r.URL.Query().Get("after")
			if after != "" {
				t, err := time.Parse(time.RFC3339Nano, after)
				if err != nil {
					http.Error(w, "invalid history time", 400)
					return
				}
				after = t.UTC().Format("2006-01-02T15:04:05.000000000Z")
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			events, err := mcp.ListHintEvents(ctx, user, after, limit)
			if err != nil {
				http.Error(w, "hint history unavailable", 503)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"events": events})
		default:
			http.Error(w, "method not allowed", 405)
		}
	}
}
