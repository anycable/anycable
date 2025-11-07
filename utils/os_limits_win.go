//go:build windows

package utils

// OpenFileLimit is not supported on Windows
func OpenFileLimit() string {
	return "unsupported"
}
