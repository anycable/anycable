package ds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeOffset(t *testing.T) {
	tests := []struct {
		name   string
		offset uint64
		epoch  string
		want   string
	}{
		{"without epoch", 123, "", "0"},
		{"with epoch", 123, "epoch1", "123::epoch1"},
		{"zero offset", 0, "epoch1", "0::epoch1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeOffset(tt.offset, tt.epoch)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecodeOffset(t *testing.T) {
	tests := []struct {
		name       string
		offsetStr  string
		wantOffset uint64
		wantEpoch  string
		wantErr    bool
	}{
		{"simple offset", "123", 123, "", false},
		{"with epoch", "123::epoch1", 123, "epoch1", false},
		{"start", "-1", 0, "", false},
		{"now", "now", 0, "", false},
		{"empty", "", 0, "", false},
		{"invalid", "abc", 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset, epoch, err := DecodeOffset(tt.offsetStr)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantOffset, offset)
				assert.Equal(t, tt.wantEpoch, epoch)
			}
		})
	}
}
