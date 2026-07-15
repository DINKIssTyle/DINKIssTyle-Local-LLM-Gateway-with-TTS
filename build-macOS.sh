#!/bin/bash

set -euo pipefail

# Created by DINKIssTyle on 2026.
# Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

echo "Cleaning build artifacts..."
rm -rf build/bin
rm -rf frontend/dist

# Setup PATH for Go and Wails
export PATH="$HOME/go/bin:$PATH"
export PATH="/usr/local/go/bin:$PATH"
export PATH="/opt/homebrew/bin:$PATH"

# Wails references UTType on recent macOS SDKs. Some Wails versions omit this
# framework from the x86_64 link step, which breaks universal builds.
export CGO_LDFLAGS="${CGO_LDFLAGS:-} -framework UniformTypeIdentifiers"

# Verify wails is available
if ! command -v wails &> /dev/null; then
    echo "Error: wails not found. Please install wails: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    exit 1
fi

# --- Version Sync Logic ---
# Extract version from internal/config/config.go
APP_VERSION=$(grep 'AppVersion =' internal/config/config.go | cut -d'"' -f2)

if [ -z "$APP_VERSION" ]; then
    echo "Error: Could not extract AppVersion from internal/config/config.go"
    exit 1
fi

echo "Synced App Version: $APP_VERSION"

# Update wails.json
if command -v python3 &> /dev/null; then
    python3 -c "import json; d=json.load(open('wails.json')); d['info']['productVersion']='$APP_VERSION'; json.dump(d, open('wails.json', 'w'), indent=4)"
elif command -v sed &> /dev/null; then
    # Fallback to sed if python3 is not available (less robust for JSON but works for simple replacement)
    sed -i '' "s/\"productVersion\": \".*\"/\"productVersion\": \"$APP_VERSION\"/" wails.json
fi

# Update frontend/package.json
if [ -f "frontend/package.json" ]; then
    if command -v python3 &> /dev/null; then
        python3 -c "import json; d=json.load(open('frontend/package.json')); d['version']='$APP_VERSION'; json.dump(d, open('frontend/package.json', 'w'), indent=4)"
    fi
fi
# -------------------------

resolve_signing_identity() {
    if [ -n "${MACOS_SIGN_IDENTITY:-}" ]; then
        echo "$MACOS_SIGN_IDENTITY"
        return 0
    fi

    local detected_identity
    local local_identity_name="${MACOS_LOCAL_SIGN_IDENTITY:-DINKIssTyle Local Code Signing}"
    detected_identity=$(
        security find-identity -v -p codesigning 2>/dev/null \
            | awk -v name="$local_identity_name" 'index($0, "\"" name "\"") { print $2; exit }'
    )
    if [ -n "$detected_identity" ]; then
        echo "$detected_identity"
        return 0
    fi

    detected_identity=$(
        security find-identity -v -p codesigning 2>/dev/null \
            | sed -n '/"Developer ID Application:/s/^[[:space:]]*[0-9][0-9]*) \([A-Fa-f0-9]*\) .*/\1/p' \
            | head -n 1
    )
    if [ -n "$detected_identity" ]; then
        echo "$detected_identity"
        return 0
    fi

    detected_identity=$(
        security find-identity -v -p codesigning 2>/dev/null \
            | sed -n '/"Apple Development:/s/^[[:space:]]*[0-9][0-9]*) \([A-Fa-f0-9]*\) .*/\1/p' \
            | head -n 1
    )
    if [ -n "$detected_identity" ]; then
        echo "$detected_identity"
        return 0
    fi

    # Also support a stable, user-created Code Signing certificate. The first
    # valid identity is safe here because Developer ID and Apple Development
    # identities were already preferred above.
    detected_identity=$(
        security find-identity -v -p codesigning 2>/dev/null \
            | sed -n 's/^[[:space:]]*[0-9][0-9]*) \([A-Fa-f0-9]*\) ".*/\1/p' \
            | head -n 1
    )
    if [ -n "$detected_identity" ]; then
        echo "$detected_identity"
        return 0
    fi

    return 1
}

