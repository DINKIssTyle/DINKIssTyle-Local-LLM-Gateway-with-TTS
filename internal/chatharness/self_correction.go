package chatharness

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type SelfCorrectionInput struct {
	Body           []byte
	Endpoint       string
	APIToken       string
	LLMMode        string
	ModelID        string
	EnableMCP      bool
	LastResponseID string
	Prompt         string
}

func ExecuteSelfCorrection(input SelfCorrectionInput, emitLine func(string) error, onContent func(string)) error {
	correctionReq := buildSelfCorrectionRequest(input)
	if correctionReq == nil {
		return nil
	}

	jsonPayload, _ := json.Marshal(correctionReq)
	reqURL := input.Endpoint
	if input.LLMMode == "stateful" && !strings.Contains(reqURL, "chat") {
		reqURL = strings.TrimSuffix(reqURL, "/") + "/api/v1/chat"
	} else if !strings.Contains(reqURL, "chat") {
		reqURL = strings.TrimSuffix(reqURL, "/") + "/v1/chat/completions"
	}

	req, _ := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(input.APIToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(input.APIToken))
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataStr := strings.TrimPrefix(line, "data: ")
			if dataStr != "[DONE]" {
				var chunk map[string]interface{}
				if err := json.Unmarshal([]byte(dataStr), &chunk); err == nil {
					if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]interface{}); ok {
							if delta, ok := choice["delta"].(map[string]interface{}); ok {
								if c, ok := delta["content"].(string); ok && onContent != nil {
									onContent(c)
								}
							}
						}
					}
				}
			}
			if emitLine != nil {
				if err := emitLine(line); err != nil {
					return err
				}
			}
		}
	}
	return scanner.Err()
}

func buildSelfCorrectionRequest(input SelfCorrectionInput) map[string]interface{} {
	if input.LLMMode == "stateful" {
		correctionReq := map[string]interface{}{
			"model":       input.ModelID,
			"input":       input.Prompt,
			"stream":      true,
			"temperature": 0.1,
		}
		if input.EnableMCP {
			correctionReq["integrations"] = []string{"mcp/dinkisstyle-gateway"}
		}
		if strings.TrimSpace(input.LastResponseID) != "" {
			correctionReq["previous_response_id"] = strings.TrimSpace(input.LastResponseID)
		} else {
			var tempMap map[string]interface{}
			if err := json.Unmarshal(input.Body, &tempMap); err == nil {
				if pid, ok := tempMap["previous_response_id"].(string); ok && pid != "" {
					correctionReq["previous_response_id"] = pid
				}
			}
		}
		return correctionReq
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
	if input.EnableMCP {
		correctionReq["integrations"] = []string{"mcp/dinkisstyle-gateway"}
	}
	return correctionReq
}
