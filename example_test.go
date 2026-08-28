package program_test

import (
	"errors"
	"fmt"

	"github.com/hemantjadon/program"
)

// ExampleMain shows the canonical use of [program.Main] from func main.
//
// A real func main is typically a single line:
//
//	func main() { os.Exit(int(program.Main(runner))) }
//
// The example here calls Main inline so it can also assert the returned
// [program.ExitCode].
func ExampleMain() {
	runner := program.RunnerFunc(func(p *program.Process) error {
		_, _ = fmt.Fprintln(p.Stdout(), "hello from runner")
		// A real runner would loop here, returning when
		// <-p.Shutdown() unblocks.
		return nil
	})

	code := program.Main(runner)
	fmt.Println("exit code:", code)

	// Output:
	// hello from runner
	// exit code: 0
}

// ExampleMain_error shows how a plain (non-[program.ExitError]) error
// returned by the runner becomes [program.ExitCodeErr].
//
// [program.Main] also writes the error's message to stderr; that line is not
// part of the verified output below because Go's example harness only captures
// stdout.
func ExampleMain_error() {
	runner := program.RunnerFunc(func(_ *program.Process) error {
		// A real runner would do some work here, then return when there is
		// some error condition is detected.
		return errors.New("boom")
	})

	code := program.Main(runner)
	fmt.Println("exit code:", code)

	// Output:
	// exit code: 2
}

// ExampleMain_exit shows the canonical "exit with this code, no
// message" pattern using [program.Exit]. [program.Main] detects the
// [program.ExitError] interface and returns the requested code; because
// [program.Exit]'s error has an empty message, nothing is written to
// stderr.
func ExampleMain_exit() {
	runner := program.RunnerFunc(func(_ *program.Process) error {
		return program.Exit(program.ExitCode(78))
	})

	code := program.Main(runner)
	fmt.Println("exit code:", code)

	// Output:
	// exit code: 78
}

// ExampleMain_exitErr shows how to pair a custom exit code with an error
// message using [program.ExitErr]. [program.Main] writes the wrapped
// error's message to stderr (not captured by Go's example harness) and
// returns the requested code.
func ExampleMain_exitErr() {
	runner := program.RunnerFunc(func(_ *program.Process) error {
		return program.ExitErr(errors.New("config invalid"), program.ExitCode(64))
	})

	code := program.Main(runner)
	fmt.Println("exit code:", code)

	// Output:
	// exit code: 64
}
