package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
)

// IsTTY returns true if program is running with TTY
func IsTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

func ToJSON[T any](val T) []byte {
	jsonStr, err := json.Marshal(&val)
	if err != nil {
		panic(fmt.Sprintf("😲 Failed to build JSON for %v: %v", val, err))
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

// NextRetry returns a cooldown duration before next attempt using
// a simple exponential backoff
func NextRetry(step int) time.Duration {
	if step == 0 {
		return 250 * time.Millisecond
	}

	left := math.Pow(2, float64(step))
	right := 2 * left

	secs := left + (right-left)*rand.Float64() // nolint:gosec
	return time.Duration(secs) * time.Second
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}
