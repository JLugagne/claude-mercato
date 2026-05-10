//go:build unix

package txadapter

import (
	"errors"
	"syscall"
)

func isCrossDevice(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}
