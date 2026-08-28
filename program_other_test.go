//go:build !unix

package program_test

import "testing"

// testMainFuncSignalDelivery is a no-op on non-unix targets.
//
// Unix variant relies on SIGUSR1/SIGUSR2, which are not defined on windows,
// plan9, js, or wasip1.
//
// See program_unix_test.go for real test body.
func testMainFuncSignalDelivery(t *testing.T) {
	t.Skip("signal_delivery uses SIGUSR1/SIGUSR2, which are only defined on unix")
}
