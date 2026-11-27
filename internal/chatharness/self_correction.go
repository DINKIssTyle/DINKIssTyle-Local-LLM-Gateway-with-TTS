/*
 * Created by DINKIssTyle on 2024.
 * Copyright (C) 2024 DINKI'ssTyle. All rights reserved.
 */

package chatharness

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// chatCompletionChunk는 SSE 응답에서 데이터 스트림의 개별 조각을 언마샬링하기 위한 구조체입니다.
type chatCompletionChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

type SelfCorrectionInput struct {
	Body           []byte
	Endpoint       string
	APIToken       string
	LLMMode        string
	ModelID        string
	LastResponseID string
	Prompt         string
}

// ExecuteSelfCorrection은 잘못된 도구 호출에 대해 자동 수정을 시도합니다.
func ExecuteSelfCorrection(input SelfCorrectionInput, emitLine func(string) error, onContent func(string)) error {
	correctionReq, err := buildSelfCorrectionRequest(input)
	if err != nil {
		return fmt.Errorf("failed to build self-correction request: %w", err)
	}
	if correctionReq == nil {
		return nil
	}

	jsonPayload, err := json.Marshal(correctionReq)
	if err != nil {
		return fmt.Errorf("failed to marshal correction request: %w", err)
	}

	reqURL, err := buildURL(input.Endpoint, input.LLMMode)
	if err != nil {
		return fmt.Errorf("failed to construct request URL: %w", err)
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(input.APIToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(input.APIToken))
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("self-correction api returned error status: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataStr := strings.TrimPrefix(line, "data: ")
			if dataStr != "[DONE]" {
				var chunk chatCompletionChunk
				if err := json.Unmarshal([]byte(dataStr), &chunk); err == nil {
					if len(chunk.Choices) > 0 {
						content := chunk.Choices[0].Delta.Content
						if content != "" && onContent != nil {
							onContent(content)
						}
					}
				}
			}
			if emitLine != nil {
				if err := emitLine(line); err != nil {
					return fmt.Errorf("failed to emit line: %w", err)
				}
			}
		}
	}
	return scanner.Err()
}

// buildURL은 Endpoint와 LLMMode에 따라 요청 URL을 안전하게 구성합니다.
func buildURL(endpoint, llmMode string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	// 이미 'chat'이 포함되어 있다면 그대로 반환하거나 적절히 처리
	if strings.Contains(u.Path, "chat") {
		return u.String(), nil
	}

	var subPath string
	if llmMode == "stateful" {
		subPath = "/api/v1/chat"
	} else {
		subPath = "/v1/chat/completions"
	}

	// url.JoinPath는 Go 1.19+에서 사용 가능합니다.
	// 슬래시 중복 처리가 자동으로 이루어집니다.
	joinedURL, err := url.JoinPath(endpoint, subPath)
	if err != nil {
		return "", err
	}
	return joinedURL, nil
}

func buildSelfCorrectionRequest(input SelfCorrectionInput) (map[string]interface{}, error) {
	if input.LLMMode == "stateful" {
		correctionReq := map[string]interface{}{
			"model":       input.ModelID,
			"input":       input.Prompt,
			"stream":      true,
			"temperature": 0.1,
		}
		if IsValidResponseID(input.LastResponseID) {
			correctionReq["previous_response_id"] = strings.TrimSpace(input.LastResponseID)
		} else if len(input.Body) > 0 {
			var tempMap map[string]interface{}
			if err := json.Unmarshal(input.Body, &tempMap); err != nil {
				// Body 파싱 실패 시 에러를 반환하여 문제 파악을 돕습니다.
				return nil, fmt.Errorf("failed to parse original body for last_response_id: %w", err)
			}
			if pid, ok := tempMap["previous_response_id"].(string); ok && IsValidResponseID(pid) {
				correctionReq["previous_response_id"] = strings.TrimSpace(pid)
			}
		}
		return correctionReq, nil
	}

	correctionReq := map[string]interface{}{
		"model": input.ModelID,
		"messages": []map[string]string{
			{"role": "system", "content": "Return only the corrected tool call or plain answer."},
			{"role": "user", "content": input.Prompt},
		},
		"stream":      true,
		"temperature": 0.1,
	}
	return correctionReq, nil
}

// ClassifyModelFamily normalizes a raw model identifier into a coarse family
// name and whether it is a small local model (<= 8B) for per-model failure
// tracking and recovery strategy selection.
func ClassifyModelFamily(modelID string) (family string, isSmall bool) {
	lower := strings.ToLower(strings.TrimSpace(modelID))
	if lower == "" {
		return "other", false
	}
	switch {
	case strings.Contains(lower, "gemma-4") || strings.Contains(lower, "gemma4"):
		family = "gemma4"
	case strings.Contains(lower, "gemma-3") || strings.Contains(lower, "gemma3"):
		family = "gemma3"
	case strings.Contains(lower, "gemma"):
		family = "gemma"
	case strings.Contains(lower, "qwen"):
		family = "qwen"
	case strings.Contains(lower, "llama"):
		family = "llama"
	case strings.Contains(lower, "mistral") || strings.Contains(lower, "mixtral"):
		family = "mistral"
	case strings.Contains(lower, "deepseek"):
		family = "deepseek"
	case strings.Contains(lower, "claude"):
		family = "claude"
	case strings.Contains(lower, "gpt-") || strings.Contains(lower, "o1") || strings.Contains(lower, "o3") || strings.Contains(lower, "o4"):
		family = "gpt"
	case strings.Contains(lower, "gemini"):
		family = "gemini"
	default:
		family = "other"
	}
	isSmall = smallModelSizePattern.MatchString(lower)
	return family, isSmall
}

var smallModelSizePattern = regexp.MustCompile(`(?:^|[^0-9a-z])(?:e?[1-4]b|0\.5b|1\.5b|[78]b)(?:$|[^0-9a-z])`)

// ExtractBalancedJSONObject deterministically recovers the outermost valid JSON
// object from prose-wrapped or markdown-fenced model output without an extra LLM call.
func ExtractBalancedJSONObject(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	if strings.HasPrefix(s, "```") {
		if nl := strings.IndexByte(s, '\n'); nl >= 0 {
			s = s[nl+1:]
		}
		if end := strings.LastIndex(s, "```"); end >= 0 {
			s = s[:end]
		}
		s = strings.TrimSpace(s)
	}
	if strings.HasPrefix(s, "{") && json.Valid([]byte(s)) {
		return s, true
	}
	for start := 0; start < len(s); start++ {
		if s[start] != '{' {
			continue
		}
		depth := 0
		inString := false
		escaped := false
		for i := start; i < len(s); i++ {
			ch := s[i]
			if inString {
				if escaped {
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
				} else if ch == '"' {
					inString = false
				}
				continue
			}
			switch ch {
			case '"':
				inString = true
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					candidate := s[start : i+1]
					if json.Valid([]byte(candidate)) {
						return candidate, true
					}
					break
				}
			}
			if depth == 0 {
				break
			}
		}
	}
	return "", false
}
