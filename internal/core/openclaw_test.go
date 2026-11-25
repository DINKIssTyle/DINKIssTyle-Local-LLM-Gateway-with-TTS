package core

import (
	"context"
	"dinkisstyle-chat/internal/chatharness"
	"dinkisstyle-chat/internal/mcp"
	"dinkisstyle-chat/internal/toolruntime"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenClawCompatibilityChat(t *testing.T) {
	for _, routed := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("routed=%v/stream=%v", routed, stream), func(t *testing.T) {
				called := false
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					called = true
					if r.URL.Path != "/v1/chat/completions" {
						t.Error(r.URL.Path)
					}
					if r.Header.Get("Authorization") != "Bearer upstream-secret" {
						t.Error("wrong upstream authentication")
					}
					var body map[string]any
					json.NewDecoder(r.Body).Decode(&body)
					if body["model"] != "actual-model" || body["stream"] != true {
						t.Error(body)
					}
					if _, ok := body["messages"].([]any); !ok {
						t.Error("messages missing")
					}
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\ndata: [DONE]\n\n")
				}))
				defer upstream.Close()
				app := &App{llmEndpoint: upstream.URL, llmApiToken: "upstream-secret", llmMode: "stateful"}
				disabled := false
				am := &AuthManager{users: map[string]*User{"alice": {ID: "alice", Settings: UserSettings{EnableMemory: &disabled, OpenClaw: OpenClawSettings{Enabled: true, Model: "actual-model", TokenHash: hashToken("dkst_oc_test")}}}}}
				if routed {
					app.llmEndpoint = "http://127.0.0.1:1"
					am.users["alice"].Settings.OpenClaw.Model = routedModel("remote", "actual-model")
					app.llmConnections = []LLMConnection{{ID: "remote", Endpoint: upstream.URL, Mode: "standard", APIKey: "upstream-secret"}}
				}
				req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(fmt.Sprintf(`{"model":"openclaw","messages":[{"role":"user","content":"Hi"}],"stream":%v}`, stream)))
				out := httptest.NewRecorder()
				req.Header.Set("Authorization", "Bearer dkst_oc_test")
				AuthMiddleware(am, func(w http.ResponseWriter, r *http.Request) { handleChat(w, r, app, am) }).ServeHTTP(out, req)
				if !called || out.Code != 200 {
					t.Fatal(called, out.Code, out.Body.String())
				}
				if stream {
					if !strings.HasPrefix(out.Header().Get("Content-Type"), "text/event-stream") || !strings.Contains(out.Body.String(), "Hello") {
						t.Fatal(out.Body.String())
					}
				} else {
					var result struct {
						Object  string `json:"object"`
						Model   string `json:"model"`
						Choices []struct {
							Message struct {
								Content string `json:"content"`
							} `json:"message"`
						} `json:"choices"`
					}
					if err := json.Unmarshal(out.Body.Bytes(), &result); err != nil {
						t.Fatal(err, out.Body.String())
					}
					if result.Object != "chat.completion" || result.Model != "openclaw" || len(result.Choices) != 1 || result.Choices[0].Message.Content != "Hello world" {
						t.Fatalf("%+v", result)
					}
				}
			})
		}
	}
}

