package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCliConfig(t *testing.T) {
	_, err, _ := NewConfigFromCLI([]string{"-h"})
	require.NoError(t, err)
}
