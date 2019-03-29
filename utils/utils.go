package utils

import (
	"net/http"
	"os"

	nanoid "github.com/matoous/go-nanoid"
	"github.com/mattn/go-isatty"
)

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