func TestOpenClawSettingPersistenceAndIsolation(t *testing.T) {
	am := &AuthManager{usersFile: filepath.Join(t.TempDir(), "users.json"), users: map[string]*User{"alice": {ID: "alice"}, "bob": {ID: "bob"}}}
	app := &App{authMgr: am, port: "2806"}
	if err := app.SetUserOpenClawSettings("alice", true, "", ""); err == nil {
		t.Fatal("accepted empty model")
	}
	if err := app.SetUserOpenClawSettings("alice", true, " alice-model ", ""); err != nil {
		t.Fatal(err)
	}
	token, err := app.IssueUserOpenClawToken("alice")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(am.usersFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), token) {
		t.Fatal("plaintext token persisted")
	}
	var users []*User
	if err := json.Unmarshal(data, &users); err != nil {
		t.Fatal(err)
	}
	restored := &AuthManager{users: map[string]*User{}}
	for _, user := range users {
		restored.users[user.ID] = user
	}
	if user, ok := restored.validateOpenClawToken(token); !ok || user.ID != "alice" {
		t.Fatal("token did not survive reload")
	}
	status, err := app.GetUserOpenClawSettings("alice")
	if err != nil || !status.HasToken || status.Model != "alice-model" {
		t.Fatal(status, err)
	}
	encoded, _ := json.Marshal(status)
	if strings.Contains(string(encoded), token) || strings.Contains(string(encoded), hashToken(token)) {
		t.Fatal("status leaked credentials")
	}
	if _, _, err := am.resolveOpenClawModel("openclaw", "bob"); err == nil {
		t.Fatal("inherited another account's model")
	}
	if model, applied, err := am.resolveOpenClawModel("other", "bob"); err != nil || applied || model != "other" {
		t.Fatal(model, applied, err)
	}
	replacement, err := app.IssueUserOpenClawToken("alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := am.validateOpenClawToken(token); ok {
		t.Fatal("rotation left old token valid")
	}
	if _, ok := am.validateOpenClawToken(replacement); !ok {
		t.Fatal("new token rejected")
	}
	if err := app.SetUserOpenClawSettings("alice", false, "alice-model", ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := am.validateOpenClawToken(replacement); ok {
		t.Fatal("disabled account accepted token")
	}
}

