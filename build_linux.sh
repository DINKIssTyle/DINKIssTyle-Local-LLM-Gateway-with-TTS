#!/bin/bash

# Created by DINKIssTyle on 2026.
# Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

echo "Cleaning build artifacts..."
rm -rf build/bin
rm -rf frontend/dist

echo "Clean complete. Building for Linux..."
echo "Cheking for wails..."

# Determine Wails path
WAILS_CMD="wails"
if ! command -v wails &> /dev/null; then
    if [ -f "$HOME/go/bin/wails" ]; then
        WAILS_CMD="$HOME/go/bin/wails"
        echo "Found wails at $WAILS_CMD"
    else
        echo "Wails not found in PATH or ~/go/bin."
        if command -v go &> /dev/null; then
            echo "Go found. Installing wails..."
            go install github.com/wailsapp/wails/v2/cmd/wails@latest
            if [ -f "$HOME/go/bin/wails" ]; then
                WAILS_CMD="$HOME/go/bin/wails"
                echo "Wails installed successfully."
            else
                echo "Failed to install wails. Please install it manually."
                exit 1
            fi
        else
            echo "Go not found. Please install Go and Wails to continue."
            exit 1
        fi
    fi
fi

# Check for Webkit version (Ubuntu 24 uses webkit2gtk-4.1)
BUILD_TAGS=""
if pkg-config --exists webkit2gtk-4.0; then
    echo "Found webkit2gtk-4.0"
elif pkg-config --exists webkit2gtk-4.1; then
    echo "Found webkit2gtk-4.1, adding build tag..."
    BUILD_TAGS="-tags webkit2_41"
else
    echo "Warning: Neither webkit2gtk-4.0 nor webkit2gtk-4.1 found. Build might fail."
fi

echo "Clean complete. Building for Linux..."
$WAILS_CMD build -platform linux/amd64 $BUILD_TAGS


if [ $? -eq 0 ]; then
    APP_CONTENT_DIR="build/bin"
    # Copy onnxruntime library and license
    mkdir -p "$APP_CONTENT_DIR/onnxruntime"
    cp onnxruntime/libonnxruntime.so "$APP_CONTENT_DIR/onnxruntime/libonnxruntime.so"
    cp onnxruntime/LICENSE.txt "$APP_CONTENT_DIR/onnxruntime/LICENSE.txt"

    # cp -r assets "$APP_CONTENT_DIR"
    cp -r frontend "$APP_CONTENT_DIR"
    cp users.json "$APP_CONTENT_DIR" 2>/dev/null || echo "{}" > "$APP_CONTENT_DIR/users.json"
    cp config.json "$APP_CONTENT_DIR" 2>/dev/null || true
    
    # Clean up unnecessary files
    rm -rf "$APP_CONTENT_DIR/assets/.git"
    rm -rf "$APP_CONTENT_DIR/frontend/.git"

    echo "Build success!"
else
    echo "Build failed!"
    exit 1
fi
