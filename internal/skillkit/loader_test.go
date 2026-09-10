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
		if len(result.Available) == 0 || result.Available[0].Namespace != "builtin:msn-weather-current" {
			t.Fatalf("bundled weather skill was not selected for %q: %#v diagnostics=%#v", request, result.Available, result.Diagnostics)
		}
		if result.Available[0].DisplayName != "MSN 현재 날씨" {
			t.Fatalf("unexpected display name: %q", result.Available[0].DisplayName)
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
