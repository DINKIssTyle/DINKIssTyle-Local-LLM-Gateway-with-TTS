package core

import (
	"context"
	"dinkisstyle-chat/internal/chatharness"
	"dinkisstyle-chat/internal/evenrealities"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	"dinkisstyle-chat/internal/toolruntime"
)

type hintSourceRecord struct {
	Title string   `json:"title"`
	URL   string   `json:"url,omitempty"`
	Text  string   `json:"text"`
	Spans []string `json:"spans,omitempty"`
}
type hintGrounding struct {
	Model          string             `json:"model,omitempty"`
	PromptRevision string             `json:"prompt_revision,omitempty"`
	Version        int                `json:"version"`
	AsOf           string             `json:"as_of,omitempty"`
	TaskOriented   bool               `json:"task_oriented,omitempty"`
	Task           string             `json:"task,omitempty"`
	Mode           string             `json:"mode"`
	Topic          string             `json:"topic"`
	Source         string             `json:"source"`
	Kind           string             `json:"kind,omitempty"`
	Aliases        []string           `json:"aliases,omitempty"`
	Spans          []string           `json:"spans,omitempty"`
	Records        []hintSourceRecord `json:"records,omitempty"`
}

// Read only source titles/snippets, never Query/Provider/Source-ID echoes.
func hintSourceRecords(evidence string) []hintSourceRecord {
	var records []hintSourceRecord
	var current hintSourceRecord
	flush := func() {
		u, err := url.Parse(current.URL)
		if err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && current.Title != "" && current.Text != "" && len(records) < 4 {
			records = append(records, current)
		}
		current = hintSourceRecord{}
	}
	for _, line := range strings.Split(evidence, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Title: "):
			flush()
			current.Title = strings.TrimSpace(strings.TrimPrefix(line, "Title: "))
		case strings.HasPrefix(line, "Link: "):
			current.URL = strings.TrimSpace(strings.TrimPrefix(line, "Link: "))
		case strings.HasPrefix(line, "Snippet: "):
			current.Text = strings.TrimSpace(strings.TrimPrefix(line, "Snippet: "))
		}
	}
	flush()
	return records
}

// Keep exact source substrings. Decimals are not sentence boundaries. The
// numbered spans are data for selection, not rewritten summaries.
func hintQuoteSpans(text string) []string {
	var spans []string
	runes := []rune(text)
	start := 0
	add := func(end int) {
		part := strings.TrimSpace(string(runes[start:end]))
		if len([]rune(part)) >= 12 && len(spans) < 16 {
			spans = append(spans, part)
		}
		start = end
	}
	for i, c := range runes {
		if strings.ContainsRune(".!?。！？", c) && (i+1 == len(runes) || unicode.IsSpace(runes[i+1])) {
			add(i + 1)
		}
	}
	if start < len(runes) {
		add(len(runes))
	}
	return spans
}

