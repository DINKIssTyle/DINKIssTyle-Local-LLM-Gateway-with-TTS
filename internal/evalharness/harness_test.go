package evalharness

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dinkisstyle-chat/internal/chatharness"
)

func TestLoadConfigReadsLocalEnvWithoutExposingValues(t *testing.T) {
	t.Setenv("DKST_EVAL_ENDPOINT", "")
	t.Setenv("DKST_EVAL_MODEL", "")
	t.Setenv("DKST_EVAL_API_KEY", "")
	path := filepath.Join(t.TempDir(), ".env.eval.local")
	content := "DKST_EVAL_ENDPOINT=http://127.0.0.1:8094\nDKST_EVAL_MODEL=test-model\nDKST_EVAL_API_KEY=secret-value\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Endpoint != "http://127.0.0.1:8094" || config.Model != "test-model" || config.APIKey != "secret-value" {
		t.Fatalf("unexpected config values")
	}
}

func TestSafeToolDefinitionsExcludeSideEffects(t *testing.T) {
	_, names := SafeToolDefinitions()
	joined := "," + strings.Join(names, ",") + ","
	for _, forbidden := range []string{"execute_command", "delete_memory", "save_user_fact", "delete_user_fact"} {
		if strings.Contains(joined, ","+forbidden+",") {
			t.Fatalf("side-effecting tool %s was exposed", forbidden)
		}
	}
	for _, required := range []string{"search_web", "search_web_multi", "read_web_page"} {
		if !strings.Contains(joined, ","+required+",") {
			t.Fatalf("read-only tool %s was omitted", required)
		}
	}
}

func TestFinalChecksScoreSearchAnglesSourcesAndCitations(t *testing.T) {
	source := chatharness.WebEvidenceSource{Title: "Report", URL: "https://example.com/report"}
	report := Report{
		FinalAnswer: "확인했습니다. https://example.com/report",
		Sources:     []chatharness.WebEvidenceSource{source},
		ToolTrace: []ToolTrace{{
			Name:      "search_web_multi",
			Arguments: json.RawMessage(`{"queries":["first angle","second angle"]}`),
		}},
	}
	scenario := Scenario{Expectations: Expectations{
		RequireToolCall: true, RequireWebTool: true, MinSearchAngles: 2,
		MinSources: 1, RequireCitations: true, ForbidDuplicates: true,
	}}
	graded := finalizeReport(report, scenario)
	if !graded.Passed {
		t.Fatalf("expected report to pass: %#v", graded.Checks)
	}
}

func TestFinalChecksRejectRawToolMarkup(t *testing.T) {
	report := Report{FinalAnswer: `<tool_call><function=read_web_page><parameter=url>https://example.com</parameter></function></tool_call>`}
	graded := finalizeReport(report, Scenario{})
	if graded.Passed {
		t.Fatalf("raw textual tool markup must not pass: %#v", graded.Checks)
	}
}

func TestFinalChecksNormalizeScientificSubscripts(t *testing.T) {
	report := Report{FinalAnswer: "물의 화학식은 H₂O입니다.", LLMRounds: 1}
	scenario := Scenario{Expectations: Expectations{
		ForbidToolCall: true, MaxLLMRounds: 1, RequiredAnswerSubstrings: []string{"H2O"},
	}}
	graded := finalizeReport(report, scenario)
	if !graded.Passed {
		t.Fatalf("scientific subscript normalization failed: %#v", graded.Checks)
	}
}

func TestFinalChecksEnforceSearchAngleBudget(t *testing.T) {
	report := Report{
		FinalAnswer: "answer",
		ToolTrace: []ToolTrace{{
			Name: "search_web_multi", Arguments: json.RawMessage(`{"queries":["one","two"]}`),
		}, {
			Name: "search_web_multi", Arguments: json.RawMessage(`{"queries":["three","four"]}`),
		}},
	}
	graded := finalizeReport(report, Scenario{Expectations: Expectations{MaxSearchAngles: 3}})
	if graded.Passed {
		t.Fatalf("four search angles exceeded the configured budget: %#v", graded.Checks)
	}
}

func TestFinalChecksRequireSpecificEvidenceURL(t *testing.T) {
	report := Report{FinalAnswer: "answer", Sources: []chatharness.WebEvidenceSource{{URL: "https://go.dev/"}}}
	graded := finalizeReport(report, Scenario{Expectations: Expectations{RequiredURLSubstrings: []string{"go.dev/doc/go1.25"}}})
	if graded.Passed {
		t.Fatalf("generic domain passed a specific evidence URL requirement: %#v", graded.Checks)
	}
}

