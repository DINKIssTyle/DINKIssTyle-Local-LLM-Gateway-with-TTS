//go:build darwin && cgo

package computeruse

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#include <stdlib.h>
char *DKSTComputerExecute(const char *action, const char *arguments);
*/
import "C"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"strings"
	"time"
	"unsafe"
)

type nativeBackend struct{}

func newNativeBackend() Backend       { return nativeBackend{} }
func (nativeBackend) Available() bool { return true }

func nativeCall(action string, req Request) (json.RawMessage, error) {
	args, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if action == "key" {
		var fields map[string]any
		if err := json.Unmarshal(args, &fields); err != nil {
			return nil, err
		}
		fields["key_code"] = KeyCodes[req.Key]
		args, err = json.Marshal(fields)
		if err != nil {
			return nil, err
		}
	}
	a, b := C.CString(action), C.CString(string(args))
	defer C.free(unsafe.Pointer(a))
	defer C.free(unsafe.Pointer(b))
	result := C.DKSTComputerExecute(a, b)
	if result == nil {
		return nil, fmt.Errorf("native computer use returned no result")
	}
	defer C.free(unsafe.Pointer(result))
	raw := json.RawMessage(C.GoString(result))
	var status struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, err
	}
	if status.Error != "" {
		return nil, fmt.Errorf("%s", status.Error)
	}
	return raw, nil
}

func (nativeBackend) Execute(ctx context.Context, action string, r Request) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if action == "launch_app" {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		resolved, err := nativeCall("resolve_app", r)
		if err != nil {
			return nil, err
		}
		var target struct {
			Application string `json:"application"`
		}
		if err := json.Unmarshal(resolved, &target); err != nil {
			return nil, err
		}
		// Arguments are passed directly to Launch Services, never to a shell.
		out, err := exec.CommandContext(ctx, "/usr/bin/open", "-a", target.Application).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("launch application: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return json.Marshal(map[string]any{"status": "launch_requested", "application": r.Application, "next": "Call computer_observe to verify the app opened and became frontmost."})
	}
	return nativeCall(action, r)
}

func (nativeBackend) Observe(ctx context.Context) (Screen, []byte, error) {
	raw, err := nativeCall("observe", Request{})
	if err != nil {
		return Screen{}, nil, err
	}
	var screen Screen
	if err := json.Unmarshal(raw, &screen); err != nil {
		return Screen{}, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "dkst-computer-")
	if err != nil {
		return Screen{}, nil, err
	}
	defer os.RemoveAll(dir)
	path := dir + "/screen.png"
	if out, err := exec.CommandContext(ctx, "/usr/sbin/screencapture", "-x", "-D", "1", "-t", "png", path).CombinedOutput(); err != nil {
		return Screen{}, nil, fmt.Errorf("screen capture failed: %w: %s. Enable Screen Recording for the gateway app in System Settings > Privacy & Security, then restart the app", err, strings.TrimSpace(string(out)))
	}
	f, err := os.Open(path)
	if err != nil {
		return Screen{}, nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return Screen{}, nil, fmt.Errorf("decode screenshot: %w", err)
	}
	img = fitScreenshot(img)
	screen.Width, screen.Height = img.Bounds().Dx(), img.Bounds().Dy()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return Screen{}, nil, err
	}
	// Refuse to associate a capture with the wrong foreground app or geometry.
	after, err := nativeCall("observe", Request{})
	if err != nil {
		return Screen{}, nil, err
	}
	var current Screen
	if err := json.Unmarshal(after, &current); err != nil {
		return Screen{}, nil, err
	}
	if current.FrontmostPID != screen.FrontmostPID || current.DesktopWidth != screen.DesktopWidth || current.DesktopHeight != screen.DesktopHeight {
		return Screen{}, nil, fmt.Errorf("desktop changed during capture; call computer_observe again")
	}
	if err := ctx.Err(); err != nil {
		return Screen{}, nil, err
	}
	return screen, encoded.Bytes(), nil
}

// Keep model input bounded while preserving the coordinate mapping in Screen.
func fitScreenshot(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 1440 && h <= 1440 {
		return src
	}
	if w >= h {
		h = max(1, h*1440/w)
		w = 1440
	} else {
		w = max(1, w*1440/h)
		h = 1440
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(b.Min.X+x*b.Dx()/w, b.Min.Y+y*b.Dy()/h))
		}
	}
	return dst
}
