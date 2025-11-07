//go:build !windows

package utils

import (
	"fmt"
	"syscall"
)

// OpenFileLimit returns a string displaying the current open file limit
// for the process or unknown if it's not possible to detect it
func OpenFileLimit() (limitStr string) {
	var rLimit syscall.Rlimit

	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		limitStr = "unknown"
	} else {
		limitStr = fmt.Sprintf("%d", rLimit.Cur)
	}

	return
}
