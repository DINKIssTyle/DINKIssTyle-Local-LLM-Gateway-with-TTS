/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"embed"
	"log"
	"os"

	"dinkisstyle-chat/internal/harness"
	"dinkisstyle-chat/internal/mediapipeworker"
)

//go:embed build/darwin/trayicon.png
var trayIconPng []byte

//go:embed build/linux/trayicon.png
var trayIconLinuxPng []byte

//go:embed build/windows/trayicon.ico
var trayIconIco []byte

//go:embed build/windows/icon.ico
var windowIcon []byte

//go:embed frontend/*
var assets embed.FS

func main() {
	if len(os.Args) > 1 && os.Args[1] == mediapipeworker.Argument {
		if len(os.Args) != 4 {
			log.Fatal("MediaPipe worker requires a library path and a model path")
		}
		if err := mediapipeworker.Run(os.Args[2], os.Args[3]); err != nil {
			log.Fatal(err)
		}
		return
	}
	desktop := harness.NewDesktop(harness.DesktopAssets{
		Frontend:         assets,
		TrayIconMacOSPNG: trayIconPng,
		TrayIconLinuxPNG: trayIconLinuxPng,
		TrayIconWindows:  trayIconIco,
	})

	err := desktop.Run()

	if err != nil {
		log.Fatal(err)
	}
}
