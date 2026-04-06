/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"embed"
	"log"

	"dinkisstyle-chat/internal/harness"
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
	desktop := harness.NewDesktop(harness.DesktopAssets{
		Frontend:    assets,
		TrayIconPNG: trayIconPng,
		TrayIconICO: trayIconIco,
	})

	err := desktop.Run()

	if err != nil {
		log.Fatal(err)
	}
}