func TestOpenClawTokenAccountAndEndpointScope(t *testing.T) {
	am := &AuthManager{users: map[string]*User{}}
	for _, id := range []string{"alice", "bob"} {
		am.users[id] = &User{ID: id, Role: "user", Settings: UserSettings{OpenClaw: OpenClawSettings{Enabled: true, Model: id + "-model", TokenHash: hashToken("dkst_oc_" + id)}}}
	}
	for _, id := range []string{"alice", "bob"} {
		for _, path := range []string{"/v1/chat/completions", "/api/v1/chat", "/api/config", "/api/users", "/api/chat"} {
			reached := false
			out := httptest.NewRecorder()
			req := httptest.NewRequest("POST", path, nil)
			req.Header.Set("Authorization", "Bearer dkst_oc_"+id)
			req.Header.Set("X-User-ID", "forged-other-account")
			req.AddCookie(&http.Cookie{Name: "session", Value: "unrelated-browser-session"})
			AuthMiddleware(am, func(w http.ResponseWriter, r *http.Request) {
				reached = true
				if r.Header.Get("X-User-ID") != id || r.Header.Get("X-User-Role") != "user" {
					t.Fatal("wrong account identity")
				}
				target, _, err := am.resolveOpenClawModel("openclaw", r.Header.Get("X-User-ID"))
				if err != nil || target != id+"-model" {
					t.Fatal("wrong account model", target, err)
				}
			}).ServeHTTP(out, req)
			allowed := path == "/v1/chat/completions" || path == "/api/v1/chat"
			if reached != allowed || (!allowed && out.Code != 401) {
				t.Fatal(path, reached, out.Code)
			}
		}
	}
	out := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"bob-model","messages":[]}`))
	req.Header.Set("Authorization", "Bearer dkst_oc_alice")
	AuthMiddleware(am, func(w http.ResponseWriter, r *http.Request) { handleChat(w, r, &App{}, am) }).ServeHTTP(out, req)
	if out.Code != 403 {
		t.Fatal("external token overrode model", out.Code, out.Body.String())
	}
	delete(am.users, "alice")
	if _, ok := am.validateOpenClawToken("dkst_oc_alice"); ok {
		t.Fatal("deleted account accepted token")
	}
}

func TestOpenClawCatalogRoutes(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer account-secret" {
			t.Error("missing catalog credentials")
		}
		fmt.Fprint(w, `{"data":[{"id":"same-model"},{"id":"embed","type":"embedding"}]}`)
	}))
	defer upstream.Close()
	app := &App{llmEndpoint: upstream.URL, llmApiToken: "secret", llmMode: "standard", llmConnections: []LLMConnection{{ID: "extra", Name: "Extra", Endpoint: upstream.URL, APIKey: "secret"}}}
	key := "account-secret"
	app.authMgr = &AuthManager{users: map[string]*User{"alice": {ID: "alice", Settings: UserSettings{ApiToken: &key, LLMAPIKeys: map[string]string{"extra": "account-secret"}}}}}
	rows, err := app.GetOpenClawModels("alice")
	if err != nil || len(rows) != 2 || rows[0]["id"] != "same-model" || rows[1]["id"] != routedModel("extra", "same-model") {
		t.Fatal(rows, err)
	}
}

func TestOpenClawHTTPCompatibilityPreservesPOSTAndAuth(t *testing.T) {
	called := false
	protected := AuthMiddleware(&AuthManager{}, func(w http.ResponseWriter, r *http.Request) { called = true })
	handler := httpCompatibilityHandler(protected, "2806")
	for _, path := range []string{"/v1/chat/completions", "/api/v1/chat"} {
		req := httptest.NewRequest(http.MethodPost, "http://localhost:2807"+path, strings.NewReader(`{"model":"openclaw"}`))
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		if out.Code != http.StatusUnauthorized || out.Header().Get("Location") != "" || called {
			t.Fatalf("%s: status=%d location=%q reached protected handler=%v", path, out.Code, out.Header().Get("Location"), called)
		}
	}
	handler = httpCompatibilityHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Error("POST method changed")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["model"] != "openclaw" {
			t.Error("request body lost", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}), "2806")
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, httptest.NewRequest(http.MethodPost, "http://localhost:2807/v1/chat/completions", strings.NewReader(`{"model":"openclaw"}`)))
	if out.Code != http.StatusNoContent {
		t.Fatal(out.Code)
	}
	out = httptest.NewRecorder()
	handler.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "http://localhost:2807/?lang=ko", nil))
	if out.Code != http.StatusMovedPermanently || out.Header().Get("Location") != "https://localhost:2806/?lang=ko" {
		t.Fatal(out.Code, out.Header())
	}
}

func TestOpenClawPersistenceFailureDoesNotChangeAccess(t *testing.T) {
	am := &AuthManager{usersFile: filepath.Join(t.TempDir(), "users.json"), users: map[string]*User{"alice": {ID: "alice", Settings: UserSettings{OpenClaw: OpenClawSettings{Enabled: true, Model: "a", TokenHash: hashToken("dkst_oc_old")}}}}}
	app := &App{authMgr: am}
	am.usersFile = t.TempDir() // A directory cannot be overwritten with the users file.
	if _, err := app.IssueUserOpenClawToken("alice"); err == nil {
		t.Fatal("ignored persistence failure")
	}
	if _, ok := am.validateOpenClawToken("dkst_oc_old"); !ok {
		t.Fatal("failed rotation revoked the old token")
	}
	if err := app.SetUserOpenClawSettings("alice", false, "a", ""); err == nil {
		t.Fatal("ignored persistence failure")
	}
	if _, ok := am.validateOpenClawToken("dkst_oc_old"); !ok {
		t.Fatal("failed disable changed account state")
	}
}

func TestOpenClawUsesOnlyTokenOwnersPersonalDatabase(t *testing.T) {
	if err := mcp.InitDB(filepath.Join(t.TempDir(), "personal.db")); err != nil {
		t.Fatal(err)
	}
	defer mcp.CloseDB()
	am := &AuthManager{users: map[string]*User{}}
	names := map[string]string{"alice": "ALICE_PRIVATE_NICKNAME", "bob": "BOB_PRIVATE_NICKNAME"}
	for id, name := range names {
		if _, err := mcp.UpsertUserProfileFact(id, "ai_nickname", name, "identity", "test"); err != nil {
			t.Fatal(err)
		}
		am.users[id] = &User{ID: id, Settings: UserSettings{OpenClaw: OpenClawSettings{Enabled: true, Model: "test-model", TokenHash: hashToken("dkst_oc_" + id)}}}
	}
	app := &App{llmEndpoint: "http://127.0.0.1:1", llmMode: "standard"}
	for _, id := range []string{"alice", "bob"} {
		other := "bob"
		if id == "bob" {
			other = "alice"
		}
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"openclaw","messages":[{"role":"user","content":"니 이름이 뭐야?"}]}`))
		req.Header.Set("Authorization", "Bearer dkst_oc_"+id)
		req.Header.Set("X-User-ID", other)
		out := httptest.NewRecorder()
		AuthMiddleware(am, func(w http.ResponseWriter, r *http.Request) { handleChat(w, r, app, am) }).ServeHTTP(out, req)
		if out.Code != 200 || !strings.Contains(out.Body.String(), names[id]) || strings.Contains(out.Body.String(), names[other]) {
			t.Fatalf("personal database isolation failed for %s: %d %s", id, out.Code, out.Body.String())
		}
	}
}

