package skillkit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeTestSkill(t *testing.T, root, dir, frontmatter, body string) string {
	t.Helper()
	path := filepath.Join(root, dir)
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	document := "---\n" + frontmatter + "\n---\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(document), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestComputerSkillFitsBundledBudgetAndCompactDiscovery(t *testing.T) {
	builtin := filepath.Join("..", "..", "bundle", "skills", "builtin")
	for _, query := range []string{"$computer-use 앱 전환", "사파리 앱 실행해줘", "LM Studio 에서 다운로드 중인 작업이 얼마나 진행되었는지 확인해봐", "편집기의 작업 진행 상태를 확인해줘", "Check the app status"} {
		result := LoadAndCompile(Config{BuiltinDir: builtin, CompactDiscovery: true}, query)
		if len(result.Diagnostics) > 0 {
			t.Fatalf("invalid computer skill: %#v", result.Diagnostics)
		}
		found := false
		for _, skill := range result.Available {
			if skill.Name == "computer-use" {
				found = true
			}
		}
		if !found || !strings.Contains(result.Prompt, "observation_id") {
			t.Fatalf("computer workflow missing for %q", query)
		}
	}
}

func TestLoadAndCompileSelectsRelevantKoreanWeatherSkill(t *testing.T) {
	builtin := t.TempDir()
	writeTestSkill(t, builtin, "msn-weather-current",
		"name: msn-weather-current\ndescription: Use for 현재 날씨, 기온, 습도, 강수, 바람 requests.",
		"Open the MSN Weather page and inspect 현재 날씨.")

	result := LoadAndCompile(Config{BuiltinDir: builtin}, "서울 날씨를 알려줘")
	if len(result.Available) != 1 || result.Available[0].Namespace != "builtin:msn-weather-current" {
		t.Fatalf("unexpected selection: %#v", result.Available)
	}
	for _, expected := range []string{"### AVAILABLE SKILL PROCEDURES ###", "builtin:msn-weather-current", "inspect 현재 날씨"} {
		if !strings.Contains(result.Prompt, expected) {
			t.Fatalf("compiled prompt missing %q: %s", expected, result.Prompt)
		}
	}

	unrelated := LoadAndCompile(Config{BuiltinDir: builtin}, "고마워요")
	if len(unrelated.Selected) != 0 || unrelated.Prompt == "" {
		t.Fatalf("unrelated request selected skills: %#v", unrelated.Selected)
	}
}

func TestLoadAndCompileSupportsExplicitInvocation(t *testing.T) {
	user := t.TempDir()
	writeTestSkill(t, user, "special-helper",
		"name: special-helper\ndescription: Handles a narrow workflow.",
		"Follow the narrow workflow.")

	result := LoadAndCompile(Config{UserDir: user}, "Use $special-helper now")
	if len(result.Selected) != 1 || result.Selected[0].Namespace != "user:special-helper" {
		t.Fatalf("explicit skill was not selected: %#v", result.Available)
	}
}

func TestLoadAndCompileKeepsBuiltinAndUserNamespacesSeparate(t *testing.T) {
	builtin := t.TempDir()
	user := t.TempDir()
	frontmatter := "name: weather-helper\ndescription: Handles weather requests."
	writeTestSkill(t, builtin, "weather-helper", frontmatter, "Builtin weather rules.")
	writeTestSkill(t, user, "weather-helper", frontmatter, "User weather rules.")

	result := LoadAndCompile(Config{BuiltinDir: builtin, UserDir: user}, "weather please")
	if len(result.Available) != 2 {
		t.Fatalf("expected both namespaced skills, got %#v", result.Available)
	}
	if result.Available[0].Namespace == result.Available[1].Namespace {
		t.Fatalf("namespaces collided: %#v", result.Available)
	}
}

