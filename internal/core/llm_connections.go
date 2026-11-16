package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

type LLMConnection struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Endpoint     string `json:"endpoint"`
	Mode         string `json:"mode"`
	ManageModels bool   `json:"manage_models"`
	APIKey       string `json:"api_key,omitempty"`
	HasAPIKey    bool   `json:"has_api_key,omitempty"`
}
type LLMConnectionUpdate struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Endpoint     string  `json:"endpoint"`
	Mode         string  `json:"mode"`
	ManageModels bool    `json:"manage_models"`
	APIKey       *string `json:"api_key,omitempty"`
}

var connectionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

const llmRoutePrefix = "@llm/"

func routedModel(id, model string) string {
	return llmRoutePrefix + id + "/" + base64.RawURLEncoding.EncodeToString([]byte(model))
}
func splitRoutedModel(model string) (string, string, error) {
	if !strings.HasPrefix(model, llmRoutePrefix) {
		return "", model, nil
	}
	parts := strings.Split(strings.TrimPrefix(model, llmRoutePrefix), "/")
	if len(parts) != 2 || !connectionIDPattern.MatchString(parts[0]) {
		return "", "", fmt.Errorf("잘못된 LLM 연결 모델 ID")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(raw) == 0 || len(raw) > 4096 {
		return "", "", fmt.Errorf("잘못된 LLM 모델 ID")
	}
	return parts[0], string(raw), nil
}
func (a *App) GetLLMConnections() []LLMConnection {
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	result := append([]LLMConnection{}, a.llmConnections...)
	for i := range result {
		result[i].HasAPIKey = result[i].APIKey != ""
		result[i].APIKey = ""
	}
	return result
}
func (a *App) SetLLMConnections(updates []LLMConnectionUpdate) error {
	if len(updates) > 8 {
		return fmt.Errorf("추가 LLM 연결은 최대 8개입니다")
	}
	a.serverMux.Lock()
	old := make(map[string]string)
	for _, c := range a.llmConnections {
		old[c.ID] = c.APIKey
	}
	a.serverMux.Unlock()
	next := make([]LLMConnection, 0, len(updates))
	seen := map[string]bool{}
	for _, u := range updates {
		if !connectionIDPattern.MatchString(u.ID) || seen[u.ID] {
			return fmt.Errorf("연결 ID가 잘못되었거나 중복되었습니다")
		}
		seen[u.ID] = true
		parsed, err := url.Parse(strings.TrimSpace(u.Endpoint))
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("연결 주소는 http(s) API 주소로 입력하세요")
		}
		name := strings.TrimSpace(u.Name)
		if name == "" || len([]rune(name)) > 80 {
			return fmt.Errorf("연결 이름은 1~80자입니다")
		}
		key := old[u.ID]
		if u.APIKey != nil {
			key = sanitizeLLMToken(*u.APIKey)
		}
		next = append(next, LLMConnection{ID: u.ID, Name: name, Endpoint: normalizeLLMEndpoint(u.Endpoint), Mode: normalizeLLMMode(u.Mode), ManageModels: u.ManageModels, APIKey: key})
	}
	a.serverMux.Lock()
	a.llmConnections = next
	defer a.serverMux.Unlock()
	a.saveConfig()
	return nil
}
func (a *App) resolveLLMConnection(id, userID string, am *AuthManager) (LLMConnection, error) {
	a.serverMux.Lock()
	var chosen *LLMConnection
	for _, c := range a.llmConnections {
		if c.ID == id {
			copy := c
			chosen = &copy
			break
		}
	}
	a.serverMux.Unlock()
	if chosen == nil {
		return LLMConnection{}, fmt.Errorf("LLM 연결이 삭제되었거나 없습니다. 모델을 다시 선택하세요")
	}
	if am != nil && userID != "" {
		am.mu.RLock()
		if user := am.users[userID]; user != nil {
			if key, ok := user.Settings.LLMAPIKeys[id]; ok {
				chosen.APIKey = key
			}
		}
		am.mu.RUnlock()
	}
	return *chosen, nil
}
func (a *App) GetUserLLMConnections(userID string) ([]LLMConnection, error) {
	rows := a.GetLLMConnections()
	if a.authMgr == nil {
		return rows, nil
	}
	a.authMgr.mu.RLock()
	defer a.authMgr.mu.RUnlock()
	user := a.authMgr.users[userID]
	if user == nil {
		return nil, fmt.Errorf("계정을 찾을 수 없습니다")
	}
	for i := range rows {
		rows[i].Endpoint = ""
		rows[i].HasAPIKey = user.Settings.LLMAPIKeys[rows[i].ID] != ""
	}
	return rows, nil
}
func (a *App) SetUserLLMAPIKeys(userID string, keys map[string]string) error {
	return a.setConnectionKeys(a.authMgr, userID, keys)
}
func (a *App) setConnectionKeys(am *AuthManager, userID string, keys map[string]string) error {
	if am == nil {
		return fmt.Errorf("로그인이 필요합니다")
	}
	if len(keys) > 8 {
		return fmt.Errorf("연결 수가 너무 많습니다")
	}
	valid := map[string]bool{}
	for _, c := range a.GetLLMConnections() {
		valid[c.ID] = true
	}
	for id, key := range keys {
		if !valid[id] || len(key) > 8192 {
			return fmt.Errorf("등록되지 않은 연결 또는 잘못된 키")
		}
	}
	am.mu.Lock()
	defer am.mu.Unlock()
	user := am.users[userID]
	if user == nil {
		return fmt.Errorf("로그인이 필요합니다")
	}
	if user.Settings.LLMAPIKeys == nil {
		user.Settings.LLMAPIKeys = map[string]string{}
	}
	for id, key := range keys {
		if strings.TrimSpace(key) == "" {
			delete(user.Settings.LLMAPIKeys, id)
		} else {
			user.Settings.LLMAPIKeys[id] = sanitizeLLMToken(key)
		}
	}
	return am.saveUsersLocked()
}

