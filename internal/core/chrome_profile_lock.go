package core

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

var errChromeProfileInUse = errors.New("Chrome 번역 프로필을 다른 게이트웨이가 사용 중입니다. 다른 게이트웨이를 종료한 뒤 다시 준비해 주세요")

// Keep the same inode for the lifetime of the profile. Removing this file
// would allow another process to lock a different inode while an owner lives.
func acquireChromeProfileLock(profile string) (*os.File, error) {
	file, err := os.OpenFile(filepath.Join(profile, ".dkst-worker.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errors.New("Chrome 번역 프로필에 접근할 수 없습니다")
	}
	if err = lockChromeProfileFile(file); err != nil {
		file.Close()
		return nil, errChromeProfileInUse
	}
	_ = file.Truncate(0)
	_, _ = file.WriteAt([]byte(strconv.Itoa(os.Getpid())), 0)
	return file, nil
}
