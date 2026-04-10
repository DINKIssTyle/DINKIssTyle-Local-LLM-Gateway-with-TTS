@echo off
rem Created by DINKIssTyle on 2026.
rem Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

echo Cleaning build artifacts...
rem Only delete the binary to preserve assets, configuration, and downloaded models
if exist "build\bin\DKST LLM Chat Server.exe" del "build\bin\DKST LLM Chat Server.exe"
if not exist "build\bin" mkdir "build\bin"
if exist "frontend\dist" rmdir /s /q "frontend\dist"

echo Clean complete.
echo Syncing version from config...
powershell -Command "$content = Get-Content internal/config/config.go -Raw; if ($content -match 'AppVersion\s*=\s*\"([^\"]+)\"') { $version = $Matches[1]; echo \"Extracted Version: $version\"; $wails = Get-Content wails.json | ConvertFrom-Json; $wails.info.productVersion = $version; $wails | ConvertTo-Json -Depth 10 | Set-Content wails.json; if (Test-Path bundle\versioninfo.json) { $vi = Get-Content bundle\versioninfo.json | ConvertFrom-Json; $vi.FixedFileInfo.FileVersion.Major = [int]$version.Split('.')[0]; $vi.FixedFileInfo.FileVersion.Minor = [int]$version.Split('.')[1]; $vi.FixedFileInfo.FileVersion.Patch = [int]$version.Split('.')[2]; $vi.FixedFileInfo.ProductVersion = $vi.FixedFileInfo.FileVersion; $vi.StringFileInfo.FileVersion = $version; $vi.StringFileInfo.ProductVersion = $version; $vi | ConvertTo-Json -Depth 10 | Set-Content bundle\versioninfo.json; } if (Test-Path frontend\package.json) { $pkg = Get-Content frontend\package.json | ConvertFrom-Json; $pkg.version = $version; $pkg | ConvertTo-Json -Depth 10 | Set-Content frontend\package.json; } } else { Write-Error 'Could not extract version'; exit 1 }"

if %ERRORLEVEL% NEQ 0 (
    echo Version sync failed!
    exit /b 1
)

echo Building...
rem Using manual build (generate + go build) because wails build CLI is failing in this environment.
wails generate bindings
rem Generate Windows resources (icon, manifest, version info)
goversioninfo -64 -o resource_windows.syso bundle\versioninfo.json
go build -ldflags "-s -w -H windowsgui" -tags desktop,production -o "build\bin\DKST LLM Chat Server.exe" .

if exist "build\bin\DKST LLM Chat Server.exe" (
    echo Copying assets...
    if exist "bundle\assets" xcopy /E /I /Y "bundle\assets" "build\bin\assets" >nul
    if exist "bundle\dictionary" xcopy /E /I /Y "bundle\dictionary" "build\bin\dictionary" >nul
    if not exist "build\bin\users.json" copy /Y "bundle\users.json" "build\bin\" >nul
    if not exist "build\bin\config.json" copy /Y "bundle\config.json" "build\bin\" 2>nul
    copy /Y "bundle\system_prompts.json" "build\bin\" >nul
    copy /Y "bundle\ThirdPartyNotices.md" "build\bin\" >nul
    echo Build success!
) else (
    echo Build failed!
)

rem Clean up auto-generated Windows resource after build
if exist "resource_windows.syso" del resource_windows.syso
