package utils

import (
	"os"

	"github.com/mattn/go-isatty"
)

// IsTTY returns true if program is running with TTY
func IsTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}
