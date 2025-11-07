//go:build (!darwin && !linux) || !mrb

package metrics

import (
	"errors"
	"log/slog"
)

// NewCustomPrinter generates log formatter from the provided (as path)
// Ruby script
func NewCustomPrinter(path string, l *slog.Logger) (*BasePrinter, error) {
	return nil, errors.New("unsupported")
}
