//go:build darwin && cgo

package computeruse

import (
	"encoding/json"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestScreenshotSizingPreservesAspectAndPixelOrigin(t *testing.T) {
	for _, tc := range []struct{ width, height, wantW, wantH int }{
		{2880, 1800, 1440, 900}, {1800, 2880, 900, 1440}, {100, 200, 100, 200},
	} {
		src := image.NewRGBA(image.Rect(10, 20, 10+tc.width, 20+tc.height))
		src.Set(10, 20, color.RGBA{R: 255, A: 255})
		out := fitScreenshot(src)
		if out.Bounds().Dx() != tc.wantW || out.Bounds().Dy() != tc.wantH {
			t.Fatalf("wrong size: %v", out.Bounds())
		}
		r, _, _, _ := out.At(out.Bounds().Min.X, out.Bounds().Min.Y).RGBA()
		if r != 65535 {
			t.Fatal("image origin lost when resizing")
		}
	}
}

// Compile the actual Objective-C serializer into a small command-line fixture.
// This tests Foundation -> JSON -> Go without capturing a screen, requesting
// permissions, running an AppKit event loop, or posting desktop input.
func TestNativeScreenMetadataSerializesBooleans(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "contract.m")
	fixture := `#include "native_darwin.m"
#include <stdio.h>
int main(void) {
    @autoreleasepool {
        for (NSNumber *value in @[@0, @1, @255]) {
            Boolean enabled = value.unsignedCharValue;
            NSDictionary *screen = DKSTScreenMetadata(CGRectMake(0, 0, 1512, 982), 42, @"Test app", @[], enabled);
            NSDictionary *result = @{ @"screen": screen, @"frontmost": DKSTJSONBool(enabled) };
            NSData *json = [NSJSONSerialization dataWithJSONObject:result options:0 error:nil];
            puts([[NSString alloc] initWithData:json encoding:NSUTF8StringEncoding].UTF8String);
        }
    }
    return 0;
}
`
	if err := os.WriteFile(source, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "contract")
	if out, err := exec.Command("/usr/bin/clang", "-fobjc-arc", "-I", root, source, "-framework", "Cocoa", "-framework", "ApplicationServices", "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("compile native contract: %v\n%s", err, out)
	}
	out, err := exec.Command(binary).CombinedOutput()
	if err != nil {
		t.Fatalf("native contract: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected three native Boolean cases: %s", out)
	}
	for i, line := range lines {
		var got struct {
			Screen    Screen `json:"screen"`
			Frontmost bool   `json:"frontmost"`
		}
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("native JSON incompatible with Go: %v\n%s", err, line)
		}
		want := i != 0
		if got.Screen.Accessibility != want || got.Frontmost != want || got.Screen.FrontmostPID != 42 || got.Screen.DesktopWidth != 1512 {
			t.Fatalf("native metadata lost values: %s", line)
		}
	}
}
