//go:build darwin
// +build darwin

/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const launchAgentPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.dinkisstyle.dkst-llm-chat</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.AppPath}}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>
`

// RegisterStartup registers the app to start on login (macOS)
func RegisterStartup() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	launchAgentsDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	plistPath := filepath.Join(launchAgentsDir, "com.dinkisstyle.dkst-llm-chat.plist")

	// Get the app bundle path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// If running from app bundle, use the bundle path
	appPath := exePath
	if filepath.Ext(filepath.Dir(filepath.Dir(exePath))) == ".app" {
		// We're inside an app bundle, use open command
		bundlePath := filepath.Dir(filepath.Dir(filepath.Dir(exePath)))
		appPath = bundlePath
	}

	// Create plist file
	file, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("failed to create plist file: %w", err)
	}
	defer file.Close()

	tmpl, err := template.New("plist").Parse(launchAgentPlist)
	if err != nil {
		return fmt.Errorf("failed to parse plist template: %w", err)
	}

	data := struct {
		AppPath string
	}{
		AppPath: appPath,
	}

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}

	// Load the launch agent
	cmd := exec.Command("launchctl", "load", plistPath)
	if err := cmd.Run(); err != nil {
		// Ignore error if already loaded
		fmt.Printf("Note: launchctl load returned: %v\n", err)
	}

	fmt.Println("Startup registration complete (macOS)")
	return nil
}

// UnregisterStartup removes the app from login items (macOS)
func UnregisterStartup() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.dinkisstyle.dkst-llm-chat.plist")

	// Unload the launch agent
	cmd := exec.Command("launchctl", "unload", plistPath)
	cmd.Run() // Ignore error

	// Remove the plist file
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	fmt.Println("Startup unregistration complete (macOS)")
	return nil
}
