package core

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GetOpenClawModels lists model IDs without exposing upstream credentials.
func (a *App) GetOpenClawModels(userID string) ([]map[string]string, error) {
	if a.authMgr == nil {
		return nil, fmt.Errorf("계정 인증이 초기화되지 않았습니다")
	}
	a.authMgr.mu.RLock()
	user := a.authMgr.users[userID]
	if user == nil {
		a.authMgr.mu.RUnlock()
		return nil, fmt.Errorf("계정을 찾을 수 없습니다")
	}
	settings := user.Settings
	a.authMgr.mu.RUnlock()
	endpoint, token, mode := a.currentLLMRequestConfig()
	if settings.ApiToken != nil {
		token = sanitizeLLMToken(*settings.ApiToken)
	}
	if settings.LLMMode != nil {
		mode = normalizeLLMMode(*settings.LLMMode)
	}
	a.serverMux.Lock()
	connections := append([]LLMConnection{{Name: "Default LLM", Endpoint: endpoint, APIKey: token, Mode: mode}}, a.llmConnections...)
	a.serverMux.Unlock()
	for i := 1; i < len(connections); i++ {
		resolved, err := a.resolveLLMConnection(connections[i].ID, userID, a.authMgr)
		if err != nil {
			return nil, err
		}
		connections[i] = resolved
	}
	results := make([][]map[string]string, len(connections))
	var wg sync.WaitGroup
	for i, c := range connections {
		wg.Add(1)
		go func(i int, c LLMConnection) {
			defer wg.Done()
			models, err := fetchConnectionCatalog(context.Background(), c)
			if err != nil {
				return
			}
			for _, model := range models {
				if kind, _ := model["type"].(string); kind != "" && kind != "llm" {
					continue
				}
				id, _ := model["id"].(string)
				if id == "" {
					id, _ = model["key"].(string)
				}
				if id == "" {
					continue
				}
				name, _ := model["display_name"].(string)
				if name == "" {
					name = id
				}
				if c.ID != "" {
					id = routedModel(c.ID, id)
				}
				results[i] = append(results[i], map[string]string{"id": id, "name": "[" + c.Name + "] " + name})
			}
		}(i, c)
	}
	wg.Wait()
	rows := []map[string]string{}
	for _, result := range results {
		rows = append(rows, result...)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("모델 목록을 가져오지 못했습니다. LLM 연결과 API 키를 확인하세요")
	}
	return rows, nil
}

// The common chat pipeline streams internally. Buffer only non-streaming
// compatibility requests and return the OpenAI Chat Completions JSON shape.
type openClawJSONResponse struct {
	header http.Header
	status int
	bytes.Buffer
}

func (w *openClawJSONResponse) Header() http.Header { return w.header }
func (w *openClawJSONResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *openClawJSONResponse) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.Buffer.Write(data)
}
func (w *openClawJSONResponse) Flush() {}

func (w *openClawJSONResponse) finish(dst http.ResponseWriter) {
	if w.status >= 400 {
		http.Error(dst, w.String(), w.status)
		return
	}
	reader := bufio.NewReader(&w.Buffer)
	var content strings.Builder
	for {
		block, err := readUpstreamSSEBlock(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(dst, err.Error(), http.StatusBadGateway)
			return
		}
		if block.EventName == "error" {
			http.Error(dst, block.Data, http.StatusBadGateway)
			return
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(block.Data), &chunk) == nil {
			for _, choice := range chunk.Choices {
				content.WriteString(choice.Delta.Content)
			}
		}
	}
	dst.Header().Set("Content-Type", "application/json")
	json.NewEncoder(dst).Encode(map[string]any{
		"id": "chatcmpl-" + rand.Text(), "object": "chat.completion", "created": time.Now().Unix(), "model": "openclaw",
		"choices": []any{map[string]any{"index": 0, "message": map[string]string{"role": "assistant", "content": content.String()}, "finish_reason": "stop"}},
	})
}

