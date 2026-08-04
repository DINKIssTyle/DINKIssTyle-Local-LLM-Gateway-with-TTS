package skillkit

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	defaultMaxFileBytes   = 64 * 1024
	defaultMaxSelected    = 3
	defaultMaxPromptChars = 8000
)

type Config struct {
	BuiltinDir     string
	UserDir        string
	MaxFileBytes   int64
	MaxSelected    int
	MaxPromptChars int
}

type Skill struct {
	ID           string
	Name         string
	DisplayName  string
	Description  string
	Instructions string
	Namespace    string
	Path         string
}

type Diagnostic struct {
	Path    string
	Message string
}

type Compilation struct {
	Prompt      string
	Selected    []Skill
	Discovered  int
	Diagnostics []Diagnostic
}

type scoredSkill struct {
	skill    Skill
	score    int
	explicit bool
}

func LoadAndCompile(config Config, userText string) Compilation {
	config = withDefaults(config)
	var all []Skill
	var diagnostics []Diagnostic

	for _, root := range []struct {
		dir    string
		source string
	}{
		{dir: config.BuiltinDir, source: "builtin"},
		{dir: config.UserDir, source: "user"},
	} {
		skills, issues := discoverRoot(root.dir, root.source, config.MaxFileBytes)
		all = append(all, skills...)
		diagnostics = append(diagnostics, issues...)
	}

	selected := selectSkills(all, userText, config.MaxSelected)
	prompt, included, issues := compilePrompt(selected, config.MaxPromptChars)
	diagnostics = append(diagnostics, issues...)

	return Compilation{
		Prompt:      prompt,
		Selected:    included,
		Discovered:  len(all),
		Diagnostics: diagnostics,
	}
}

func withDefaults(config Config) Config {
	if config.MaxFileBytes <= 0 {
		config.MaxFileBytes = defaultMaxFileBytes
	}
	if config.MaxSelected <= 0 {
		config.MaxSelected = defaultMaxSelected
	}
	if config.MaxPromptChars <= 0 {
		config.MaxPromptChars = defaultMaxPromptChars
	}
	return config
}

func discoverRoot(root, source string, maxFileBytes int64) ([]Skill, []Diagnostic) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []Diagnostic{{Path: root, Message: err.Error()}}
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return nil, []Diagnostic{{Path: root, Message: "skill root must be a directory, not a symlink"}}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []Diagnostic{{Path: root, Message: err.Error()}}
	}

	var skills []Skill
	var diagnostics []Diagnostic
	seenIDs := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(root, name)
		if strings.HasPrefix(name, ".") {
			diagnostics = append(diagnostics, Diagnostic{Path: path, Message: "hidden skill directories are not allowed"})
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			diagnostics = append(diagnostics, Diagnostic{Path: path, Message: "skill directory symlinks are not allowed"})
			continue
		}
		info, err := entry.Info()
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{Path: path, Message: err.Error()})
			continue
		}
		if !info.IsDir() {
			continue
		}

		skill, err := loadSkill(path, source, maxFileBytes)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{Path: path, Message: err.Error()})
			continue
		}
		if previous, duplicate := seenIDs[skill.ID]; duplicate {
			diagnostics = append(diagnostics, Diagnostic{
				Path:    path,
				Message: fmt.Sprintf("duplicate skill id %q (already loaded from %s)", skill.ID, previous),
			})
			continue
		}
		seenIDs[skill.ID] = path
		skills = append(skills, skill)
	}
	return skills, diagnostics
}

