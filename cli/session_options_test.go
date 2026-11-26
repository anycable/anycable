package cli

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/anycable/anycable-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionOptionsFromParamsPingInterval(t *testing.T) {
	r := &Runner{log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	tests := []struct {
		val   string
		valid bool
	}{
		{"-1", false},
		{"0", false},
		{"abc", false},
		{"1", true},
		{"86400", true},
		{"86401", true},
		{"3074457346", true},
		{"9223372036", true},
		{"9223372037", true},
		{"6148914691236517206", true},
		{"20211507185753197", true},
		{"99999999999999999999", false},
	}

	for _, tt := range tests {
		t.Run(tt.val, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/cable?pi="+tt.val, nil)
			info, err := server.NewRequestInfo(req, nil)
			require.NoError(t, err)

			opts := r.sessionOptionsFromParams(info)

			if tt.valid {
				assert.Len(t, opts, 1)
			} else {
				assert.Empty(t, opts)
			}
		})
	}
}
