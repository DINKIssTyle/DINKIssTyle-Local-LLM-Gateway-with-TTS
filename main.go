/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"context"
	"embed"
	"log"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed trayicon.png
var trayIconPng []byte

//go:embed trayicon.ico
var trayIconIco []byte

//go:embed build/windows/icon.ico
var windowIcon []byte

//go:embed frontend/*
var assets embed.FS

func main() {
	app := NewApp(assets)

	// Select tray icon based on OS (Windows prefers ICO)
	var trayIcon []byte
	if runtime.GOOS == "windows" {
		trayIcon = trayIconIco
	} else {
		trayIcon = trayIconPng
	}

	// Initialize system tray
	InitSystemTray(app, trayIcon)

	err := wails.Run(&options.App{
		Title:     "DKST LLM Chat Server",
		Width:     780,
		Height:    760,
		MinWidth:  600,
		MinHeight: 760,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:         app.startup,
		OnShutdown:        app.shutdown,
		HideWindowOnClose: false, // Handled by OnBeforeClose
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			if app.isQuitting {
				return false
			}
			minimize := app.GetMinimizeToTray()
			if minimize {
				wruntime.WindowHide(ctx)
				return true
			}
			return false
		},
		Menu: createAppMenu(app),
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarDefault(),
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			Theme:                windows.SystemDefault,
			CustomTheme:          nil,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
	// Process exit is handled by onTrayExit callback in systray
}

func createAppMenu(app *App) *menu.Menu {
	men := menu.NewMenu()

	// App Menu
	appMenu := men.AddSubmenu("App")
	appMenu.AddText("About DKST LLM Chat Server", keys.CmdOrCtrl("i"), func(_ *menu.CallbackData) {
		app.ShowAbout()
	})
	appMenu.AddSeparator()
	appMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		app.Quit()
	})

	// Edit Menu - Use Wails built-in EditMenu for proper clipboard support
	men.Append(menu.EditMenu())

	// Window Menu
	windowMenu := men.AddSubmenu("Window")
	windowMenu.AddText("Minimize", keys.CmdOrCtrl("m"), func(_ *menu.CallbackData) {
		if app.ctx != nil {
			wruntime.WindowMinimise(app.ctx)
		}
	})
	windowMenu.AddText("Zoom", nil, func(_ *menu.CallbackData) {
		if app.ctx != nil {
			wruntime.WindowMaximise(app.ctx)
		}
	})

	return men
}
