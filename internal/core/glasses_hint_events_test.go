package core

import (
	"dinkisstyle-chat/internal/mcp"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHintEventValidation(t *testing.T) {
	base := mcp.HintEvent{EventID: "event", JobID: "job", Stage: "phone_display", Text: "synthetic card"}
	if !validHintEvent(base) {
		t.Fatal("valid event rejected")
	}
	diagnostic := base
	diagnostic.Stage = "decision_response"
	diagnostic.Text = `{"reason":"judge_timeout","elapsedMs":12000}`
	if !validHintEvent(diagnostic) {
		t.Fatal("bounded judge diagnostics rejected")
	}
	bad := base
	bad.Stage = "execute_command"
	if validHintEvent(bad) {
		t.Fatal("arbitrary event")
	}
	bad = base
	bad.Stage = "device_display"
	if validHintEvent(bad) {
		t.Fatal("body copied into metadata stage")
	}
	bad = base
	bad.Topic = strings.Repeat("한", 81)
	if validHintEvent(bad) {
		t.Fatal("oversized topic")
	}
	bad = base
	bad.Topic = strings.Repeat("한", 81)
	bad.Text = bad.Topic
	if validHintEvent(bad) {
		t.Fatal("repeated values must not bypass field limits")
	}
	for _, tc := range []struct {
		method, body, user string
		status             int
	}{{"GET", "", "", 401}, {"POST", `{"events":[]}`, "alice", 400}, {"POST", `{"events":[{"eventId":"e","jobId":"j","stage":"bad"}]}`, "alice", 400}, {"PUT", "", "alice", 405}} {
		req := httptest.NewRequest(tc.method, "/api/glasses/hint-events", strings.NewReader(tc.body))
		if tc.user != "" {
			req.Header.Set("X-User-ID", tc.user)
		}
		w := httptest.NewRecorder()
		handleGlassesHintEvents()(w, req)
		if w.Code != tc.status {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
	if !evenHubAPIPath("/api/glasses/hint-events") {
		t.Fatal("missing authenticated webview route")
	}
}
