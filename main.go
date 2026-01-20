/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed frontend/*
var assets embed.FS

func main() {
	app := NewApp(assets)

	// Initialize system tray
	InitSystemTray(app)

	err := wails.Run(&options.App{
		Title:     "DKST LLM Chat Server",
		Width:     780,
		Height:    720,
		MinWidth:  600,
		MinHeight: 720,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:         app.startup,
		HideWindowOnClose: true, // Minimize to tray instead of quitting
		Menu:              createAppMenu(app),
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarDefault(),
		},
	})

	if err != nil {
		log.Fatal(err)
	}
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
		if app.ctx != nil {
			runtime.Quit(app.ctx)
		}
	})

	// Edit Menu
	editMenu := men.AddSubmenu("Edit")
	editMenu.AddText("Undo", keys.CmdOrCtrl("z"), func(_ *menu.CallbackData) {
		if app.ctx != nil {
			runtime.WindowExecJS(app.ctx, "document.execCommand('undo')")
		}
	})
	editMenu.AddText("Redo", keys.CmdOrCtrl("shift+z"), func(_ *menu.CallbackData) {
		if app.ctx != nil {
			runtime.WindowExecJS(app.ctx, "document.execCommand('redo')")
		}
	})
	editMenu.AddSeparator()
	editMenu.AddText("Cut", keys.CmdOrCtrl("x"), func(_ *menu.CallbackData) {
		if app.ctx != nil {
			runtime.WindowExecJS(app.ctx, "document.execCommand('cut')")
		}
	})
	editMenu.AddText("Copy", keys.CmdOrCtrl("c"), func(_ *menu.CallbackData) {
		if app.ctx != nil {
			runtime.WindowExecJS(app.ctx, "document.execCommand('copy')")
		}
	})
	editMenu.AddText("Paste", keys.CmdOrCtrl("v"), func(_ *menu.CallbackData) {
		if app.ctx != nil {
			runtime.WindowExecJS(app.ctx, "document.execCommand('paste')")
		}
	})
	editMenu.AddText("Select All", keys.CmdOrCtrl("a"), func(_ *menu.CallbackData) {
		if app.ctx != nil {
			runtime.WindowExecJS(app.ctx, "document.execCommand('selectAll')")
		}
	})

	// Window Menu
	windowMenu := men.AddSubmenu("Window")
	windowMenu.AddText("Minimize", keys.CmdOrCtrl("m"), func(_ *menu.CallbackData) {
		if app.ctx != nil {
			runtime.WindowMinimise(app.ctx)
		}
	})
	windowMenu.AddText("Zoom", nil, func(_ *menu.CallbackData) {
		if app.ctx != nil {
			runtime.WindowMaximise(app.ctx)
		}
	})

	return men
}
