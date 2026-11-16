package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestLLMConnectionModelIdentity(t *testing.T) {
	for _, raw := range []string{"same-model", "vendor/model:2", "한글 모델/이름"} {
		encoded := routedModel("provider_1", raw)
		id, decoded, err := splitRoutedModel(encoded)
		if err != nil || id != "provider_1" || decoded != raw {
			t.Fatal(id, decoded, err)
		}
	}
	for _, bad := range []string{"@llm/a/", "@llm/a/!!!", "@llm/../YQ", "@llm/a/YQ/extra"} {
		if _, _, err := splitRoutedModel(bad); err == nil {
			t.Fatal("accepted malformed route", bad)
		}
	}
	if id, raw, err := splitRoutedModel("original-model"); err != nil || id != "" || raw != "original-model" {
		t.Fatal(id, raw, err)
	}
}
func TestLLMConnectionCatalogIsolation(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer primary-secret" {
			t.Error("primary key changed")
		}
		fmt.Fprint(w, `{"data":[{"id":"same-model"}]}`)
	}))
	defer primary.Close()
	var wantedKey atomic.Value
	wantedKey.Store("account-a-secret")
	extra := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+wantedKey.Load().(string) {
			t.Error("wrong account key", r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, `{"data":[{"id":"same-model","display_name":"Same Model"}]}`)
	}))
	defer extra.Close()
	app := &App{llmEndpoint: primary.URL, llmApiToken: "primary-secret", llmConnections: []LLMConnection{{ID: "remote", Name: "외부 LLM", Endpoint: extra.URL, Mode: "standard", APIKey: "server-default-secret"}}}
	am := &AuthManager{users: map[string]*User{"a": {ID: "a", Settings: UserSettings{LLMAPIKeys: map[string]string{"remote": "account-a-secret"}}}, "b": {ID: "b", Settings: UserSettings{LLMAPIKeys: map[string]string{"remote": "account-b-secret"}}}}}
	for _, user := range []string{"a", "b"} {
		wantedKey.Store("account-" + user + "-secret")
		request := httptest.NewRequest("GET", "/api/models", nil)
		request.Header.Set("X-User-ID", user)
		response := httptest.NewRecorder()
		handleModels(response, request, app, am)
		var result struct {
			Models []map[string]any `json:"models"`
		}
		if json.Unmarshal(response.Body.Bytes(), &result) != nil || len(result.Models) != 2 {
			t.Fatal(response.Body.String())
		}
		if result.Models[0]["display_name"] != "same-model" {
			t.Fatal("default model must not have a connection prefix", result.Models[0])
		}
		if result.Models[0]["id"] == result.Models[1]["id"] {
			t.Fatal("same-name models collide")
		}
		if result.Models[1]["id"] != routedModel("remote", "same-model") || !strings.HasPrefix(result.Models[1]["display_name"].(string), "[외부 LLM]") {
			t.Fatal(result.Models)
		}
		for _, secret := range []string{"account-a-secret", "account-b-secret", "server-default-secret", "primary-secret", extra.URL} {
			if strings.Contains(response.Body.String(), secret) {
				t.Fatal("secret or private endpoint leaked")
			}
		}

	}
	public := app.GetLLMConnections()
	if public[0].APIKey != "" || !public[0].HasAPIKey {
		t.Fatal("desktop read disclosed key or lost status")
	}
}
func TestLLMConnectionChatRouting(t *testing.T) {
	var called atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		if r.URL.Path != "/v1/chat/completions" {
			t.Error(r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer account-extra-key" {
			t.Error("primary/other key sent to additional server")
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "same-model" {
			t.Error("route leaked upstream", body["model"])
		}
		if _, ok := body["messages"].([]any); !ok {
			http.Error(w, "messages required", 400)
			return
		}
		if _, ok := body["input"]; ok {
			t.Error("stateful format leaked")
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"routed answer\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	disabled := false
	primaryKey := "must-not-leak-primary-key"
	app := &App{llmEndpoint: "http://127.0.0.1:1", llmMode: "stateful", llmConnections: []LLMConnection{{ID: "remote", Name: "Remote", Endpoint: upstream.URL, Mode: "standard", APIKey: "global-extra-key"}}}
	am := &AuthManager{users: map[string]*User{"a": {ID: "a", Settings: UserSettings{ApiToken: &primaryKey, EnableMemory: &disabled, LLMAPIKeys: map[string]string{"remote": "account-extra-key"}}}}}
	for _, translation := range []bool{false, true} {
		body := map[string]any{"model": routedModel("remote", "same-model"), "stream": true, "input": "Hello", "system_prompt": "Be helpful", "store": false}
		if translation {
			body["glasses_client"] = true
			body["glasses_task"] = "translation_replies"
		}
		encoded, _ := json.Marshal(body)
		request := httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(encoded)))
		request.Header.Set("X-User-ID", "a")
		response := httptest.NewRecorder()
		handleChat(response, request, app, am)
		if response.Code != 200 || !strings.Contains(response.Body.String(), "routed answer") {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	if called.Load() != 2 {
		t.Fatal("wrong server selected")
	}
}
func TestLLMConnectionEnsureDoesNotManageCloudModels(t *testing.T) {
	app := &App{llmConnections: []LLMConnection{{ID: "cloud", Name: "Cloud", Endpoint: "http://127.0.0.1:1", Mode: "standard"}}}
	request := httptest.NewRequest("POST", "/api/models/ensure", strings.NewReader(fmt.Sprintf(`{"model":%q}`, routedModel("cloud", "same-model"))))
	response := httptest.NewRecorder()
	handleModels(response, request, app, &AuthManager{users: map[string]*User{}})
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var body map[string]string
	json.Unmarshal(response.Body.Bytes(), &body)
	if body["instance_id"] != routedModel("cloud", "same-model") {
		t.Fatal("provider identity lost", body)
	}
}
func TestLLMConnectionKeyPersistenceAndPatch(t *testing.T) {
	am := &AuthManager{usersFile: filepath.Join(t.TempDir(), "users.json"), users: map[string]*User{"a": {ID: "a", Settings: UserSettings{LLMAPIKeys: map[string]string{"second": "keep"}}}, "b": {ID: "b", Settings: UserSettings{LLMAPIKeys: map[string]string{"remote": "other-user-key"}}}}}
	app := &App{llmConnections: []LLMConnection{{ID: "remote"}, {ID: "second"}}}
	if err := app.setConnectionKeys(am, "a", map[string]string{"remote": "Bearer account-key"}); err != nil {
		t.Fatal(err)
	}
	if am.users["a"].Settings.LLMAPIKeys["remote"] != "account-key" || am.users["a"].Settings.LLMAPIKeys["second"] != "keep" || am.users["b"].Settings.LLMAPIKeys["remote"] != "other-user-key" {
		t.Fatal("patch mixed accounts or overwrote omitted keys")
	}
	data, _ := json.Marshal(am.users["a"].Settings)
	var restored UserSettings
	if json.Unmarshal(data, &restored) != nil || restored.LLMAPIKeys["remote"] != "account-key" {
		t.Fatal("key settings did not round-trip")
	}
	if err := app.setConnectionKeys(am, "a", map[string]string{"remote": ""}); err != nil {
		t.Fatal(err)
	}
	if _, ok := am.users["a"].Settings.LLMAPIKeys["remote"]; ok {
		t.Fatal("explicit key removal failed")
	}
	if err := app.setConnectionKeys(am, "a", map[string]string{"unknown": "bad"}); err == nil {
		t.Fatal("unregistered connection accepted")
	}
}

func TestLLMConnectionSecondaryTitleRouting(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer user-remote-key" {
			t.Error("wrong title credential")
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "title-model" {
			t.Error(body["model"])
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Remote title\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	previous := globalApp
	defer func() { globalApp = previous }()
	globalApp = &App{llmEndpoint: "http://127.0.0.1:1", llmApiToken: "never-send-primary-key", llmConnections: []LLMConnection{{ID: "titles", Endpoint: upstream.URL, Mode: "standard"}}, authMgr: &AuthManager{users: map[string]*User{"a": {ID: "a", Settings: UserSettings{LLMAPIKeys: map[string]string{"titles": "user-remote-key"}}}}}}
	answer := callLLMInternal(context.Background(), "Write a title", savedTurnTitleOptions{UserID: "a", SecondaryModel: routedModel("titles", "title-model"), APIToken: "never-send-client-primary-key"})
	if !strings.Contains(answer, "Remote title") {
		t.Fatal(answer)
	}
}
func TestLLMConnectionValidationBeforeSaving(t *testing.T) {
	app := &App{}
	invalid := [][]LLMConnectionUpdate{
		make([]LLMConnectionUpdate, 9),
		{{ID: "../bad", Name: "Name", Endpoint: "https://example.test", Mode: "standard"}},
		{{ID: "ok", Name: "", Endpoint: "https://example.test", Mode: "standard"}},
		{{ID: "ok", Name: "Name", Endpoint: "https://token@example.test", Mode: "standard"}},
		{{ID: "ok", Name: "Name", Endpoint: "https://example.test", Mode: "standard"}, {ID: "ok", Name: "Duplicate", Endpoint: "https://second.test", Mode: "standard"}},
	}
	for _, updates := range invalid {
		if err := app.SetLLMConnections(updates); err == nil {
			t.Fatal("invalid connection set was saved")
		}
	}
	original := AppConfig{LLMEndpoint: "http://primary.test", LLMConnections: []LLMConnection{{ID: "remote", Name: "Remote", Endpoint: "https://remote.test", Mode: "standard", APIKey: "stored-key"}}}
	data, _ := json.Marshal(original)
	var restored AppConfig
	if json.Unmarshal(data, &restored) != nil || len(restored.LLMConnections) != 1 || restored.LLMConnections[0].APIKey != "stored-key" || restored.LLMEndpoint != original.LLMEndpoint {
		t.Fatal("configuration did not round trip")
	}
}

func TestAdditionalNativeAmbientRoles(t *testing.T) {
	for _, role := range []string{"judge", "plan", "answer", "translation", "chat"} {
		t.Run(role, func(t *testing.T) {
			var calls int
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/api/v1/chat" || r.Header.Get("Authorization") != "Bearer account-native" {
					t.Error("wrong endpoint or account credential", r.URL.Path)
				}
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body["model"] != "same-model" || !strings.Contains(fmt.Sprint(body["input"]), "synthetic speech") {
					t.Error("model route or speech lost")
				}
				if _, ok := body["messages"]; ok {
					t.Error("compatible messages leaked into Native request")
				}
				if role == "chat" {
					if body["reasoning"] != "high" {
						t.Error("explicit chat reasoning changed")
					}
				} else {
					if body["reasoning"] != "off" || body["store"] != false {
						t.Error("ambient reasoning/store policy lost")
					}
					for _, key := range []string{"max_tokens", "response_format", "chat_template_kwargs", "previous_response_id", "integrations", "tools"} {
						if _, ok := body[key]; ok {
							t.Errorf("unsupported/control field leaked: %s", key)
						}
					}
					if body["max_output_tokens"] == nil {
						t.Error("Native output limit absent")
					}
					if role == "judge" && (body["max_output_tokens"] != float64(384) || !strings.Contains(fmt.Sprint(body["system_prompt"]), "JSON output contract")) {
						t.Error("judge budget/schema contract lost")
					}
				}
				fmt.Fprint(w, "data: {\"type\":\"message.delta\",\"content\":\"native role answer\"}\n\ndata: {\"type\":\"chat.end\",\"result\":{\"output\":[{\"type\":\"message\",\"content\":\"native role answer\"}]}}\n\n")
			}))
			defer upstream.Close()
			app := &App{llmEndpoint: "http://127.0.0.1:1", llmMode: "standard", llmApiToken: "primary-must-not-leak", llmConnections: []LLMConnection{{ID: "remote", Endpoint: upstream.URL, Mode: "stateful", APIKey: "default-native"}}}
			am := &AuthManager{users: map[string]*User{"u": {ID: "u", Settings: UserSettings{LLMAPIKeys: map[string]string{"remote": "account-native"}}}}}
			body := map[string]any{"model": routedModel("remote", "same-model"), "stream": true, "messages": []any{map[string]any{"role": "system", "content": "select JSON"}, map[string]any{"role": "user", "content": "synthetic speech"}}}
			if role == "chat" {
				body["reasoning"] = "high"
			} else if role == "translation" {
				body["glasses_client"], body["glasses_task"] = true, "translation"
			} else {
				body["glasses_hint"], body["glasses_hint_phase"] = true, role
				if role == "judge" {
					body["response_format"] = map[string]any{"type": "json_schema", "json_schema": map[string]any{"schema": map[string]any{"type": "object", "required": []string{"hint"}}}}
				}
			}
			encoded, _ := json.Marshal(body)
			r := httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(encoded)))
			r.Header.Set("X-User-ID", "u")
			w := httptest.NewRecorder()
			handleChat(w, r, app, am)
			if w.Code != 200 || calls != 1 || !strings.Contains(w.Body.String(), "native role answer") {
				t.Fatalf("role=%s status=%d calls=%d body=%s", role, w.Code, calls, w.Body.String())
			}
		})
	}
}

