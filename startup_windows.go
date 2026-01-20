//go:build windows
// +build windows

/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const registryKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const appName = "DKST LLM Chat Server"

// RegisterStartup registers the app to start on login (Windows)
func RegisterStartup() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	if err := key.SetStringValue(appName, exePath); err != nil {
		return fmt.Errorf("failed to set registry value: %w", err)
	}

	fmt.Println("Startup registration complete (Windows)")
	return nil
}

// UnregisterStartup removes the app from startup (Windows)
func UnregisterStartup() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	if err := key.DeleteValue(appName); err != nil {
		// Ignore error if value doesn't exist
		fmt.Printf("Note: registry delete returned: %v\n", err)
	}

	fmt.Println("Startup unregistration complete (Windows)")
	return nil
}
