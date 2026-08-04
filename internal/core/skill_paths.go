/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package core

import (
	bundledata "dinkisstyle-chat/bundle"
	"dinkisstyle-chat/internal/skillkit"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const userSkillsReadme = `# User skills

Create one folder per skill and place a SKILL.md file inside it.

Example:

skills/user/my-skill/SKILL.md

Bundled skills are maintained by the application separately. Do not copy or
edit bundled skills here. Valid user skills are selected per request and become
available on the next chat request.
`

func ensureUserSkillsDir(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	readmePath := filepath.Join(dir, "README.md")
	if _, err := os.Stat(readmePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(readmePath, []byte(userSkillsReadme), 0644)
}

func directoryOpenCommand(goos, dir string) (*exec.Cmd, error) {
	switch goos {
	case "windows":
		return exec.Command("explorer", dir), nil
	case "darwin":
		return exec.Command("open", dir), nil
	case "linux":
		return exec.Command("xdg-open", dir), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", goos)
	}
}

// GetSkillsFolderPath returns the platform-specific writable user-skill path.
func (a *App) GetSkillsFolderPath() string {
	return getWritableSkillsDir()
}

// OpenSkillsFolder creates and opens the writable user-skill directory.
func (a *App) OpenSkillsFolder() error {
	dir := getWritableSkillsDir()
	if err := ensureUserSkillsDir(dir); err != nil {
		return fmt.Errorf("prepare skills directory: %w", err)
	}
	cmd, err := directoryOpenCommand(runtime.GOOS, dir)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open skills directory: %w", err)
	}
	return nil
}

func getBundledSkillsDir() string {
	exePath, _ := os.Executable()
	cwd, _ := os.Getwd()
	candidate := resolveBundledSkillsDir(exePath, cwd, runtime.GOOS)
	if hasBundledSkillDocuments(candidate) {
		return candidate
	}

	embeddedFallback := joinAppDataPath(skillsDirName, "builtin")
	if err := bundledata.MaterializeBuiltinSkills(embeddedFallback); err != nil {
		log.Printf("[Skills] failed to materialize embedded built-in skills: %v", err)
		return candidate
	}
	return embeddedFallback
}

func hasBundledSkillDocuments(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*", "SKILL.md"))
	return err == nil && len(matches) > 0
}

func resolveBundledSkillsDir(exePath, cwd, goos string) string {
	var candidates []string
	if exePath != "" {
		exeDir := filepath.Dir(exePath)
		if goos == "darwin" {
			candidates = append(candidates, filepath.Join(filepath.Dir(exeDir), "Resources", skillsDirName, "builtin"))
		}
		candidates = append(candidates, filepath.Join(exeDir, skillsDirName, "builtin"))
	}
	if cwd != "" {
		candidates = append(candidates,
			filepath.Join(cwd, "bundle", skillsDirName, "builtin"),
			filepath.Join(cwd, skillsDirName, "builtin"),
		)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return filepath.Join("bundle", skillsDirName, "builtin")
}

func compileActiveSkills(userText string) skillkit.Compilation {
	userDir := getWritableSkillsDir()
	var setupDiagnostic *skillkit.Diagnostic
	if err := ensureUserSkillsDir(userDir); err != nil {
		setupDiagnostic = &skillkit.Diagnostic{Path: userDir, Message: err.Error()}
	}
	result := skillkit.LoadAndCompile(skillkit.Config{
		BuiltinDir: getBundledSkillsDir(),
		UserDir:    userDir,
	}, userText)
	if setupDiagnostic != nil {
		result.Diagnostics = append(result.Diagnostics, *setupDiagnostic)
	}
	for _, diagnostic := range result.Diagnostics {
		log.Printf("[Skills] %s: %s", diagnostic.Path, diagnostic.Message)
	}
	return result
}
