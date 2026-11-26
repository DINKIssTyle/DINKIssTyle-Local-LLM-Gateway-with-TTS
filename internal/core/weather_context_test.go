package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dinkisstyle-chat/internal/toolruntime"
)

// Exercise location preparation before the model responds, so this regression
// does not depend on whether a particular model decides to call the tool.
func TestChatPreparesWeatherLocationForNaturalQuestions(t *testing.T) {
	for _, model := range []string{"qwen-test", "gemma-4-e2b-it-mlx"} {
		for _, tc := range []struct {
			name, question, location string
			enableTools              bool
			wantLookups              int
			wantCoordinates          bool
		}{
			{"natural", "지금 밖에 날씨 어때?", `{"lat":37.56,"lon":126.98}`, true, 1, true},
			{"explicit_place", "부산 날씨 알려줘", `{"lat":37.56,"lon":126.98}`, true, 1, true},
			{"missing_location", "지금 밖에 날씨 어때?", "", true, 1, false},
			{"invalid_location", "지금 밖에 날씨 어때?", `{"lat":200,"lon":126.98}`, true, 1, false},
			{"unrelated", "안녕하세요", `{"lat":37.56,"lon":126.98}`, true, 0, false},
			{"disabled", "지금 밖에 날씨 어때?", `{"lat":37.56,"lon":126.98}`, false, 0, false},
		} {
			t.Run(model+"/"+tc.name, func(t *testing.T) {
				registry := toolruntime.NewRegistry()
				lookups := 0
				registerDiagnosticTestTool(t, registry, "get_current_location", toolruntime.Metadata{ReadOnly: true}, func(_ context.Context, execCtx toolruntime.ExecutionContext, _ json.RawMessage) (toolruntime.Result, error) {
					lookups++
					return toolruntime.Result{Content: execCtx.LocationInfo}, nil
				})
				old := toolruntime.Default
				toolruntime.Default = registry
				t.Cleanup(func() { toolruntime.Default = old })
				requests := 0
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						return
					}
					encoded, _ := json.Marshal(body)
					if got := strings.Contains(string(encoded), "37.56, 126.98"); got != tc.wantCoordinates {
						t.Errorf("coordinates provided to model = %v, want %v", got, tc.wantCoordinates)
					}
					if !strings.Contains(string(encoded), tc.question) {
						t.Error("original request, including any explicit place, was lost")
					}
					if lookups != tc.wantLookups {
						t.Errorf("location lookups before model response = %d, want %d", lookups, tc.wantLookups)
					}
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"완료했습니다.\"}}]}\n\ndata: [DONE]\n\n")
				}))
				defer upstream.Close()
				body, _ := json.Marshal(map[string]any{"model": model, "stream": true, "messages": []any{map[string]any{"role": "user", "content": tc.question}}})
				request := httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(body)))
				request.Header.Set("X-User-Location", tc.location)
				recorder := httptest.NewRecorder()
				handleChat(recorder, request, &App{llmEndpoint: upstream.URL, llmMode: "standard", enableTools: tc.enableTools}, &AuthManager{users: map[string]*User{}})
				if requests != 1 || !strings.Contains(recorder.Body.String(), "완료했습니다.") {
					t.Fatalf("unexpected chat result (%d requests): %s", requests, recorder.Body.String())
				}
			})
		}
	}
}
