//go:build !windows

package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func lockChromeProfileFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}

func verifiedOrphanChrome(parent int, args []string, executable, profile string) bool {
	if parent != 1 || len(args) == 0 || filepath.Clean(args[0]) != filepath.Clean(executable) {
		return false
	}
	headless, matchingProfile := false, false
	for _, arg := range args[1:] {
		if arg == "--headless" || arg == "--headless=new" {
			headless = true
		}
		if arg == "--user-data-dir="+profile {
			matchingProfile = true
		}
	}
	return headless && matchingProfile
}
func recoverOrphanChrome(profile, executable string) error {
	lock, err := os.Readlink(filepath.Join(profile, "SingletonLock"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errChromeProfileInUse
	}
	split := strings.LastIndex(lock, "-")
	if split < 1 {
		return errChromeProfileInUse
	}
	host, err := os.Hostname()
	if err != nil || lock[:split] != host {
		return errChromeProfileInUse
	}
	pid, err := strconv.Atoi(lock[split+1:])
	if err != nil || pid <= 1 {
		return errChromeProfileInUse
	}
	if err = unix.Kill(pid, 0); errors.Is(err, unix.ESRCH) {
		return nil
	} else if err != nil {
		return errChromeProfileInUse
	}
	inspect, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	row, err := exec.CommandContext(inspect, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "ppid=").Output()
	parent, parentErr := strconv.Atoi(strings.TrimSpace(string(row)))
	args, argsErr := chromeProcessArgs(pid)
	if err != nil || parentErr != nil || argsErr != nil || !verifiedOrphanChrome(parent, args, executable, profile) {
		return errChromeProfileInUse
	}
	// Only the exact dedicated headless process, already adopted by init, is
	// eligible. Never remove Chromium's singleton files or terminate live owners.
	if err = unix.Kill(pid, unix.SIGTERM); err != nil && !errors.Is(err, unix.ESRCH) {
		return errChromeProfileInUse
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if errors.Is(unix.Kill(pid, 0), unix.ESRCH) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("이전 Chrome 번역 프로세스가 종료 중입니다. 잠시 후 다시 준비해 주세요")
}
