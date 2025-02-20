package broker

import "github.com/anycable/anycable-go/common"

// Presenter is responsible for handling presence related events (join, leave, etc.)
//
//go:generate mockery --name Presenter --structname MockPresenter --filename presenter_test.go --output "../broker" --outpkg broker
type Presenter interface {
	HandleJoin(stream string, msg *common.PresenceEvent)
	HandleLeave(stream string, msg *common.PresenceEvent)
}

// We can extend the presence read functionality in the future
// (e.g., add pagination, filtering, etc.)
type PresenceInfoOptions struct {
	ReturnRecords bool `json:"return_records,omitempty"`
}

func NewPresenceInfoOptions() *PresenceInfoOptions {
	return &PresenceInfoOptions{ReturnRecords: true}
}

type PresenceInfoOption func(*PresenceInfoOptions)

func WithPresenceInfoOptions(opts *PresenceInfoOptions) PresenceInfoOption {
	return func(o *PresenceInfoOptions) {
		if opts != nil {
			*o = *opts
		}
	}
}
