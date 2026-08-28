package program

import (
	"io"
	"os"
	"slices"
	"strings"
)

// Process provides access to process-level resources and interactions.
//
// Static resources such as command-line arguments, environment variables,
// and standard I/O streams are available for the lifetime of the process.
//
// Dynamic interactions such as OS signal delivery and graceful shutdown
// notification occur while the process is running.
type Process struct {
	args   []string
	env    []string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	signalsCh  <-chan os.Signal
	shutdownCh <-chan struct{}
}

// NewProcess creates a new [Process] with OS defaults.
//
// Use [ProcessOption] functions to override defaults. This is primarily
// useful for testing [Runner] implementations outside [Main].
func NewProcess(opts ...ProcessOption) *Process {
	p := Process{
		args:   slices.Clone(os.Args),
		env:    os.Environ(),
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,

		signalsCh:  make(<-chan os.Signal),
		shutdownCh: make(<-chan struct{}),
	}
	for _, opt := range opts {
		opt(&p)
	}
	return &p
}

// ProcessOption configures a [Process] created via [NewProcess].
type ProcessOption func(*Process)

// WithArgs overrides the command-line arguments visible to the process.
func WithArgs(args []string) ProcessOption {
	return func(p *Process) {
		p.args = slices.Clone(args)
	}
}

// WithEnv overrides the environment variables visible to the process.
func WithEnv(env []string) ProcessOption {
	return func(p *Process) {
		p.env = slices.Clone(env)
	}
}

// WithStdin overrides the standard input reader.
//
// If the reader is nil, this is essentially a no-op.
func WithStdin(r io.Reader) ProcessOption {
	return func(p *Process) {
		if !isNil(r) {
			p.stdin = r
		}
	}
}

// WithStdout overrides the standard output writer.
//
// If the writer is nil, this is essentially a no-op.
func WithStdout(w io.Writer) ProcessOption {
	return func(p *Process) {
		if !isNil(w) {
			p.stdout = w
		}
	}
}

// WithStderr overrides the standard error writer.
//
// If the writer is nil, this is essentially a no-op.
func WithStderr(w io.Writer) ProcessOption {
	return func(p *Process) {
		if !isNil(w) {
			p.stderr = w
		}
	}
}

// WithSignalsChan sets the channel from which the [Runner] receives OS signals
// via [Process.Signals].
//
// This is useful for testing signal handling behavior outside [Main].
//
// If ch is nil, WithSignalsChan is a no-op and [Process.Signals] continues to
// return a never-firing channel.
func WithSignalsChan(ch <-chan os.Signal) ProcessOption {
	return func(p *Process) {
		if !isNil(ch) {
			p.signalsCh = ch
		}
	}
}

// WithShutdownChan sets the channel observed by the [Runner] via
// [Process.Shutdown] to detect graceful shutdown requests.
//
// This is useful for testing shutdown handling behavior outside [Main].
//
// If ch is nil, WithShutdownChan is a no-op and [Process.Shutdown] continues
// to return a never-firing channel.
func WithShutdownChan(ch <-chan struct{}) ProcessOption {
	return func(p *Process) {
		if !isNil(ch) {
			p.shutdownCh = ch
		}
	}
}

// Args returns the command-line arguments passed to the process.
//
// The returned slice is shared with all callers and should not be modified.
func (p *Process) Args() []string {
	return p.args
}

// Env returns the environment variables passed to the process.
//
// The returned slice is shared with all callers and should not be modified.
func (p *Process) Env() []string {
	return p.env
}

// LookupEnv retrieves the value of the environment variable named by the
// key. If the variable is present, the value (which may be empty) is
// returned and the boolean is true. Otherwise the returned value will be
// empty and the boolean will be false.
//
// Key matching is case-sensitive, matching [os.LookupEnv] on Unix.
//
// When the environment contains duplicate entries for the same key, the
// value of the first occurrence is returned.
func (p *Process) LookupEnv(key string) (string, bool) {
	if key == "" {
		return "", false
	}
	for _, e := range p.env {
		if k, v, ok := strings.Cut(e, "="); ok && k == key {
			return v, true
		}
	}
	return "", false
}

// Getenv retrieves the value of the environment variable named by the key.
// It returns the value, which will be empty if the variable is not present.
//
// To distinguish between an empty value and an unset value, use
// [Process.LookupEnv].
func (p *Process) Getenv(key string) string {
	v, _ := p.LookupEnv(key)
	return v
}

// Stdin returns the standard input stream.
func (p *Process) Stdin() io.Reader {
	return p.stdin
}

// Stdout returns the standard output stream.
func (p *Process) Stdout() io.Writer {
	return p.stdout
}

// Stderr returns the standard error stream.
func (p *Process) Stderr() io.Writer {
	return p.stderr
}

// Signals returns a receive-only channel that delivers OS signals registered
// via [WithSignals].
//
// This channel will never receive SIGINT or SIGTERM signals, as they are
// handled by [Main] directly. Users of [Process] must use [Process.Shutdown]
// to detect when the program has been asked to shut down gracefully.
//
// Reads should not block: [signal.Notify] performs a non-blocking send and will
// drop signals if the channel is full.
//
// If no signals were registered, the returned channel never receives any value.
func (p *Process) Signals() <-chan os.Signal {
	return p.signalsCh
}

// Shutdown returns a receive-only channel that is closed when the program has
// been asked to shut down gracefully (e.g. on SIGINT or SIGTERM under [Main]).
//
// Runners should select on this channel and begin a graceful shutdown when it
// closes, then return.
//
// If no shutdown channel was set via [WithShutdownChan], the returned channel
// never receives any value and is never closed.
func (p *Process) Shutdown() <-chan struct{} {
	return p.shutdownCh
}