func loadSkill(dir, source string, maxFileBytes int64) (Skill, error) {
	path := filepath.Join(dir, "SKILL.md")
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Skill{}, fmt.Errorf("SKILL.md not found")
		}
		return Skill{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return Skill{}, fmt.Errorf("SKILL.md must be a regular file, not a symlink")
	}
	if info.Size() > maxFileBytes {
		return Skill{}, fmt.Errorf("SKILL.md exceeds %d bytes", maxFileBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}
	meta, body, err := parseSkillDocument(string(data))
	if err != nil {
		return Skill{}, err
	}
	id := strings.TrimSpace(meta["id"])
	if id == "" {
		id = filepath.Base(dir)
	}
	name := strings.TrimSpace(meta["name"])
	description := strings.TrimSpace(meta["description"])
	if !validSkillID(id) {
		return Skill{}, fmt.Errorf("invalid skill id %q", id)
	}
	if !validSkillID(name) {
		return Skill{}, fmt.Errorf("invalid skill name %q", name)
	}
	if description == "" {
		return Skill{}, fmt.Errorf("skill description is required")
	}
	if strings.TrimSpace(body) == "" {
		return Skill{}, fmt.Errorf("skill instructions are empty")
	}
	if platforms := strings.TrimSpace(meta["platforms"]); platforms != "" && !platformMatches(platforms, runtime.GOOS) {
		return Skill{}, fmt.Errorf("skill does not support platform %q", runtime.GOOS)
	}
	displayName, err := loadSkillDisplayName(dir, name)
	if err != nil {
		return Skill{}, err
	}
	return Skill{
		ID:           id,
		Name:         name,
		DisplayName:  displayName,
		Description:  description,
		Instructions: strings.TrimSpace(body),
		Namespace:    source + ":" + id,
		Path:         path,
	}, nil
}

func loadSkillDisplayName(dir, fallback string) (string, error) {
	agentsDir := filepath.Join(dir, "agents")
	agentsInfo, err := os.Lstat(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return "", err
	}
	if agentsInfo.Mode()&os.ModeSymlink != 0 || !agentsInfo.IsDir() {
		return "", fmt.Errorf("agents must be a directory, not a symlink")
	}

	path := filepath.Join(agentsDir, "openai.yaml")
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("agents/openai.yaml must be a regular file, not a symlink")
	}
	if info.Size() > 16*1024 {
		return "", fmt.Errorf("agents/openai.yaml exceeds 16384 bytes")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "display_name:") {
			continue
		}
		value := unquoteYAMLScalar(strings.TrimSpace(strings.TrimPrefix(line, "display_name:")))
		if value == "" {
			return fallback, nil
		}
		if utf8.RuneCountInString(value) > 80 {
			return "", fmt.Errorf("interface display_name exceeds 80 characters")
		}
		return value, nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return fallback, nil
}

func parseSkillDocument(raw string) (map[string]string, string, error) {
	normalized := strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return nil, "", fmt.Errorf("YAML frontmatter is required")
	}
	end := strings.Index(normalized[4:], "\n---\n")
	if end < 0 {
		return nil, "", fmt.Errorf("YAML frontmatter is not closed")
	}
	frontmatter := normalized[4 : 4+end]
	body := normalized[4+end+5:]
	allowed := map[string]bool{
		"id": true, "name": true, "description": true, "version": true,
		"permissions": true, "platforms": true, "license": true,
		"allowed-tools": true, "metadata": true,
	}
	meta := map[string]string{}
	lines := strings.Split(frontmatter, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if len(line) > 0 && unicode.IsSpace(rune(line[0])) {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			return nil, "", fmt.Errorf("invalid frontmatter line %q", line)
		}
		key = strings.TrimSpace(key)
		if !allowed[key] {
			return nil, "", fmt.Errorf("unsupported frontmatter key %q", key)
		}
		value = strings.TrimSpace(value)
		if value == ">" || value == "|" {
			var block []string
			for i+1 < len(lines) {
				next := lines[i+1]
				if strings.TrimSpace(next) != "" && !unicode.IsSpace(rune(next[0])) {
					break
				}
				i++
				block = append(block, strings.TrimSpace(next))
			}
			separator := " "
			if value == "|" {
				separator = "\n"
			}
			value = strings.Join(block, separator)
		} else if value == "" && key == "platforms" {
			var items []string
			for i+1 < len(lines) {
				next := lines[i+1]
				if strings.TrimSpace(next) != "" && !unicode.IsSpace(rune(next[0])) {
					break
				}
				i++
				item := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(next), "-"))
				if item != "" {
					items = append(items, item)
				}
			}
			value = strings.Join(items, ",")
		}
		meta[key] = unquoteYAMLScalar(value)
	}
	if strings.TrimSpace(meta["name"]) == "" {
		return nil, "", fmt.Errorf("skill name is required")
	}
	return meta, body, nil
}