// Account credentials never enter the returned model list or the shared disk cache.
func fetchConnectionCatalog(ctx context.Context, c LLMConnection) ([]map[string]any, error) {
	path := "/v1/models"
	if c.Mode == "stateful" {
		path = "/api/v1/models"
	}
	req, err := http.NewRequestWithContext(ctx, "GET", normalizeLLMEndpoint(c.Endpoint)+path, nil)
	if err != nil {
		return nil, err
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data   []map[string]any `json:"data"`
		Models []map[string]any `json:"models"`
	}
	if err = json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	models := envelope.Data
	if models == nil {
		models = envelope.Models
	}
	return models, nil
}
func mergedModelCatalog(w http.ResponseWriter, r *http.Request, app *App, am *AuthManager) {
	endpoint, token, mode := resolveModelAPIConfig(r, app, am, r.URL.Query().Get("mode"))
	connections := []LLMConnection{{Name: "기본 LLM", Endpoint: endpoint, Mode: mode, APIKey: token}}
	for _, c := range app.GetLLMConnections() {
		resolved, err := app.resolveLLMConnection(c.ID, r.Header.Get("X-User-ID"), am)
		if err == nil {
			connections = append(connections, resolved)
		}
	}
	results := make([][]map[string]any, len(connections))
	states := make([]map[string]string, len(connections))
	var wg sync.WaitGroup
	for i, c := range connections {
		wg.Add(1)
		go func(i int, c LLMConnection) {
			defer wg.Done()
			models, err := fetchConnectionCatalog(r.Context(), c)
			states[i] = map[string]string{"id": c.ID, "name": c.Name, "status": "ready"}
			if err != nil {
				states[i]["status"] = "unavailable"
				states[i]["error"] = err.Error()
				return
			}
			for _, m := range models {
				if kind, ok := m["type"].(string); ok && kind != "" && kind != "llm" {
					continue
				}
				id, _ := m["id"].(string)
				if id == "" {
					id, _ = m["key"].(string)
				}
				if id == "" {
					continue
				}
				label, _ := m["display_name"].(string)
				if label == "" {
					label, _ = m["name"].(string)
				}
				if label == "" {
					label = id
				}
				if c.ID != "" {
					id = routedModel(c.ID, id)
					if instances, ok := m["loaded_instances"].([]any); ok {
						for _, v := range instances {
							if inst, ok := v.(map[string]any); ok {
								if raw, ok := inst["id"].(string); ok {
									inst["id"] = routedModel(c.ID, raw)
								}
							}
						}
					}
				}
				m["id"] = id
				m["key"] = id
				m["display_name"] = label
				if c.ID != "" {
					m["display_name"] = "[" + c.Name + "] " + label
				}
				m["connection_id"] = c.ID
				m["connection_name"] = c.Name
				m["can_unload"] = c.ID == "" || c.ManageModels
				results[i] = append(results[i], m)
			}
		}(i, c)
	}
	wg.Wait()
	all := []map[string]any{}
	for _, models := range results {
		all = append(all, models...)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]any{"models": all, "connections": states})
}

