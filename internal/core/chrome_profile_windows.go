//go:build windows

package core

import (
	"golang.org/x/sys/windows"
	"os"
)

func lockChromeProfileFile(file *os.File) error {
	return windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
}

// Windows Chrome owns its process singleton; never guess a PID from it.
func recoverOrphanChrome(profile, executable string) error { return nil }