func TestFinalChecksRejectContextDriftAndToolArgumentErrors(t *testing.T) {
	report := Report{
		FinalAnswer: "테스트인물A의 자녀는 테스트인물B와 테스트인물C입니다.",
		ToolTrace: []ToolTrace{{
			Name: "search_web", Arguments: json.RawMessage(`{}`),
			Error: `invalid arguments for search_web: argument "query" is required`,
		}},
	}
	expectations := Expectations{
		RequiredAnswerSubstrings:  []string{"크루즈"},
		ForbiddenAnswerSubstrings: []string{"테스트인물A", "테스트인물B", "테스트인물C"},
		ForbidToolArgumentErrors:  true,
	}
	graded := finalizeReport(report, Scenario{Expectations: expectations})
	if graded.Passed {
		t.Fatalf("context drift and missing search arguments must fail: %#v", graded.Checks)
	}

	report.FinalAnswer = "톰 크루즈의 자녀 정보입니다."
	report.ToolTrace[0].Arguments = json.RawMessage(`{"query":"Tom Cruise children"}`)
	report.ToolTrace[0].Error = ""
	graded = finalizeReport(report, Scenario{Expectations: expectations})
	if !graded.Passed {
		t.Fatalf("correct contextual answer was rejected: %#v", graded.Checks)
	}
}

func TestFinalChecksRequireSelectedSkill(t *testing.T) {
	scenario := Scenario{Expectations: Expectations{RequiredSkills: []string{"builtin:msn-weather-current"}}}
	graded := finalizeReport(Report{FinalAnswer: "answer"}, scenario)
	if graded.Passed {
		t.Fatalf("missing required skill must fail: %#v", graded.Checks)
	}
	graded = finalizeReport(Report{
		FinalAnswer:    "answer",
		SelectedSkills: []string{"builtin:msn-weather-current"},
	}, scenario)
	if !graded.Passed {
		t.Fatalf("selected required skill was rejected: %#v", graded.Checks)
	}
}

func TestRunnerLoadsAndInjectsBundledWeatherSkill(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	skillDir := filepath.Join(builtin, "msn-weather-current")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	document := `---
name: msn-weather-current
description: Use for 현재 날씨 requests in Korean.
---
Open the MSN current weather page directly and read the 현재 날씨 section.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "Open the MSN current weather page directly") {
			t.Fatalf("skill instructions were not injected: %s", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"테스트 응답"}}]}`)),
			Request:    r,
		}, nil
	})}
	runner := Runner{Config: Config{
		Endpoint: "http://example.invalid", Model: "test-model", APIKey: "test-key",
		BuiltinSkillsDir: builtin,
	}, Client: client}
	report := runner.Run(context.Background(), Scenario{
		ID: "weather-skill", Prompt: "부산의 현재 날씨를 알려주세요", MaxTurns: 1,
		Expectations: Expectations{RequiredSkills: []string{"builtin:msn-weather-current"}},
	})
	if !report.Passed || report.SkillPromptChars == 0 || len(report.SelectedSkills) != 1 {
		t.Fatalf("weather skill was not loaded and reported: %#v", report)
	}
}

func TestRunnerRecoversQwenTextualToolCall(t *testing.T) {
	responses := []string{
		`{"choices":[{"message":{"role":"assistant","content":"<tool_call><function=get_current_time><parameter=timezone>Asia/Seoul</parameter></function></tool_call>"}}]}`,
		`{"choices":[{"message":{"role":"assistant","content":"현재 시간을 도구로 확인했습니다."}}]}`,
	}
	requestCount := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		index := requestCount
		requestCount++
		if index >= len(responses) {
			t.Fatalf("unexpected extra LLM request")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(responses[index])),
			Request:    r,
		}, nil
	})}

	runner := Runner{Config: Config{Endpoint: "http://example.invalid", Model: "test-model", APIKey: "test-key"}, Client: client}
	report := runner.Run(context.Background(), Scenario{ID: "text-tool-test", Prompt: "지금 시간은?", MaxTurns: 2})
	if !report.Passed || report.FinalAnswer != "현재 시간을 도구로 확인했습니다." {
		t.Fatalf("unexpected report: %#v", report)
	}
	if len(report.ToolTrace) != 1 || report.ToolTrace[0].Name != "get_current_time" || !report.ToolTrace[0].RecoveredText {
		t.Fatalf("textual tool call was not recovered: %#v", report.ToolTrace)
	}
}