func TestOpenClawExternalEndpointConfiguration(t *testing.T) {
	am := &AuthManager{usersFile: filepath.Join(t.TempDir(), "users.json"), users: map[string]*User{
		"alice": {ID: "alice", Settings: UserSettings{OpenClaw: OpenClawSettings{Enabled: true, Model: "model", TokenHash: hashToken("dkst_oc_alice")}}},
		"bob":   {ID: "bob"},
	}}
	app := &App{authMgr: am, port: "2806", llmEndpoint: "http://127.0.0.1:8094"}
	for _, address := range []string{"https://chat.example.com", "https://chat.example.com/v1/"} {
		if err := app.SetUserOpenClawSettings("alice", true, "model", address); err != nil {
			t.Fatal(err)
		}
		status, err := app.GetUserOpenClawSettings("alice")
		if err != nil || status.Endpoint != "https://chat.example.com/v1/chat/completions" || status.CustomEndpoint != status.Endpoint {
			t.Fatal(status, err)
		}
	}
	address := "https://chat.example.com:8443/gateway/v1/chat/completions"
	if err := app.SetUserOpenClawSettings("alice", true, "model", address); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(am.usersFile)
	if err != nil || !strings.Contains(string(data), address) {
		t.Fatal("endpoint not persisted", err)
	}
	other, err := app.GetUserOpenClawSettings("bob")
	if err != nil || other.CustomEndpoint != "" || other.Endpoint == address {
		t.Fatal("endpoint crossed accounts", other, err)
	}
	for _, invalid := range []string{"example.com", "file:///etc/passwd", "https://user:secret@example.com", "https://example.com?token=secret", "https://example.com#secret", "https://example.com:99999"} {
		if err := app.SetUserOpenClawSettings("alice", true, "model", invalid); err == nil {
			t.Fatal("accepted invalid endpoint", invalid)
		}
		if am.users["alice"].Settings.OpenClaw.Endpoint != address {
			t.Fatal("invalid URL overwrote endpoint")
		}
	}
	if app.llmEndpoint != "http://127.0.0.1:8094" || app.port != "2806" {
		t.Fatal("public address changed listener or upstream")
	}
	if _, valid := am.validateOpenClawToken("dkst_oc_alice"); !valid {
		t.Fatal("address update revoked token")
	}
	if err := app.SetUserOpenClawSettings("alice", true, "model", ""); err != nil {
		t.Fatal(err)
	}
	status, err := app.GetUserOpenClawSettings("alice")
	if err != nil || status.CustomEndpoint != "" || !strings.Contains(status.Endpoint, ":2807/v1/chat/completions") {
		t.Fatal(status, err)
	}
}

