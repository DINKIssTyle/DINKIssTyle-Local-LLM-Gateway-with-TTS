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

DEPENDENCIES_MET=true
if ! pkg-config --exists gtk+-3.0; then DEPENDENCIES_MET=false; fi
if ! pkg-config --exists webkit2gtk-4.0 && ! pkg-config --exists webkit2gtk-4.1; then DEPENDENCIES_MET=false; fi

if [ "$DEPENDENCIES_MET" = "true" ]; then
    echo "Common dependencies (GTK3, WebKit2GTK) already met. Skipping installation."
else
    echo "Missing dependencies. Attempting installation..."
    if command -v apt-get &> /dev/null; then
        echo "Detected Debian/Ubuntu system."
        $SUDO apt-get update
        $SUDO apt-get install -y build-essential libgtk-3-dev pkg-config
        
        # Try installing WebKit2GTK 4.0, fallback to 4.1
        if ! $SUDO apt-get install -y libwebkit2gtk-4.0-dev; then
            echo "WebKit2GTK 4.0 not found, trying 4.1..."
            $SUDO apt-get install -y libwebkit2gtk-4.1-dev
        fi

    elif command -v dnf &> /dev/null; then
        echo "Detected Fedora/RHEL system."
        $SUDO dnf groupinstall -y "Development Tools"
        $SUDO dnf install -y gtk3-devel pkgconf-pkg-config
        
        if ! $SUDO dnf install -y webkit2gtk3-devel; then
             $SUDO dnf install -y webkit2gtk4.1-devel
        fi

    elif command -v pacman &> /dev/null; then
        echo "Detected Arch Linux system."
        $SUDO pacman -Sy --noconfirm base-devel gtk3 webkit2gtk

    elif command -v apk &> /dev/null; then
        echo "Detected Alpine Linux system."
        $SUDO apk update
        $SUDO apk add build-base gtk+3.0-dev webkit2gtk-dev pkgconf
    else
        echo "Warning: Unsupported package manager. Make sure you have GTK3, WebKit2GTK, and Go installed manually."
    fi
fi

# --- 2. Setup Environment ---
# Ensure /usr/local/go/bin is in PATH for this script session
export PATH=/usr/local/go/bin:$PATH

# --- 3. Check Go Installation ---
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed or not in PATH. Please install Go (https://go.dev/dl/)."
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
echo "Using Go: $GO_VERSION"

# --- 4. Check/Install Wails ---
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
fi

# Ensure GOBIN is in PATH
GOBIN=$(go env GOPATH)/bin
export PATH=$PATH:$GOBIN

# --- Version Sync Logic ---
echo "Syncing version from config..."
APP_VERSION=$(grep 'AppVersion =' internal/config/config.go | cut -d'"' -f2)

if [ -z "$APP_VERSION" ]; then
    echo "Error: Could not extract AppVersion from internal/config/config.go"
    exit 1
fi
echo "Current Version: $APP_VERSION"

# Update wails.json
if command -v python3 &> /dev/null; then
    python3 -c "import json; d=json.load(open('wails.json')); d['info']['productVersion']='$APP_VERSION'; json.dump(d, open('wails.json', 'w'), indent=4)"
elif command -v sed &> /dev/null; then
    sed -i "s/\"productVersion\": \".*\"/\"productVersion\": \"$APP_VERSION\"/" wails.json
fi

# Update frontend/package.json
if [ -f "frontend/package.json" ]; then
    if command -v python3 &> /dev/null; then
        python3 -c "import json; d=json.load(open('frontend/package.json')); d['version']='$APP_VERSION'; json.dump(d, open('frontend/package.json', 'w'), indent=4)"
    fi
fi
# -------------------------

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
# Only delete the binary to preserve assets, configuration, and downloaded models
rm -f "build/bin/DKST LLM Chat Server"
mkdir -p build/bin
rm -rf frontend/dist

echo "Building application for Linux..."
$WAILS_CMD build -platform linux/amd64 $BUILD_TAGS

# --- 6. Organize Output ---
APP_CONTENT_DIR="build/bin"
echo "Organizing artifacts..."

if [ -f "$APP_CONTENT_DIR/DKST LLM Chat Server" ]; then
    # Create required directory structure
    mkdir -p "$APP_CONTENT_DIR/assets/runtime/onnxruntime"
    
    # Explicitly copy ONLY the Linux ONNX runtime library
    if [ -f "bundle/assets/runtime/onnxruntime/libonnxruntime.so" ]; then
        cp "bundle/assets/runtime/onnxruntime/libonnxruntime.so" "$APP_CONTENT_DIR/assets/runtime/onnxruntime/"
    fi

    if [ -d "bundle/dictionary" ]; then
        cp -R bundle/dictionary "$APP_CONTENT_DIR"
    fi

    cp bundle/users.json "$APP_CONTENT_DIR" 2>/dev/null || echo "{}" > "$APP_CONTENT_DIR/users.json"
    cp -f bundle/config.json "$APP_CONTENT_DIR"
    cp bundle/system_prompts.json "$APP_CONTENT_DIR" 2>/dev/null || true
    cp bundle/ThirdPartyNotices.md "$APP_CONTENT_DIR" 2>/dev/null || true
    
    echo "Build success! Output directory: $APP_CONTENT_DIR"
else
    echo "Build failed! Binary 'DKST LLM Chat Server' was not created."
    exit 1
fi
