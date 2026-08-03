package chatharness

import (
	"encoding/json"
	"sort"
	"strings"
)

// ProviderToolCall is the provider-neutral representation consumed by the
// app-owned orchestration loop.
type ProviderToolCall struct {
	Index     int
	ID        string
	Name      string
	Arguments string
}

// ParseProviderToolArgumentsEvent normalizes LM Studio's stateful
// tool_call.arguments event into the same call shape used by chat completions.
func ParseProviderToolArgumentsEvent(event map[string]interface{}, fallbackName string) (ProviderToolCall, interface{}, bool) {
	if event == nil {
		return ProviderToolCall{}, nil, false
	}
	name, _ := event["tool"].(string)
	if strings.TrimSpace(name) == "" {
		name, _ = event["tool_name"].(string)
	}
	if strings.TrimSpace(name) == "" {
		name = fallbackName
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ProviderToolCall{}, nil, false
	}

	callID, _ := event["call_id"].(string)
	if strings.TrimSpace(callID) == "" {
		callID, _ = event["id"].(string)
	}
	call := ProviderToolCall{ID: strings.TrimSpace(callID), Name: name, Arguments: "{}"}
	var parsed interface{} = map[string]interface{}{}
	if rawArguments, exists := event["arguments"]; exists {
		switch arguments := rawArguments.(type) {
		case string:
			if strings.TrimSpace(arguments) != "" {
				call.Arguments = strings.TrimSpace(arguments)
				if err := json.Unmarshal([]byte(call.Arguments), &parsed); err != nil {
					parsed = arguments
				}
			}
		default:
			parsed = rawArguments
			if encoded, err := json.Marshal(rawArguments); err == nil {
				call.Arguments = string(encoded)
			}
		}
	}
	return call, parsed, true
}

// ChatToolAccumulator rebuilds OpenAI-compatible streaming tool-call deltas.
type ChatToolAccumulator struct {
	calls map[int]*ProviderToolCall
}

func NewChatToolAccumulator() *ChatToolAccumulator {
	return &ChatToolAccumulator{calls: make(map[int]*ProviderToolCall)}
}

func (a *ChatToolAccumulator) AddChunk(chunk map[string]interface{}) bool {
	if a == nil || chunk == nil {
		return false
	}
	found := false
	choices, _ := chunk["choices"].([]interface{})
	for _, rawChoice := range choices {
		choice, _ := rawChoice.(map[string]interface{})
		for _, field := range []string{"delta", "message"} {
			payload, _ := choice[field].(map[string]interface{})
			if a.addPayload(payload) {
				found = true
			}
		}
	}
	return found
}

func (a *ChatToolAccumulator) addPayload(payload map[string]interface{}) bool {
	if a == nil || payload == nil {
		return false
	}
	found := false
	toolCalls, _ := payload["tool_calls"].([]interface{})
	for _, rawCall := range toolCalls {
		callMap, _ := rawCall.(map[string]interface{})
		if callMap == nil {
			continue
		}
		index := int(numberValue(callMap["index"]))
		if a.addFunction(index, callMap["id"], callMap["function"]) {
			found = true
		}
	}

	// Some OpenAI-compatible servers still emit the pre-tool_calls shape.
	if function, ok := payload["function_call"].(map[string]interface{}); ok {
		if a.addFunction(0, nil, function) {
			found = true
		}
	}
	return found
}

func (a *ChatToolAccumulator) addFunction(index int, rawID interface{}, rawFunction interface{}) bool {
	function, _ := rawFunction.(map[string]interface{})
	if function == nil {
		return false
	}
	call := a.calls[index]
	if call == nil {
		call = &ProviderToolCall{Index: index}
		a.calls[index] = call
	}
	if id, _ := rawID.(string); strings.TrimSpace(id) != "" {
		call.ID = strings.TrimSpace(id)
	}
	if name, _ := function["name"].(string); strings.TrimSpace(name) != "" {
		call.Name = strings.TrimSpace(name)
	}
	switch arguments := function["arguments"].(type) {
	case string:
		call.Arguments += arguments
	case nil:
	default:
		if encoded, err := json.Marshal(arguments); err == nil {
			call.Arguments += string(encoded)
		}
	}
	return strings.TrimSpace(call.Name) != "" || strings.TrimSpace(call.Arguments) != "" || strings.TrimSpace(call.ID) != ""
}

func (a *ChatToolAccumulator) HasCalls() bool {
	return a != nil && len(a.calls) > 0
}

func (a *ChatToolAccumulator) Calls() []ProviderToolCall {
	if a == nil {
		return nil
	}
	indexes := make([]int, 0, len(a.calls))
	for index := range a.calls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	result := make([]ProviderToolCall, 0, len(indexes))
	for _, index := range indexes {
		call := *a.calls[index]
		if strings.TrimSpace(call.Arguments) == "" {
			call.Arguments = "{}"
		}
		result = append(result, call)
	}
	return result
}

func numberValue(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}
