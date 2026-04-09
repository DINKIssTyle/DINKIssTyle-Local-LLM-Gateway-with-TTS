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
goversioninfo -64 -o resource_windows.syso bundle\versioninfo.json
go build -ldflags "-s -w -H windowsgui" -tags desktop,production -o "build\bin\DKST LLM Chat Server.exe" .

if exist "build\bin\DKST LLM Chat Server.exe" (
    echo Copying assets...
    if exist "bundle\assets" xcopy /E /I /Y "bundle\assets" "build\bin\assets" >nul
    if exist "bundle\dictionary" xcopy /E /I /Y "bundle\dictionary" "build\bin\dictionary" >nul
    copy /Y "bundle\users.json" "build\bin\" >nul
    copy /Y "bundle\config.json" "build\bin\" 2>nul
    copy /Y "bundle\system_prompts.json" "build\bin\" >nul
    copy /Y "bundle\ThirdPartyNotices.md" "build\bin\" >nul
    echo Build success!
) else (
    echo Build failed!
)

rem Clean up auto-generated Windows resource after build
if exist "resource_windows.syso" del resource_windows.syso
