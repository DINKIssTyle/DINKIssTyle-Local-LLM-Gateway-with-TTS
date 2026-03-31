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

	"dinkisstyle-chat/internal/core"
	"dinkisstyle-chat/internal/mcp"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed bundle/trayicon.png
var trayIconPng []byte

//go:embed bundle/trayicon.ico
var trayIconIco []byte

//go:embed build/windows/icon.ico
var windowIcon []byte

//go:embed frontend/*
var assets embed.FS

func main() {
	core.InitLoggingFilter()
	app := core.NewApp(assets)

	// Select tray icon based on OS (Windows prefers ICO)
	var trayIcon []byte
	if runtime.GOOS == "windows" {
		trayIcon = trayIconIco
	} else {
		trayIcon = trayIconPng
	}

	// Initialize system tray
	core.InitSystemTray(app, trayIcon)

	err := wails.Run(&options.App{
		Title:     "DKST LLM Chat Server",
		Width:     755,
		Height:    800,
		MinWidth:  755,
		MinHeight: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			// Initialize SQLite DB
			dbPath, err := mcp.GetUserMemoryFilePath("default", "memory.db")
			if err != nil {
				log.Printf("Failed to resolve DB path: %v", err)
			} else {
				if err := mcp.InitDB(dbPath); err != nil {
					log.Printf("Failed to init SQLite: %v", err)
				}
			}
			app.Startup(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			mcp.CloseDB()
			app.Shutdown(ctx)
		},
		HideWindowOnClose: false, // Handled by OnBeforeClose
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			if app.IsQuitting {
				return false
			}
			minimize := app.GetMinimizeToTray()
			if minimize {
				wruntime.WindowHide(ctx)
				return true
			}
			return false
		},
		Menu: core.CreateAppMenu(app),
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

