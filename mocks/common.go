package mocks

import "github.com/anycable/anycable-go/common"

// NewMockResult builds a new result with sid as transmission
func NewMockResult(sid string) *common.CommandResult {
	return &common.CommandResult{Transmissions: []string{sid}, Disconnect: false, StopAllStreams: false}
}