func TestOpenClawToolsAndSkillUseAccountPolicy(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		for _, stream := range []bool{true, false} {
			t.Run(fmt.Sprintf("tools=%v/stream=%v", enabled, stream), func(t *testing.T) {
				registry := toolruntime.NewRegistry()
				executions := 0
				if err := registry.Register(toolruntime.Definition{Name: "get_current_time", Description: "Read the clock", InputSchema: json.RawMessage(`{"type":"object"}`), Metadata: toolruntime.Metadata{ReadOnly: true}}, func(_ context.Context, exec toolruntime.ExecutionContext, _ json.RawMessage) (toolruntime.Result, error) {
					executions++
					if exec.UserID != "alice" || !strings.HasPrefix(exec.RequestID, "openclaw-") {
						t.Error("lost account or turn identity", exec.UserID, exec.RequestID)
					}
					return toolruntime.Result{Content: "CLOCK_PROOF"}, nil
				}); err != nil {
					t.Fatal(err)
				}
				previous := toolruntime.Default
				toolruntime.Default = registry
				defer func() { toolruntime.Default = previous }()
				calls := 0
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					var body map[string]any
					json.NewDecoder(r.Body).Decode(&body)
					raw, _ := json.Marshal(body)
					if calls == 1 {
						if enabled && body["tools"] == nil || !enabled && body["tools"] != nil {
							t.Error("tool policy ignored", body["tools"])
						}
						if !strings.Contains(string(raw), "python-calculator") {
							t.Error("skill instructions absent")
						}
						if enabled {
							fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"clock\",\"type\":\"function\",\"function\":{\"name\":\"get_current_time\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
							return
						}
					} else if !strings.Contains(string(raw), "CLOCK_PROOF") {
						t.Error("tool evidence not returned to model")
					}
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"FINAL_ANSWER\"}}]}\n\ndata: [DONE]\n\n")
				}))
				defer upstream.Close()
				memory := false
				am := &AuthManager{users: map[string]*User{"alice": {ID: "alice", Settings: UserSettings{EnableTools: &enabled, EnableMemory: &memory, OpenClaw: OpenClawSettings{Enabled: true, Model: "gemma-4-e2b-it-mlx", TokenHash: hashToken("dkst_oc_alice")}}}}}
				app := &App{llmEndpoint: upstream.URL, llmMode: "standard"}
				req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(fmt.Sprintf(`{"model":"openclaw","stream":%v,"messages":[{"role":"user","content":"$python-calculator get_current_time 도구로 현재 시각을 확인해줘"}]}`, stream)))
				req.Header.Set("Authorization", "Bearer dkst_oc_alice")
				out := httptest.NewRecorder()
				AuthMiddleware(am, func(w http.ResponseWriter, r *http.Request) { handleChat(w, r, app, am) }).ServeHTTP(out, req)
				expected := 0
				if enabled {
					expected = 1
				}
				if out.Code != 200 || executions != expected || !strings.Contains(out.Body.String(), "FINAL_ANSWER") {
					t.Fatal(out.Code, executions, out.Body.String())
				}
			})
		}
	}
}

func TestOpenClawRetrievalKeepsTurnsWithoutWebHeaders(t *testing.T) {
	if err := mcp.InitDB(filepath.Join(t.TempDir(), "conversation.db")); err != nil {
		t.Fatal(err)
	}
	defer mcp.CloseDB()
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		raw, _ := json.Marshal(body)
		if calls == 2 && (!strings.Contains(string(raw), "FIRST_USER_MARKER") || !strings.Contains(string(raw), "FIRST_ANSWER_MARKER")) {
			t.Error("follow-up lost previous turn")
		}
		answer := "FIRST_ANSWER_MARKER"
		if calls == 2 {
			answer = "SECOND_ANSWER_MARKER"
		}
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\ndata: [DONE]\n\n", answer)
	}))
	defer upstream.Close()
	strategy := "retrieval"
	memory := false
	am := &AuthManager{users: map[string]*User{"alice": {ID: "alice", Settings: UserSettings{ContextStrategy: &strategy, EnableMemory: &memory, OpenClaw: OpenClawSettings{Enabled: true, Model: "gemma-4-e2b-it-mlx", TokenHash: hashToken("dkst_oc_alice")}}}}}
	app := &App{llmEndpoint: upstream.URL, llmMode: "stateful"}
	for _, question := range []string{"FIRST_USER_MARKER", "SECOND_USER_MARKER"} {
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(fmt.Sprintf(`{"model":"openclaw","messages":[{"role":"user","content":%q}]}`, question)))
		req.Header.Set("Authorization", "Bearer dkst_oc_alice")
		out := httptest.NewRecorder()
		AuthMiddleware(am, func(w http.ResponseWriter, r *http.Request) { handleChat(w, r, app, am) }).ServeHTTP(out, req)
		if out.Code != 200 {
			t.Fatal(out.Code, out.Body.String())
		}
	}
	entry, err := mcp.GetCurrentChatSession("alice")
	if err != nil {
		t.Fatal(err)
	}
	turns := chatharness.ParseUISnapshot(entry.UIStateJSON).Turns
	if calls != 2 || len(turns) != 2 || turns[0].TurnID == "" || turns[0].TurnID == turns[1].TurnID {
		t.Fatal("invalid turn tracking", calls, turns)
	}
}
