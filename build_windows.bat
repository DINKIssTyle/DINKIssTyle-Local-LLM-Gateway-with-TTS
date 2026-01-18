@echo off
rem Created by DINKIssTyle on 2026.
rem Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

echo Cleaning build artifacts...
if exist "build\bin" rmdir /s /q "build\bin"
if exist "frontend\dist" rmdir /s /q "frontend\dist"

echo Clean complete. Building...
rem Using manual build (generate + go build) because wails build CLI is failing in this environment.
wails generate bindings
rem Generate Windows resources (icon, manifest, version info)
goversioninfo -64 -o resource_windows.syso
go build -ldflags "-s -w -H windowsgui" -tags desktop,production -o "build\bin\DKST LLM Chat Server.exe" .

if exist "build\bin\DKST LLM Chat Server.exe" (
    echo Copying assets...
    copy /Y "onnxruntime\onnxruntime.dll" "build\bin\" 2>nul
    copy /Y "users.json" "build\bin\" 2>nul
    copy /Y "config.json" "build\bin\" 2>nul
    echo Build success!
) else (
    echo Build failed!
)
