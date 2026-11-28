package toolruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"dinkisstyle-chat/internal/skillkit"
)

// SkillToolNames lists all registered skill-management tools.
var SkillToolNames = []string{
	"list_skills",
	"read_skill",
	"create_user_skill",
}

const (
	maxSkillReferenceFiles = 10
	maxSkillReferenceBytes = 64 * 1024
	maxSkillFileBytes      = 64 * 1024
)

var allowedSkillReferenceExtensions = map[string]bool{
	".md":   true,
	".txt":  true,
	".json": true,
	".yaml": true,
	".yml":  true,
}

func registerSkillTools(registry *Registry) {
	listSchema, _ := json.Marshal(map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"required":             []string{},
		"additionalProperties": false,
	})
	_ = registry.Register(Definition{
		Name:        "list_skills",
		Description: "List installed built-in and user skills along with the real tools currently available in this session. Use before planning or creating a new user skill to avoid duplicate IDs and verify real tool names.",
		InputSchema: listSchema,
		Metadata:    Metadata{Category: "skills", ReadOnly: true, ParallelSafe: true},
	}, func(ctx context.Context, execCtx ExecutionContext, _ json.RawMessage) (Result, error) {
		return handleListSkills(registry, execCtx)
	})

	readSchema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]any{
				"type":        "string",
				"description": "Skill namespace (e.g. user:my-skill or builtin:wttr-weather) or bare skill ID.",
			},
		},
		"required":             []string{"namespace"},
		"additionalProperties": false,
	})
	_ = registry.Register(Definition{
		Name:        "read_skill",
		Description: "Read an existing built-in or user skill's SKILL.md, agents/openai.yaml, and reference file list. Use when inspecting an existing skill as a template or before updating a user skill.",
		InputSchema: readSchema,
		Metadata:    Metadata{Category: "skills", ReadOnly: true, ParallelSafe: true},
	}, func(ctx context.Context, execCtx ExecutionContext, args json.RawMessage) (Result, error) {
		return handleReadSkill(execCtx, args)
	})

	createSchema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"user_approved": map[string]any{
				"type":        "boolean",
				"description": "Must be true ONLY after presenting the skill plan to the user in chat and receiving explicit user approval.",
			},
			"skill_id": map[string]any{
				"type":        "string",
				"description": "Lowercase hyphen-separated skill ID (e.g. exchange-rate-check). Max 64 chars, [a-z0-9-], no leading/trailing hyphen or double hyphen.",
			},
			"display_name": map[string]any{
				"type":        "string",
				"description": "Human-friendly display title for the skill picker UI (max 80 chars).",
			},
			"short_description": map[string]any{
				"type":        "string",
				"description": "Optional short one-line summary for agents/openai.yaml.",
			},
			"default_prompt": map[string]any{
				"type":        "string",
				"description": "Optional default prompt hint for agents/openai.yaml.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Skill description for SKILL.md frontmatter explaining when and why to use this skill.",
			},
			"instructions": map[string]any{
				"type":        "string",
				"description": "Markdown instruction body for SKILL.md (do NOT include YAML frontmatter delimiters).",
			},
			"requires_tools": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional list of real registered tool names required by this skill.",
			},
			"triggers": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional trigger keywords or phrases that prioritize this skill.",
			},
			"platforms": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional supported platforms (e.g. darwin, windows, linux, all).",
			},
			"discovery": map[string]any{
				"type":        "string",
				"enum":        []string{"on-trigger", "always"},
				"description": "Optional discovery mode: on-trigger (default) or always.",
			},
			"priority": map[string]any{
				"type":        "integer",
				"description": "Optional integer priority when multiple skills match.",
			},
			"references": map[string]any{
				"type":        "array",
				"description": "Optional reference documents to scaffold under references/.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"filename": map[string]any{
							"type":        "string",
							"description": "File name under references/ (e.g. guide.md). Allowed extensions: .md, .txt, .json, .yaml, .yml.",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "UTF-8 text content of the reference file.",
						},
					},
					"required": []string{"filename", "content"},
				},
			},
			"overwrite": map[string]any{
				"type":        "boolean",
				"description": "Set true ONLY when explicitly updating an existing user skill with user approval.",
			},
		},
		"required":             []string{"user_approved", "skill_id", "display_name", "description", "instructions"},
		"additionalProperties": false,
	})
	_ = registry.Register(Definition{
		Name:        "create_user_skill",
		Description: "Create or update a user skill with standard scaffolding (SKILL.md, agents/openai.yaml, and optional references/) in the application's user skills directory. NEVER call this before presenting the skill plan to the user in chat and receiving explicit user approval (user_approved=true).",
		InputSchema: createSchema,
		Metadata:    Metadata{Category: "skills", SideEffecting: true},
	}, func(ctx context.Context, execCtx ExecutionContext, args json.RawMessage) (Result, error) {
		return handleCreateUserSkill(registry, execCtx, args)
	})
}

