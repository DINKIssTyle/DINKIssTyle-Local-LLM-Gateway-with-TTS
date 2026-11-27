package core

import (
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"dinkisstyle-chat/internal/skillkit"
)

const (
	jevThinkingDisableThreshold = 0.85
	jevMemorySkipThreshold      = 0.88
	jevSkillNoneThreshold       = 0.90
	jevThinkingMaxSimpleRunes   = 180
	// Keep skill prompt stable for Prefix KV Cache reuse when total skills are small.
	// Activate JEV 2-tier narrowing only once the catalog grows large (12+ skills).
	jevSkillNarrowingMinCount = 12
)

type jevChoiceFunc func(query string, criteria map[string]string, instructions string) (jevReply, bool)

// jevDecisionChooseFn can be overridden in unit tests to exercise decision
// gates deterministically without launching the local MediaPipe worker.
var jevDecisionChooseFn jevChoiceFunc = runLocalJevDecision

func runLocalJevDecision(query string, criteria map[string]string, instructions string) (jevReply, bool) {
	query = strings.TrimSpace(query)
	if query == "" || len(criteria) == 0 {
		return jevReply{}, false
	}
	cfg := currentEmbeddingModelConfig()
	if !cfg.Enabled || !cfg.UseJevDecisions || cfg.ModelID != jevModelID {
		return jevReply{}, false
	}
	rt := getEmbeddingRuntime()
	if rt == nil || rt.modelID != jevModelID {
		return jevReply{}, false
	}
	// Never block interactive chat if the local worker is busy reindexing.
	if !rt.mu.TryLock() {
		return jevReply{}, false
	}
	defer rt.mu.Unlock()
	if err := rt.ensureLoadedLocked(); err != nil {
		return jevReply{}, false
	}
	encoder, ok := rt.encoder.(*jevEncoder)
	if !ok {
		return jevReply{}, false
	}
	reply, err := encoder.ChooseWithInstructions(query, criteria, instructions)
	if err != nil {
		return jevReply{}, false
	}
	return reply, true
}

type jevTurnGateDecision struct {
	Evaluated        bool
	SkipMemorySearch bool
	DisableThinking  bool
	Selected         string
	Confidence       float64
	NoMemoryProb     float64
	SimpleDirectProb float64
	ElapsedMS        int64
	Reason           string
}

func hasExplicitReasoningControl(reqMap map[string]interface{}) bool {
	if reqMap == nil {
		return false
	}
	if _, exists := reqMap["reasoning"]; exists {
		return true
	}
	if _, exists := reqMap["reasoning_effort"]; exists {
		return true
	}
	if _, exists := reqMap["chat_template_kwargs"]; exists {
		return true
	}
	return false
}

// decideChatTurnGates evaluates both memory pre-fetch gating and adaptive
// thinking disable in a single local JEV choice call to minimize latency.
func decideChatTurnGates(userText string, reqMap map[string]interface{}, hasExplicitSkill bool) jevTurnGateDecision {
	text := strings.TrimSpace(userText)
	if text == "" {
		return jevTurnGateDecision{Reason: "empty_text"}
	}

	canDisableThinking := !hasExplicitSkill &&
		utf8.RuneCountInString(text) <= jevThinkingMaxSimpleRunes &&
		!hasExplicitReasoningControl(reqMap) &&
		!hasImageAttachmentInRequest(reqMap)

	criteria := map[string]string{
		"simple_direct_no_memory": "A simple greeting, acknowledgement, short joke, quick translation, or direct general knowledge question that needs neither personal memory nor multi-step reasoning, coding, math, or tool planning.",
		"general_reasoning":       "A self-contained coding, math, analysis, comparison, weather, web lookup, or multi-step reasoning request that does not depend on the user's personal history, preferences, or saved notes.",
		"personal_memory":         "A request that asks about or depends on the user's personal information, preferences, saved notes, past conversations, prior decisions, schedule, or follow-up references like 'earlier', 'last time', 'that', or 'my'.",
	}

	started := time.Now()
	reply, ok := jevDecisionChooseFn(
		text,
		criteria,
		"Classify the user's request into one of the three categories. When unsure about personal memory, choose personal_memory. When unsure about reasoning complexity, choose general_reasoning.",
	)
	elapsedMS := time.Since(started).Milliseconds()
	if !ok {
		return jevTurnGateDecision{ElapsedMS: elapsedMS, Reason: "jev_unavailable"}
	}

	selected := strings.TrimSpace(reply.Selected)
	simpleProb := reply.Probabilities["simple_direct_no_memory"]
	generalProb := reply.Probabilities["general_reasoning"]
	noMemoryProb := simpleProb + generalProb
	confidence := reply.Probabilities[selected]

	skipMemory := selected != "personal_memory" && noMemoryProb >= jevMemorySkipThreshold
	disableThinking := canDisableThinking && selected == "simple_direct_no_memory" && simpleProb >= jevThinkingDisableThreshold

	reason := "keep_defaults"
	switch {
	case skipMemory && disableThinking:
		reason = "skip_memory+disable_thinking"
	case disableThinking:
		reason = "disable_thinking"
	case skipMemory:
		reason = "skip_memory"
	}

	return jevTurnGateDecision{
		Evaluated:        true,
		SkipMemorySearch: skipMemory,
		DisableThinking:  disableThinking,
		Selected:         selected,
		Confidence:       confidence,
		NoMemoryProb:     noMemoryProb,
		SimpleDirectProb: simpleProb,
		ElapsedMS:        elapsedMS,
		Reason:           reason,
	}
}

