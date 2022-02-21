//go:build (!darwin && !linux) || !mrb
// +build !darwin,!linux !mrb

package metrics

import "errors"

// NewCustomPrinter generates log formatter from the provided (as path)
// Ruby script
func NewCustomPrinter(path string) (*BasePrinter, error) {
	return nil, errors.New("Not supported")
}
