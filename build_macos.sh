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
    
    # Copy onnxruntime folder but clean out binary files (keep only LICENSE and metadata)
    cp -r onnxruntime "$APP_CONTENT_DIR"
    rm -f "$APP_CONTENT_DIR/onnxruntime/"*.so*
    rm -f "$APP_CONTENT_DIR/onnxruntime/"*.dll
    rm -f "$APP_CONTENT_DIR/onnxruntime/"*.lib
    rm -f "$APP_CONTENT_DIR/onnxruntime/"*.dylib
    
    # Copy the dylib to root MacOS folder for linking
    cp onnxruntime/libonnxruntime.dylib "$APP_CONTENT_DIR"
    
    cp users.json "$APP_RESOURCE_DIR" 2>/dev/null || echo "{}" > "$APP_RESOURCE_DIR/users.json"
    cp config.json "$APP_RESOURCE_DIR" 2>/dev/null || true
    cp dictionary_*.txt "$APP_RESOURCE_DIR" 2>/dev/null || true
    cp Dictionary_editor.py "$APP_RESOURCE_DIR" 2>/dev/null || true
    cp system_prompts.json "$APP_RESOURCE_DIR" 2>/dev/null || true
    cp ThirdPartyNotices.md "$APP_RESOURCE_DIR" 2>/dev/null || true
    
    # Clean up unnecessary files from bundle
    rm -rf "$APP_RESOURCE_DIR/assets/.git"
    
    # Fix RPATH and Dylib ID for portability
    EXE_PATH="$APP_CONTENT_DIR/DKST LLM Chat Server"
    DYLIB_PATH="$APP_CONTENT_DIR/libonnxruntime.dylib"
    
    install_name_tool -add_rpath "@executable_path/" "$EXE_PATH" 2>/dev/null || true
    install_name_tool -id "@rpath/libonnxruntime.dylib" "$DYLIB_PATH"

    # Re-sign binaries to fix "Code Signature Invalid" crash
    echo "Cleaning detritus and re-signing binaries..."
    APP_BUNDLE_PATH="build/bin/DKST LLM Chat Server.app"
    
    # Remove hidden metadata attributes that can break code signing
    xattr -cr "$APP_BUNDLE_PATH"
    
    BUNDLE_ID="com.dinkisstyle.llmchat"
    ENTITLEMENTS="entitlements.plist"
    
    codesign --force --sign "$SIGN_IDENTITY" --timestamp=none --identifier "$BUNDLE_ID" --options runtime "$DYLIB_PATH"
    codesign --force --sign "$SIGN_IDENTITY" --timestamp=none --identifier "$BUNDLE_ID" --options runtime --entitlements "$ENTITLEMENTS" "$EXE_PATH"
    codesign --force --sign "$SIGN_IDENTITY" --timestamp=none --identifier "$BUNDLE_ID" --options runtime --entitlements "$ENTITLEMENTS" --deep "$APP_BUNDLE_PATH"

    echo "Build success!"
else
    echo "Build failed!"
    exit 1
fi
