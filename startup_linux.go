//go:build linux
// +build linux

/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

const desktopEntry = `[Desktop Entry]
Type=Application
Name=DKST LLM Chat Server
Exec={{.AppPath}}
Icon=utilities-terminal
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true
`

// RegisterStartup registers the app to start on login (Linux)
func RegisterStartup() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	autostartDir := filepath.Join(homeDir, ".config", "autostart")
	if err := os.MkdirAll(autostartDir, 0755); err != nil {
		return fmt.Errorf("failed to create autostart directory: %w", err)
	}

	desktopPath := filepath.Join(autostartDir, "dkst-llm-chat.desktop")

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	file, err := os.Create(desktopPath)
	if err != nil {
		return fmt.Errorf("failed to create desktop file: %w", err)
	}
	defer file.Close()

	tmpl, err := template.New("desktop").Parse(desktopEntry)
	if err != nil {
		return fmt.Errorf("failed to parse desktop template: %w", err)
	}

	data := struct {
		AppPath string
	}{
		AppPath: exePath,
	}

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to write desktop file: %w", err)
	}

	fmt.Println("Startup registration complete (Linux)")
	return nil
}

// UnregisterStartup removes the app from autostart (Linux)
func UnregisterStartup() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	desktopPath := filepath.Join(homeDir, ".config", "autostart", "dkst-llm-chat.desktop")

	if err := os.Remove(desktopPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove desktop file: %w", err)
	}

	fmt.Println("Startup unregistration complete (Linux)")
	return nil
}
