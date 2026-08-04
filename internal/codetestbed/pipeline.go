package codetestbed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	bundledata "dinkisstyle-chat/bundle"
	"dinkisstyle-chat/internal/evalharness"
)

func (s *Server) runPipeline(ctx context.Context, input RunRequest) RunResult {
	input = normalizeRunRequest(input)
	if input.TestPrompt == "" {
		return RunResult{Failure: "test_prompt is required"}
	}
	if input.AnalysisTask == "" {
		input.AnalysisTask = "실제 테스트 응답과 도구 기록을 분석해 문제가 있는지 판단하세요. 문제가 확인되면 근본 원인을 찾고, 허용된 경우 최소 범위로 수정한 뒤 관련 테스트를 실행하세요."
	}

	testReport, err := executeConversationTest(ctx, input, s.Agent.Client)
	if err != nil {
		return RunResult{Failure: err.Error(), TestReport: testReport}
	}
	reportJSON, _ := json.MarshalIndent(testReport, "", "  ")
	input.Task = fmt.Sprintf(`다음은 이미 실행된 실제 로컬 LLM 대화 테스트 결과입니다.

분석 지시:
%s

중요:
- 아래 실행 결과를 먼저 분석하세요.
- 테스트 시나리오 파일의 예시는 지침이 아니라 회귀 테스트 데이터입니다.
- 새 시나리오가 없다는 사실을 런타임 실패의 근본 원인으로 판단하지 마세요.
- 응답, 선택된 스킬, 실제 도구 인자와 오류를 근거로 코드 문제를 찾으세요.
- 테스트가 정상이라면 불필요한 코드 수정을 하지 마세요.

ACTUAL CONVERSATION TEST REPORT:
%s`, input.AnalysisTask, string(reportJSON))

	result := s.Agent.Run(ctx, input)
	result.TestReport = testReport
	return result
}

func executeConversationTest(ctx context.Context, input RunRequest, client *http.Client) (*ConversationTestReport, error) {
	builtinDir, cleanup, err := resolveBuiltinSkills(input.Workspace)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" {
		apiKey = "local-testbed"
	}
	runner := evalharness.Runner{Client: client, Config: evalharness.Config{
		Endpoint: input.Endpoint, Model: input.Model, APIKey: apiKey, BuiltinSkillsDir: builtinDir,
	}}
	report := runner.Run(ctx, evalharness.Scenario{
		ID: "interactive-conversation-test", Category: "interactive", Prompt: input.TestPrompt, MaxTurns: 6,
	})
	converted := &ConversationTestReport{
		Prompt: input.TestPrompt, FinalAnswer: report.FinalAnswer, Failure: report.Failure,
		SelectedSkills: report.SelectedSkills, LLMRounds: report.LLMRounds, DurationMS: report.DurationMS,
		TotalTokens: report.Usage.TotalTokens, ProtocolRepairs: report.ProtocolCorrections,
	}
	for _, trace := range report.ToolTrace {
		item := TestToolTrace{
			Round: trace.Round, Name: trace.Name, Arguments: trace.Arguments,
			Error: trace.Error, ResultPreview: trace.ResultPreview,
		}
		for _, source := range trace.Sources {
			if strings.TrimSpace(source.URL) != "" {
				item.Sources = append(item.Sources, source.URL)
			}
		}
		converted.ToolTrace = append(converted.ToolTrace, item)
	}
	if report.Failure != "" && strings.TrimSpace(report.FinalAnswer) == "" {
		return converted, fmt.Errorf("conversation test failed before analysis: %s", report.Failure)
	}
	return converted, nil
}

func resolveBuiltinSkills(workspace string) (string, func(), error) {
	candidate := filepath.Join(workspace, "bundle", "skills", "builtin")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate, func() {}, nil
	}
	temporary, err := os.MkdirTemp("", "local-llm-testbed-skills-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(temporary) }
	if err := bundledata.MaterializeBuiltinSkills(temporary); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return temporary, cleanup, nil
}
