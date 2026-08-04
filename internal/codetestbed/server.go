package codetestbed

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

//go:embed web/*
var webAssets embed.FS

type Server struct {
	Agent Agent
	busy  chan struct{}
}

func NewServer() *Server {
	return &Server{busy: make(chan struct{}, 1)}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	assets, _ := fs.Sub(webAssets, "web")
	mux.Handle("GET /", http.FileServer(http.FS(assets)))
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("GET /api/models", s.handleModels)
	mux.HandleFunc("POST /api/run", s.handleRun)
	return securityHeaders(mux)
}

func (s *Server) handleConfig(writer http.ResponseWriter, _ *http.Request) {
	workspace, _ := os.Getwd()
	writeJSON(writer, http.StatusOK, map[string]interface{}{
		"workspace": workspace,
		"endpoint":  "http://127.0.0.1:1234/v1",
		"max_steps": 8,
	})
}

func (s *Server) handleModels(writer http.ResponseWriter, request *http.Request) {
	endpoint := strings.TrimRight(strings.TrimSpace(request.URL.Query().Get("endpoint")), "/")
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Hostname() == "" || !isLoopbackHost(parsed.Hostname()) {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "a local endpoint is required"})
		return
	}
	modelsURL := endpoint
	if !strings.HasSuffix(modelsURL, "/models") {
		modelsURL += "/models"
	}
	ctx, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	httpRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if key := strings.TrimSpace(request.Header.Get("X-Testbed-API-Key")); key != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+key)
	}
	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		writeJSON(writer, http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("model endpoint returned HTTP %d", response.StatusCode)})
		return
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &payload) != nil {
		writeJSON(writer, http.StatusBadGateway, map[string]string{"error": "invalid model list response"})
		return
	}
	models := make([]string, 0, len(payload.Data))
	for _, model := range payload.Data {
		if strings.TrimSpace(model.ID) != "" {
			models = append(models, model.ID)
		}
	}
	writeJSON(writer, http.StatusOK, map[string]interface{}{"models": models})
}

func (s *Server) handleRun(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1024*1024)
	defer request.Body.Close()
	var input RunRequest
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}
	select {
	case s.busy <- struct{}{}:
		defer func() { <-s.busy }()
	default:
		writeJSON(writer, http.StatusConflict, map[string]string{"error": "another testbed run is already active"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Minute)
	defer cancel()
	result := s.runPipeline(ctx, input)
	status := http.StatusOK
	if result.Failure != "" && result.Rounds == 0 {
		status = http.StatusBadRequest
	}
	writeJSON(writer, status, result)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(writer, request)
	})
}

func writeJSON(writer http.ResponseWriter, status int, value interface{}) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