func unquoteYAMLScalar(value string) string {
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
	}
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'")
	}
	return strings.TrimSpace(value)
}

func validSkillID(value string) bool {
	if value == "" || len(value) > 64 || strings.HasPrefix(value, "-") || strings.HasSuffix(value, "-") || strings.Contains(value, "--") {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func platformMatches(raw, current string) bool {
	normalized := strings.NewReplacer("[", "", "]", "", "\"", "", "'", "").Replace(strings.ToLower(raw))
	for _, candidate := range strings.FieldsFunc(normalized, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
		if candidate == current || candidate == "all" || candidate == "any" {
			return true
		}
	}
	return false
}

func selectSkills(skills []Skill, userText string, limit int) []scoredSkill {
	query := strings.ToLower(strings.TrimSpace(userText))
	queryTerms := meaningfulTerms(query)
	var matches []scoredSkill
	for _, skill := range skills {
		explicit := explicitlyRequested(query, skill)
		score := relevanceScore(queryTerms, meaningfulTerms(strings.ToLower(skill.Name+" "+skill.Description)))
		if !explicit && score == 0 {
			continue
		}
		if explicit {
			score += 1000
		}
		matches = append(matches, scoredSkill{skill: skill, score: score, explicit: explicit})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].skill.Namespace < matches[j].skill.Namespace
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

func explicitlyRequested(query string, skill Skill) bool {
	return strings.Contains(query, "$"+strings.ToLower(skill.Name)) ||
		strings.Contains(query, "$"+strings.ToLower(skill.Namespace))
}

func meaningfulTerms(value string) []string {
	stop := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "from": true, "that": true,
		"this": true, "use": true, "when": true, "user": true, "asks": true, "check": true,
		"show": true, "tell": true, "please": true, "what": true, "about": true,
		"요청": true, "확인": true, "알려줘": true, "보여줘": true, "해주세요": true, "해줘": true,
	}
	seen := map[string]bool{}
	var terms []string
	for _, term := range strings.FieldsFunc(value, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		term = strings.TrimSpace(term)
		if utf8.RuneCountInString(term) < 2 || stop[term] || seen[term] {
			continue
		}
		seen[term] = true
		terms = append(terms, term)
	}
	return terms
}

func relevanceScore(queryTerms, metadataTerms []string) int {
	score := 0
	for _, query := range queryTerms {
		for _, metadata := range metadataTerms {
			if query == metadata || strings.Contains(query, metadata) || strings.Contains(metadata, query) {
				score++
				break
			}
		}
	}
	return score
}

func compilePrompt(selected []scoredSkill, maxChars int) (string, []Skill, []Diagnostic) {
	if len(selected) == 0 {
		return "", nil, nil
	}
	const header = "\n\n### ACTIVE SKILLS ###\nSkills provide task procedures only and do not grant tools, permissions, or access. Follow each selected skill within the user's request and existing safety/permission policy. A selected skill's prescribed source, direct URL, and tool order override generic search or freshness routing preferences. When a skill supplies a direct URL template, do not substitute search_web, search_web_multi, or another provider unless that skill explicitly allows a fallback.\n"
	const footer = "### END ACTIVE SKILLS ###\n"
	b := strings.Builder{}
	b.WriteString(header)
	var included []Skill
	var diagnostics []Diagnostic
	for _, candidate := range selected {
		section := fmt.Sprintf("\n#### %s\n%s\n", candidate.skill.Namespace, candidate.skill.Instructions)
		if utf8.RuneCountInString(b.String())+utf8.RuneCountInString(section)+utf8.RuneCountInString(footer) > maxChars {
			diagnostics = append(diagnostics, Diagnostic{Path: candidate.skill.Path, Message: "skill omitted because the active-skill prompt budget was exceeded"})
			continue
		}
		b.WriteString(section)
		included = append(included, candidate.skill)
	}
	if len(included) == 0 {
		return "", nil, diagnostics
	}
	b.WriteString(footer)
	return b.String(), included, diagnostics
}