func TestLoadAndCompileRejectsInvalidAndOversizedSkills(t *testing.T) {
	builtin := t.TempDir()
	writeTestSkill(t, builtin, "invalid", "name: Invalid Name\ndescription: bad", "Bad.")
	writeTestSkill(t, builtin, "too-large", "name: too-large\ndescription: large", strings.Repeat("x", 200))

	result := LoadAndCompile(Config{BuiltinDir: builtin, MaxFileBytes: 100}, "large")
	if result.Discovered != 0 || len(result.Diagnostics) != 2 {
		t.Fatalf("expected two rejected skills, got discovered=%d diagnostics=%#v", result.Discovered, result.Diagnostics)
	}
}

func TestLoadAndCompileHonorsPlatformAndPromptBudgets(t *testing.T) {
	builtin := t.TempDir()
	otherPlatform := "windows"
	if runtime.GOOS == "windows" {
		otherPlatform = "darwin"
	}
	writeTestSkill(t, builtin, "wrong-platform",
		"name: wrong-platform\ndescription: platform request\nplatforms: ["+otherPlatform+"]",
		"Wrong platform instructions.")
	writeTestSkill(t, builtin, "large-helper",
		"name: large-helper\ndescription: budget request",
		strings.Repeat("instruction ", 100))

	result := LoadAndCompile(Config{BuiltinDir: builtin, MaxPromptChars: 250}, "platform budget request")
	if len(result.Available) != 0 {
		t.Fatalf("skills should not fit or match the platform: %#v", result.Available)
	}
	if len(result.Diagnostics) < 2 {
		t.Fatalf("expected platform and budget diagnostics: %#v", result.Diagnostics)
	}
}

func TestBundledMSNWeatherSkillMatchesKoreanRequest(t *testing.T) {
	builtin := filepath.Join("..", "..", "bundle", "skills", "builtin")
	if _, err := os.Stat(builtin); err != nil {
		t.Skipf("bundled skills are unavailable: %v", err)
	}
	for _, request := range []string{"요청한 지역의 현재 날씨를 알려주세요", "날씨 스킬을 이용하세요"} {
		result := LoadAndCompile(Config{BuiltinDir: builtin}, request)
		var weather *Skill
		for i := range result.Available {
			if result.Available[i].Namespace == "builtin:msn-weather-current" {
				weather = &result.Available[i]
				break
			}
		}
		if weather == nil {
			t.Fatalf("bundled weather skill was not selected for %q: %#v diagnostics=%#v", request, result.Available, result.Diagnostics)
		}
		if weather.DisplayName != "MSN 현재 날씨" {
			t.Fatalf("unexpected display name: %q", weather.DisplayName)
		}
		if !strings.Contains(result.Prompt, "https://www.msn.com/ko-kr/weather/forecast/in-지역,도시") {
			t.Fatalf("compiled weather instructions are incomplete: %s", result.Prompt)
		}
		for _, required := range []string{"first web tool call MUST be `read_web_page`", "instead of generic search routing"} {
			if !strings.Contains(result.Prompt, required) {
				t.Fatalf("compiled weather instructions omit route precedence %q: %s", required, result.Prompt)
			}
		}
	}
}

func TestLoadAndCompileRejectsSymlinkedSkillRoot(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "skills-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	result := LoadAndCompile(Config{UserDir: link}, "anything")
	if result.Discovered != 0 || len(result.Diagnostics) != 1 || !strings.Contains(result.Diagnostics[0].Message, "not a symlink") {
		t.Fatalf("symlinked root was not rejected: %#v", result)
	}
}

func TestLoadAndCompileParsesBlockPlatformList(t *testing.T) {
	builtin := t.TempDir()
	writeTestSkill(t, builtin, "platform-helper",
		"name: platform-helper\ndescription: platform-specific task\nplatforms:\n  - "+runtime.GOOS,
		"Use platform instructions.")
	result := LoadAndCompile(Config{BuiltinDir: builtin}, "platform-specific task")
	if len(result.Available) != 1 {
		t.Fatalf("block platform list was not accepted: %#v", result)
	}
}

