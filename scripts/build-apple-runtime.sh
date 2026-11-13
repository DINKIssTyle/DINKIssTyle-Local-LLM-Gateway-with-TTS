#!/bin/sh
set -eu
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
app_root=${1:-"$repo_root/build/bin/DKST LLM Chat Server.app"}
sdk_version=$(xcrun --sdk macosx --show-sdk-version)
sdk_major=${sdk_version%%.*}
if [ "$sdk_major" -lt 26 ]; then
  echo "Apple runtime skipped: macOS 26 SDK required (other engines remain available)."
  exit 0
fi
helper_root="$app_root/Contents/Helpers/DKSTAppleRuntime.app"
mkdir -p "$helper_root/Contents/MacOS"
helper_arm=$(mktemp /private/tmp/dkst-apple-arm.XXXXXX)
helper_intel=$(mktemp /private/tmp/dkst-apple-intel.XXXXXX)
trap 'rm -f "$helper_arm" "$helper_intel"' EXIT
for architecture in arm64 x86_64; do
  destination="$helper_arm"
  if [ "$architecture" = x86_64 ]; then destination="$helper_intel"; fi
  xcrun swiftc -swift-version 5 -parse-as-library -O -target "$architecture-apple-macosx26.0" -module-cache-path /private/tmp/dkst-apple-module-cache "$repo_root/bundle/runtimes/apple/AppleRuntime.swift" -o "$destination"
done
xcrun lipo -create "$helper_arm" "$helper_intel" -output "$helper_root/Contents/MacOS/DKSTAppleRuntime"
cat > "$helper_root/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.dinkisstyle.llmchat.apple-runtime</string><key>CFBundleExecutable</key><string>DKSTAppleRuntime</string><key>CFBundleName</key><string>DKST Apple Runtime</string><key>CFBundlePackageType</key><string>APPL</string><key>LSMinimumSystemVersion</key><string>26.0</string><key>LSUIElement</key><true/></dict></plist>
PLIST
codesign --force --sign "${APPLE_CODESIGN_IDENTITY:--}" "$helper_root"
if [ -f "$app_root/Contents/Info.plist" ]; then
  codesign --force --sign "${APPLE_CODESIGN_IDENTITY:--}" --preserve-metadata=entitlements,requirements,flags "$app_root"
fi
