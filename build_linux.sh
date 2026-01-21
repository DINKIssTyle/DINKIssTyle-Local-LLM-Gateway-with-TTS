#!/bin/bash
# Created by DINKIssTyle on 2026.
# Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

set -e # Exit immediately if a command exits with a non-zero status

echo "Initializing build environment for Linux..."

# Helper to determine sudo usage
SUDO=""
if [ "$EUID" -ne 0 ]; then
    if command -v sudo &> /dev/null; then
        SUDO="sudo"
    else
        echo "Warning: Running as non-root and sudo not found. Installation commands might fail."
    fi
fi

# --- 1. Install System Dependencies ---
echo "Checking system dependencies..."

if command -v apt-get &> /dev/null; then
    echo "Detected Debian/Ubuntu system."
    $SUDO apt-get update
    $SUDO apt-get install -y build-essential libgtk-3-dev pkg-config
    
    # Try installing WebKit2GTK 4.0, fallback to 4.1
    if ! $SUDO apt-get install -y libwebkit2gtk-4.0-dev; then
        echo "WebKit2GTK 4.0 not found, trying 4.1..."
        $SUDO apt-get install -y libwebkit2gtk-4.1-dev
    fi

    if ! command -v go &> /dev/null; then
        echo "Installing Go..."
        $SUDO apt-get install -y golang-go
    fi

elif command -v dnf &> /dev/null; then
    echo "Detected Fedora/RHEL system."
    $SUDO dnf groupinstall -y "Development Tools"
    $SUDO dnf install -y gtk3-devel pkgconf-pkg-config
    
    if ! $SUDO dnf install -y webkit2gtk3-devel; then
         $SUDO dnf install -y webkit2gtk4.1-devel
    fi

    if ! command -v go &> /dev/null; then
        echo "Installing Go..."
        $SUDO dnf install -y golang
    fi

elif command -v pacman &> /dev/null; then
    echo "Detected Arch Linux system."
    $SUDO pacman -Sy --noconfirm base-devel gtk3 webkit2gtk go

elif command -v apk &> /dev/null; then
    echo "Detected Alpine Linux system."
    $SUDO apk update
    $SUDO apk add build-base gtk+3.0-dev webkit2gtk-dev go pkgconf
else
    echo "Warning: Unsupported package manager. Make sure you have GTK3, WebKit2GTK, and Go installed manually."
fi

# --- 2. Check Go Installation ---
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed or not in PATH. Please install Go (https://go.dev/dl/)."
    exit 1
fi

# --- 3. Check/Install Wails ---
echo "Checking Wails..."
WAILS_CMD="wails"
if ! command -v wails &> /dev/null; then
    GOBIN=$(go env GOPATH)/bin
    if [ -f "$GOBIN/wails" ]; then
        WAILS_CMD="$GOBIN/wails"
        echo "Found Wails at $WAILS_CMD"
    else
        echo "Wails not found. Installing..."
        go install github.com/wailsapp/wails/v2/cmd/wails@latest
        WAILS_CMD="$GOBIN/wails"
        
        if [ ! -f "$WAILS_CMD" ]; then
             echo "Error: Failed to install Wails."
             exit 1
        fi
        echo "Wails installed successfully."
    fi
    # Add GOBIN to PATH for this session
    export PATH=$PATH:$GOBIN
fi

# --- 4. Verify WebKit Version for Build Tags ---
BUILD_TAGS=""
if pkg-config --exists webkit2gtk-4.0; then
    echo "Found webkit2gtk-4.0"
elif pkg-config --exists webkit2gtk-4.1; then
    echo "Found webkit2gtk-4.1, adding build tag..."
    BUILD_TAGS="-tags webkit2_41"
else
    echo "Warning: WebKit2GTK development libraries not found via pkg-config. Build might fail."
fi

# --- 5. Clean & Build ---
echo "Cleaning old artifacts..."
rm -rf build/bin
rm -rf frontend/dist

echo "Building application for Linux..."
$WAILS_CMD build -platform linux/amd64 $BUILD_TAGS

# --- 6. Organize Output ---
APP_CONTENT_DIR="build/bin"
if [ -d "$APP_CONTENT_DIR" ]; then
    echo "Organizing artifacts..."
    mkdir -p "$APP_CONTENT_DIR/onnxruntime"
    
    # Copy ONNX Runtime lib if exists
    if [ -f "onnxruntime/libonnxruntime.so" ]; then
        cp onnxruntime/libonnxruntime.so "$APP_CONTENT_DIR/onnxruntime/libonnxruntime.so"
        cp onnxruntime/LICENSE.txt "$APP_CONTENT_DIR/onnxruntime/LICENSE.txt"
    else
        echo "Warning: libonnxruntime.so not found in project root."
    fi

    # Copy resources
    # cp -r assets "$APP_CONTENT_DIR"
    cp -r frontend "$APP_CONTENT_DIR"
    cp users.json "$APP_CONTENT_DIR" 2>/dev/null || echo "{}" > "$APP_CONTENT_DIR/users.json"
    cp config.json "$APP_CONTENT_DIR" 2>/dev/null || true
    cp dictionary_*.txt "$APP_CONTENT_DIR" 2>/dev/null || true
    cp Dictionary_editor.py "$APP_CONTENT_DIR" 2>/dev/null || true
    cp system_prompts.json "$APP_CONTENT_DIR" 2>/dev/null || true
    
    # Cleanup
    rm -rf "$APP_CONTENT_DIR/assets/.git"
    rm -rf "$APP_CONTENT_DIR/frontend/.git"

    echo "Build success! Output directory: $APP_CONTENT_DIR"
else
    echo "Build failed!"
    exit 1
fi
