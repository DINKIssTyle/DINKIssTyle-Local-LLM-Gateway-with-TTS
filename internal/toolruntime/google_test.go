package toolruntime

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeGoogle struct {
	allowed  bool
	services []string
	calls    int
}

func (g *fakeGoogle) Allowed(user, service string) bool {
	if user != "alice" || !g.allowed {
		return false
	}
	if g.services == nil {
		return service == "gmail"
	}
	for _, enabled := range g.services {
		if service == enabled {
			return true
		}
	}
	return false
}
func (g *fakeGoogle) Execute(context.Context, string, string, json.RawMessage) (string, error) {
	g.calls++
	return "mail", nil
}
func TestGoogleToolsRequireCurrentAccountGrant(t *testing.T) {
	g := &fakeGoogle{allowed: true}
	exec := ExecutionContext{UserID: "alice", Google: g}
	names := map[string]bool{}
	for _, d := range Default.List(exec) {
		names[d.Name] = true
	}
	if !names["gmail_search"] || !names["gmail_read"] || names["google_drive_search"] || names["google_calendar_read"] {
		t.Fatalf("incorrect tools: %v", names)
	}
	result, err := Default.Call(context.Background(), exec, "gmail_read", json.RawMessage(`{"id":"m1"}`))
	if err != nil || result.Content != "mail" {
		t.Fatal(err)
	}
	g.allowed = false
	if _, err = Default.Call(context.Background(), exec, "gmail_read", json.RawMessage(`{"id":"m1"}`)); err == nil || g.calls != 1 {
		t.Fatal("stale grant executed tool")
	}
	g.allowed = true
	exec.UserID = "bob"
	if _, err = Default.Call(context.Background(), exec, "gmail_read", json.RawMessage(`{"id":"m1"}`)); err == nil {
		t.Fatal("another account used grant")
	}
	exec.UserID = "alice"
	exec.DisabledTools = []string{"gmail_read"}
	if _, err = Default.Call(context.Background(), exec, "gmail_read", json.RawMessage(`{"id":"m1"}`)); err == nil {
		t.Fatal("explicit tool restriction bypassed")
	}
}

func TestRecentGmailNeedsNoSearchArguments(t *testing.T) {
	g := &fakeGoogle{allowed: true}
	exec := ExecutionContext{UserID: "alice", Google: g}
	for _, args := range []string{`{}`, `{"query":""}`, `{"limit":5}`} {
		if _, err := Default.Call(context.Background(), exec, "gmail_search", json.RawMessage(args)); err != nil {
			t.Fatalf("recent email call %s rejected: %v", args, err)
		}
	}
	if g.calls != 3 {
		t.Fatalf("expected three tool executions, got %d", g.calls)
	}
}

func TestDriveAndCalendarNeedNoSearchArguments(t *testing.T) {
	g := &fakeGoogle{allowed: true, services: []string{"drive", "calendar"}}
	exec := ExecutionContext{UserID: "alice", Google: g}
	for _, tc := range []struct{ name, args string }{
		{"google_drive_search", `{}`},
		{"google_drive_search", `{"query":""}`},
		{"google_calendar_search", `{}`},
		{"google_calendar_search", `{"period":"today"}`},
		{"google_calendar_search", `{"period":"tomorrow"}`},
	} {
		if _, err := Default.Call(context.Background(), exec, tc.name, json.RawMessage(tc.args)); err != nil {
			t.Fatalf("%s %s rejected: %v", tc.name, tc.args, err)
		}
	}
	if g.calls != 5 {
		t.Fatalf("expected five executions, got %d", g.calls)
	}
	for _, name := range []string{"google_drive_read", "google_calendar_read"} {
		if _, err := Default.Call(context.Background(), exec, name, json.RawMessage(`{}`)); err == nil {
			t.Fatalf("%s must still require a result ID", name)
		}
	}
}
