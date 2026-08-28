package program

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"slices"
)

// ExitCode is the exit code returned by [Main].
type ExitCode int

const (
	// ExitCodeOK is returned when the program runs successfully, along with all
	// the cleanups.
	ExitCodeOK ExitCode = 0

	// ExitCodeShutdown is returned when the program is forcefully shut down
	// by repeated Interrupt or Term Signals (SIGINT or SIGTERM).
	ExitCodeShutdown ExitCode = 1

	// ExitCodeErr is returned when the program exits with an error while
	// running.
	ExitCodeErr ExitCode = 2
)

// ExitError is an error that carries a specific [ExitCode].
//
// Any error type can implement this interface to control the exit code.
type ExitError interface {
	error
	ExitCode() ExitCode
}

// Exit returns an error that implements [ExitError] with the given exit code
// and no message. When returned by [Runner.Run], [Main] returns code without
// writing anything to [Process.Stderr].
//
// It is recommended to use a value between 3 and 255 for domain-specific
// codes. The values 0, 1, 2 are reserved for generic success, shutdown, and
// failure cases respectively.
func Exit(code ExitCode) error {
	return &exitError{code: code}
}

// ExitErr returns an error that implements [ExitError], wrapping err with the
// given exit code. When returned by [Runner.Run], [Main] writes err's message
// to [Process.Stderr] and returns code.
//
// If err is nil, the returned error behaves like [Exit](code) ie. no message is
// written to [Process.Stderr].
//
// It is recommended to use a value between 3 and 255 for domain-specific
// codes. The values 0, 1, 2 are reserved for generic success, shutdown, and
// failure cases respectively.
func ExitErr(err error, code ExitCode) error {
	return &exitError{err: err, code: code}
}

type exitError struct {
	err  error
	code ExitCode
}

func (e *exitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	return e.err
}

func (e *exitError) ExitCode() ExitCode {
	return e.code
}

// Runner is the interface that wraps the Run method.
type Runner interface {
	// Run runs the program. It receives a [Process] that provides access to
	// process-level resources and interactions.
	//
	// The Runner is responsible for its own resource lifecycle: it should
	// set up any resources it needs, run the program, and clean up
	// before returning.
	//
	// Runners are responsible for handling their panics.
	//
	// Runner should select on [Process.Shutdown] and begin graceful shutdown
	// when the channel is closed.
	//
	// If the program ran successfully to completion along with graceful
	// shutdown, it should return nil, which makes the Main return [ExitCodeOK].
	//
	// If the Runner returns an error, Main will return the appropriate
	// [ExitCode] based on the error type.
	//
	// If the returned error implements [ExitError], its [ExitError.ExitCode]
	// determines the [ExitCode] to return.
	//
	// If the returned error does not implement [ExitError], [ExitCodeErr] is
	// used instead.
	//
	// If the Runner doesn't return upon the closure of the [Process.Shutdown]
	// channel and the process receives further termination signals, then Main
	// returns with [ExitCodeShutdown], indicating a forceful termination.
	Run(process *Process) error
}

// RunnerFunc is a function that implements the [Runner] interface.
type RunnerFunc func(*Process) error

// Run calls the function with the given process.
func (f RunnerFunc) Run(process *Process) error {
	return f(process)
}

