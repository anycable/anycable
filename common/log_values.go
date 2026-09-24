package common

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/anycable/anycable-go/logger"
)

// Signed streams (Turbo Streams, CableReady, AnyCable signed streams) have the form of "<data>--<digest>",
// where the data part is a Base64-encoded stream name and the digest is a hex-encoded HMAC.
// Only the digest is sensitive, so we match it by its shape; that makes it work for escaped identifiers, too
// (e.g., within confirmation messages).
var signedStreamDigestRx = regexp.MustCompile(`--([0-9a-fA-F]{32,})`)

type filteredIdentifier struct {
	val string
}

func (f *filteredIdentifier) String() string {
	if !logger.FilteringEnabled() {
		return f.val
	}

	matches := signedStreamDigestRx.FindAllStringSubmatchIndex(f.val, -1)

	if matches == nil {
		return f.val
	}

	var result strings.Builder

	last := 0

	for _, m := range matches {
		start, end := m[2], m[3]

		result.WriteString(f.val[last:start])
		result.WriteString(logger.MaskValue(f.val[start:end]))
		last = end
	}

	result.WriteString(f.val[last:])

	return result.String()
}

func (f *filteredIdentifier) LogValue() slog.Value {
	return slog.StringValue(f.String())
}

// FilteredIdentifier wraps a channel identifier (or any string containing it, e.g., a confirmation message)
// to show it in log with signed stream digests masked
func FilteredIdentifier(identifier string) *filteredIdentifier {
	return &filteredIdentifier{identifier}
}

// filteredTransmissions masks signed stream digests in transmissions and truncates them
func filteredTransmissions(transmissions []string) []slog.LogValuer {
	res := make([]slog.LogValuer, len(transmissions))

	for i, msg := range transmissions {
		res[i] = logger.CompactValue(FilteredIdentifier(msg).String())
	}

	return res
}
