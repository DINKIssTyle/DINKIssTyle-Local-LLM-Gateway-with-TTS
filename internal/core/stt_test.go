package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSTTModelStatusAndPlan(t *testing.T) {
	status := getSTTModelStatusFor(defaultSTTModelID)
	if status.Key != "stt:whisper-base" {
		t.Fatalf("expected key stt:whisper-base, got %s", status.Key)
	}
	if status.Kind != "stt" {
		t.Fatalf("expected kind stt, got %s", status.Kind)
	}
	if !status.CanDownload {
		t.Fatalf("expected CanDownload to be true")
	}

	app := &App{}
	managed := app.GetManagedModels()
	found := false
	for _, m := range managed {
		if m.Key == "stt:whisper-base" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("stt:whisper-base not found in GetManagedModels()")
	}

	planStatus, specs, afterDownload, err := app.getManagedModelDownloadPlan("stt:whisper-base")
	if err != nil {
		t.Fatalf("failed to get download plan: %v", err)
	}
	if planStatus.Key != "stt:whisper-base" {
		t.Fatalf("expected plan status key stt:whisper-base, got %s", planStatus.Key)
	}
	if len(specs) != len(getRequiredSTTFiles(defaultSTTModelID)) {
		t.Fatalf("unexpected download specs for whisper-base, got %d", len(specs))
	}
	if afterDownload == nil {
		t.Fatalf("expected afterDownload callback to be non-nil")
	}
}

func TestSTTHandlers(t *testing.T) {
	app := &App{}

	// Test GET /api/stt/status
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/stt/status", nil)
	rrStatus := httptest.NewRecorder()
	handleSTTStatus(app)(rrStatus, reqStatus)

	if rrStatus.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rrStatus.Code)
	}

	var statusResp ManagedModelStatus
	if err := json.Unmarshal(rrStatus.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("failed to parse status response: %v", err)
	}
	if statusResp.Key != "stt:"+selectedSTTModelID() {
		t.Fatalf("expected key stt:whisper-base, got %s", statusResp.Key)
	}

	// Test POST /api/stt/transcribe with missing model
	reqTranscribe := httptest.NewRequest(http.MethodPost, "/api/stt/transcribe", strings.NewReader(`{"audio":""}`))
	reqTranscribe.Header.Set("Content-Type", "application/json")
	rrTranscribe := httptest.NewRecorder()
	handleSTTTranscribe(app)(rrTranscribe, reqTranscribe)

	// Since model is not installed by default in test env, it should respond 400
	if rrTranscribe.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for uninstalled model, got %d", rrTranscribe.Code)
	}
}

func TestEvenRealitiesPluginAssetsExist(t *testing.T) {
	pluginFiles := []string{
		filepath.Join("..", "..", "plugins", "even-realities", "package.json"),
		filepath.Join("..", "..", "plugins", "even-realities", "app.json"),
		filepath.Join("..", "..", "plugins", "even-realities", "index.html"),
		filepath.Join("..", "..", "plugins", "even-realities", "style.css"),
		filepath.Join("..", "..", "plugins", "even-realities", "plugin.js"),
	}

	for _, file := range pluginFiles {
		if !fileExists(file) {
			t.Errorf("expected plugin file %s to exist", file)
		}
	}
}