// Main orchestrates the lifecycle of the [Runner]. It calls [Runner.Run] to
// execute it and returns the appropriate [ExitCode].
//
// It assembles the process-level resources as a [Process] and passes it to the
// [Runner] for cleaner interactions.
//
// Any signals registered via [WithSignals] are delivered to the [Runner]
// through [Process.Signals].
//
// It handles OS signals (SIGINT, SIGTERM) internally with two-stage escalation:
//
//  1. The first signal closes the [Process.Shutdown] channel, signaling the
//     [Runner] to stop and begin a graceful shutdown.
//  2. Any subsequent signal indicates a forceful termination, and Main returns
//     even if [Runner] has not completed yet.
//
// The [ExitCode] to be returned is determined as follows:
//
//   - [ExitCodeOK] if [Runner.Run] completes without error.
//
//   - [ExitError.ExitCode] if error returned by [Runner.Run] implements
//     [ExitError].
//
//   - [ExitCodeErr] for any other non-nil error returned by [Runner.Run].
//
//   - [ExitCodeShutdown] if multiple OS signals (SIGINT, SIGTERM) are received,
//     but the [Runner] has not stopped/returned.
//
// When [Runner.Run] returns a non-nil error, its message is written to
// [Process.Stderr] before Main returns. As a deliberate exception, an error
// whose Error method returns the empty string is not printed: this keeps
// [Exit] a clean "exit with this code, no message" pattern.
//
// If the [Runner] panics, Main will NOT recover the panic and hence it will
// remain unhandled, and all the graceful shutdown mechanisms will be bypassed.
func Main(runner Runner, opts ...Option) ExitCode {
	if isNil(runner) {
		panic("program.Main: nil runner")
	}

	cfg := config{
		signalNotifier: osSignalNotifier{},
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	done := make(chan struct{})
	defer close(done)

	// The size of the channel is 2 to handle the first signal and the second
	// signal, if the second signal is received before the first signal is
	// processed, it is buffered, because [signal.Notify] performs a
	// non-blocking send and will drop signals if the channel is full.
	stopCh := make(chan os.Signal, 2)
	cfg.signalNotifier.Notify(stopCh, shutdownSignals...)
	defer cfg.signalNotifier.Stop(stopCh)

	signalsCh := make(chan os.Signal, max(1, len(cfg.signals)))
	if len(cfg.signals) > 0 {
		cfg.signalNotifier.Notify(signalsCh, cfg.signals...)
		defer cfg.signalNotifier.Stop(signalsCh)
	}

	shutdownCh := make(chan struct{})
	go func() {
		select {
		case <-stopCh:
			close(shutdownCh)
		case <-done:
			return
		}
	}()

	process := NewProcess(WithSignalsChan(signalsCh), WithShutdownChan(shutdownCh))

	exitCodeCh := make(chan ExitCode, 1)
	go func() {
		var code ExitCode
		err := runner.Run(process)
		if err != nil {
			errMsg := err.Error()
			if errMsg != "" {
				_, _ = fmt.Fprintln(process.Stderr(), errMsg)
			}
			var ee ExitError
			if errors.As(err, &ee) {
				code = ee.ExitCode()
			} else {
				code = ExitCodeErr
			}
		} else {
			code = ExitCodeOK
		}
		exitCodeCh <- code
	}()

	// Pick the dominant exit pathway: either the runner returns before
	// any signal arrives (use its exit code as-is), or the first
	// SIGINT/SIGTERM closes shutdownCh and we move into the
	// graceful-shutdown phase below.
	select {
	case exitCode := <-exitCodeCh:
		return exitCode
	case <-shutdownCh:
		// First signal received. Wait for the runner to finish
		// gracefully (<-exitCodeCh) or for a second signal to force
		// termination (<-stopCh).
		select {
		case exitCode := <-exitCodeCh:
			return exitCode
		case <-stopCh:
			// Second signal escalated to forced shutdown, but the
			// runner may have returned at the same instant: when both
			// stopCh and exitCodeCh are ready, the middle select
			// above picks one at random, so <-stopCh winning that
			// toss does not imply the runner is still running. The
			// non-blocking <-exitCodeCh below recovers the runner's
			// exit code in that race; the default branch reports
			// ExitCodeShutdown only when the runner truly has not
			// finished.
			//
			// This race is reachable only via Go's random select
			// choice and goroutine scheduling, which makes a
			// deterministic regression test through Main impractical;
			// the structure here is the contract.
			select {
			case exitCode := <-exitCodeCh:
				return exitCode
			default:
				return ExitCodeShutdown
			}
		}
	}
}

type config struct {
	signals        []os.Signal
	signalNotifier signalNotifier
}

// Option configures [Main].
type Option func(*config)

// WithSignals registers OS signals to be delivered to the [Runner] via
// [Process.Signals].
//
// Shutdown signals (SIGINT, SIGTERM) are handled internally and are
// silently excluded even if listed here.
func WithSignals(signals ...os.Signal) Option {
	return func(c *config) {
		for _, sig := range signals {
			if slices.Contains(shutdownSignals, sig) {
				continue
			}
			c.signals = append(c.signals, sig)
		}
	}
}

func withSignalNotifier(sn signalNotifier) Option {
	return func(c *config) {
		if !isNil(sn) {
			c.signalNotifier = sn
		}
	}
}

type signalNotifier interface {
	Notify(channel chan<- os.Signal, signals ...os.Signal)
	Stop(channel chan<- os.Signal)
}

type osSignalNotifier struct{}

// Notify is the wrapper around [signal.Notify].
func (osSignalNotifier) Notify(channel chan<- os.Signal, signals ...os.Signal) {
	signal.Notify(channel, signals...)
}

// Stop is the wrapper around [signal.Stop].
func (osSignalNotifier) Stop(channel chan<- os.Signal) {
	signal.Stop(channel)
}

// isNil returns true if the value is nil.
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Chan, reflect.Func, reflect.Map, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
