//go:build !unix

package program

import (
	"os"
)

var shutdownSignals = []os.Signal{os.Interrupt}