func normalizeConnectionPayload(body map[string]any, mode string) error {
	if mode != "stateful" {
		if _, ok := body["messages"]; !ok {
			if input, ok := body["input"]; ok {
				messages := []any{}
				if system, ok := body["system_prompt"].(string); ok && system != "" {
					messages = append(messages, map[string]any{"role": "system", "content": system})
				}
				messages = append(messages, map[string]any{"role": "user", "content": input})
				body["messages"] = messages
			}
		}
		if _, ok := body["messages"]; ok {
			delete(body, "input")
			delete(body, "system_prompt")
			delete(body, "previous_response_id")
		}
	} else if _, ok := body["input"]; !ok {
		if messages, ok := body["messages"].([]any); ok {
			systems, turns := []string{}, []string{}
			for _, item := range messages {
				m, ok := item.(map[string]any)
				if !ok {
					return fmt.Errorf("잘못된 대화 메시지")
				}
				content, ok := m["content"].(string)
				if !ok {
					return fmt.Errorf("이 Native 연결은 텍스트 메시지를 사용하세요. 이미지 대화는 OpenAI 호환 방식을 선택하세요")
				}
				role, _ := m["role"].(string)
				if role == "system" {
					systems = append(systems, content)
				} else {
					turns = append(turns, role+": "+content)
				}
			}
			body["input"] = strings.Join(turns, "\n\n")
			if len(systems) > 0 {
				body["system_prompt"] = strings.Join(systems, "\n\n")
			}
			delete(body, "messages")
		}
	}
	return nil
}

// Native LM Studio uses different token/reasoning controls. JSON schema is
// supplied as an output contract because /api/v1/chat has no response_format.
// The client still validates the result; no hidden reasoning becomes an answer.
func prepareNativeAmbientPayload(body map[string]any) {
	body["reasoning"] = "off"
	if limit, ok := body["max_tokens"]; ok {
		body["max_output_tokens"] = limit
	} else if _, ok := body["max_output_tokens"]; !ok {
		body["max_output_tokens"] = 2048
	}
	if format, ok := body["response_format"].(map[string]any); ok {
		if spec, ok := format["json_schema"].(map[string]any); ok {
			if schema, ok := spec["schema"]; ok {
				contract, _ := json.Marshal(schema)
				system, _ := body["system_prompt"].(string)
				if spec["name"] == "hint_judgment" {
					contract = []byte("Return one JSON object with span (integer source number; 0 means skip), topic (exact phrase in that span; empty for skip), need (short missing-information task; empty for skip), kind (key/term/verify/schedule), source (internal/web/memory). No hint/evidence/continuity fields, schema definitions or null values. A positive span must be one of the numbered sources in the input.")
				}
				body["system_prompt"] = system + "\nJSON output contract:\n" + string(contract)
			}
		}
	}
	for _, key := range []string{"max_tokens", "max_completion_tokens", "chat_template_kwargs", "reasoning_effort", "response_format", "previous_response_id"} {
		delete(body, key)
	}
	body["store"] = false
}
func nativeReasoningUnsupported(status int, message string) bool {
	if status != 400 && status != 422 {
		return false
	}
	text := strings.ToLower(message)
	return strings.Contains(text, "reasoning") && (strings.Contains(text, "not support") || strings.Contains(text, "unsupported") || strings.Contains(text, "not allowed"))
}
