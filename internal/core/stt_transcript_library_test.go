package core

import (
	"dinkisstyle-chat/internal/mcp"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestTranscriptLibraryAPIRequiresIdentityAndExplicitDeleteDate(t *testing.T) {
	if err := mcp.InitDB(filepath.Join(t.TempDir(), "transcripts.db")); err != nil {
		t.Fatal(err)
	}
	defer mcp.CloseDB()
	_ = mcp.SaveTranscript(mcp.TranscriptEntry{UserID: "alice", StartedAt: "2026-11-20T15:00:00Z", EndedAt: "2026-11-20T15:00:01Z", Text: "원문"})
	for _, tc := range []struct {
		method, url, user string
		want              int
	}{
		{"GET", "/api/stt/transcript-days", "", 401},
		{"GET", "/api/stt/transcript-days?tz=wrong", "alice", 400},
		{"GET", "/api/stt/transcript-days?before=oops", "alice", 400},
		{"GET", "/api/stt/transcript-days?q=원문", "alice", 200},
		{"DELETE", "/api/stt/transcripts", "alice", 400},
		{"DELETE", "/api/stt/transcripts?date=2026-11-21", "bob", 200},
		{"GET", "/api/stt/transcripts?date=2026-11-21", "alice", 200},
	} {
		req := httptest.NewRequest(tc.method, tc.url, nil)
		req.Header.Set("X-User-ID", tc.user)
		w := httptest.NewRecorder()
		if strings.Contains(tc.url, "transcript-days") {
			handleSTTTranscriptDays()(w, req)
		} else {
			handleSTTTranscripts()(w, req)
		}
		if w.Code != tc.want {
			t.Errorf("%s %s: %d %s", tc.method, tc.url, w.Code, w.Body.String())
		}
		if tc.method == "GET" && tc.user == "alice" && tc.want == 200 && !strings.Contains(w.Body.String(), "原文") && !strings.Contains(w.Body.String(), "원문") {
			t.Errorf("source missing: %s", w.Body.String())
		}
	}
}
