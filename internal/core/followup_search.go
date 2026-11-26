package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"dinkisstyle-chat/internal/chatharness"
)

type priorConversationTurn struct {
	User      string
	Assistant string
}

var (
	followupThinkingStrip = regexp.MustCompile(`(?is)<think>.*?(?:</think>|$)|<\|channel>\s*thought\b.*?(?:<channel\|>|$)`)
	followupStrongCode    = regexp.MustCompile(`\b(?:[A-Za-z]{1,4}[-]?\d{2,5}[A-Za-z]?|\d{3,4})\b`)
	followupSubjectNumber = regexp.MustCompile(`\b(?:19\d{2}|20\d{2}|\d{2,5})\b`)
	followupSubjectLatin  = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9-]{2,}\b`)
	followupSubjectHangul = regexp.MustCompile(`[가-힣]{2,}`)
)

var followupAnaphoraCues = []string{
	"좀더", "좀 더", "더 자세", "자세한", "자세히", "추가로", "추가 정보", "추가 내용",
	"그 사고", "그 사건", "그 사람", "그 영화", "그 책", "그 회사", "그 모델", "그 제품",
	"그거", "그것", "이거", "이것", "해당 ", "방금", "아까", "위에서", "위의 ", "거기",
	"원인이", "원인은", "이유가", "이유는", "결과가", "결과는", "생존자", "사망자", "피해",
	"몇년", "몇 년", "몇월", "몇 월", "며칠", "언제", "어느 날", "정확한 날짜", "정확한 시점",
	"알려줘야지", "말해줘야지", "바뀌었어", "바뀐", "이후",
	"more detail", "more details", "more about", "tell me more", "further detail",
	"that accident", "the accident", "that incident", "the incident", "that person",
	"what happened", "why did it", "how did it", "what date", "which year", "which month",
}

var followupGenericLatin = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true, "that": true,
	"this": true, "was": true, "were": true, "are": true, "have": true, "had": true,
	"has": true, "not": true, "but": true, "all": true, "one": true, "two": true,
	"more": true, "about": true, "into": true, "over": true, "after": true, "could": true,
	"report": true, "noted": true, "user": true, "assistant": true, "detail": true,
	"details": true, "accident": true, "incident": true, "crash": true, "plane": true,
	"jet": true, "engine": true, "issue": true, "fire": true, "piece": true, "ice": true,
	"http": true, "https": true, "www": true, "com": true, "org": true, "net": true,
	"html": true, "news": true, "view": true, "article": true,
}

var followupGenericHangul = map[string]bool{
	"사고": true, "사건": true, "내용": true, "내용을": true, "내용이": true, "자세한": true,
	"자세히": true, "좀더": true, "추가": true, "정보": true, "정보를": true, "검색": true,
	"찾아": true, "찾아와줘": true, "찾아줘": true, "알려": true, "알려줘": true, "설명": true,
	"번역": true, "한국어": true, "한국어로": true, "원인": true, "원인은": true, "이유": true,
	"결과": true, "보고서": true, "비행기": true, "추락": true, "생존": true, "화재": true,
	"발생": true, "전원": true, "탑승자": true, "승무원": true, "문제": true, "파손": true,
	"파괴": true, "고장": true, "언급": true, "사람": true, "최근": true, "작품": true,
	"영화": true, "회사": true, "제품": true, "모델": true, "정도": true, "경우": true,
	"대한": true, "관련": true, "정말": true, "너무": true, "계속": true, "그냥": true,
	"편명": true, "변경": true, "날짜": true, "시점": true, "발표": true, "보도": true,
	"기사": true, "링크": true, "참고": true, "출처": true, "노선": true, "착륙": true,
}

func extractRecentPriorTurns(reqMap map[string]interface{}, snapshot chatharness.SessionUISnapshot, maxTurns int) []priorConversationTurn {
	if maxTurns <= 0 {
		maxTurns = 3
	}
	var turns []priorConversationTurn
	if reqMap != nil {
		if messages, ok := reqMap["messages"].([]interface{}); ok && len(messages) > 0 {
			lastUser := -1
			for i := len(messages) - 1; i >= 0; i-- {
				m, _ := messages[i].(map[string]interface{})
				if m["role"] == "user" {
					lastUser = i
					break
				}
			}
			if lastUser > 0 {
				var cur priorConversationTurn
				for _, raw := range messages[:lastUser] {
					m, _ := raw.(map[string]interface{})
					role, _ := m["role"].(string)
					content, _ := m["content"].(string)
					switch role {
					case "user":
						if cur.User != "" && cur.Assistant != "" {
							turns = append(turns, cur)
						}
						cur = priorConversationTurn{User: strings.TrimSpace(content)}
					case "assistant":
						if m["tool_calls"] == nil {
							cleaned := strings.TrimSpace(followupThinkingStrip.ReplaceAllString(content, ""))
							if cleaned != "" {
								cur.Assistant = cleaned
							}
						}
					}
				}
				if cur.User != "" && cur.Assistant != "" {
					turns = append(turns, cur)
				}
			}
		}
	}
	if len(turns) == 0 {
		for _, t := range snapshot.Turns {
			if t.Status != "" && t.Status != "completed" {
				continue
			}
			u := strings.TrimSpace(t.UserContent)
			a := strings.TrimSpace(followupThinkingStrip.ReplaceAllString(t.AssistantContent, ""))
			if u != "" && a != "" {
				turns = append(turns, priorConversationTurn{User: u, Assistant: a})
			}
		}
	}
	if len(turns) > maxTurns {
		turns = turns[len(turns)-maxTurns:]
	}
	return turns
}

func extractPrimarySearchQuery(toolName, argsJSON string) (string, map[string]interface{}, bool) {
	switch strings.TrimSpace(toolName) {
	case "search_web", "naver_search", "namu_wiki", "search_web_multi":
	default:
		return "", nil, false
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(argsJSON)), &payload); err != nil || payload == nil {
		payload = map[string]interface{}{}
	}
	switch strings.TrimSpace(toolName) {
	case "search_web", "naver_search":
		q, _ := payload["query"].(string)
		return strings.TrimSpace(q), payload, true
	case "namu_wiki":
		q, _ := payload["keyword"].(string)
		return strings.TrimSpace(q), payload, true
	case "search_web_multi":
		if arr, ok := payload["queries"].([]interface{}); ok && len(arr) > 0 {
			q, _ := arr[0].(string)
			return strings.TrimSpace(q), payload, true
		}
		return "", payload, true
	}
	return "", nil, false
}

func normalizeCompactLower(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(s)), " "))
}

func hasFollowupCue(text string) bool {
	lower := normalizeCompactLower(text)
	for _, cue := range followupAnaphoraCues {
		if strings.Contains(lower, cue) {
			return true
		}
	}
	return false
}

var urlStripPattern = regexp.MustCompile(`https?://\S+`)