// OpenClawSettings belongs to one account; only a hash of the external token
// is persisted. The raw token is returned once, at issuance.
type OpenClawSettings struct {
	Enabled   bool   `json:"enabled"`
	Model     string `json:"model"`
	TokenHash string `json:"token_hash,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
}

type OpenClawStatus struct {
	Enabled        bool   `json:"enabled"`
	Model          string `json:"model"`
	HasToken       bool   `json:"has_token"`
	Endpoint       string `json:"endpoint"`
	CustomEndpoint string `json:"custom_endpoint"`
}

const openClawTokenPrefix = "dkst_oc_"

type openClawAuthKey struct{}

func (a *App) GetUserOpenClawSettings(userID string) (OpenClawStatus, error) {
	if a.authMgr == nil {
		return OpenClawStatus{}, fmt.Errorf("계정 인증이 초기화되지 않았습니다")
	}
	a.authMgr.mu.RLock()
	user := a.authMgr.users[userID]
	if user == nil {
		a.authMgr.mu.RUnlock()
		return OpenClawStatus{}, fmt.Errorf("계정을 찾을 수 없습니다")
	}
	settings := user.Settings.OpenClaw
	a.authMgr.mu.RUnlock()
	a.serverMux.Lock()
	port, _ := strconv.Atoi(a.port)
	a.serverMux.Unlock()
	host := "127.0.0.1"
	if addresses, err := net.InterfaceAddrs(); err == nil {
		for _, address := range addresses {
			if network, ok := address.(*net.IPNet); ok && network.IP.To4() != nil && network.IP.IsPrivate() && !network.IP.IsLoopback() {
				host = network.IP.String()
				break
			}
		}
	}
	endpoint := settings.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("http://%s:%d/v1/chat/completions", host, port+1)
	}
	return OpenClawStatus{Enabled: settings.Enabled, Model: settings.Model, HasToken: settings.TokenHash != "", Endpoint: endpoint, CustomEndpoint: settings.Endpoint}, nil
}

// This is the client-facing address to copy into Even. It does not change
// the local listener or the upstream LLM connection.
func normalizeOpenClawEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || len(endpoint) > 8192 || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", fmt.Errorf("서버 주소는 인증 정보·쿼리 없이 http:// 또는 https:// 전체 주소로 입력하세요")
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", fmt.Errorf("서버 포트는 1~65535로 입력하세요")
		}
	}
	if parsed.Path == "" || parsed.Path == "/" || parsed.Path == "/v1" || parsed.Path == "/v1/" {
		parsed.Path = "/v1/chat/completions"
		parsed.RawPath = ""
	}
	return parsed.String(), nil
}

func (a *App) SetUserOpenClawSettings(userID string, enabled bool, model, endpoint string) error {
	if a.authMgr == nil {
		return fmt.Errorf("계정 인증이 초기화되지 않았습니다")
	}
	endpoint, err := normalizeOpenClawEndpoint(endpoint)
	if err != nil {
		return err
	}
	model = strings.TrimSpace(model)
	if enabled && model == "" {
		return fmt.Errorf("대상 모델을 선택하세요")
	}
	if len(model) > 4096 {
		return fmt.Errorf("모델 ID가 너무 깁니다")
	}
	id, _, err := splitRoutedModel(model)
	if err != nil {
		return err
	}
	if id != "" {
		if _, err := a.resolveLLMConnection(id, userID, a.authMgr); err != nil {
			return err
		}
	}
	am := a.authMgr
	am.mu.Lock()
	defer am.mu.Unlock()
	user := am.users[userID]
	if user == nil {
		return fmt.Errorf("계정을 찾을 수 없습니다")
	}
	old := user.Settings.OpenClaw
	next := old
	next.Enabled, next.Model, next.Endpoint = enabled, model, endpoint
	if !enabled {
		next.TokenHash = ""
	}
	user.Settings.OpenClaw = next
	if err := am.saveUsersLocked(); err != nil {
		user.Settings.OpenClaw = old
		return err
	}
	return nil
}

func (a *App) IssueUserOpenClawToken(userID string) (string, error) {
	if a.authMgr == nil {
		return "", fmt.Errorf("계정 인증이 초기화되지 않았습니다")
	}
	am := a.authMgr
	am.mu.Lock()
	defer am.mu.Unlock()
	user := am.users[userID]
	if user == nil {
		return "", fmt.Errorf("계정을 찾을 수 없습니다")
	}
	old := user.Settings.OpenClaw
	if !old.Enabled || old.Model == "" {
		return "", fmt.Errorf("모델을 선택하고 openclaw 연결을 켜세요")
	}
	token := openClawTokenPrefix + generateToken()
	user.Settings.OpenClaw.TokenHash = hashToken(token)
	if err := am.saveUsersLocked(); err != nil {
		user.Settings.OpenClaw = old
		return "", err
	}
	return token, nil
}

func (am *AuthManager) validateOpenClawToken(token string) (*User, bool) {
	if am == nil || !strings.HasPrefix(token, openClawTokenPrefix) {
		return nil, false
	}
	hashed := hashToken(token)
	am.mu.RLock()
	defer am.mu.RUnlock()
	for _, user := range am.users {
		settings := user.Settings.OpenClaw
		if settings.Enabled && settings.Model != "" && settings.TokenHash != "" && subtle.ConstantTimeCompare([]byte(settings.TokenHash), []byte(hashed)) == 1 {
			return user, true
		}
	}
	return nil, false
}

func (am *AuthManager) resolveOpenClawModel(model, userID string) (string, bool, error) {
	if model != "openclaw" {
		return model, false, nil
	}
	if am == nil {
		return "", false, fmt.Errorf("계정 인증이 필요합니다")
	}
	am.mu.RLock()
	defer am.mu.RUnlock()
	user := am.users[userID]
	if user == nil || !user.Settings.OpenClaw.Enabled || user.Settings.OpenClaw.Model == "" {
		return "", false, fmt.Errorf("계정 설정에서 openclaw 연결을 켜고 대상 모델을 선택하세요")
	}
	return user.Settings.OpenClaw.Model, true, nil
}
