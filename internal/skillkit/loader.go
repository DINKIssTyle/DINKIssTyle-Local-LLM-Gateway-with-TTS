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
	defaultMaxPromptChars = 16000
)

type Config struct {
	ExcludedNames    []string
	AvailableTools   []string // When non-nil, skills declaring requires-tools are excluded if any required tool is missing.
	BuiltinDir       string
	UserDir          string
	MaxFileBytes     int64
	MaxSelected      int // Deprecated: relevance is decided by the LLM; MaxPromptChars bounds references.
	MaxPromptChars   int
	CompactDiscovery bool
}

type Skill struct {
	ID            string
	Name          string
	DisplayName   string
	Description   string
	Instructions  string
	Namespace     string
	Path          string
	RequiresTools []string
	Triggers      []string
	Discovery     string
	Priority      int
}

type Diagnostic struct {
	Path    string
	Message string
}

type Compilation struct {
	Prompt      string
	Selected    []Skill
	Available   []Skill
	Discovered  int
	Diagnostics []Diagnostic
}

type scoredSkill struct {
	skill    Skill
	score    int
	explicit bool
}

// Discover loads valid skills without applying a request's prompt budget.
func Discover(config Config) ([]Skill, []Diagnostic) {
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
	return all, diagnostics
}

func LoadAndCompile(config Config, userText string) Compilation {
	config = withDefaults(config)
	all, diagnostics := Discover(config)

	var availableTools map[string]bool
	if config.AvailableTools != nil {
		availableTools = make(map[string]bool, len(config.AvailableTools))
		for _, tool := range config.AvailableTools {
			if trimmed := strings.TrimSpace(tool); trimmed != "" {
				availableTools[trimmed] = true
			}
		}
	}

	filtered := all[:0]
	for _, skill := range all {
		excluded := false
		for _, name := range config.ExcludedNames {
			if skill.Name == name {
				excluded = true
				break
			}
		}
		if !excluded && availableTools != nil && !hasRequiredTools(skill.RequiresTools, availableTools) {
			excluded = true
		}
		if !excluded {
			filtered = append(filtered, skill)
		}
	}
	selected := selectSkills(filtered, userText, len(filtered))
	if config.CompactDiscovery {
		var relevant []scoredSkill
		for _, candidate := range selected {
			if candidate.explicit || candidate.score > 0 || candidate.skill.Discovery == "always" {
				relevant = append(relevant, candidate)
			}
		}
		selected = relevant
	}
	prompt, included, issues := compilePrompt(selected, config.MaxPromptChars)
	diagnostics = append(diagnostics, issues...)

	return Compilation{
		Prompt:      prompt,
		Selected:    explicitlySelected(included, userText),
		Available:   included,
		Discovered:  len(all),
		Diagnostics: diagnostics,
	}
}

func hasRequiredTools(required []string, available map[string]bool) bool {
	for _, tool := range required {
		if !available[tool] {
			return false
		}
	}
	return true
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
		if name == ".DS_Store" && entry.Type().IsRegular() {
			continue
		}
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
	meta, metaLists, body, err := parseSkillDocument(string(data))
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
	discovery := strings.ToLower(strings.TrimSpace(meta["discovery"]))
	if discovery == "" {
		discovery = "on-trigger"
	}
	if discovery != "on-trigger" && discovery != "always" {
		return Skill{}, fmt.Errorf("invalid skill discovery %q", discovery)
	}
	priority := 0
	if rawPriority := strings.TrimSpace(meta["priority"]); rawPriority != "" {
		parsedPriority, err := strconv.Atoi(rawPriority)
		if err != nil {
			return Skill{}, fmt.Errorf("invalid skill priority %q", rawPriority)
		}
		priority = parsedPriority
	}
	displayName, err := loadSkillDisplayName(dir, name)
	if err != nil {
		return Skill{}, err
	}
	return Skill{
		ID:            id,
		Name:          name,
		DisplayName:   displayName,
		Description:   description,
		Instructions:  strings.TrimSpace(body),
		Namespace:     source + ":" + id,
		Path:          path,
		RequiresTools: metaLists["requires-tools"],
		Triggers:      metaLists["triggers"],
		Discovery:     discovery,
		Priority:      priority,
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

func parseSkillDocument(raw string) (map[string]string, map[string][]string, string, error) {
	normalized := strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return nil, nil, "", fmt.Errorf("YAML frontmatter is required")
	}
	end := strings.Index(normalized[4:], "\n---\n")
	if end < 0 {
		return nil, nil, "", fmt.Errorf("YAML frontmatter is not closed")
	}
	frontmatter := normalized[4 : 4+end]
	body := normalized[4+end+5:]
	allowed := map[string]bool{
		"id": true, "name": true, "description": true, "version": true,
		"permissions": true, "platforms": true, "license": true,
		"allowed-tools": true, "metadata": true,
		"requires-tools": true, "triggers": true, "discovery": true, "priority": true,
	}
	listKeys := map[string]bool{
		"platforms":      true,
		"requires-tools": true,
		"triggers":       true,
	}
	meta := map[string]string{}
	metaLists := map[string][]string{}
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
			return nil, nil, "", fmt.Errorf("invalid frontmatter line %q", line)
		}
		key = strings.TrimSpace(key)
		if !allowed[key] {
			return nil, nil, "", fmt.Errorf("unsupported frontmatter key %q", key)
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
			meta[key] = unquoteYAMLScalar(value)
			continue
		}
		if listKeys[key] {
			var items []string
			if value == "" {
				for i+1 < len(lines) {
					next := lines[i+1]
					if strings.TrimSpace(next) != "" && !unicode.IsSpace(rune(next[0])) {
						break
					}
					i++
					trimmed := strings.TrimSpace(next)
					if trimmed == "" || strings.HasPrefix(trimmed, "#") {
						continue
					}
					item := unquoteYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")))
					if item != "" {
						items = append(items, item)
					}
				}
			} else {
				items = parseInlineYAMLList(value)
			}
			metaLists[key] = items
			meta[key] = strings.Join(items, ",")
			continue
		}
		meta[key] = unquoteYAMLScalar(value)
	}
	if strings.TrimSpace(meta["name"]) == "" {
		return nil, nil, "", fmt.Errorf("skill name is required")
	}
	return meta, metaLists, body, nil
}

