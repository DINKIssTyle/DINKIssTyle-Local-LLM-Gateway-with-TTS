package core

import (
	"crypto/sha1"
	"database/sql"
	"dinkisstyle-chat/internal/mcp"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

type ToolExecutionEvent struct {
	Tool        string `json:"tool"`
	ArgsSummary string `json:"args_summary"`
	LatencyMS   int64  `json:"latency_ms"`
	Success     bool   `json:"success"`
	Skipped     bool   `json:"skipped,omitempty"`
}

type RequestExecutionContext struct {
	UserID             string
	IntentKey          string
	RawQuery           string
	NormalizedQuery    string
	PatternID          int64
	RequestStartedAt   time.Time
	CompletedAt        time.Time
	ToolEvents         []ToolExecutionEvent
	Success            bool
	FallbackUsed       bool
	RepeatedBlocked    bool
	SelfCorrectionUsed bool
	FollowupObserved   bool
	UserFeedbackScore  float64
	RecipeVersion      string
}

var whitespaceRE = regexp.MustCompile(`\s+`)
var punctuationRE = regexp.MustCompile(`[^\p{L}\p{N}\s]+`)

const (
	minRecipeUsageForHint = 3
	minRecipeScoreForHint = 0.55
	maxProcedureHintTools = 3
)

func buildRequestExecutionContext(userID string, reqMap map[string]interface{}, startedAt time.Time) (*RequestExecutionContext, error) {
	rawQuery := extractLastUserQuery(reqMap)
	if strings.TrimSpace(rawQuery) == "" {
		return nil, nil
	}

	normalized := normalizeUserQuery(rawQuery)
	intentKey := classifyIntentKey(normalized)
	fingerprint := fingerprintQuery(normalized)

	patternID, err := mcp.UpsertRequestPattern(userID, intentKey, compactText(strings.TrimSpace(rawQuery), 240), fingerprint)
	if err != nil {
		return nil, err
	}

	return &RequestExecutionContext{
		UserID:           userID,
		IntentKey:        intentKey,
		RawQuery:         strings.TrimSpace(rawQuery),
		NormalizedQuery:  normalized,
		PatternID:        patternID,
		RequestStartedAt: startedAt,
	}, nil
}

func extractLastUserQuery(reqMap map[string]interface{}) string {
	if messages, ok := reqMap["messages"].([]interface{}); ok {
		for i := len(messages) - 1; i >= 0; i-- {
			msg, ok := messages[i].(map[string]interface{})
			if !ok {
				continue
			}
			role, _ := msg["role"].(string)
			if role != "user" {
				continue
			}
			if content, ok := msg["content"].(string); ok && strings.TrimSpace(content) != "" {
				return content
			}
		}
	}

	if input, ok := reqMap["input"].(string); ok {
		return input
	}

	return ""
}

func normalizeUserQuery(input string) string {
	normalized := strings.ToLower(strings.TrimSpace(input))
	normalized = punctuationRE.ReplaceAllString(normalized, " ")
	normalized = whitespaceRE.ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}

func classifyIntentKey(normalized string) string {
	if normalized == "" {
		return "unknown"
	}

	switch {
	case containsAny(normalized, "미세먼지", "초미세먼지", "pm10", "pm2 5", "pm25", "air quality", "fine dust", "dust"):
		return "air_quality.current"
	case containsAny(normalized, "날씨", "기온", "비와", "비 ", "weather", "forecast", "temperature"):
		return "weather.current"
	case containsAny(normalized, "현재 시간", "지금 시간", "몇 시", "몇시", "what time", "current time", "time now"):
		return "time.current"
	case containsAny(normalized, "메모리", "램", "ram", "memory usage", "memory status", "free memory"):
		return "system.memory_status"
	case containsAny(normalized, "cpu", "processor", "load average", "cpu 사용", "cpu 상태"):
		return "system.cpu_status"
	case containsAny(normalized, "디스크", "storage", "disk", "용량", "free space"):
		return "system.disk_status"
	case containsAny(normalized, "network", "네트워크", "인터넷 속도", "ping"):
		return "system.network_status"
	case containsAny(normalized, "process", "프로세스", "실행 중", "running app"):
		return "system.process_status"
	default:
		return "search.general"
	}
}

func containsAny(input string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(input, term) {
			return true
		}
	}
	return false
}

