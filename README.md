# program

A small, opinionated Go harness for writing programs whose `func main` is a
one-liner. It centralizes signal handling, graceful shutdown, and exit-code
translation so application code can focus on what it actually does.

```go
import "github.com/hemantjadon/program"
```

## Why

Most Go binaries grow a near-identical preamble in `main`: parse signals, plumb
a context for cancellation, translate errors into exit codes, and print error 
messages to stderr. `program` packages that boilerplate behind three types so a
typical `main` collapses to:

```go
func main() {
	os.Exit(int(program.Main(myRunner)))
}
```

## Concepts

The harness is built around three types defined in `program.go` and `process.go`:

- [`Runner`](program.go) — interface implemented by application code. Its single `Run(*Process) error` method does the work and returns an error. [`RunnerFunc`](program.go) adapts a plain function into a `Runner`.
- [`Process`](process.go) — abstraction over process-level resources (args, env, stdio) and process-level interactions (signal delivery, graceful-shutdown notification). Runners interact with the outside world through a `Process` instead of reaching into `os` directly, which keeps them straightforward to test via `NewProcess` and the `ProcessOption` helpers.
- [`Main`](program.go) — wires everything together: builds a `Process`, installs signal handlers, invokes `Runner.Run`, and translates the returned error into a numeric [`ExitCode`](program.go) suitable for `os.Exit`.

## Usage

A minimal program:

```go
package main

import (
	"fmt"
	"os"

	"github.com/hemantjadon/program"
)

func main() {
	runner := program.RunnerFunc(func(p *program.Process) error {
		fmt.Fprintln(p.Stdout(), "hello")
		<-p.Shutdown() // block until SIGINT/SIGTERM
		return nil
	})
	os.Exit(int(program.Main(runner)))
}
```

See `example_test.go` for runnable variants covering plain errors, [`Exit`](program.go), and [`ExitErr`](program.go).

## Shutdown and signals

`Main` handles `SIGINT` and `SIGTERM` internally with two-stage escalation:

1. The first signal closes the channel returned by `Process.Shutdown()`. The `Runner` is expected to observe this and begin a graceful shutdown, then return.
2. A subsequent signal received before the `Runner` returns is treated as a request for forceful termination: `Main` returns `ExitCodeShutdown` without waiting for the `Runner`.

Additional signals can be delivered to the `Runner` via `Process.Signals()` by
registering them with `WithSignals`. `SIGINT` and `SIGTERM` are reserved for the
shutdown protocol above and are silently excluded from this channel even if 
listed.

## Exit codes

`Main` returns one of:

|  Code | Constant             | Meaning                                                                      |
|------:|----------------------|------------------------------------------------------------------------------|
|     0 | `ExitCodeOK`         | `Runner.Run` returned `nil`.                                                 |
|     1 | `ExitCodeShutdown`   | Forceful termination requested before the `Runner` returned.                 |
|     2 | `ExitCodeErr`        | `Runner.Run` returned a non-nil error that does not implement `ExitError`.   |
| 3-255 | `ExitError.ExitCode` | Recommended range for domain-specific codes returned via `Exit` / `ExitErr`. |

`Exit(code)` produces an `ExitError` with no message — a clean "exit with this 
code, no message" primitive. `ExitErr(err, code)` wraps an existing error with a
custom code; its message is written to `Process.Stderr()` before `Main` returns.

When `Runner.Run` returns a non-nil error, its message is written to 
`Process.Stderr()` before `Main` returns, except that an error whose `Error`
method returns the empty string is not printed.

## Panics

`Main` does not recover panics from `Runner.Run`; a panicking `Runner` bypasses
the shutdown protocol and terminates the process in the usual Go fashion. 
Runners that want to survive panics in their own goroutines must recover them 
themselves.

## Testing

`Process` is intentionally constructed via options so runners can be exercised 
without `Main`:

```go
p := program.NewProcess(
	program.WithArgs([]string{"prog", "--flag"}),
	program.WithStdout(&buf),
	program.WithShutdownChan(shutdownCh),
)
err := myRunner.Run(p)
```

See `program_test.go` and `process_test.go` for the full set of patterns.

## Development

Run the test suite the same way CI does (race detector on, coverage emitted to `coverage.out`):

```sh
go test -v -race -timeout 5m -coverprofile=coverage.out -covermode=atomic ./...
```

Render coverage as a function-level summary:

```sh
go tool cover -func=coverage.out
```

Lint with the version pinned in `.github/workflows/ci.yaml` ([`golangci-lint`](https://golangci-lint.run) v2.13.2,
configured by `.golangci.yaml`):

```sh
golangci-lint run --timeout 5m ./...
```

If `golangci-lint` is not on your `PATH`, install the pinned version into 
`$(go env GOPATH)/bin`. Either of the following works; pick the one that matches
how the rest of your tooling is installed.

Via the upstream install script (matches the binary CI uses):

```sh
curl -sSfL raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
	| sh -s -- -b "$(go env GOPATH)/bin" v2.13.2
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`, then verify:

```sh
golangci-lint --version
```