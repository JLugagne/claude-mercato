//go:build !windows

package cfgadapter

import (
	"errors"
	"syscall"
)

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return !errors.Is(err, syscall.ESRCH)
}
