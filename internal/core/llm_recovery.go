package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"dinkisstyle-chat/internal/chatharness"
	"dinkisstyle-chat/internal/mcp"
	"dinkisstyle-chat/internal/promptkit"
)

type modelAdaptiveHint struct {
	DisableStrictSchema    bool
	MinStructuredMaxTokens int
}

var modelAdaptiveState = struct {
	mu    sync.RWMutex
	hints map[string]modelAdaptiveHint
}{
	hints: map[string]modelAdaptiveHint{},
}

func normalizeAdaptiveModelKey(modelID string) string {
	return strings.ToLower(strings.TrimSpace(modelID))
}

func getModelAdaptiveHint(modelID string) modelAdaptiveHint {
	key := normalizeAdaptiveModelKey(modelID)
	if key == "" {
		return modelAdaptiveHint{}
	}
	modelAdaptiveState.mu.RLock()
	defer modelAdaptiveState.mu.RUnlock()
	return modelAdaptiveState.hints[key]
}

func noteModelSchemaUnsupported(modelID string) {
	key := normalizeAdaptiveModelKey(modelID)
	if key == "" {
		return
	}
	modelAdaptiveState.mu.Lock()
	h := modelAdaptiveState.hints[key]
	h.DisableStrictSchema = true
	modelAdaptiveState.hints[key] = h
	modelAdaptiveState.mu.Unlock()
}

func noteModelJSONTruncated(modelID string) {
	key := normalizeAdaptiveModelKey(modelID)
	if key == "" {
		return
	}
	modelAdaptiveState.mu.Lock()
	h := modelAdaptiveState.hints[key]
	if h.MinStructuredMaxTokens < 3200 {
		h.MinStructuredMaxTokens = 3200
	}
	modelAdaptiveState.hints[key] = h
	modelAdaptiveState.mu.Unlock()
}

func resetModelAdaptiveHints() {
	modelAdaptiveState.mu.Lock()
	modelAdaptiveState.hints = map[string]modelAdaptiveHint{}
	modelAdaptiveState.mu.Unlock()
}

// recordLLMIncident persists a failure/recovery record regardless of whether
// the UI debug trace toggle is enabled, and also emits a live debug trace entry.
func recordLLMIncident(entry mcp.LLMIncident) {
	if strings.TrimSpace(entry.ModelFamily) == "" {
		entry.ModelFamily, _ = chatharness.ClassifyModelFamily(entry.ModelID)
	}
	if strings.TrimSpace(entry.LLMMode) == "" {
		entry.LLMMode = "standard"
	}
	if strings.TrimSpace(entry.RecoveryStrategy) == "" {
		entry.RecoveryStrategy = "none"
	}
	if err := mcp.RecordLLMIncident(entry); err != nil {
		log.Printf("[llm-recovery] failed to persist incident: %v", err)
	}
	AddDebugTrace("recovery", entry.Scope+"."+entry.FailureKind, "Recorded LLM incident", map[string]interface{}{
		"model":     entry.ModelID,
		"family":    entry.ModelFamily,
		"mode":      entry.LLMMode,
		"strategy":  entry.RecoveryStrategy,
		"recovered": entry.Recovered,
		"status":    entry.HTTPStatus,
		"error":     entry.ErrorDetail,
	})
}

