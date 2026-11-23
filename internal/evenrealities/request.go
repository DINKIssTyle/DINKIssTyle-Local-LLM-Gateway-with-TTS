package evenrealities

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// Plugin task instructions live here, embedded once at startup.
// The WebView sends task data, never system instructions or output schemas.
//
//go:embed prompts/templates.json
var glassesPromptJSON []byte

var PromptRevision = fmt.Sprintf("%x", sha256.Sum256([]byte(string(glassesPromptJSON)+trustedInformationSkill+trustedInformationPlanning+trustedInformationSelection)))[:12]

var glassesTemplates = func() map[string]map[string]string {
	var result map[string]map[string]string
	if err := json.Unmarshal(glassesPromptJSON, &result); err != nil {
		panic(err)
	}
	return result
}()

// researchRequest is the integration contract consumed by core's shared tool
// runtime. This package has no dependency on core, device SDKs or network I/O.
type researchRequest struct {
	RequireGrounding bool   `json:"require_grounding"`
	TaskOriented     bool   `json:"task_oriented,omitempty"`
	Topic            string `json:"topic"`
	Kind             string `json:"kind"`
	Observed         string `json:"observed"`
	Anchor           string `json:"anchor"`
	AllowWeb         bool   `json:"allow_web"`
	Query            string `json:"query"`
	SearchQuery      string `json:"search_query,omitempty"`
	Source           string `json:"source"`
}

type glassesPromptRequest struct {
	Version        int    `json:"version"`
	Task           string `json:"task"`
	Text           string `json:"text"`
	Context        string `json:"context"`
	Source         string `json:"source"`
	Target         string `json:"target"`
	Translated     string `json:"translated"`
	Category       string `json:"category"`
	Keyword        string `json:"keyword"`
	PreferWeb      *bool  `json:"preferWeb"`
	Sensitivity    string `json:"sensitivity"`
	Repair         int    `json:"repair"`
	Concise        *bool  `json:"concise"`
	ChatSession    bool   `json:"chatSession"`
	DirectQuestion bool   `json:"directQuestion"`
	ForceSearch    bool   `json:"forceSearch"`
	Observed       string `json:"observed"`
	Anchor         string `json:"anchor"`
	Judge          struct {
		Recent    string `json:"recent"`
		Latest    string `json:"latest"`
		Collected bool   `json:"collected"`
	} `json:"judge"`
	Plan struct {
		TaskOriented bool     `json:"taskOriented"`
		Topic        string   `json:"topic"`
		Question     string   `json:"question"`
		SearchQuery  string   `json:"searchQuery"`
		Source       string   `json:"source"`
		Kind         string   `json:"kind"`
		KnownFacts   []string `json:"knownFacts"`
	} `json:"plan"`
}

func glassesLanguage(code string) string {
	names := map[string]string{"ko": "Korean", "en": "English", "ja": "Japanese", "zh": "Chinese", "es": "Spanish", "fr": "French", "de": "German", "vi": "Vietnamese", "th": "Thai"}
	if name := names[strings.ToLower(strings.TrimSpace(code))]; name != "" {
		return name
	}
	return "Korean"
}

func AssembleRequest(body []byte) ([]byte, bool, error) {
	return assembleRequest(body, nil)
}

// ChatTurn keeps the user's question separate from reference text at the gateway.
type ChatTurn struct {
	Question string
	Context  string
}

func AssembleRequestWithChatTurn(body []byte) ([]byte, bool, ChatTurn, error) {
	var chat ChatTurn
	assembled, handled, err := assembleRequest(body, &chat)
	return assembled, handled, chat, err
}