func fingerprintQuery(normalized string) string {
	sum := sha1.Sum([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func (c *RequestExecutionContext) AddToolEvent(toolName, args string, latency time.Duration, success bool, skipped bool) {
	if c == nil {
		return
	}
	c.ToolEvents = append(c.ToolEvents, ToolExecutionEvent{
		Tool:        toolName,
		ArgsSummary: compactText(strings.TrimSpace(args), 160),
		LatencyMS:   latency.Milliseconds(),
		Success:     success,
		Skipped:     skipped,
	})
}

func persistRequestExecution(ctx *RequestExecutionContext) {
	if ctx == nil {
		return
	}

	ctx.CompletedAt = time.Now()
	toolChainJSON, err := json.Marshal(ctx.ToolEvents)
	if err != nil {
		log.Printf("[ProceduralMemory] failed to marshal tool chain: %v", err)
		return
	}

	var toolLatency int64
	for _, event := range ctx.ToolEvents {
		toolLatency += event.LatencyMS
	}

	entry := mcp.RequestExecutionEntry{
		UserID:               ctx.UserID,
		IntentKey:            ctx.IntentKey,
		RequestPatternID:     sql.NullInt64{Int64: ctx.PatternID, Valid: ctx.PatternID > 0},
		RawQuery:             ctx.RawQuery,
		NormalizedQuery:      ctx.NormalizedQuery,
		ToolChainJSON:        string(toolChainJSON),
		ToolCount:            len(ctx.ToolEvents),
		TotalLatencyMS:       ctx.CompletedAt.Sub(ctx.RequestStartedAt).Milliseconds(),
		ToolLatencyMS:        toolLatency,
		Success:              ctx.Success,
		FallbackUsed:         ctx.FallbackUsed,
		RepeatedToolBlocked:  ctx.RepeatedBlocked,
		SelfCorrectionUsed:   ctx.SelfCorrectionUsed,
		FollowupWithinTwoMin: ctx.FollowupObserved,
		UserFeedbackScore:    ctx.UserFeedbackScore,
		RecipeVersion:        ctx.RecipeVersion,
		CreatedAt:            ctx.CompletedAt,
	}

	if err := mcp.InsertRequestExecution(entry); err != nil {
		log.Printf("[ProceduralMemory] failed to insert request execution: %v", err)
		return
	}

	if err := mcp.UpsertRequestIntentStat(ctx.UserID, ctx.IntentKey, ctx.Success, entry.TotalLatencyMS, ctx.CompletedAt); err != nil {
		log.Printf("[ProceduralMemory] failed to update request intent stats: %v", err)
	}

	if err := mcp.RefreshProcedureRecipes(ctx.UserID, ctx.IntentKey); err != nil {
		log.Printf("[ProceduralMemory] failed to refresh procedure recipes: %v", err)
	}
}

func getProceduralHint(ctx *RequestExecutionContext) (string, string, error) {
	if ctx == nil || strings.TrimSpace(ctx.UserID) == "" || strings.TrimSpace(ctx.IntentKey) == "" {
		return "", "", nil
	}

	recipe, err := mcp.GetTopProcedureRecipe(ctx.UserID, ctx.IntentKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", nil
		}
		return "", "", err
	}

	if recipe.UsageCount < minRecipeUsageForHint || recipe.FinalScore < minRecipeScoreForHint {
		return "", "", nil
	}

	toolNames := extractToolNamesFromRecipe(recipe.ToolChainTemplateJSON)
	if len(toolNames) == 0 {
		return "", recipe.RecipeName, nil
	}
	if len(toolNames) > maxProcedureHintTools {
		toolNames = toolNames[:maxProcedureHintTools]
	}

	return fmt.Sprintf(`
### PROCEDURAL HINT ###
For %s, prefer the shortest proven path for this user.
Preferred tool sequence: %s.
Reuse this path when it clearly fits, and avoid unnecessary detours or repeated searches.
`, ctx.IntentKey, strings.Join(toolNames, " -> ")), recipe.RecipeName, nil
}

func extractToolNamesFromRecipe(toolChainJSON string) []string {
	var events []ToolExecutionEvent
	if err := json.Unmarshal([]byte(toolChainJSON), &events); err != nil {
		return nil
	}

	names := make([]string, 0, len(events))
	for _, event := range events {
		name := strings.TrimSpace(event.Tool)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}
