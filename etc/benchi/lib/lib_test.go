package lib_test

import (
	"testing"

	"github.com/anycable/anycable-go/cli"
	anycable "github.com/anycable/anycable-go/etc/benchi/client"
	"github.com/stretchr/testify/require"
)

// Compile-time references that anchor symbols from both the root anycable-go
// module (resolved via the etc/benchi/go.mod replace directive) and the
// forked WebSocket client. If either link breaks, this file fails to build.
var (
	_ *cli.Embedded = nil
	_ *anycable.Client
)

// TestModuleLinks proves the benchi module's wiring is real: the test binary
// links against both anycable-go/cli (replace target) and the local forked
// client package. The assertion itself is trivial; the linkage is the test.
func TestModuleLinks(t *testing.T) {
	require.Equal(t, 1, 1)
}