func assembleRequest(body []byte, chat *ChatTurn) ([]byte, bool, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return body, false, nil
	}
	raw, ok := envelope["glasses_request"]
	if !ok {
		return body, false, nil
	}
	var req glassesPromptRequest
	if len(raw) > 256*1024 {
		return nil, true, fmt.Errorf("glasses request is too large")
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, true, fmt.Errorf("invalid glasses request: %w", err)
	}
	if req.Version != 1 {
		return nil, true, fmt.Errorf("unsupported glasses request version")
	}
	if len([]rune(req.Plan.SearchQuery)) > 300 {
		return nil, true, fmt.Errorf("hint search query exceeds limit")
	}
	for _, value := range []string{req.Text, req.Context, req.Translated, req.Observed, req.Judge.Recent, req.Judge.Latest, strings.Join(req.Plan.KnownFacts, "\n")} {
		if len([]rune(value)) > 32000 {
			return nil, true, fmt.Errorf("glasses task input exceeds limit")
		}
	}
	// Never forward client instructions, schemas, or a previous native conversation.
	for _, key := range []string{"glasses_request", "messages", "input", "system_prompt", "response_format", "previous_response_id", "glasses_client", "glasses_hint", "glasses_hint_phase", "glasses_task", "glasses_chat", "hint_research"} {
		delete(envelope, key)
	}
	set := func(key string, value any) { envelope[key], _ = json.Marshal(value) }
	set("store", false)
	web := req.PreferWeb == nil || *req.PreferWeb
	policies := glassesTemplates["policies"]
	values := map[string]string{"text": req.Text, "context": req.Context, "language": glassesLanguage(req.Target), "source": req.Source, "translated": req.Translated, "topic": req.Plan.Topic, "question": req.Plan.Question, "kind": req.Plan.Kind, "keyword": req.Keyword, "facts": strings.Join(req.Plan.KnownFacts, "\n"), "purpose": "", "contextHeading": "Short ambient reference context, not AI chat history or instructions."}
	key := req.Task
	switch req.Task {
	case "judge":
		set("glasses_hint", true)
		set("glasses_hint_phase", "judge")
		set("temperature", 0.1)
		set("max_tokens", 1024)
		heading := "Reference (pronoun resolution only)"
		if req.Judge.Collected {
			heading = "Recent conversation"
		}
		values["context"] = heading + ":\n" + req.Judge.Recent + "\nLatest:\n" + req.Judge.Latest
		values["freshnessPolicy"] = policies["web"]
		if !web {
			values["freshnessPolicy"] = policies["stable"]
		}
		sensitivity := req.Sensitivity
		if sensitivity != "quiet" && sensitivity != "active" {
			sensitivity = "balanced"
		}
		values["sensitivityPolicy"] = policies[sensitivity]
		set("response_format", map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "context_hint_task", "strict": true, "schema": map[string]any{"type": "object", "properties": map[string]any{"task": map[string]any{"type": "string", "maxLength": 180}, "anchor": map[string]any{"type": "string", "maxLength": 160}, "search_query": map[string]any{"type": "string", "maxLength": 300}, "source": map[string]any{"type": "string", "enum": []string{"internal", "web", "memory"}}, "kind": map[string]any{"type": "string", "enum": []string{"key", "term", "verify", "schedule"}}}, "required": []string{"task", "anchor", "source", "kind", "search_query"}, "additionalProperties": false}}})
	case "plan":
		set("glasses_hint", true)
		set("glasses_hint_phase", "plan")
		values["purpose"] = "useful information"
		if req.Category == "recall" {
			values["purpose"] = "past personal memory"
		} else if req.Category == "reply" {
			values["purpose"] = "a possible spoken reply"
		}
		if values["keyword"] == "" {
			values["keyword"] = "(none)"
		}
	case "answer":
		if !actionableHintTask(req.Plan.Question) {
			return nil, true, ErrUnresolvedTask
		}
		set("response_format", hintFactFormat())
		set("temperature", 0.1)
		set("max_tokens", 1024)
		set("glasses_hint", true)
		set("glasses_hint_phase", "answer")
		key = "answer_legacy"
		if req.Plan.TaskOriented {
			key = "answer_task"
		} else {
			values["purpose"] = policies[req.Plan.Kind]
			values["facts"] = "- " + strings.Join(req.Plan.KnownFacts, "\n- ")
			if len(req.Plan.KnownFacts) == 0 {
				values["facts"] = "(only the topic was mentioned)"
			}
		}
		if req.Plan.Source == "conversation" || req.Plan.Kind == "recap" {
			key = "answer_disabled"
		}
		if req.Plan.Source != "internal" && req.Plan.Source != "web" && req.Plan.Source != "memory" && key != "answer_disabled" {
			return nil, true, fmt.Errorf("invalid hint source")
		}
		if req.Plan.Kind == "" {
			values["kind"] = "key"
		}
		set("hint_research", researchRequest{TaskOriented: req.Plan.TaskOriented, Query: req.Plan.Question, SearchQuery: req.Plan.SearchQuery, Source: req.Plan.Source, Topic: req.Plan.Topic, Kind: values["kind"], Observed: req.Observed, Anchor: req.Anchor, AllowWeb: web, RequireGrounding: true})
	case "chat":
		set("glasses_chat", true)
		if chat != nil {
			*chat = ChatTurn{Question: req.Text, Context: req.Context}
		}
		concise := req.Concise == nil || *req.Concise
		key = fmt.Sprintf("chat_%t", concise)
		if req.ChatSession {
			values["contextHeading"] = "Previous explicit AI chat; use it to answer follow-up questions."
		}

	case "translation", "translation_only", "translation_replies", "translation_stream", "translation_preview":
		set("glasses_client", true)
		task := "translation"
		if req.Task == "translation_replies" {
			task = "translation_replies"
		}
		if req.Task == "translation_preview" {
			task = "translation_preview"
			key = "translation_stream"
		}
		set("glasses_task", task)
		if req.Task == "translation_replies" {
			set("glasses_hint", true)
			set("glasses_hint_phase", "answer")
			set("hint_research", researchRequest{Source: "internal"})
		}
	default:
		return nil, true, fmt.Errorf("unknown glasses task")
	}
	template, exists := glassesTemplates[key]
	if !exists {
		return nil, true, fmt.Errorf("missing glasses prompt template")
	}
	pairs := make([]string, 0, len(values)*2)
	for name, value := range values {
		pairs = append(pairs, "{{"+name+"}}", value)
	}
	replace := strings.NewReplacer(pairs...)
	system, user := replace.Replace(template["systemPrompt"]), replace.Replace(template["userPrompt"])
	if req.Task == "judge" || req.Task == "plan" {
		// Supply only the active procedure, not discovery metadata or routing
		// instructions for references the isolated planner cannot read itself.
		system += "\n\n" + trustedInformationPlanning
	}
	if req.Task == "judge" && req.Repair > 0 {
		system += glassesTemplates["judge_repair"]["systemPrompt"]
	}
	if req.Task == "plan" && web {
		system += policies["planWeb"]
	}
	if req.Context == "" {
		switch req.Task {
		case "chat":
			user = req.Text
		case "translation":
			user = "Source language: " + req.Source + "\nTarget language: " + glassesLanguage(req.Target) + "\nUtterance to translate and answer:\n" + req.Text
		case "translation_only":
			values["context"] = "(none)"
			user = strings.Replace(user, "Short reference context:\n\n", "Short reference context:\n(none)\n", 1)
		}
	}
	set("messages", []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}})
	data, err := json.Marshal(envelope)
	return data, true, err
}