func isSchemaUnsupportedResponse(statusCode int, body string) bool {
	if statusCode != http.StatusBadRequest && statusCode != http.StatusUnprocessableEntity {
		return false
	}
	lower := strings.ToLower(body)
	for _, kw := range []string{"response_format", "json_schema", "schema", "grammar", "structured"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func isLikelyTruncatedJSON(raw string, unmarshalErr error) bool {
	if unmarshalErr != nil && strings.Contains(strings.ToLower(unmarshalErr.Error()), "unexpected end of json") {
		return true
	}
	trimmed := strings.TrimSpace(raw)
	return strings.HasPrefix(trimmed, "{") && !strings.HasSuffix(trimmed, "}")
}

func scopeForTaskPhase(phase string) string {
	if strings.HasPrefix(phase, "workflow_") {
		return "task.workflow." + strings.TrimPrefix(phase, "workflow_")
	}
	return "task.desktop." + phase
}

func executeStructuredWithRecovery(
	ctx context.Context,
	m desktopTaskModel,
	phase string,
	systemRole string,
	prompt string,
	schema map[string]any,
	images []string,
	baseMaxTokens int,
	timeout time.Duration,
	out any,
) error {
	scope := scopeForTaskPhase(phase)
	family, isSmall := chatharness.ClassifyModelFamily(m.model)
	hint := getModelAdaptiveHint(m.model)
	maxTokens := baseMaxTokens
	if hint.MinStructuredMaxTokens > maxTokens {
		maxTokens = hint.MinStructuredMaxTokens
	}
	if isSmall && phase == "workflow_review" && maxTokens < 3200 {
		maxTokens = 3200
	}
	schemaBytes, _ := json.Marshal(schema)

	doCall := func(userPrompt string, useStrictSchema bool, callMaxTokens int, recoveryHeader string) (int, string, string, string, time.Duration, error) {
		actualPrompt := userPrompt
		if !useStrictSchema {
			actualPrompt += "\n\nReturn ONLY one valid JSON object matching this schema (no markdown or explanation):\n" + string(schemaBytes)
		}
		content := any(actualPrompt)
		if len(images) > 0 {
			parts := []any{map[string]any{"type": "text", "text": actualPrompt}}
			for _, u := range images {
				parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": u}})
			}
			content = parts
		}
		payload := map[string]any{
			"model":       m.model,
			"stream":      false,
			"temperature": 0,
			"max_tokens":  callMaxTokens,
			"messages": []any{
				map[string]any{"role": "system", "content": systemRole},
				map[string]any{"role": "user", "content": content},
			},
		}
		if useStrictSchema {
			payload["response_format"] = map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name":   "desktop_task_" + phase,
					"strict": true,
					"schema": schema,
				},
			}
		}
		if family == "qwen" {
			payload["chat_template_kwargs"] = map[string]any{"enable_thinking": false}
		}
		body, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(m.endpoint, "/")+"/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			return 0, "", "", "", 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-DKST-Task-Phase", phase)
		if recoveryHeader != "" {
			req.Header.Set("X-DKST-Recovery-Strategy", recoveryHeader)
		}
		if m.token != "" {
			req.Header.Set("Authorization", "Bearer "+m.token)
		}
		started := time.Now()
		resp, err := (&http.Client{Timeout: timeout}).Do(req)
		elapsed := time.Since(started)
		if err != nil {
			return 0, "", "", "", elapsed, err
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		if err != nil {
			return resp.StatusCode, "", "", "", elapsed, err
		}
		if resp.StatusCode != http.StatusOK {
			return resp.StatusCode, string(raw), "", "", elapsed, fmt.Errorf("task %s HTTP %d: %s", phase, resp.StatusCode, compactText(string(raw), 600))
		}
		var parsed struct {
			Choices []struct {
				Message struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return resp.StatusCode, string(raw), "", "", elapsed, err
		}
		if len(parsed.Choices) != 1 {
			return resp.StatusCode, string(raw), "", "", elapsed, fmt.Errorf("task %s returned no structured answer", phase)
		}
		return resp.StatusCode, string(raw), parsed.Choices[0].Message.Content, parsed.Choices[0].Message.ReasoningContent, elapsed, nil
	}

	useStrict := !hint.DisableStrictSchema
	status, rawResp, contentText, reasoningText, elapsed, callErr := doCall(prompt, useStrict, maxTokens, "")
	if callErr != nil {
		// Automatic schema downgrade when an endpoint/model rejects response_format: json_schema.
		if useStrict && isSchemaUnsupportedResponse(status, rawResp) {
			noteModelSchemaUnsupported(m.model)
			status2, rawResp2, content2, reasoning2, elapsed2, err2 := doCall(prompt, false, maxTokens, "schema_downgrade")
			if err2 == nil {
				status, rawResp, contentText, reasoningText, elapsed, callErr = status2, rawResp2, content2, reasoning2, elapsed2, nil
				recordLLMIncident(mcp.LLMIncident{
					ModelID:          m.model,
					ModelFamily:      family,
					LLMMode:          "standard",
					Scope:            scope,
					FailureKind:      "schema_unsupported",
					RecoveryStrategy: "schema_downgrade",
					Recovered:        true,
					HTTPStatus:       status,
					ErrorDetail:      compactText(rawResp, 300),
					ElapsedMS:        elapsed2.Milliseconds(),
				})
			} else {
				recordLLMIncident(mcp.LLMIncident{
					ModelID:          m.model,
					ModelFamily:      family,
					LLMMode:          "standard",
					Scope:            scope,
					FailureKind:      "schema_unsupported",
					RecoveryStrategy: "schema_downgrade",
					Recovered:        false,
					HTTPStatus:       status2,
					ErrorDetail:      err2.Error(),
					RawSnippet:       compactText(rawResp2, 500),
					ElapsedMS:        elapsed2.Milliseconds(),
				})
				return err2
			}
		} else {
			recordLLMIncident(mcp.LLMIncident{
				ModelID:          m.model,
				ModelFamily:      family,
				LLMMode:          "standard",
				Scope:            scope,
				FailureKind:      "http_error",
				RecoveryStrategy: "none",
				Recovered:        false,
				HTTPStatus:       status,
				ErrorDetail:      callErr.Error(),
				RawSnippet:       compactText(rawResp, 500),
				ElapsedMS:        elapsed.Milliseconds(),
			})
			return callErr
		}
	}

	parseOutput := func(contentStr, reasoningStr string) (string, bool, error) {
		controlJSON := strings.TrimSpace(contentStr)
		if controlJSON == "" {
			candidate := strings.TrimSpace(reasoningStr)
			if strings.HasPrefix(candidate, "{") && json.Valid([]byte(candidate)) {
				controlJSON = candidate
			}
		}
		controlJSON = stripJSONFence(controlJSON)
		if err := json.Unmarshal([]byte(controlJSON), out); err == nil {
			return controlJSON, false, nil
		} else {
			// Stage 1 deterministic recovery: extract balanced JSON object from prose/fences.
			if extracted, ok := chatharness.ExtractBalancedJSONObject(contentStr); ok {
				if err2 := json.Unmarshal([]byte(extracted), out); err2 == nil {
					return extracted, true, nil
				}
			}
			if extracted, ok := chatharness.ExtractBalancedJSONObject(reasoningStr); ok {
				if err2 := json.Unmarshal([]byte(extracted), out); err2 == nil {
					return extracted, true, nil
				}
			}
			return controlJSON, false, err
		}
	}

	controlJSON, deterministicallyRecovered, parseErr := parseOutput(contentText, reasoningText)
	if parseErr == nil {
		if deterministicallyRecovered {
			recordLLMIncident(mcp.LLMIncident{
				ModelID:          m.model,
				ModelFamily:      family,
				LLMMode:          "standard",
				Scope:            scope,
				FailureKind:      "json_parse",
				RecoveryStrategy: "extract_json",
				Recovered:        true,
				HTTPStatus:       status,
				RawSnippet:       compactText(contentText, 500),
				ElapsedMS:        elapsed.Milliseconds(),
			})
		}
		return nil
	}

	// Stage 2 LLM self-repair (bounded to 1 attempt).
	failureKind := "json_parse"
	repairMaxTokens := maxTokens
	if isLikelyTruncatedJSON(controlJSON, parseErr) {
		failureKind = "json_truncated"
		noteModelJSONTruncated(m.model)
		if repairMaxTokens < 3200 {
			repairMaxTokens = 3200
		}
	} else if strings.TrimSpace(contentText) == "" && strings.TrimSpace(reasoningText) == "" {
		failureKind = "empty_response"
	}

	repairPrompt := promptkit.StructuredJSONRepairPrompt(phase, parseErr.Error(), contentText, schemaBytes)
	repairStatus, _, repairContent, repairReasoning, repairElapsed, repairCallErr := doCall(repairPrompt, !getModelAdaptiveHint(m.model).DisableStrictSchema, repairMaxTokens, "json_repair_llm")
	if repairCallErr == nil {
		if _, _, repairParseErr := parseOutput(repairContent, repairReasoning); repairParseErr == nil {
			recordLLMIncident(mcp.LLMIncident{
				ModelID:          m.model,
				ModelFamily:      family,
				LLMMode:          "standard",
				Scope:            scope,
				FailureKind:      failureKind,
				RecoveryStrategy: "json_repair_llm",
				Recovered:        true,
				HTTPStatus:       repairStatus,
				ErrorDetail:      parseErr.Error(),
				RawSnippet:       compactText(contentText, 500),
				ElapsedMS:        repairElapsed.Milliseconds(),
			})
			return nil
		}
	}

	recordLLMIncident(mcp.LLMIncident{
		ModelID:          m.model,
		ModelFamily:      family,
		LLMMode:          "standard",
		Scope:            scope,
		FailureKind:      failureKind,
		RecoveryStrategy: "json_repair_llm",
		Recovered:        false,
		HTTPStatus:       repairStatus,
		ErrorDetail:      parseErr.Error(),
		RawSnippet:       compactText(controlJSON, 500),
		ElapsedMS:        elapsed.Milliseconds() + repairElapsed.Milliseconds(),
	})
	return fmt.Errorf("task %s returned invalid JSON (%v): %s", phase, parseErr, compactText(controlJSON, 400))
}
