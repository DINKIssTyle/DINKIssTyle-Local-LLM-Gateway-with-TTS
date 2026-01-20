//go:build darwin
// +build darwin

/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

// InitSystemTray is a no-op on macOS due to AppDelegate conflict with Wails
// macOS users can use Dock icon to restore the window
func InitSystemTray(app *App) {
	// No-op on macOS - HideWindowOnClose will minimize to Dock
}

// UpdateTrayServerState is a no-op on macOS
func UpdateTrayServerState() {
	// No-op on macOS
}
