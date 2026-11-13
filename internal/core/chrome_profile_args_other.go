//go:build !windows && !linux && !darwin

package core

import "errors"

func chromeProcessArgs(pid int) ([]string, error) {
	return nil, errors.New("process verification unavailable")
}
