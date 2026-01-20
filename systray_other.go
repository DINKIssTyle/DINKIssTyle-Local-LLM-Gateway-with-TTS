//go:build !darwin
// +build !darwin

/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"github.com/energye/systray"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	trayApp       *App
	mServerToggle *systray.MenuItem
)

// Icon data - minimal 16x16 PNG (gray square placeholder)
// Replace with actual icon data for production
var iconData = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x10,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0xF3, 0xFF, 0x61, 0x00, 0x00, 0x00,
	0x01, 0x73, 0x52, 0x47, 0x42, 0x00, 0xAE, 0xCE, 0x1C, 0xE9, 0x00, 0x00,
	0x00, 0x44, 0x49, 0x44, 0x41, 0x54, 0x38, 0x4F, 0x63, 0x60, 0x18, 0x05,
	0xA3, 0x60, 0x14, 0x8C, 0x02, 0x08, 0x18, 0x19, 0x19, 0xFF, 0x63, 0x93,
	0x64, 0x64, 0x64, 0xFC, 0x0F, 0x00, 0xB2, 0x00, 0x00, 0x06, 0xDC, 0x01,
	0x3D, 0x4D, 0x9F, 0x2F, 0x08, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
	0x44, 0xAE, 0x42, 0x60, 0x82,
}

// InitSystemTray initializes the system tray (Windows/Linux only)
func InitSystemTray(app *App, icon []byte) {
	trayApp = app
	if len(icon) > 0 {
		iconData = icon
	}
	go systray.Run(onTrayReady, onTrayExit)
}

func onTrayReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("DKST LLM")
	systray.SetTooltip("DKST LLM Chat Server")

	// Show control window on icon click
	systray.SetOnClick(func(menu systray.IMenu) {
		if trayApp != nil && trayApp.ctx != nil {
			wruntime.WindowShow(trayApp.ctx)
			wruntime.WindowSetAlwaysOnTop(trayApp.ctx, true)
			wruntime.WindowSetAlwaysOnTop(trayApp.ctx, false)
		}
	})

	// Right-click menu
	systray.SetOnRClick(func(menu systray.IMenu) {
		menu.ShowMenu()
	})

	// Menu items
	mShowWindow := systray.AddMenuItem("컨트롤 창 보이기", "Show Control Window")
	mShowWindow.Click(func() {
		if trayApp != nil && trayApp.ctx != nil {
			wruntime.WindowShow(trayApp.ctx)
			wruntime.WindowSetAlwaysOnTop(trayApp.ctx, true)
			wruntime.WindowSetAlwaysOnTop(trayApp.ctx, false)
		}
	})

	systray.AddSeparator()

	mServerToggle = systray.AddMenuItem("서버 시작", "Start/Stop Server")
	mServerToggle.Click(func() {
		if trayApp != nil {
			if trayApp.isRunning {
				trayApp.StopServer()
			} else {
				go trayApp.StartServerWithCurrentConfig()
			}
			updateServerMenuItem()
		}
	})

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("종료", "Quit Application")
	mQuit.Click(func() {
		if trayApp != nil {
			trayApp.StopServer()
			if trayApp.ctx != nil {
				wruntime.Quit(trayApp.ctx)
			}
		}
		systray.Quit()
	})

	// Update menu based on initial server state
	updateServerMenuItem()
}

func onTrayExit() {
	// Cleanup if needed
}

// updateServerMenuItem updates the server menu item text based on server state
func updateServerMenuItem() {
	if mServerToggle == nil {
		return
	}
	if trayApp != nil && trayApp.isRunning {
		mServerToggle.SetTitle("서버 종료")
		mServerToggle.SetTooltip("Stop Server")
	} else {
		mServerToggle.SetTitle("서버 시작")
		mServerToggle.SetTooltip("Start Server")
	}
}

// UpdateTrayServerState is called from App to update tray menu
func UpdateTrayServerState() {
	updateServerMenuItem()
}
