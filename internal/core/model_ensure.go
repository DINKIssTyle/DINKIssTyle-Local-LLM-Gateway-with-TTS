package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type modelReady struct {
	Model      string `json:"model"`
	InstanceID string `json:"instance_id"`
}
type modelEnsureJob struct {
	done  chan struct{}
	ready modelReady
	err   error
}

var modelEnsures = struct {
	sync.Mutex
	jobs map[[32]byte]*modelEnsureJob
}{jobs: make(map[[32]byte]*modelEnsureJob)}

// All callers, including separate WebViews, share one load. A canceled caller
// stops waiting, but does not abandon a native load and launch another copy.
func ensureLoadedModel(ctx context.Context, model, endpoint, token string) (modelReady, error) {
	if err := ctx.Err(); err != nil {
		return modelReady{}, err
	}
	endpoint = normalizeLLMEndpoint(endpoint)
	token = sanitizeLLMToken(token)
	model = strings.TrimSpace(model)
	if model == "" || len(model) > 1024 {
		return modelReady{}, fmt.Errorf("invalid model ID")
	}
	// Resolve aliases before coalescing so "gemma" and "vendor/gemma"
	// cannot independently load two copies of the same underlying model.
	selected, instance, err := queryNativeModel(ctx, model, endpoint, token)
	if err != nil {
		return modelReady{}, err
	}
	if instance != "" {
		return modelReady{selected.Key, instance}, nil
	}
	model = selected.Key
	key := sha256.Sum256([]byte(endpoint + "\x00" + token + "\x00" + model))
	modelEnsures.Lock()
	job := modelEnsures.jobs[key]
	if job == nil {
		if len(modelEnsures.jobs) >= 32 {
			modelEnsures.Unlock()
			return modelReady{}, fmt.Errorf("모델 준비 요청이 많습니다. 잠시 후 다시 시도하세요")
		}
		job = &modelEnsureJob{done: make(chan struct{})}
		modelEnsures.jobs[key] = job
		go func() {
			work, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()
			job.ready, job.err = prepareLoadedModel(work, model, endpoint, token)
			modelEnsures.Lock()
			delete(modelEnsures.jobs, key)
			close(job.done)
			modelEnsures.Unlock()
		}()
	}
	modelEnsures.Unlock()
	select {
	case <-ctx.Done():
		return modelReady{}, ctx.Err()
	case <-job.done:
		return job.ready, job.err
	}
}

type nativeLoadModel struct {
	Key    string `json:"key"`
	Loaded []struct {
		ID string `json:"id"`
	} `json:"loaded_instances"`
}

func modelManagementRequest(ctx context.Context, endpoint, token, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		data, _ := json.Marshal(payload)
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint+path, body)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		req.Header.Set("Authorization", "Bearer lm-studio")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(data) > 500 {
			data = data[:500]
		}
		return fmt.Errorf("모델 관리 API HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("잘못된 모델 관리 응답: %w", err)
		}
	}
	return nil
}
func queryNativeModel(ctx context.Context, model, endpoint, token string) (nativeLoadModel, string, error) {
	query, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var listing struct {
		Models []nativeLoadModel `json:"models"`
	}
	if err := modelManagementRequest(query, endpoint, token, "GET", "/api/v1/models", nil, &listing); err != nil {
		return nativeLoadModel{}, "", err
	}
	for _, candidate := range listing.Models {
		// An explicitly selected existing instance is reused as-is.
		for _, instance := range candidate.Loaded {
			if instance.ID == model {
				return candidate, instance.ID, nil
			}
		}
		if candidate.Key == model {
			for _, instance := range candidate.Loaded {
				if instance.ID != "" {
					return candidate, instance.ID, nil
				}
			}
			return candidate, "", nil
		}
	}
	// OpenAI-compatible model lists can omit the publisher prefix. Resolve only
	// an unambiguous exact basename, never a fuzzy model/quantization match.
	var aliases []nativeLoadModel
	for _, candidate := range listing.Models {
		if strings.HasSuffix(candidate.Key, "/"+model) {
			aliases = append(aliases, candidate)
		}
	}
	if len(aliases) == 1 {
		candidate := aliases[0]
		for _, instance := range candidate.Loaded {
			if instance.ID != "" {
				return candidate, instance.ID, nil
			}
		}
		return candidate, "", nil
	}
	return nativeLoadModel{}, "", fmt.Errorf("모델 목록에서 %q을 찾지 못했습니다. 모델 목록을 새로고침하세요", model)
}
func prepareLoadedModel(ctx context.Context, model, endpoint, token string) (modelReady, error) {
	selected, instance, err := queryNativeModel(ctx, model, endpoint, token)
	if err != nil {
		return modelReady{}, err
	}
	if instance != "" {
		return modelReady{selected.Key, instance}, nil
	}
	// Model-management API is independent of the chat protocol (standard/stateful).
	var loaded struct {
		InstanceID string `json:"instance_id"`
		Error      string `json:"error"`
	}
	if err = modelManagementRequest(ctx, endpoint, token, "POST", "/api/v1/models/load", map[string]string{"model": selected.Key}, &loaded); err != nil {
		return modelReady{}, err
	}
	if loaded.Error != "" {
		return modelReady{}, fmt.Errorf("모델 로드 실패: %s", loaded.Error)
	}
	for {
		_, instance, err = queryNativeModel(ctx, selected.Key, endpoint, token)
		if err != nil {
			return modelReady{}, err
		}
		if instance != "" {
			return modelReady{selected.Key, instance}, nil
		}
		select {
		case <-ctx.Done():
			return modelReady{}, fmt.Errorf("모델 준비 시간 초과: %w", ctx.Err())
		case <-time.After(300 * time.Millisecond):
		}
	}
}
