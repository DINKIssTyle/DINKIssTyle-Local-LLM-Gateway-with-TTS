package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestModelEnsureReuseAndColdLoad(t *testing.T) {
	for _, mode := range []string{"standard", "stateful"} {
		for _, existing := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/existing=%t", mode, existing), func(t *testing.T) {
				var loaded atomic.Bool
				loaded.Store(existing)
				var loads atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Authorization") != "Bearer test-upstream" {
						t.Error("upstream credential missing")
					}
					switch r.URL.Path {
					case "/api/v1/models":
						instances := []any{}
						if loaded.Load() {
							instances = append(instances, map[string]string{"id": "gemma:2"})
						}
						json.NewEncoder(w).Encode(map[string]any{"models": []any{map[string]any{"key": "gemma", "loaded_instances": instances}}})
					case "/api/v1/models/load":
						loads.Add(1)
						var p map[string]string
						json.NewDecoder(r.Body).Decode(&p)
						if p["model"] != "gemma" {
							t.Error(p)
						}
						loaded.Store(true)
						fmt.Fprint(w, `{"status":"loaded","instance_id":"gemma:2"}`)
					default:
						t.Errorf("wrong model management path %s", r.URL.Path)
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				app := &App{llmEndpoint: server.URL, llmApiToken: "test-upstream", llmMode: mode}
				for i := 0; i < 2; i++ {
					req := httptest.NewRequest("POST", "/api/models/ensure", strings.NewReader(`{"model":"gemma"}`))
					w := httptest.NewRecorder()
					handleModels(w, req, app, &AuthManager{users: map[string]*User{}})
					if w.Code != 200 {
						t.Fatal(w.Code, w.Body.String())
					}
					var result map[string]string
					json.Unmarshal(w.Body.Bytes(), &result)
					if result["status"] != "ready" || result["instance_id"] != "gemma:2" {
						t.Fatal(result)
					}
				}
				want := int32(1)
				if existing {
					want = 0
				}
				if loads.Load() != want {
					t.Fatalf("loads=%d want %d", loads.Load(), want)
				}
			})
		}
	}
}

func TestModelEnsureConcurrentCanceledWaiter(t *testing.T) {
	var loaded atomic.Bool
	var loads atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/models/load" {
			if loads.Add(1) == 1 {
				close(started)
			}
			<-release
			loaded.Store(true)
			fmt.Fprint(w, `{"status":"loaded","instance_id":"gemma"}`)
			return
		}
		instances := []any{}
		if loaded.Load() {
			instances = append(instances, map[string]string{"id": "gemma"})
		}
		json.NewEncoder(w).Encode(map[string]any{"models": []any{map[string]any{"key": "vendor/gemma", "loaded_instances": instances}}})
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := ensureLoadedModel(ctx, "gemma", server.URL, ""); done <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("load never started")
	}
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		model := "gemma"
		if i%2 == 1 {
			model = "vendor/gemma"
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := ensureLoadedModel(context.Background(), model, server.URL, "")
			if err != nil || result.InstanceID != "gemma" {
				t.Errorf("%+v %v", result, err)
			}
		}()
	}
	close(release)
	wg.Wait()
	if loads.Load() != 1 {
		t.Fatalf("canceled/parallel calls created %d copies", loads.Load())
	}
}

func TestModelEnsureFailureDoesNotPretendReady(t *testing.T) {
	for _, status := range []int{401, 404, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var loads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					loads.Add(1)
				}
				http.Error(w, "cannot list models", status)
			}))
			defer server.Close()
			_, err := ensureLoadedModel(context.Background(), "gemma", server.URL, "")
			if err == nil {
				t.Fatal("failed inspection pretended ready")
			}
			if loads.Load() != 0 {
				t.Fatal("blindly loaded on inspection failure")
			}
		})
	}
}

func TestModelEnsureFailedLoadCanRetry(t *testing.T) {
	var attempts atomic.Int32
	var loaded atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			if attempts.Add(1) == 1 {
				http.Error(w, "not enough memory", 500)
				return
			}
			loaded.Store(true)
			fmt.Fprint(w, `{"status":"loaded","instance_id":"gemma"}`)
			return
		}
		instances := []any{}
		if loaded.Load() {
			instances = append(instances, map[string]string{"id": "gemma"})
		}
		json.NewEncoder(w).Encode(map[string]any{"models": []any{map[string]any{"key": "gemma", "loaded_instances": instances}}})
	}))
	defer server.Close()
	if _, err := ensureLoadedModel(context.Background(), "gemma", server.URL, ""); err == nil {
		t.Fatal("failed load accepted")
	}
	if _, err := ensureLoadedModel(context.Background(), "gemma", server.URL, ""); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 {
		t.Fatal(attempts.Load())
	}
}

func TestModelEnsureUnambiguousBasename(t *testing.T) {
	for _, ambiguous := range []bool{false, true} {
		t.Run(fmt.Sprint(ambiguous), func(t *testing.T) {
			var loads atomic.Int32
			var loaded atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					loads.Add(1)
					var body map[string]string
					json.NewDecoder(r.Body).Decode(&body)
					if body["model"] != "vendor/gemma" {
						t.Error(body)
					}
					loaded.Store(true)
					fmt.Fprint(w, `{"instance_id":"gemma:2"}`)
					return
				}
				instances := []any{}
				if loaded.Load() {
					instances = append(instances, map[string]string{"id": "gemma:2"})
				}
				models := []any{map[string]any{"key": "vendor/gemma", "loaded_instances": instances}}
				if ambiguous {
					models = append(models, map[string]any{"key": "other/gemma", "loaded_instances": []any{}})
				}
				json.NewEncoder(w).Encode(map[string]any{"models": models})
			}))
			defer server.Close()
			result, err := ensureLoadedModel(context.Background(), "gemma", server.URL, "")
			if ambiguous {
				if err == nil || loads.Load() != 0 {
					t.Fatal("ambiguous model must not load", result, err)
				}
			} else if err != nil || result.InstanceID != "gemma:2" || loads.Load() != 1 {
				t.Fatal(result, err, loads.Load())
			}
		})
	}
}

func TestModelEnsurePackagedAppPreflight(t *testing.T) {
	handler := evenHubCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("preflight reached authenticated model loader") }))
	request := httptest.NewRequest("OPTIONS", "/api/models/ensure", nil)
	request.Header.Set("Origin", "http://127.0.0.1:34567")
	request.Header.Set("Access-Control-Request-Method", "POST")
	request.Header.Set("Access-Control-Request-Headers", "Authorization,Content-Type")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 204 || response.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:34567" {
		t.Fatal(response.Code, response.Header())
	}
}