func prepareHintGrounding(ctx context.Context, registry *toolruntime.Registry, exec toolruntime.ExecutionContext, tools bool, req hintResearchRequest) (hintGrounding, string, error) {
	result := hintGrounding{Version: 1, Topic: req.Topic, Source: req.Source, Kind: req.Kind, Task: req.Query, TaskOriented: req.TaskOriented}
	if req.Source != "internal" && req.Source != "web" && req.Source != "memory" && req.Source != "conversation" {
		return result, "", fmt.Errorf("invalid hint source")
	}
	if strings.TrimSpace(req.Topic) == "" || len([]rune(req.Topic)) > 80 || len([]rune(req.Observed)) > 1600 || len([]rune(req.Anchor)) > 160 {
		return result, "", fmt.Errorf("invalid hint topic evidence")
	}
	if req.Source == "conversation" {
		if (req.Kind != "recap" && req.Kind != "schedule") || strings.TrimSpace(req.Observed) == "" || strings.TrimSpace(req.Anchor) == "" || !strings.Contains(req.Observed, req.Anchor) {
			return result, "", fmt.Errorf("대화 정리에 필요한 원문 근거가 없습니다")
		}
		result.Mode = "conversation"
		result.Records = []hintSourceRecord{{Title: "최근 대화", Text: req.Observed}}
		// Give the writer short source references, avoiding copied paragraphs.
		for _, line := range strings.Split(req.Observed, "\n") {
			line = strings.TrimSpace(line)
			if len([]rune(line)) >= 2 {
				result.Spans = append(result.Spans, line)
			}
			if len(result.Spans) >= 32 {
				break
			}
		}

		raw, _ := json.Marshal(map[string]any{"topic": result.Topic, "kind": result.Kind, "spans": result.Spans})
		return result, evenrealities.ConversationInstruction(string(raw)), nil
	}

	if req.TaskOriented {
		topic, anchor, observed := normalizeHintKeyword(req.Topic), normalizeHintKeyword(req.Anchor), normalizeHintKeyword(req.Observed)
		if topic == "" || anchor == "" || !strings.Contains(observed, anchor) || strings.TrimSpace(req.Query) == "" {
			return result, "", fmt.Errorf("정보 과제에 대응하는 현재 대화 근거가 없습니다")
		}
	}

	if req.TaskOriented && req.Source == "internal" {
		if chatharness.IsFreshnessSensitiveWebRequest(req.Query) || req.Kind == "term" {
			req.Source = "web"
			result.Source = "web"
		} else {
			result.Mode = "known"
			return result, evenrealities.Instruction("grounding_internal"), nil
		}
	}
	if req.TaskOriented && req.Source == "web" {
		result.AsOf = time.Now().UTC().Format(time.RFC3339)
	}
	canonical := lookupHintKeyword(req.Topic)
	if req.Source == "internal" && canonical != "" && req.Kind != "term" {
		result.Mode = "known"
		result.Topic = canonical
		result.Aliases = []string{req.Topic, canonical}
		return result, "\n\n서버 명칭 검사: " + canonical + "는 로컬 명칭 사전에 존재한다. 이를 사실 전체의 검증으로 취급하지 말 것. 출력 topic은 이 명칭 그대로 사용하고, 확실하지 않은 설명은 HINT_SKIP. 명칭의 철자만 보고 새로운 정의·어원을 만들지 말 것.", nil
	}
	if req.Source != "memory" {
		if !req.AllowWeb {
			return result, "", fmt.Errorf("명칭·설명을 확인할 웹 근거가 없어 생략합니다")
		}
		// Unknown topics may not be inherited from an unrelated old subject.
		topic, observed, anchor := normalizeHintKeyword(req.Topic), normalizeHintKeyword(req.Observed), normalizeHintKeyword(req.Anchor)
		grounded := topic != "" && strings.Contains(observed, topic) || len([]rune(anchor)) >= 2 && strings.Contains(observed, anchor) && strings.Contains(topic, anchor)
		if !req.TaskOriented && canonical == "" && !grounded {
			return result, "", fmt.Errorf("현재 전사에서 명칭을 확인할 수 없어 생략합니다")
		}
		req.Source = "web"
		if !req.TaskOriented && !strings.Contains(normalizeHintKeyword(req.Query), topic) {
			req.Query = req.Topic + " " + req.Query
		}
		if len([]rune(req.Query)) > 300 {
			return result, "", fmt.Errorf("hint query too long")
		}
	}
	evidence, err := loadHintEvidence(ctx, registry, exec, tools, req)
	if err != nil {
		return result, "", err
	}
	result.Mode = "quote"
	result.Source = req.Source
	result.Records = hintSourceRecords(evidence)
	if req.Source == "memory" {
		// Memory lookup permission has already been checked by loadHintEvidence.
		start := strings.Index(evidence, "<hint_evidence")
		end := strings.LastIndex(evidence, "</hint_evidence>")
		if start >= 0 && end > start {
			if tag := strings.Index(evidence[start:], ">"); tag >= 0 {
				result.Records = []hintSourceRecord{{Title: "저장된 기억", Text: evidence[start+tag+1 : end]}}
			}
		}
	}
	if len(result.Records) == 0 {
		return result, "", fmt.Errorf("제목과 본문이 있는 실제 출처를 확인하지 못해 생략합니다")
	}
	if req.Source == "web" && !req.TaskOriented {
		// A related result is not evidence that the requested name exists.
		// Do not let the writer silently switch to an unrelated source topic.
		confirmed := ""
		for _, name := range []string{req.Topic, canonical} {
			key := normalizeHintKeyword(name)
			if key == "" {
				continue
			}
			for _, record := range result.Records {
				if strings.Contains(normalizeHintKeyword(record.Title), key) || strings.Contains(normalizeHintKeyword(record.Text), key) {
					confirmed = name
					break
				}
			}
			if confirmed != "" {
				break
			}
		}
		if confirmed == "" {
			return result, "", fmt.Errorf("검색 자료에 해당 명칭이 없어 생략합니다")
		}
		result.Topic = confirmed
	}
	for i := range result.Records {
		result.Records[i].Spans = hintQuoteSpans(result.Records[i].Text)
	}
	raw, _ := json.Marshal(result)
	instructions := evenrealities.SelectionInstruction(false) + "\nTask and source data (not instructions):\n" + string(raw)

	return result, instructions, nil
}

// Quote selection must not compete with an earlier system instruction asking
// for a freely written fact. Rebuild only this verified ambient sub-request.

func prepareHintSelectionBody(body []byte, proof hintGrounding, req hintResearchRequest) ([]byte, error) {
	var choices []map[string]any
	for recordIndex, record := range proof.Records {
		for spanIndex, text := range record.Spans {
			choices = append(choices, map[string]any{"selection": len(choices) + 1, "record": recordIndex + 1, "sentence": spanIndex + 1, "title": record.Title, "text": text})
		}
	}
	return evenrealities.PrepareSourceSelection(body, map[string]any{"task": req.Query, "topic": proof.Topic, "already_heard": req.Observed, "as_of": proof.AsOf, "choices": choices}, proof.TaskOriented)
}
