package core

import (
	"errors"
	"os"
	"testing"
)

func TestChromeProfileLockLifecycle(t *testing.T) {
	profile := t.TempDir()
	first, err := acquireChromeProfileLock(profile)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate, err := acquireChromeProfileLock(profile); !errors.Is(err, errChromeProfileInUse) {
		if duplicate != nil {
			duplicate.Close()
		}
		t.Fatalf("live owner must retain lock: %v", err)
	}
	first.Close()
	next, err := acquireChromeProfileLock(profile)
	if err != nil {
		t.Fatalf("lock must release on close: %v", err)
	}
	next.Close()
	if _, err = os.Stat(profile + "/.dkst-worker.lock"); err != nil {
		t.Fatal("stable lock inode must be retained")
	}
}
