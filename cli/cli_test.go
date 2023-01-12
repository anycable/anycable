package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestCliConfig(t *testing.T) {
	_, err, _ := NewConfigFromCLI([]string{"-h"})
	require.NoError(t, err)
}

func TestCliConfigCustom(t *testing.T) {
	var custom string

	_, err, _ := NewConfigFromCLI(
		[]string{"", "-custom=foo"},
		WithCLICustomOptions(func() ([]cli.Flag, error) {
			flag := &cli.StringFlag{
				Name:        "custom",
				Destination: &custom,
				Value:       "bar",
				Required:    false,
			}
			return []cli.Flag{flag}, nil
		}),
	)

	require.NoError(t, err)
	assert.Equal(t, "foo", custom)
}