func handleListSkills(registry *Registry, execCtx ExecutionContext) (Result, error) {
	skills, diagnostics := skillkit.Discover(skillkit.Config{
		BuiltinDir: execCtx.BuiltinSkillsDir,
		UserDir:    execCtx.UserSkillsDir,
	})
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Namespace < skills[j].Namespace
	})

	type skillEntry struct {
		Namespace     string   `json:"namespace"`
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		DisplayName   string   `json:"display_name"`
		Description   string   `json:"description"`
		RequiresTools []string `json:"requires_tools,omitempty"`
		Triggers      []string `json:"triggers,omitempty"`
		Editable      bool     `json:"editable"`
	}
	entries := make([]skillEntry, 0, len(skills))
	for _, s := range skills {
		entries = append(entries, skillEntry{
			Namespace:     s.Namespace,
			ID:            s.ID,
			Name:          s.Name,
			DisplayName:   s.DisplayName,
			Description:   s.Description,
			RequiresTools: s.RequiresTools,
			Triggers:      s.Triggers,
			Editable:      strings.HasPrefix(s.Namespace, "user:"),
		})
	}

	availableTools := make([]string, 0)
	for _, def := range registry.List(execCtx) {
		availableTools = append(availableTools, def.Name)
	}
	sort.Strings(availableTools)

	diagMessages := make([]string, 0, len(diagnostics))
	for _, d := range diagnostics {
		diagMessages = append(diagMessages, fmt.Sprintf("%s: %s", filepath.Base(d.Path), d.Message))
	}

	payload := map[string]any{
		"user_skills_dir": execCtx.UserSkillsDir,
		"skills":          entries,
		"available_tools": availableTools,
	}
	if len(diagMessages) > 0 {
		payload["diagnostics"] = diagMessages
	}
	raw, err := json.Marshal(payload)
	return Result{Content: string(raw)}, err
}

