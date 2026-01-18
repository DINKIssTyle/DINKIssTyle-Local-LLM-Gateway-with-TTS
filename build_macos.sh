#!/bin/bash

# Created by DINKIssTyle on 2026.
# Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

echo "Cleaning build artifacts..."
rm -rf build/bin
rm -rf frontend/dist

echo "Clean complete. Building for macOS..."
# You can change darwin/universal to darwin/amd64 or darwin/arm64 if needed
/Users/dinki/go/bin/wails build -platform darwin/universal -skipbindings

if [ $? -eq 0 ]; then
    APP_CONTENT_DIR="build/bin/DKST LLM Chat Server.app/Contents/MacOS/"
    
    # Copy onnxruntime folder but clean it
    cp -r onnxruntime "$APP_CONTENT_DIR"
    rm -f "$APP_CONTENT_DIR/onnxruntime/"*.so*
    rm -f "$APP_CONTENT_DIR/onnxruntime/"*.dll
    rm -f "$APP_CONTENT_DIR/onnxruntime/"*.lib
    
    # Also keep the dylib in root for linking if needed, or rely on RPATH pointing to inner folder?
    # The install_name_tool logic below uses "$APP_CONTENT_DIR/libonnxruntime.dylib"
    # So we probably want the dylib in the root of MacOS too, or point to the one in onnxruntime folder.
    # To be safe and consistent with previous working state (linking), let's copy the dylib to root AND keep the folder metadata for app.go logic.
    cp onnxruntime/libonnxruntime.dylib "$APP_CONTENT_DIR"
    
    # cp -r assets "$APP_CONTENT_DIR"
    cp -r frontend "$APP_CONTENT_DIR"
    cp users.json "$APP_CONTENT_DIR" 2>/dev/null || echo "{}" > "$APP_CONTENT_DIR/users.json"
    cp config.json "$APP_CONTENT_DIR" 2>/dev/null || true
    
    # Clean up unnecessary files from bundle
    rm -rf "$APP_CONTENT_DIR/assets/.git"
    rm -rf "$APP_CONTENT_DIR/frontend/.git"
    
    # Fix RPATH and Dylib ID for portability
    EXE_PATH="$APP_CONTENT_DIR/DKST LLM Chat Server"
    DYLIB_PATH="$APP_CONTENT_DIR/libonnxruntime.dylib"
    
    install_name_tool -add_rpath "@executable_path/" "$EXE_PATH" 2>/dev/null || true
    install_name_tool -id "@rpath/libonnxruntime.dylib" "$DYLIB_PATH"

    # Re-sign binaries to fix "Code Signature Invalid" crash
    echo "Re-signing binaries..."
    codesign -f -s - "$DYLIB_PATH"
    
    APP_BUNDLE_PATH="build/bin/DKST LLM Chat Server.app"
    codesign -f -s - --deep "$APP_BUNDLE_PATH"

    echo "Build success!"
else
    echo "Build failed!"
    exit 1
fi
