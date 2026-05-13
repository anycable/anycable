package anycable

import "testing"

// TestClientPackageCompiles is a compile-only sanity check that proves the
// forked client package builds and that its exported symbols resolve. Real
// connect/subscribe/receive coverage lands with U4.
func TestClientPackageCompiles(t *testing.T) {
	var (
		_ *Client       = nil
		_ *Subscription = nil
		_ Event
		_ Command
	)
}
