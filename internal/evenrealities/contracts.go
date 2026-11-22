package evenrealities

import (
	"encoding/json"
	"errors"
	"regexp"
)

var ErrUnresolvedTask = errors.New("hint_task_not_actionable")

// Structural requests for missing context cannot be fulfilled by an autonomous
// information card. Do not blacklist names or require explicit user questions.
var unresolvedTask = regexp.MustCompile(`(?i)((?:어떤|누구|무엇|어디|정체|특정|식별|명확).{0,90}(?:추가|더\s*많은)\s*(?:정보|문맥).{0,25}(?:필요|요청)|추가\s*문맥.{0,25}(?:필요|요청)|(?:more|additional)\s+(?:information|details).{0,35}(?:needed|required).{0,50}(?:identify|specify|clarify)|(?:identify|specify|clarify).{0,80}(?:more|additional)\s+(?:information|details).{0,25}(?:needed|required)|(?:more|additional)\s+context.{0,25}(?:needed|required)|(?:ask|request).{0,16}(?:clarification|more context))`)

func actionableHintTask(task string) bool { return !unresolvedTask.MatchString(task) }

func hintFactFormat() map[string]any {
	return map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "hint_fact", "strict": true, "schema": map[string]any{"type": "object", "properties": map[string]any{"fact": map[string]any{"type": "string", "maxLength": 500}, "adds_information": map[string]any{"type": "boolean"}, "kind": map[string]any{"type": "string", "enum": []string{"key", "term", "verify", "schedule"}}}, "required": []string{"fact", "adds_information", "kind"}, "additionalProperties": false}}}
}

// Context source selection numbers sentences inside ONE chosen record. Even
// without grammar support this avoids asking a small model to juggle flat IDs
// across several sources. Independent client verification remains mandatory.
func contextualSelectionData(sourceData any) (any, map[string]any, error) {
	raw, err := json.Marshal(sourceData)
	if err != nil {
		return nil, nil, err
	}
	var data map[string]any
	if err = json.Unmarshal(raw, &data); err != nil {
		return nil, nil, err
	}
	records := []map[string]any{}
	recordIDs := []int{0}
	maxSentences := 0
	if choices, ok := data["choices"].([]any); ok {
		indices := map[int]int{}
		for _, item := range choices {
			choice, ok := item.(map[string]any)
			if !ok {
				continue
			}
			recordNumber, _ := choice["record"].(float64)
			sentence, _ := choice["sentence"].(float64)
			id := int(recordNumber)
			index, exists := indices[id]
			if !exists {
				index = len(records)
				indices[id] = index
				records = append(records, map[string]any{"record": id, "title": choice["title"], "url": choice["url"], "sentences": []any{}})
				recordIDs = append(recordIDs, id)
			}
			entries := records[index]["sentences"].([]any)
			entries = append(entries, map[string]any{"sentence": int(sentence), "text": choice["text"]})
			records[index]["sentences"] = entries
			if int(sentence) > maxSentences {
				maxSentences = int(sentence)
			}
		}
	}
	delete(data, "choices")
	data["records"] = records
	sentenceIDs := []int{}
	for i := 1; i <= maxSentences; i++ {
		sentenceIDs = append(sentenceIDs, i)
	}
	schema := map[string]any{"type": "object", "properties": map[string]any{"record": map[string]any{"type": "integer", "enum": recordIDs}, "sentences": map[string]any{"type": "array", "items": map[string]any{"type": "integer", "enum": sentenceIDs}, "maxItems": 3}}, "required": []string{"record", "sentences"}, "additionalProperties": false}
	return data, map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "hint_source_sentences", "strict": true, "schema": schema}}, nil
}