func TestRunnerRejectsTextualToolCallAfterFinalAnswerBoundary(t *testing.T) {
	responses := []string{
		`{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_read","type":"function","function":{"name":"read_buffered_source","arguments":"{\"source_id\":\"missing\"}"}}]}}]}`,
		`{"choices":[{"message":{"role":"assistant","content":"<tool_call><function=search_web><parameter=query>more searching</parameter></function></tool_call>"}}]}`,
		`{"choices":[{"message":{"role":"assistant","content":"확보한 근거가 부족하여 확인할 수 없었습니다."}}]}`,
	}
	requestCount := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		index := requestCount
		requestCount++
		if index >= len(responses) {
			t.Fatalf("unexpected extra LLM request")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(responses[index])), Request: r,
		}, nil
	})}
	runner := Runner{Config: Config{Endpoint: "http://example.invalid", Model: "test-model", APIKey: "test-key"}, Client: client}
	report := runner.Run(context.Background(), Scenario{ID: "final-boundary", Prompt: "lookup", MaxTurns: 2})
	if !report.Passed || report.FinalAnswer == "" {
		t.Fatalf("protocol correction did not recover a final answer: %#v", report)
	}
	if len(report.ToolTrace) != 1 || report.ToolTrace[0].Name != "read_buffered_source" {
		t.Fatalf("tool intent after final boundary was executed: %#v", report.ToolTrace)
	}
}

func TestRunnerDoesNotWriteAPIKeyToReport(t *testing.T) {
	const apiKey = "secret-eval-key"
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer "+apiKey {
			t.Errorf("authorization header missing")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"final answer"}}]}`)),
			Request:    r,
		}, nil
	})}

	runner := Runner{Config: Config{Endpoint: "http://example.invalid", Model: "test-model", APIKey: apiKey}, Client: client}
	report := runner.Run(context.Background(), Scenario{ID: "secret-test", Prompt: "hello", MaxTurns: 1})
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), apiKey) {
		t.Fatal("API key leaked into evaluation report")
	}
	if !report.Passed || report.FinalAnswer != "final answer" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestRunnerRecordsRoundUsageAndRequestOverhead(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"42"}}],
				"usage":{"prompt_tokens":12,"completion_tokens":3,"total_tokens":15,"prompt_tokens_details":{"cached_tokens":4},"completion_tokens_details":{"reasoning_tokens":2}}
			}`)),
			Request: r,
		}, nil
	})}
	toolsOff := false
	runner := Runner{Config: Config{Endpoint: "http://example.invalid", Model: "test-model", APIKey: "test-key"}, Client: client}
	report := runner.Run(context.Background(), Scenario{
		ID: "metrics", Prompt: "17+25", EnableTools: &toolsOff, ReasoningEffort: "off", MaxTurns: 1,
	})
	if !report.Passed || report.Usage.TotalTokens != 15 || report.Usage.CachedTokens != 4 || report.Usage.ReasoningTokens != 2 {
		t.Fatalf("usage was not aggregated: %#v", report)
	}
	if len(report.RoundMetrics) != 1 || report.RoundMetrics[0].RequestBytes == 0 || report.RoundMetrics[0].FinishReason != "stop" {
		t.Fatalf("round overhead was not recorded: %#v", report.RoundMetrics)
	}
}

func TestFutureDateMentionsRejectsUnsupportedFutureDate(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.FixedZone("KST", 9*60*60))
	mentions := futureDateMentions("7월 31일과 8월 3일은 지났지만 8월 31일은 미래입니다.", now)
	if len(mentions) != 1 || mentions[0] != "8월 31일" {
		t.Fatalf("unexpected future-date detection: %#v", mentions)
	}
	if got := futureDateMentions("2025년 12월 1일", now); len(got) != 0 {
		t.Fatalf("past year was classified as future: %#v", got)
	}
}

func TestAuthoritativeSourceClassification(t *testing.T) {
	for _, rawURL := range []string{"https://go.dev/doc/go1.25", "https://www.reuters.com/world/", "https://www.nia.or.kr/report"} {
		if !isAuthoritativeSource(rawURL) {
			t.Fatalf("expected authoritative source: %s", rawURL)
		}
	}
	for _, rawURL := range []string{"https://example.tistory.com/post", "https://seo-news.example/article"} {
		if isAuthoritativeSource(rawURL) {
			t.Fatalf("unexpected authoritative source: %s", rawURL)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