func TestLoadAndCompileBundledPythonCalculatorSkill(t *testing.T) {
	// Locate repository bundle/skills/builtin directory
	repoRoot, err := filepath.Abs("../../bundle/skills/builtin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(repoRoot); os.IsNotExist(err) {
		t.Skip("bundled skills directory not found from test path")
	}

	result := LoadAndCompile(Config{BuiltinDir: repoRoot}, "오늘부터 100일 뒤 날짜 계산해줘")
	if len(result.Available) == 0 {
		t.Fatalf("bundled python-calculator was not selected for date calculation: %#v", result)
	}
	found := false
	for _, s := range result.Available {
		if s.Namespace == "builtin:python-calculator" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("python-calculator skill missing from selected: %#v", result.Available)
	}
}

func TestBundledWeatherDoesNotMatchGenericCurrentQuestions(t *testing.T) {
	builtin := filepath.Join("..", "..", "bundle", "skills", "builtin")
	for _, query := range []string{"현재 ai 계 최상순위는 어떤 모델?", "current AI models", "아이폰 정보", "현재 주식 정보"} {
		result := LoadAndCompile(Config{BuiltinDir: builtin}, query)
		for _, skill := range result.Selected {
			if skill.Name == "msn-weather-current" {
				t.Fatalf("weather matched %q", query)
			}
		}
	}
}

func TestBundledWikipediaSearchLoadsWithoutDiagnostics(t *testing.T) {
	builtin := filepath.Join("..", "..", "bundle", "skills", "builtin")
	result := LoadAndCompile(Config{BuiltinDir: builtin}, "$wikipedia-search 서울")
	if len(result.Diagnostics) > 0 {
		t.Fatalf("unexpected diagnostics loading builtin skills: %#v", result.Diagnostics)
	}
	var found bool
	for _, s := range result.Available {
		if s.Namespace == "builtin:wikipedia-search" {
			found = true
			if s.DisplayName != "위키백과 검색 (Wikipedia)" {
				t.Fatalf("unexpected display name: %q", s.DisplayName)
			}
			break
		}
	}
	if !found {
		t.Fatalf("wikipedia-search skill missing from available: %#v", result.Available)
	}
	if len(result.Selected) != 1 || result.Selected[0].Namespace != "builtin:wikipedia-search" {
		t.Fatalf("expected explicit $wikipedia-search selection, got %#v", result.Selected)
	}
}

func TestHintTrustedInformationSkillLoads(t *testing.T) {
	builtin := filepath.Join("..", "evenrealities", "skills")
	result := LoadAndCompile(Config{BuiltinDir: builtin}, "$trusted-information 제품의 공식 사양 확인")
	if len(result.Diagnostics) != 0 {
		t.Fatalf("invalid bundled skills: %+v", result.Diagnostics)
	}
	for _, skill := range result.Selected {
		if skill.Namespace == "builtin:trusted-information" {
			return
		}
	}
	t.Fatal("trusted-information skill not discoverable")
}

func TestUnavailableGoogleSkillIsExcludedEvenWhenExplicit(t *testing.T) {
	builtin := filepath.Join("..", "..", "bundle", "skills", "builtin")
	result := LoadAndCompile(Config{BuiltinDir: builtin, ExcludedNames: []string{"google-gmail", "google-drive"}}, "$google-gmail $google-calendar")
	if strings.Contains(result.Prompt, "gmail_search") || strings.Contains(result.Prompt, "google_drive_search") {
		t.Fatal("unavailable Google instructions entered prompt")
	}
	if len(result.Selected) != 1 || result.Selected[0].Name != "google-calendar" {
		t.Fatalf("available Calendar skill missing: %+v", result.Selected)
	}
}

func TestCurrentWeatherProcedureGetsPromptBudgetPriority(t *testing.T) {
	builtin := filepath.Join("..", "..", "bundle", "skills", "builtin")
	result := LoadAndCompile(Config{BuiltinDir: builtin, MaxPromptChars: 3000}, "지금 서울 날씨 어때?")
	if len(result.Available) == 0 || result.Available[0].Namespace != "builtin:msn-weather-current" {
		t.Fatalf("weather procedure was not prioritized under the prompt budget: %+v", result.Available)
	}
}