var jevReasoningUnsupportedCache = struct {
	mu     sync.RWMutex
	models map[string]bool
}{
	models: map[string]bool{},
}

func jevReasoningCacheKey(modelID, llmMode string) string {
	return strings.ToLower(strings.TrimSpace(llmMode)) + "::" + strings.ToLower(strings.TrimSpace(modelID))
}

func isThinkingDisableUnsupportedForModel(modelID, llmMode string) bool {
	if strings.TrimSpace(modelID) == "" {
		return false
	}
	jevReasoningUnsupportedCache.mu.RLock()
	defer jevReasoningUnsupportedCache.mu.RUnlock()
	return jevReasoningUnsupportedCache.models[jevReasoningCacheKey(modelID, llmMode)]
}

func markThinkingDisableUnsupportedForModel(modelID, llmMode string) {
	if strings.TrimSpace(modelID) == "" {
		return
	}
	jevReasoningUnsupportedCache.mu.Lock()
	jevReasoningUnsupportedCache.models[jevReasoningCacheKey(modelID, llmMode)] = true
	jevReasoningUnsupportedCache.mu.Unlock()
}

func isUnsupportedReasoningParamError(errorMsg string) bool {
	lower := strings.ToLower(errorMsg)
	if strings.Contains(lower, "does not expose reasoning configuration") {
		return true
	}
	if strings.Contains(lower, "\"param\"") && strings.Contains(lower, "\"reasoning\"") {
		return true
	}
	if strings.Contains(lower, "chat_template_kwargs") && (strings.Contains(lower, "invalid") || strings.Contains(lower, "unknown") || strings.Contains(lower, "unsupported")) {
		return true
	}
	return false
}

// applyThinkingDisableToRequest applies the appropriate thinking-off control
// for the active upstream protocol:
//   - LM Studio native /api/v1/chat (llmMode == "stateful"): reasoning = "off"
//   - OpenAI-compatible /v1/chat/completions: chat_template_kwargs.enable_thinking = false
func applyThinkingDisableToRequest(reqMap map[string]interface{}, llmMode, modelID string) bool {
	if reqMap == nil || hasExplicitReasoningControl(reqMap) {
		return false
	}
	if isThinkingDisableUnsupportedForModel(modelID, llmMode) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(llmMode), "stateful") {
		reqMap["reasoning"] = "off"
		return true
	}
	reqMap["chat_template_kwargs"] = map[string]any{"enable_thinking": false}
	return true
}

func hasImageAttachmentInRequest(reqMap map[string]interface{}) bool {
	if reqMap == nil {
		return false
	}
	checkContent := func(content interface{}) bool {
		parts, ok := content.([]interface{})
		if !ok {
			return false
		}
		for _, part := range parts {
			partMap, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			partType, _ := partMap["type"].(string)
			if strings.Contains(strings.ToLower(partType), "image") {
				return true
			}
		}
		return false
	}
	if messages, ok := reqMap["messages"].([]interface{}); ok {
		for _, raw := range messages {
			if msg, ok := raw.(map[string]interface{}); ok && checkContent(msg["content"]) {
				return true
			}
		}
	}
	if inputs, ok := reqMap["input"].([]interface{}); ok {
		for _, raw := range inputs {
			if item, ok := raw.(map[string]interface{}); ok && checkContent(item["content"]) {
				return true
			}
		}
	}
	return false
}

type jevSkillNarrowingResult struct {
	Evaluated   bool
	Scores      map[string]float64
	NoSkillProb float64
	Selected    string
	ElapsedMS   int64
}

func scoreChatSkillsWithJev(userText string, skills []skillkit.Skill) jevSkillNarrowingResult {
	text := strings.TrimSpace(userText)
	if text == "" || len(skills) < jevSkillNarrowingMinCount {
		return jevSkillNarrowingResult{}
	}
	criteria := make(map[string]string, len(skills)+1)
	criteria["none"] = "No specialized skill procedure is needed; answer directly with general knowledge or ordinary conversation."
	for _, skill := range skills {
		desc := strings.TrimSpace(skill.Description)
		if desc == "" {
			desc = skill.DisplayName
		}
		if desc == "" {
			desc = skill.Name
		}
		criteria[skill.Namespace] = desc
	}
	started := time.Now()
	reply, ok := jevDecisionChooseFn(
		text,
		criteria,
		"Choose the skill procedure most relevant to fulfilling the user's request, or none if no specialized skill is needed.",
	)
	elapsedMS := time.Since(started).Milliseconds()
	if !ok {
		return jevSkillNarrowingResult{ElapsedMS: elapsedMS}
	}
	scores := make(map[string]float64, len(skills))
	for _, skill := range skills {
		if prob, exists := reply.Probabilities[skill.Namespace]; exists {
			scores[skill.Namespace] = prob
		}
	}
	return jevSkillNarrowingResult{
		Evaluated:   true,
		Scores:      scores,
		NoSkillProb: reply.Probabilities["none"],
		Selected:    strings.TrimSpace(reply.Selected),
		ElapsedMS:   elapsedMS,
	}
}
