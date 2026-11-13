//go:build !windows

package core

import (
	"context"
	"errors"
	"github.com/chromedp/chromedp"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOrphanChromeVerification(t *testing.T) {
	executable := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	profile := "/Users/test/App Data/chrome-translator-profile"
	good := []string{executable, "--headless", "--user-data-dir=" + profile}
	if !verifiedOrphanChrome(1, good, executable, profile) {
		t.Fatal("exact orphan must match")
	}
	for _, tc := range []struct {
		parent int
		args   []string
	}{{42, good}, {1, []string{executable, "--user-data-dir=" + profile}}, {1, []string{executable, "--headless", "--user-data-dir=" + profile + " another-profile"}}, {1, []string{"/usr/bin/other", "--headless", "--user-data-dir=" + profile}}} {
		if verifiedOrphanChrome(tc.parent, tc.args, executable, profile) {
			t.Fatal("unsafe process match", tc)
		}
	}
	args, err := chromeProcessArgs(os.Getpid())
	if err != nil || len(args) == 0 || filepath.Base(args[0]) != filepath.Base(os.Args[0]) {
		t.Fatalf("OS argument inspection: %v %v", args, err)
	}
}

func TestChromeProfileBrowserIntegration(t *testing.T) {
	if os.Getenv("DKST_CHROME_PROFILE_INTEGRATION") != "1" {
		t.Skip("explicit Chrome profile test not requested")
	}
	profile := os.Getenv("DKST_CHROME_TRANSLATION_PROFILE")
	if profile == "" {
		profile = t.TempDir()
	}
	first, second := newChromeTranslator(profile), newChromeTranslator(profile)
	defer first.Close()
	defer second.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req := chromeTranslationRequest{Source: "ko", Target: "en", Text: "커피 드시겠어요?"}
	if _, err := first.run(ctx, req, true); err != nil {
		t.Fatal(err)
	}
	if _, err := second.run(ctx, req, false); !errors.Is(err, errChromeProfileInUse) {
		t.Fatalf("duplicate gateway must be blocked: %v", err)
	}
	executable, err := regularChromeExecutable()
	if err != nil {
		t.Fatal(err)
	}
	process := chromedp.FromContext(first.tab).Browser.Process()
	args, err := chromeProcessArgs(process.Pid)
	if err != nil || !verifiedOrphanChrome(1, args, executable, profile) {
		t.Fatalf("real Chrome argument verification: %v", err)
	}
	if err := recoverOrphanChrome(profile, executable); !errors.Is(err, errChromeProfileInUse) {
		t.Fatalf("live owner must never be reaped: %v", err)
	}
	if result, err := first.run(ctx, req, false); err != nil || !strings.Contains(strings.ToLower(result.Text), "coffee") {
		t.Fatalf("live owner interrupted: %+v %v", result, err)
	}
	first.Close()
	if result, err := second.run(ctx, req, false); err != nil || !strings.Contains(strings.ToLower(result.Text), "coffee") {
		t.Fatalf("profile reuse after shutdown: %+v %v", result, err)
	}
	t.Log("duplicate worker rejected, live Chrome protected, profile reused after graceful shutdown")
}
