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

func TestGoogleAPIErrorsProvideActionWithoutLeakingResponse(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"disabled", `{"error":{"message":"SECRET has not been used in project 123 before or it is disabled","status":"PERMISSION_DENIED","details":[{"reason":"SERVICE_DISABLED","metadata":{"consumer":"projects/123"}}]}}`, "SERVICE_DISABLED"},
		{"scope", `{"error":{"message":"Request had insufficient authentication scopes. SECRET","status":"PERMISSION_DENIED"}}`, "ACCESS_TOKEN_SCOPE_INSUFFICIENT"},
		{"policy", `{"error":{"errors":[{"reason":"domainPolicy"}],"message":"SECRET"}}`, "domainPolicy"},
		{"quota", `{"error":{"errors":[{"reason":"userRateLimitExceeded"}],"message":"SECRET"}}`, "userRateLimitExceeded"},
		{"unknown", `{"error":{"message":"SECRET","errors":[{"reason":"SECRET"}]}}`, "API"},
		{"html", `<html>SECRET</html>`, "API"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := googleAPIError("gmail", 403, strings.NewReader(tc.body))
			if !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), "123") {
				t.Fatal(err)
			}
		})
	}
}
func TestGmail403RetainsStructuredReasonThroughTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		io.WriteString(w, `{"error":{"status":"PERMISSION_DENIED","details":[{"reason":"SERVICE_DISABLED"}]}}`)
	}))
	defer server.Close()
	m := New(t.TempDir())
	m.apiBase = server.URL
	m.records = map[string]record{"alice": {Settings: Settings{Services: []string{"gmail"}}, AccessToken: "secret", RefreshToken: "secret", Expiry: time.Now().Add(time.Hour), Scopes: serviceScopes["gmail"][0]}}
	_, err := m.Execute(context.Background(), "alice", "gmail_search", json.RawMessage(`{"query":"is:inbox"}`))
	if err == nil || !strings.Contains(err.Error(), "SERVICE_DISABLED") {
		t.Fatalf("actionable Google error lost: %v", err)
	}
}

func TestGoogleTokenErrorsExplainRecoveryWithoutLeakingCredentials(t *testing.T) {
	for _, code := range []string{"invalid_grant", "invalid_client", "unauthorized_client", "invalid_scope", "SECRET_UNKNOWN"} {
		body, _ := json.Marshal(map[string]string{"error": code, "error_description": "SECRET_TOKEN_AND_CLIENT"})
		err := googleTokenError(400, strings.NewReader(string(body)))
		if strings.Contains(err.Error(), "SECRET") || !strings.Contains(err.Error(), "400") {
			t.Fatal(err)
		}
		if code != "SECRET_UNKNOWN" && !strings.Contains(err.Error(), "["+code+"]") {
			t.Fatal("OAuth reason lost", err)
		}
	}
}
