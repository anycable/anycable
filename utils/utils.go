package utils

import (
	"encoding/json"
	"os"

	"github.com/apex/log"
	"github.com/mattn/go-isatty"
)

// IsTTY returns true if program is running with TTY
func IsTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

func ToJSON[T any](val T) []byte {
	jsonStr, err := json.Marshal(&val)
	if err != nil {
		log.Fatalf("😲 Failed to build JSON for %v: %v", val, err)
	}
	return jsonStr
}

func Keys[T any](val map[string]T) []string {
	var res = make([]string, len(val))

	i := 0

	for k := range val {
		res[i] = k
		i++
	}

	return res
}