func TestNativeAmbientReasoningCompatibilityRetry(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if calls == 1 {
			if body["reasoning"] != "off" {
				t.Error("missing reasoning control")
			}
			http.Error(w, `{"error":"This model does not support reasoning"}`, 400)
			return
		}
		if _, ok := body["reasoning"]; ok {
			t.Error("unsupported reasoning control not removed")
		}
		fmt.Fprint(w, "data: {\"type\":\"message.delta\",\"content\":\"ready\"}\n\ndata: {\"type\":\"chat.end\",\"result\":{\"output\":[{\"type\":\"message\",\"content\":\"ready\"}]}}\n\n")
	}))
	defer upstream.Close()
	app := &App{llmMode: "standard", llmConnections: []LLMConnection{{ID: "remote", Endpoint: upstream.URL, Mode: "stateful"}}}
	r := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"model":"`+routedModel("remote", "model")+`","stream":true,"glasses_hint":true,"glasses_hint_phase":"judge","messages":[{"role":"user","content":"synthetic"}]}`))
	w := httptest.NewRecorder()
	handleChat(w, r, app, &AuthManager{users: map[string]*User{}})
	if calls != 2 || !strings.Contains(w.Body.String(), "ready") {
		t.Fatalf("calls=%d body=%s", calls, w.Body.String())
	}
	if nativeReasoningUnsupported(401, "reasoning unsupported invalid api key") || nativeReasoningUnsupported(400, "model not found") {
		t.Fatal("unrelated errors must not trigger reasoning retry")
	}
}

func TestNativeHintContractDoesNotInviteSchemaCopying(t *testing.T) {
	body := map[string]any{"system_prompt": "Select missing information.", "response_format": map[string]any{"json_schema": map[string]any{"name": "hint_judgment", "schema": map[string]any{"type": "object", "properties": map[string]any{"span": map[string]any{"type": "integer"}}}}}}
	prepareNativeAmbientPayload(body)
	system := fmt.Sprint(body["system_prompt"])
	if !strings.Contains(system, "span (integer source number; 0 means skip)") || strings.Contains(system, `"properties"`) || strings.Contains(system, `"type":"object"`) {
		t.Fatalf("schema copied into Native prompt: %s", system)
	}
	if body["reasoning"] != "off" || body["store"] != false {
		t.Fatal("ambient policy lost")
	}
}