func handleReadSkill(execCtx ExecutionContext, args json.RawMessage) (Result, error) {
	var input struct {
		Namespace string `json:"namespace"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return Result{IsError: true}, err
	}
	target := strings.TrimSpace(strings.TrimPrefix(input.Namespace, "$"))
	if target == "" {
		return Result{IsError: true}, fmt.Errorf("namespace is required")
	}

	skills, _ := skillkit.Discover(skillkit.Config{
		BuiltinDir: execCtx.BuiltinSkillsDir,
		UserDir:    execCtx.UserSkillsDir,
	})
	var matched *skillkit.Skill
	for i := range skills {
		s := &skills[i]
		if strings.EqualFold(s.Namespace, target) {
			matched = s
			break
		}
		if !strings.Contains(target, ":") && strings.EqualFold(s.ID, target) {
			if matched == nil || strings.HasPrefix(s.Namespace, "user:") {
				matched = s
			}
		}
	}
	if matched == nil {
		return Result{IsError: true, Content: fmt.Sprintf("Skill %q was not found.", target)}, fmt.Errorf("skill not found: %s", target)
	}

	skillDir := filepath.Dir(matched.Path)
	skillMD, err := os.ReadFile(matched.Path)
	if err != nil {
		return Result{IsError: true}, err
	}

	var openAIYAML string
	openAIPath := filepath.Join(skillDir, "agents", "openai.yaml")
	if info, err := os.Lstat(openAIPath); err == nil && info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular() && info.Size() <= 16*1024 {
		if data, readErr := os.ReadFile(openAIPath); readErr == nil {
			openAIYAML = string(data)
		}
	}

	var refFiles []string
	refsDir := filepath.Join(skillDir, "references")
	if entries, err := os.ReadDir(refsDir); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") || entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
				continue
			}
			refFiles = append(refFiles, "references/"+entry.Name())
		}
		sort.Strings(refFiles)
	}

	payload := map[string]any{
		"namespace":    matched.Namespace,
		"id":           matched.ID,
		"display_name": matched.DisplayName,
		"editable":     strings.HasPrefix(matched.Namespace, "user:"),
		"skill_md":     string(skillMD),
		"openai_yaml":  openAIYAML,
		"references":   refFiles,
	}
	raw, err := json.Marshal(payload)
	return Result{Content: string(raw)}, err
}

type createUserSkillReference struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

type createUserSkillInput struct {
	UserApproved     bool                       `json:"user_approved"`
	SkillID          string                     `json:"skill_id"`
	DisplayName      string                     `json:"display_name"`
	ShortDescription string                     `json:"short_description"`
	DefaultPrompt    string                     `json:"default_prompt"`
	Description      string                     `json:"description"`
	Instructions     string                     `json:"instructions"`
	RequiresTools    []string                   `json:"requires_tools"`
	Triggers         []string                   `json:"triggers"`
	Platforms        []string                   `json:"platforms"`
	Discovery        string                     `json:"discovery"`
	Priority         int                        `json:"priority"`
	References       []createUserSkillReference `json:"references"`
	Overwrite        bool                       `json:"overwrite"`
}

func handleCreateUserSkill(registry *Registry, execCtx ExecutionContext, args json.RawMessage) (Result, error) {
	var input createUserSkillInput
	if err := json.Unmarshal(args, &input); err != nil {
		return Result{IsError: true}, err
	}
	if !input.UserApproved {
		msg := "User approval is required before creating a skill. Present the skill plan in chat, ask for explicit approval, and only call create_user_skill with user_approved=true after the user approves."
		return Result{IsError: true, Content: msg}, fmt.Errorf("user approval required")
	}

	userDir := strings.TrimSpace(execCtx.UserSkillsDir)
	if userDir == "" {
		return Result{IsError: true, Content: "User skills directory is not configured."}, fmt.Errorf("user skills directory is not configured")
	}
	skillID := strings.TrimSpace(input.SkillID)
	if !skillkit.ValidSkillID(skillID) {
		return Result{IsError: true, Content: fmt.Sprintf("Invalid skill_id %q: use lowercase letters, digits, and single hyphens (max 64 chars).", skillID)}, fmt.Errorf("invalid skill_id %q", skillID)
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" || utf8.RuneCountInString(displayName) > 80 {
		return Result{IsError: true, Content: "display_name is required and must be at most 80 characters."}, fmt.Errorf("invalid display_name")
	}
	description := strings.TrimSpace(input.Description)
	if description == "" {
		return Result{IsError: true, Content: "description is required."}, fmt.Errorf("description is required")
	}
	instructions := strings.TrimSpace(input.Instructions)
	if instructions == "" {
		return Result{IsError: true, Content: "instructions are required."}, fmt.Errorf("instructions are required")
	}
	if strings.HasPrefix(instructions, "---\n") || strings.HasPrefix(instructions, "---\r\n") {
		return Result{IsError: true, Content: "instructions must contain only the Markdown body without YAML frontmatter; frontmatter is generated automatically."}, fmt.Errorf("instructions must not contain frontmatter")
	}
	discovery := strings.ToLower(strings.TrimSpace(input.Discovery))
	if discovery != "" && discovery != "on-trigger" && discovery != "always" {
		return Result{IsError: true, Content: fmt.Sprintf("Invalid discovery %q: use \"on-trigger\" or \"always\".", discovery)}, fmt.Errorf("invalid discovery %q", discovery)
	}

	// Validate requires_tools against actually registered tools in this registry.
	registeredTools := registry.allRegisteredToolNames()
	availableTools := map[string]bool{}
	for _, def := range registry.List(execCtx) {
		availableTools[def.Name] = true
	}
	var cleanedRequiredTools []string
	var warnings []string
	seenTools := map[string]bool{}
	for _, rawTool := range input.RequiresTools {
		toolName := strings.TrimSpace(rawTool)
		if toolName == "" || seenTools[toolName] {
			continue
		}
		if !registeredTools[toolName] {
			return Result{
				IsError: true,
				Content: fmt.Sprintf("Unknown tool %q in requires_tools. Call list_skills to inspect the exact available tool names.", toolName),
			}, fmt.Errorf("unknown required tool %q", toolName)
		}
		seenTools[toolName] = true
		cleanedRequiredTools = append(cleanedRequiredTools, toolName)
		if !availableTools[toolName] {
			warnings = append(warnings, fmt.Sprintf("Required tool %q is currently disabled or unavailable for this user.", toolName))
		}
	}

	// Validate references before touching disk.
	if len(input.References) > maxSkillReferenceFiles {
		return Result{IsError: true, Content: fmt.Sprintf("Too many reference files (max %d).", maxSkillReferenceFiles)}, fmt.Errorf("too many reference files")
	}
	seenRefs := map[string]bool{}
	cleanedRefs := make([]createUserSkillReference, 0, len(input.References))
	for _, ref := range input.References {
		name, err := sanitizeReferenceFilename(ref.Filename)
		if err != nil {
			return Result{IsError: true, Content: err.Error()}, err
		}
		if seenRefs[name] {
			return Result{IsError: true, Content: fmt.Sprintf("Duplicate reference filename %q.", name)}, fmt.Errorf("duplicate reference filename %q", name)
		}
		if int64(len(ref.Content)) > maxSkillReferenceBytes {
			return Result{IsError: true, Content: fmt.Sprintf("Reference file %q exceeds %d bytes.", name, maxSkillReferenceBytes)}, fmt.Errorf("reference file too large")
		}
		seenRefs[name] = true
		cleanedRefs = append(cleanedRefs, createUserSkillReference{Filename: name, Content: ref.Content})
	}

	if err := os.MkdirAll(userDir, 0755); err != nil {
		return Result{IsError: true}, fmt.Errorf("prepare user skills directory: %w", err)
	}
	absUserDir, err := filepath.Abs(userDir)
	if err != nil {
		return Result{IsError: true}, err
	}
	targetDir := filepath.Join(absUserDir, skillID)
	rel, err := filepath.Rel(absUserDir, targetDir)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || strings.Contains(rel, string(filepath.Separator)) {
		return Result{IsError: true, Content: "Invalid target skill directory."}, fmt.Errorf("invalid target skill directory")
	}

	alreadyExists := false
	if info, statErr := os.Lstat(targetDir); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return Result{IsError: true, Content: "Target skill directory is a symlink and cannot be overwritten."}, fmt.Errorf("target skill directory is a symlink")
		}
		if !input.Overwrite {
			return Result{
				IsError: true,
				Content: fmt.Sprintf("User skill user:%s already exists. Ask the user if they want to overwrite it and pass overwrite=true.", skillID),
			}, fmt.Errorf("skill user:%s already exists", skillID)
		}
		alreadyExists = true
	} else if !os.IsNotExist(statErr) {
		return Result{IsError: true}, statErr
	}

	// Build scaffolding in an isolated staging directory under userDir so validation
	// runs before committing and rename stays atomic on the same filesystem.
	stagingParent, err := os.MkdirTemp(absUserDir, ".staging-skill-*")
	if err != nil {
		return Result{IsError: true}, fmt.Errorf("create staging directory: %w", err)
	}
	defer os.RemoveAll(stagingParent)

	stagedSkillDir := filepath.Join(stagingParent, skillID)
	if err := os.MkdirAll(filepath.Join(stagedSkillDir, "agents"), 0755); err != nil {
		return Result{IsError: true}, err
	}

	skillMDContent := renderSkillMarkdown(skillID, description, cleanedRequiredTools, input.Triggers, input.Platforms, discovery, input.Priority, instructions)
	if int64(len(skillMDContent)) > maxSkillFileBytes {
		return Result{IsError: true, Content: fmt.Sprintf("Generated SKILL.md exceeds %d bytes.", maxSkillFileBytes)}, fmt.Errorf("SKILL.md too large")
	}
	if err := os.WriteFile(filepath.Join(stagedSkillDir, "SKILL.md"), []byte(skillMDContent), 0644); err != nil {
		return Result{IsError: true}, err
	}

	openAIYAMLContent := renderOpenAIYAML(skillID, displayName, input.ShortDescription, input.DefaultPrompt, description)
	if err := os.WriteFile(filepath.Join(stagedSkillDir, "agents", "openai.yaml"), []byte(openAIYAMLContent), 0644); err != nil {
		return Result{IsError: true}, err
	}

	createdFiles := []string{"SKILL.md", "agents/openai.yaml"}
	if len(cleanedRefs) > 0 {
		refsDir := filepath.Join(stagedSkillDir, "references")
		if err := os.MkdirAll(refsDir, 0755); err != nil {
			return Result{IsError: true}, err
		}
		for _, ref := range cleanedRefs {
			refPath := filepath.Join(refsDir, ref.Filename)
			if err := os.WriteFile(refPath, []byte(ref.Content), 0644); err != nil {
				return Result{IsError: true}, err
			}
			createdFiles = append(createdFiles, "references/"+ref.Filename)
		}
	}

	validated, err := skillkit.ValidateSkillDir(stagedSkillDir, "user", maxSkillFileBytes)
	if err != nil {
		return Result{
			IsError: true,
			Content: fmt.Sprintf("Generated skill failed validation and was not saved: %v", err),
		}, fmt.Errorf("skill validation failed: %w", err)
	}

	// Atomically move validated skill into targetDir, preserving rollback on overwrite.
	if alreadyExists {
		backupDir := filepath.Join(stagingParent, "backup-"+skillID)
		if err := os.Rename(targetDir, backupDir); err != nil {
			return Result{IsError: true}, fmt.Errorf("backup existing skill: %w", err)
		}
		if err := os.Rename(stagedSkillDir, targetDir); err != nil {
			_ = os.Rename(backupDir, targetDir)
			return Result{IsError: true}, fmt.Errorf("replace existing skill: %w", err)
		}
	} else {
		if err := os.Rename(stagedSkillDir, targetDir); err != nil {
			return Result{IsError: true}, fmt.Errorf("install skill: %w", err)
		}
	}

	status := "created"
	if alreadyExists {
		status = "updated"
	}
	payload := map[string]any{
		"status":       status,
		"namespace":    validated.Namespace,
		"skill_id":     validated.ID,
		"display_name": validated.DisplayName,
		"path":         targetDir,
		"files":        createdFiles,
		"usage":        fmt.Sprintf("Select %q from the / skill picker or invoke $%s in chat.", validated.DisplayName, validated.Namespace),
	}
	if len(warnings) > 0 {
		payload["warnings"] = warnings
	}
	raw, err := json.Marshal(payload)
	return Result{Content: string(raw)}, err
}

func (r *Registry) allRegisteredToolNames() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make(map[string]bool, len(r.tools))
	for name := range r.tools {
		names[name] = true
	}
	return names
}

func sanitizeReferenceFilename(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	name = strings.TrimPrefix(name, "references/")
	if name == "" {
		return "", fmt.Errorf("reference filename is required")
	}
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("invalid reference filename %q: must be a plain file name without path separators", raw)
	}
	ext := strings.ToLower(filepath.Ext(name))
	if !allowedSkillReferenceExtensions[ext] {
		return "", fmt.Errorf("unsupported reference file extension %q (allowed: .md, .txt, .json, .yaml, .yml)", ext)
	}
	return name, nil
}

func renderSkillMarkdown(skillID, description string, requiresTools, triggers, platforms []string, discovery string, priority int, instructions string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: ")
	b.WriteString(skillID)
	b.WriteString("\n")
	b.WriteString("description: ")
	b.WriteString(strconv.Quote(strings.TrimSpace(description)))
	b.WriteString("\n")
	if len(requiresTools) > 0 {
		b.WriteString("requires-tools: ")
		b.WriteString(renderYAMLInlineList(requiresTools))
		b.WriteString("\n")
	}
	if cleaned := cleanStringList(triggers); len(cleaned) > 0 {
		b.WriteString("triggers: ")
		b.WriteString(renderYAMLInlineList(cleaned))
		b.WriteString("\n")
	}
	if cleaned := cleanStringList(platforms); len(cleaned) > 0 {
		b.WriteString("platforms: ")
		b.WriteString(renderYAMLInlineList(cleaned))
		b.WriteString("\n")
	}
	if discovery != "" && discovery != "on-trigger" {
		b.WriteString("discovery: ")
		b.WriteString(discovery)
		b.WriteString("\n")
	}
	if priority != 0 {
		b.WriteString("priority: ")
		b.WriteString(strconv.Itoa(priority))
		b.WriteString("\n")
	}
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(instructions))
	b.WriteString("\n")
	return b.String()
}

func renderOpenAIYAML(skillID, displayName, shortDescription, defaultPrompt, fallbackDescription string) string {
	shortDesc := strings.TrimSpace(shortDescription)
	if shortDesc == "" {
		shortDesc = compactSingleLine(fallbackDescription, 120)
	}
	prompt := strings.TrimSpace(defaultPrompt)
	if prompt == "" {
		prompt = fmt.Sprintf("Use $user:%s to follow this skill procedure.", skillID)
	}
	var b strings.Builder
	b.WriteString("interface:\n")
	b.WriteString("  display_name: ")
	b.WriteString(strconv.Quote(strings.TrimSpace(displayName)))
	b.WriteString("\n")
	b.WriteString("  short_description: ")
	b.WriteString(strconv.Quote(shortDesc))
	b.WriteString("\n")
	b.WriteString("  default_prompt: ")
	b.WriteString(strconv.Quote(prompt))
	b.WriteString("\n")
	return b.String()
}

func renderYAMLInlineList(items []string) string {
	quoted := make([]string, 0, len(items))
	for _, item := range items {
		quoted = append(quoted, strconv.Quote(item))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func cleanStringList(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range items {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

func compactSingleLine(text string, maxRunes int) string {
	cleaned := strings.Join(strings.Fields(text), " ")
	runes := []rune(cleaned)
	if len(runes) <= maxRunes {
		return cleaned
	}
	return string(runes[:maxRunes-1]) + "…"
}