func extractStrongIdentifiers(text string) []string {
	cleaned := urlStripPattern.ReplaceAllString(text, " ")
	raw := followupStrongCode.FindAllString(cleaned, -1)
	seen := map[string]bool{}
	out := make([]string, 0, len(raw))
	for _, tok := range raw {
		lower := strings.ToLower(tok)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		out = append(out, lower)
	}
	return out
}

func queryContainsPriorAnchor(query string, turns []priorConversationTurn) bool {
	if len(turns) == 0 {
		return false
	}
	latest := turns[len(turns)-1]
	latestText := latest.User + "\n" + latest.Assistant
	var allBuilder strings.Builder
	for _, t := range turns {
		allBuilder.WriteString(t.User)
		allBuilder.WriteByte('\n')
		allBuilder.WriteString(t.Assistant)
		allBuilder.WriteByte('\n')
	}
	allText := allBuilder.String()
	queryLower := strings.ToLower(query)

	// If the most recent turn contains distinguishing identifiers (e.g. OZ214,
	// OZ212, F-28, 221, 2013), require at least one identifier from the recent
	// conversation to appear in the query so broad names alone (e.g. "아시아나항공")
	// do not skip follow-up disambiguation.
	latestIDs := extractStrongIdentifiers(latestText)
	if len(latestIDs) > 0 {
		for _, id := range extractStrongIdentifiers(allText) {
			if strings.Contains(queryLower, id) {
				return true
			}
			// Also match the numeric core of alphanumeric flight/model codes (e.g. "214" for "OZ214").
			if digits := followupSubjectNumber.FindString(id); len(digits) >= 3 && strings.Contains(queryLower, digits) {
				return true
			}
		}
		return false
	}

	cleanedLatest := urlStripPattern.ReplaceAllString(latestText, " ")
	for _, num := range followupSubjectNumber.FindAllString(cleanedLatest, -1) {
		if strings.Contains(queryLower, strings.ToLower(num)) {
			return true
		}
	}
	for _, word := range followupSubjectLatin.FindAllString(cleanedLatest, -1) {
		w := strings.ToLower(word)
		if followupGenericLatin[w] {
			continue
		}
		if strings.Contains(queryLower, w) {
			return true
		}
	}
	for _, word := range followupSubjectHangul.FindAllString(cleanedLatest, -1) {
		if followupGenericHangul[word] || utf8.RuneCountInString(word) < 2 {
			continue
		}
		runes := []rune(word)
		stem := word
		if len(runes) > 3 {
			stem = string(runes[:len(runes)-1])
		}
		if strings.Contains(query, word) || (utf8.RuneCountInString(stem) >= 2 && !followupGenericHangul[stem] && strings.Contains(query, stem)) {
			return true
		}
	}
	return false
}

