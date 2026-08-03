//go:build !darwin
// +build !darwin

/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package core

import (
	"fmt"
	"os"
	"time"

	"github.com/energye/systray"
)

var (
	trayApp       *App
	mServerStatus *systray.MenuItem
	mServerToggle *systray.MenuItem
	mShowWindow   *systray.MenuItem
	mQuit         *systray.MenuItem
	trayQuitChan  = make(chan struct{})
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
	// Start tray in a goroutine
	go systray.Run(onTrayReady, onTrayExit)
}

func onTrayReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("DKST LLM")
	systray.SetTooltip("DKST LLM Chat Server")

	// Show control window on icon click
	systray.SetOnClick(func(menu systray.IMenu) {
		if trayApp != nil {
			trayApp.ShowMainWindow()
		}
	})

	// Right-click menu
	systray.SetOnRClick(func(menu systray.IMenu) {
		menu.ShowMenu()
	})

	// Menu items
	mServerStatus = systray.AddMenuItem("서버 상태: 중지됨", "Current server status")
	mServerStatus.Disable()

	mServerToggle = systray.AddMenuItem("서버 시작", "Start or stop the server")
	mServerToggle.Click(func() {
		if trayApp != nil {
			go trayApp.toggleServerFromTray()
		}
	})

	systray.AddSeparator()

	mShowWindow = systray.AddMenuItem("메인 창 열기", "Open Main Window")
	mShowWindow.Click(func() {
		if trayApp != nil {
			trayApp.ShowMainWindow()
		}
	})

	systray.AddSeparator()

	mQuit = systray.AddMenuItem("종료", "Quit Application")
	mQuit.Click(func() {
		if trayApp != nil {
			trayApp.Quit()
			return
		}
		systray.Quit()
	})

	// Update menu based on initial server state
	updateServerMenuItems(trayApp != nil && trayApp.IsServerRunning())
}

func onTrayExit() {
	fmt.Println("System tray exiting...")
	close(trayQuitChan)
	// Force exit after a short delay to ensure cleanup
	time.Sleep(100 * time.Millisecond)
	os.Exit(0)
}

// updateServerMenuItems updates the status and action labels atomically from a
// synchronized server-state snapshot supplied by App.
func updateServerMenuItems(running bool) {
	language := "ko"
	if trayApp != nil {
		language = trayApp.GetServerUILanguage()
	}
	labels := getTrayMenuLabels(language, running)
	if mServerStatus != nil {
		mServerStatus.SetTitle(labels.status)
	}
	if mServerToggle != nil {
		mServerToggle.SetTitle(labels.toggle)
		if running {
			mServerToggle.SetTooltip("Stop Server")
		} else {
			mServerToggle.SetTooltip("Start Server")
		}
	}
	if mShowWindow != nil {
		mShowWindow.SetTitle(labels.show)
	}
	if mQuit != nil {
		mQuit.SetTitle(labels.quit)
	}
}

// UpdateTrayServerState is called from App to update tray menu
func UpdateTrayServerState(running bool) {
	updateServerMenuItems(running)
}

// QuitSystemTray stops the system tray loop
func QuitSystemTray() {
	systray.Quit()
}
