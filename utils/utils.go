package utils

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"syscall"

	nanoid "github.com/matoous/go-nanoid"
	"github.com/mattn/go-isatty"
)

const remoteAddrHeader = "REMOTE_ADDR"

// IsTTY returns true if program is running with TTY
func IsTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

// FetchHeaders extracts specified headers from request
func FetchHeaders(r *http.Request, list []string) map[string]string {
	res := make(map[string]string)

	for _, header := range list {
		res[header] = r.Header.Get(header)
	}
	res[remoteAddrHeader], _, _ = net.SplitHostPort(r.RemoteAddr)
	return res
}

// FetchUID safely extracts uid from `X-Request-ID` header or generates a new one
func FetchUID(r *http.Request) (string, error) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		return nanoid.Nanoid()
	}

	return requestID, nil
}

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