func parseInlineYAMLList(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 2 && trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']' {
		trimmed = trimmed[1 : len(trimmed)-1]
	}
	if strings.TrimSpace(trimmed) == "" {
		return nil
	}
	var items []string
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		item := unquoteYAMLScalar(strings.TrimSpace(current.String()))
		if item != "" {
			items = append(items, item)
		}
		current.Reset()
	}
	for _, r := range trimmed {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if quote != 0 {
			current.WriteRune(r)
			if r == '\\' && quote == '"' {
				escaped = true
			} else if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'':
			quote = r
			current.WriteRune(r)
		case ',':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return items
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

// Offer procedures to the LLM; use declared triggers only for prioritization and compact discovery.
func selectSkills(skills []Skill, userText string, limit int) []scoredSkill {
	query := strings.ToLower(userText)
	var candidates []scoredSkill
	for _, skill := range skills {
		candidate := scoredSkill{skill: skill, explicit: explicitlyRequested(query, skill)}
		if matchesAnyTrigger(query, skill.Triggers) {
			score := 1 + skill.Priority
			if score < 1 {
				score = 1
			}
			candidate.score = score
		}
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].explicit != candidates[j].explicit {
			return candidates[i].explicit
		}
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].skill.Priority != candidates[j].skill.Priority {
			return candidates[i].skill.Priority > candidates[j].skill.Priority
		}
		return candidates[i].skill.Namespace < candidates[j].skill.Namespace
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates
}

func matchesAnyTrigger(query string, triggers []string) bool {
	for _, trigger := range triggers {
		cue := strings.ToLower(strings.TrimSpace(trigger))
		if cue != "" && strings.Contains(query, cue) {
			return true
		}
	}
	return false
}

func explicitlyRequested(query string, skill Skill) bool {
	for _, field := range strings.Fields(query) {
		name := strings.TrimRight(field, ",.;:!?()[]")
		if name == "$"+strings.ToLower(skill.Name) || name == "$"+strings.ToLower(skill.Namespace) {
			return true
		}
	}
	return false
}

func explicitlySelected(skills []Skill, userText string) []Skill {
	var selected []Skill
	for _, skill := range skills {
		if explicitlyRequested(strings.ToLower(userText), skill) {
			selected = append(selected, skill)
		}
	}
	return selected
}

func compilePrompt(selected []scoredSkill, maxChars int) (string, []Skill, []Diagnostic) {
	if len(selected) == 0 {
		return "", nil, nil
	}
	const header = "\n\n### AVAILABLE SKILL PROCEDURES ###\nDecide from the current user's intent whether any procedure below is relevant. These are available references, not automatically applied instructions. Ignore irrelevant procedures; a shared word is not sufficient relevance. An explicit $skill request takes priority. Skills do not grant tools or permissions. A skill ID such as builtin:wttr-weather is a procedure identifier, NOT a callable tool. Apply its instructions by calling actual tools from the provided tool definitions; never call a skill ID as a tool name. For a relevant procedure, follow its prescribed source, direct URL, and tool order instead of generic search routing. If no procedure is relevant, answer normally.\n"
	const footer = "### END AVAILABLE SKILL PROCEDURES ###\n"

	b := strings.Builder{}
	b.WriteString(header)
	var included []Skill
	var diagnostics []Diagnostic
	for _, candidate := range selected {
		section := fmt.Sprintf("\n#### %s\nDescription: %s\nExplicitly requested: %t\n%s\n", candidate.skill.Namespace, candidate.skill.Description, candidate.explicit, candidate.skill.Instructions)
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