echo "Clean complete. Building for macOS..."
echo "Using wails at: $(which wails)"
if ! SIGN_IDENTITY="$(resolve_signing_identity)"; then
    if [ "${MACOS_ALLOW_ADHOC:-0}" = "1" ]; then
        SIGN_IDENTITY="-"
        echo "Warning: MACOS_ALLOW_ADHOC=1; this build will not retain macOS privacy permissions across rebuilds."
    else
        echo "Error: no stable Code Signing identity was found."
        echo "Create a Code Signing certificate or set MACOS_SIGN_IDENTITY explicitly."
        echo "For a temporary development-only build, set MACOS_ALLOW_ADHOC=1."
        exit 1
    fi
fi
echo "Using signing identity: $SIGN_IDENTITY"
if [ "$SIGN_IDENTITY" = "-" ]; then
    echo "Warning: ad-hoc signing does not preserve macOS privacy permissions across rebuilds."
fi

SIGN_IDENTITY_LABEL="$SIGN_IDENTITY"
if [[ "$SIGN_IDENTITY" =~ ^[A-Fa-f0-9]{40}$ ]]; then
    SIGN_IDENTITY_LABEL=$(
        security find-identity -v -p codesigning 2>/dev/null \
			| awk -v identity="$SIGN_IDENTITY" '$2 == identity && match($0, /"[^"]+"$/) { print substr($0, RSTART + 1, RLENGTH - 2); exit }'
    )
fi

# You can change darwin/universal to darwin/amd64 or darwin/arm64 if needed
wails build -platform darwin/universal

if [ -d "build/bin/DKST LLM Chat Server.app" ]; then
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

    PLIST_BUNDLE_ID="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$APP_BUNDLE_PATH/Contents/Info.plist")"
    if [ "$PLIST_BUNDLE_ID" != "$BUNDLE_ID" ]; then
        echo "Error: bundle identifier is '$PLIST_BUNDLE_ID', expected '$BUNDLE_ID'."
        exit 1
    fi

    TIMESTAMP_ARGS=(--timestamp=none)
    if [[ "$SIGN_IDENTITY_LABEL" == Developer\ ID\ Application:* ]]; then
        TIMESTAMP_ARGS=(--timestamp)
    fi

    # Sign nested code first, then seal the outer app bundle. Do not use
    # --deep for signing: it can hide incorrectly signed nested code.
    codesign --force --sign "$SIGN_IDENTITY" "${TIMESTAMP_ARGS[@]}" --options runtime "$DYLIB_PATH"
    codesign --force --sign "$SIGN_IDENTITY" "${TIMESTAMP_ARGS[@]}" --identifier "$BUNDLE_ID" --options runtime --entitlements "$ENTITLEMENTS" "$APP_BUNDLE_PATH"

    echo "Verifying code signature..."
    codesign --verify --deep --strict --verbose=2 "$APP_BUNDLE_PATH"

    SIGNED_IDENTIFIER="$(codesign -dv --verbose=4 "$APP_BUNDLE_PATH" 2>&1 | sed -n 's/^Identifier=//p')"
    if [ "$SIGNED_IDENTIFIER" != "$BUNDLE_ID" ]; then
        echo "Error: signed identifier is '$SIGNED_IDENTIFIER', expected '$BUNDLE_ID'."
        exit 1
    fi
    if [ "$SIGN_IDENTITY" != "-" ] && codesign -dv --verbose=4 "$APP_BUNDLE_PATH" 2>&1 | grep -q '^Signature=adhoc$'; then
        echo "Error: expected a certificate signature but produced an ad-hoc signature."
        exit 1
    fi

    # codesign verifies the cryptographic seal but does not necessarily perform
    # Apple's online revocation check. A revoked Apple certificate otherwise
    # produces an app that builds successfully and is moved to Trash at launch.
    if [[ "$SIGN_IDENTITY_LABEL" == Apple\ Development:* || "$SIGN_IDENTITY_LABEL" == Developer\ ID\ Application:* ]]; then
        POLICY_RESULT="$(spctl --assess --type execute --verbose=4 "$APP_BUNDLE_PATH" 2>&1 || true)"
        if [[ "$POLICY_RESULT" == *CSSMERR_TP_CERT_REVOKED* ]]; then
            echo "Error: Apple has revoked the selected signing certificate: $SIGN_IDENTITY_LABEL"
            echo "Delete the revoked certificate and create a new Apple Development or Developer ID identity."
            exit 1
        fi
        echo "Gatekeeper assessment: $POLICY_RESULT"
    fi

    echo "Build and signature verification succeeded."
else
    echo "Build failed: app bundle was not created."
    exit 1
fi