func TestDeclarativeUserSkillTriggersAndDiscoveryInCompactMode(t *testing.T) {
	user := t.TempDir()
	writeTestSkill(t, user, "stock-brief",
		"name: stock-brief\ndescription: Summarize stock prices.\ntriggers: [주가, stock price]\npriority: 5",
		"Check market quotes.")
	writeTestSkill(t, user, "always-helper",
		"name: always-helper\ndescription: Always available helper.\ndiscovery: always",
		"Always check context.")
	writeTestSkill(t, user, "quiet-helper",
		"name: quiet-helper\ndescription: Explicit only in compact mode.",
		"Only when requested.")

	matched := LoadAndCompile(Config{UserDir: user, CompactDiscovery: true}, "애플 주가 알려줘")
	if len(matched.Diagnostics) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", matched.Diagnostics)
	}
	if len(matched.Available) != 2 {
		t.Fatalf("expected stock-brief and always-helper, got %+v", matched.Available)
	}
	if matched.Available[0].Namespace != "user:stock-brief" || matched.Available[1].Namespace != "user:always-helper" {
		t.Fatalf("unexpected compact order: %+v", matched.Available)
	}

	unmatched := LoadAndCompile(Config{UserDir: user, CompactDiscovery: true}, "안녕하세요")
	if len(unmatched.Available) != 1 || unmatched.Available[0].Namespace != "user:always-helper" {
		t.Fatalf("expected only discovery=always skill, got %+v", unmatched.Available)
	}
}

func TestRequiresToolsExcludesSkillsMissingTools(t *testing.T) {
	user := t.TempDir()
	writeTestSkill(t, user, "needs-web",
		"name: needs-web\ndescription: Uses web reader.\nrequires-tools:\n  - read_web_page\n  - search_web",
		"Read and search the web.")
	writeTestSkill(t, user, "no-tools",
		"name: no-tools\ndescription: Pure reasoning skill.",
		"Think carefully.")

	missing := LoadAndCompile(Config{UserDir: user, AvailableTools: []string{"read_web_page"}}, "$needs-web $no-tools")
	if len(missing.Available) != 1 || missing.Available[0].Name != "no-tools" {
		t.Fatalf("skill with missing required tool should be excluded: %+v", missing.Available)
	}
	if len(missing.Selected) != 1 || missing.Selected[0].Name != "no-tools" {
		t.Fatalf("excluded explicit skill should not appear in Selected: %+v", missing.Selected)
	}

	satisfied := LoadAndCompile(Config{UserDir: user, AvailableTools: []string{"read_web_page", "search_web"}}, "$needs-web")
	if len(satisfied.Selected) != 1 || satisfied.Selected[0].Name != "needs-web" {
		t.Fatalf("skill with satisfied required tools should be selected: %+v", satisfied.Selected)
	}
}

func TestPriorityOrdersMatchedSkillsUnderBudget(t *testing.T) {
	user := t.TempDir()
	writeTestSkill(t, user, "alpha-low",
		"name: alpha-low\ndescription: Low priority.\ntriggers: [배포]\npriority: 1",
		"Low priority deploy steps.")
	writeTestSkill(t, user, "zeta-high",
		"name: zeta-high\ndescription: High priority.\ntriggers: [배포]\npriority: 10",
		"High priority deploy steps.")

	result := LoadAndCompile(Config{UserDir: user}, "배포 체크해줘")
	if len(result.Available) != 2 || result.Available[0].Name != "zeta-high" {
		t.Fatalf("expected higher priority skill first, got %+v", result.Available)
	}
}

func TestInvalidDiscoveryAndPriorityProduceDiagnostics(t *testing.T) {
	user := t.TempDir()
	writeTestSkill(t, user, "bad-discovery",
		"name: bad-discovery\ndescription: Bad discovery.\ndiscovery: sometimes",
		"Body.")
	writeTestSkill(t, user, "bad-priority",
		"name: bad-priority\ndescription: Bad priority.\npriority: high",
		"Body.")

	result := LoadAndCompile(Config{UserDir: user}, "test")
	if result.Discovered != 0 || len(result.Diagnostics) != 2 {
		t.Fatalf("expected 2 diagnostics for invalid metadata, got discovered=%d diagnostics=%+v", result.Discovered, result.Diagnostics)
	}
}
