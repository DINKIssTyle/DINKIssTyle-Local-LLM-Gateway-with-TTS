#!/bin/bash

# Created by DINKIssTyle on 2026.
# Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

echo "Cleaning build artifacts..."
rm -rf build/bin
rm -rf frontend/dist

# Setup PATH for Go and Wails
export PATH="$HOME/go/bin:$PATH"
export PATH="/usr/local/go/bin:$PATH"
export PATH="/opt/homebrew/bin:$PATH"

# Verify wails is available
if ! command -v wails &> /dev/null; then
    echo "Error: wails not found. Please install wails: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    exit 1
fi

resolve_signing_identity() {
    if [ -n "${MACOS_SIGN_IDENTITY:-}" ]; then
        echo "$MACOS_SIGN_IDENTITY"
        return 0
    fi

    local detected_identity
    detected_identity=$(
        security find-identity -v -p codesigning 2>/dev/null \
            | sed -n 's/.*"\(Developer ID Application:[^"]*\)".*/\1/p' \
            | head -n 1
    )
    if [ -n "$detected_identity" ]; then
        echo "$detected_identity"
        return 0
    fi

    detected_identity=$(
        security find-identity -v -p codesigning 2>/dev/null \
            | sed -n 's/.*"\(Apple Development:[^"]*\)".*/\1/p' \
            | head -n 1
    )
    if [ -n "$detected_identity" ]; then
        echo "$detected_identity"
        return 0
    fi

    echo "-"
}

echo "Clean complete. Building for macOS..."
echo "Using wails at: $(which wails)"
SIGN_IDENTITY="$(resolve_signing_identity)"
if [ "$SIGN_IDENTITY" = "-" ]; then
    echo "Warning: no fixed macOS signing identity found. Falling back to ad-hoc signing; permission prompts may still reset between builds."
else
    echo "Using signing identity: $SIGN_IDENTITY"
fi

# You can change darwin/universal to darwin/amd64 or darwin/arm64 if needed
wails build -platform darwin/universal -skipbindings

if [ $? -eq 0 ]; then
    APP_CONTENT_DIR="build/bin/DKST LLM Chat Server.app/Contents/MacOS/"
    APP_RESOURCE_DIR="build/bin/DKST LLM Chat Server.app/Contents/Resources/"
    mkdir -p "$APP_RESOURCE_DIR"

    mkdir -p "$APP_RESOURCE_DIR/assets/runtime/onnxruntime"
    cp bundle/assets/runtime/onnxruntime/libonnxruntime.dylib "$APP_RESOURCE_DIR/assets/runtime/onnxruntime/"
    cp bundle/assets/runtime/onnxruntime/LICENSE.txt "$APP_RESOURCE_DIR/assets/runtime/onnxruntime/" 2>/dev/null || true
    cp bundle/assets/runtime/onnxruntime/README.md "$APP_RESOURCE_DIR/assets/runtime/onnxruntime/" 2>/dev/null || true
    cp bundle/assets/runtime/onnxruntime/ThirdPartyNotices.txt "$APP_RESOURCE_DIR/assets/runtime/onnxruntime/" 2>/dev/null || true
    cp -R bundle/dictionary "$APP_RESOURCE_DIR"
    cp bundle/users.json "$APP_RESOURCE_DIR" 2>/dev/null || echo "{}" > "$APP_RESOURCE_DIR/users.json"
    cp bundle/config.json "$APP_RESOURCE_DIR" 2>/dev/null || true
    cp bundle/system_prompts.json "$APP_RESOURCE_DIR" 2>/dev/null || true
    cp bundle/ThirdPartyNotices.md "$APP_RESOURCE_DIR" 2>/dev/null || true
    
    # Re-sign binaries to fix "Code Signature Invalid" crash
    echo "Cleaning detritus and re-signing binaries..."
    APP_BUNDLE_PATH="build/bin/DKST LLM Chat Server.app"
    
    # Remove hidden metadata attributes that can break code signing
    xattr -cr "$APP_BUNDLE_PATH"
    
    BUNDLE_ID="com.dinkisstyle.llmchat"
    ENTITLEMENTS="bundle/entitlements.plist"
    EXE_PATH="$APP_CONTENT_DIR/DKST LLM Chat Server"
    DYLIB_PATH="$APP_RESOURCE_DIR/assets/runtime/onnxruntime/libonnxruntime.dylib"

    codesign --force --sign "$SIGN_IDENTITY" --timestamp=none --identifier "$BUNDLE_ID" --options runtime "$DYLIB_PATH"
    codesign --force --sign "$SIGN_IDENTITY" --timestamp=none --identifier "$BUNDLE_ID" --options runtime --entitlements "$ENTITLEMENTS" "$EXE_PATH"
    codesign --force --sign "$SIGN_IDENTITY" --timestamp=none --identifier "$BUNDLE_ID" --options runtime --entitlements "$ENTITLEMENTS" --deep "$APP_BUNDLE_PATH"

    echo "Build success!"
else
    echo "Build failed!"
    exit 1
fi
