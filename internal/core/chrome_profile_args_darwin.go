//go:build darwin

package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"golang.org/x/sys/unix"
)

func chromeProcessArgs(pid int) ([]string, error) {
	raw, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return nil, err
	}
	if len(raw) < 5 {
		return nil, errors.New("missing process arguments")
	}
	count := int(binary.LittleEndian.Uint32(raw[:4]))
	if count < 1 || count > 4096 {
		return nil, errors.New("invalid argument count")
	}
	raw = raw[4:]
	end := bytes.IndexByte(raw, 0)
	if end < 0 {
		return nil, errors.New("missing executable")
	}
	raw = bytes.TrimLeft(raw[end+1:], "\x00")
	args := make([]string, 0, count)
	for len(args) < count {
		end = bytes.IndexByte(raw, 0)
		if end < 0 {
			return nil, errors.New("truncated arguments")
		}
		args = append(args, string(raw[:end]))
		raw = raw[end+1:]
	}
	return args, nil
}
