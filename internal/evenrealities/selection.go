package evenrealities

import (
	"encoding/json"
	"strings"
)

// Instruction returns a static integration policy, never user-provided text.
func Instruction(key string) string { return glassesTemplates[key]["systemPrompt"] }
func SelectionInstruction(contextual bool) string {
	if contextual {
		return Instruction("selection_context")
	}
	return Instruction("selection_legacy")
}

// PrepareSourceSelection replaces the draft writer prompt with the bounded
// immutable-source selection contract, using the model's resolved protocol.
func PrepareSourceSelection(body []byte, sourceData any, contextual bool) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	var format map[string]any
	if contextual {
		var err error
		sourceData, format, err = contextualSelectionData(sourceData)
		if err != nil {
			return nil, err
		}
	}
	input, err := json.Marshal(sourceData)
	if err != nil {
		return nil, err
	}
	system := SelectionInstruction(contextual)
	if _, native := payload["input"]; native {
		payload["system_prompt"] = system
		payload["input"] = string(input)
		payload["max_output_tokens"] = 192
		delete(payload, "messages")
		delete(payload, "max_tokens")
	} else {
		payload["messages"] = []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": string(input)}}
		payload["max_tokens"] = 192
		delete(payload, "system_prompt")
	}
	delete(payload, "response_format")
	if contextual {
		payload["temperature"] = 0
		if _, native := payload["input"]; !native {
			payload["response_format"] = format
		}
	}
	return json.Marshal(payload)
}

// Replace only the catalog's marker once; source content is never templated.
func ConversationInstruction(source string) string {
	return strings.Replace(Instruction("grounding_conversation"), "{{source}}", source, 1)
}

// NativeOutputContract keeps device task schemas out of the generic provider
// adapter. Other providers/tasks continue to use their original JSON schema.
func NativeOutputContract(name string) string {
	switch name {
	case "context_hint_task", "hint_judgment", "hint_fact":
		return Instruction("native_" + name)
	}
	return ""
}