// needsFollowupSearchRewrite detects when a model emits a context-less,
// overly broad, or echoed follow-up search query that omits the distinguishing
// subject/identifiers from the recent conversation.
func needsFollowupSearchRewrite(toolName, argsJSON, currentUserText string, turns []priorConversationTurn) (string, bool) {
	if len(turns) == 0 {
		return "", false
	}
	rawQuery, _, supported := extractPrimarySearchQuery(toolName, argsJSON)
	if !supported {
		return "", false
	}
	if rawQuery != "" && queryContainsPriorAnchor(rawQuery, turns) {
		return rawQuery, false
	}
	if rawQuery == "" {
		if hasFollowupCue(currentUserText) || utf8.RuneCountInString(strings.TrimSpace(currentUserText)) <= 36 {
			return "", true
		}
		return "", false
	}
	if hasFollowupCue(currentUserText) || hasFollowupCue(rawQuery) {
		return rawQuery, true
	}
	normUser := normalizeCompactLower(currentUserText)
	normQuery := normalizeCompactLower(rawQuery)
	if utf8.RuneCountInString(normUser) <= 28 && (normQuery == normUser || strings.Contains(normUser, normQuery)) &&
		(strings.Contains(normUser, "더") || strings.Contains(normUser, "자세") || strings.Contains(normUser, "상세") || strings.Contains(normUser, "추가") || strings.Contains(normUser, "이후") || strings.Contains(normUser, "후속")) {
		return rawQuery, true
	}
	return rawQuery, false
}

func formatPriorTurnsForRewrite(turns []priorConversationTurn) string {
	var b strings.Builder
	for i, t := range turns {
		if i == len(turns)-1 {
			b.WriteString("Most recent turn (primary reference):\n")
		} else {
			fmt.Fprintf(&b, "Earlier turn %d:\n", i+1)
		}
		b.WriteString("User: ")
		b.WriteString(compactText(t.User, 450))
		b.WriteString("\nAssistant: ")
		b.WriteString(compactText(t.Assistant, 550))
		b.WriteString("\n\n")
	}
	return b.String()
}

