//go:build windows

package txadapter

import (
	"errors"
	"syscall"
)

func isCrossDevice(err error) bool {
	// ERROR_NOT_SAME_DEVICE = 17
	return errors.Is(err, syscall.Errno(17))
}
