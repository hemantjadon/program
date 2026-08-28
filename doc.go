// Package program provides a small, opinionated harness for writing Go
// programs whose func main is a one-liner.
//
// The harness centers on three types:
//
//   - [Runner] is an interface implemented by application code. Its single
//     Run method receives a [Process] and returns an error. [RunnerFunc]
//     adapts a plain function into a Runner.
//
//   - [Process] is a small abstraction over process-level resources
//     (command-line arguments, environment, standard I/O streams) and
//     process-level interactions (delivered OS signals, graceful shutdown
//     notification). Runners interact with the outside world through a
//     Process rather than reaching into the [os] package directly, which
//     keeps them straightforward to test via [NewProcess] and the various
//     [ProcessOption] helpers.
//
//   - [Main] wires everything together: it builds a Process, installs signal
//     handlers, invokes Runner.Run, and translates the returned error into a
//     numeric [ExitCode] suitable for [os.Exit].
//
// # Canonical usage
//
// A typical program reduces func main to a single line:
//
//	func main() {
//	    os.Exit(int(program.Main(myRunner)))
//	}
//
// where myRunner is any value implementing [Runner]. See the package
// examples for variations covering plain errors, [Exit], and [ExitErr].
//
// # Shutdown and signals
//
// [Main] handles SIGINT and SIGTERM internally with two-stage escalation:
//
//  1. The first signal closes the channel returned by [Process.Shutdown].
//     The Runner is expected to observe this and begin a graceful shutdown,
//     then return.
//  2. A subsequent signal received before the Runner returns is treated as
//     a request for forceful termination: [Main] returns [ExitCodeShutdown]
//     without waiting for the Runner.
//
// Additional signals can be delivered to the Runner via [Process.Signals]
// by registering them with [WithSignals]. SIGINT and SIGTERM are reserved
// for the shutdown protocol above and are silently excluded from this
// channel even if listed.
//
// # Exit codes
//
// [Main] returns one of:
//
//   - [ExitCodeOK] (0) when Runner.Run returns nil.
//   - [ExitCodeShutdown] (1) when forceful termination is requested before
//     the Runner returns.
//   - [ExitCodeErr] (2) when Runner.Run returns a non-nil error that does
//     not implement [ExitError].
//   - The code reported by [ExitError.ExitCode] when Runner.Run returns an
//     error that implements [ExitError]. [Exit] and [ExitErr] are the
//     standard ways to construct such errors; values 3-255 are recommended
//     for domain-specific codes.
//
// When Runner.Run returns a non-nil error, its message is written to
// [Process.Stderr] before [Main] returns, except that an error whose
// Error method returns the empty string is not printed. This makes [Exit]
// a clean "exit with this code, no message" primitive.
//
// # Panics
//
// [Main] does not recover panics from Runner.Run; a panicking Runner
// bypasses the shutdown protocol and terminates the process in the usual
// Go fashion. Runners that want to survive panics in their own goroutines
// must recover them themselves.
package program