func rewriteFollowupSearchArguments(ctx context.Context, endpoint, token, modelID, toolName, argsJSON, currentUserText string, turns []priorConversationTurn) (string, bool) {
	rawQuery, needed := needsFollowupSearchRewrite(toolName, argsJSON, currentUserText, turns)
	if !needed || strings.TrimSpace(endpoint) == "" || strings.TrimSpace(modelID) == "" {
		return argsJSON, false
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"rewritten": map[string]any{"type": "boolean"},
			"query":     map[string]any{"type": "string"},
		},
		"required":             []string{"rewritten", "query"},
		"additionalProperties": false,
	}
	userPrompt := formatPriorTurnsForRewrite(turns) +
		"Current user follow-up:\n" + compactText(currentUserText, 300)
	if rawQuery != "" {
		userPrompt += "\n\nModel's initial search query:\n" + compactText(rawQuery, 200)
	}

	payload := map[string]any{
		"model":       modelID,
		"stream":      false,
		"temperature": 0,
		"max_tokens":  120,
		"messages": []any{
			map[string]any{
				"role": "system",
				"content": "Rewrite the user follow-up into one concise, self-contained web search query (3-10 keywords, no filler words like '관련 정보'). " +
					"Resolve the subject from the MOST RECENT turn first, and carry over key distinguishing identifiers " +
					"(flight/model numbers like OZ214/OZ212, years like 2013, specific locations or events) from earlier turns when they belong to the same ongoing topic, " +
					"so search results will not match unrelated events. " +
					"If the current request is a completely new self-contained topic unrelated to the conversation, set rewritten=false and keep the query unchanged. Return only JSON.",
			},
			map[string]any{"role": "user", "content": userPrompt},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "followup_search_rewrite",
				"strict": true,
				"schema": schema,
			},
		},
	}
	if strings.Contains(strings.ToLower(modelID), "qwen") {
		payload["chat_template_kwargs"] = map[string]any{"enable_thinking": false}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return argsJSON, false
	}
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, "POST", strings.TrimRight(endpoint, "/")+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return argsJSON, false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DKST-Task-Phase", "followup_rewrite")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	started := time.Now()
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		log.Printf("[followup-search] rewrite request failed (%s): %v", time.Since(started).Round(time.Millisecond), err)
		return argsJSON, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return argsJSON, false
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return argsJSON, false
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &completion); err != nil || len(completion.Choices) != 1 {
		return argsJSON, false
	}
	controlJSON := strings.TrimSpace(completion.Choices[0].Message.Content)
	if controlJSON == "" {
		candidate := strings.TrimSpace(completion.Choices[0].Message.ReasoningContent)
		if strings.HasPrefix(candidate, "{") && json.Valid([]byte(candidate)) {
			controlJSON = candidate
		}
	}
	controlJSON = stripJSONFence(controlJSON)
	var result struct {
		Rewritten bool   `json:"rewritten"`
		Query     string `json:"query"`
	}
	if err := json.Unmarshal([]byte(controlJSON), &result); err != nil {
		return argsJSON, false
	}
	rewrittenQuery := strings.TrimSpace(result.Query)
	if !result.Rewritten || rewrittenQuery == "" || rewrittenQuery == rawQuery {
		return argsJSON, false
	}

	_, argsMap, ok := extractPrimarySearchQuery(toolName, argsJSON)
	if !ok {
		return argsJSON, false
	}
	switch strings.TrimSpace(toolName) {
	case "search_web", "naver_search":
		argsMap["query"] = rewrittenQuery
	case "namu_wiki":
		argsMap["keyword"] = rewrittenQuery
	case "search_web_multi":
		argsMap["queries"] = []string{rewrittenQuery, rewrittenQuery + " details report"}
	default:
		return argsJSON, false
	}
	encoded, err := json.Marshal(argsMap)
	if err != nil {
		return argsJSON, false
	}
	log.Printf("[followup-search] rewritten %s query (%s): %q -> %q", toolName, time.Since(started).Round(time.Millisecond), rawQuery, rewrittenQuery)
	return string(encoded), true
}
