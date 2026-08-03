/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const userSkillsReadme = `# User skills

Create one folder per skill and place a SKILL.md file inside it.

Example:

skills/user/my-skill/SKILL.md

Bundled skills are maintained by the application separately. Do not copy or
edit bundled skills here. A future skill loader will validate skill metadata,
permissions, and conflicts before enabling a user skill.
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
