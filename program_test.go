package program_test

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/hemantjadon/program"
)

func TestExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code program.ExitCode
		want int
	}{
		{name: "ok", code: program.ExitCodeOK, want: 0},
		{name: "shutdown", code: program.ExitCodeShutdown, want: 1},
		{name: "err", code: program.ExitCodeErr, want: 2},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := int(tt.code); got != tt.want {
				t.Errorf("ExitCode = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExitErr(t *testing.T) {
	t.Parallel()

	t.Run("error_message", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			err     error
			wantMsg string
		}{
			{name: "with_message", err: errors.New("boom"), wantMsg: "boom"},
			{name: "nil_error", err: nil, wantMsg: ""},
			{name: "empty_message", err: errors.New(""), wantMsg: ""},
			{name: "wrapped_error", err: fmt.Errorf("outer: %w", errors.New("inner")), wantMsg: "outer: inner"},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ee := program.ExitErr(tt.err, program.ExitCode(3))
				if got := ee.Error(); got != tt.wantMsg {
					t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
				}
			})
		}
	})

	t.Run("exit_code", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			code program.ExitCode
		}{
			{name: "code_3", code: program.ExitCode(3)},
			{name: "code_0", code: program.ExitCodeOK},
			{name: "code_255", code: program.ExitCode(255)},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ee := program.ExitErr(errors.New("err"), tt.code)
				var exitErr program.ExitError
				if !errors.As(ee, &exitErr) {
					t.Fatal("ExitErr should implement ExitError")
				}
				if got := exitErr.ExitCode(); got != tt.code {
					t.Errorf("ExitCode() = %d, want %d", got, tt.code)
				}
			})
		}
	})

	t.Run("unwrap", func(t *testing.T) {
		t.Parallel()

		t.Run("with_underlying", func(t *testing.T) {
			t.Parallel()

			underlying := errors.New("root cause")
			ee := program.ExitErr(underlying, program.ExitCode(3))
			if !errors.Is(ee, underlying) {
				t.Error("should expose the underlying error via Unwrap")
			}
		})

		t.Run("nil_underlying", func(t *testing.T) {
			t.Parallel()

			ee := program.ExitErr(nil, program.ExitCode(3))
			unwrapper, ok := ee.(interface{ Unwrap() error })
			if !ok {
				t.Fatal("should implement Unwrap")
			}
			if got := unwrapper.Unwrap(); got != nil {
				t.Errorf("Unwrap() = %v, want nil", got)
			}
		})
	})
}

func TestExit(t *testing.T) {
	t.Parallel()

	t.Run("empty_message", func(t *testing.T) {
		t.Parallel()

		// Exit must produce an error whose Error() is empty so that Main
		// does not write anything to Process.Stderr.
		if got := program.Exit(program.ExitCode(3)).Error(); got != "" {
			t.Errorf("Error() = %q, want empty", got)
		}
	})

	t.Run("exit_code", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			code program.ExitCode
		}{
			{name: "code_3", code: program.ExitCode(3)},
			{name: "code_0", code: program.ExitCodeOK},
			{name: "code_255", code: program.ExitCode(255)},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ee := program.Exit(tt.code)
				var exitErr program.ExitError
				if !errors.As(ee, &exitErr) {
					t.Fatal("Exit should implement ExitError")
				}
				if got := exitErr.ExitCode(); got != tt.code {
					t.Errorf("ExitCode() = %d, want %d", got, tt.code)
				}
			})
		}
	})

	t.Run("unwrap_nil", func(t *testing.T) {
		t.Parallel()

		ee := program.Exit(program.ExitCode(3))
		unwrapper, ok := ee.(interface{ Unwrap() error })
		if !ok {
			t.Fatal("should implement Unwrap")
		}
		if got := unwrapper.Unwrap(); got != nil {
			t.Errorf("Unwrap() = %v, want nil", got)
		}
	})
}

func TestRunnerFunc(t *testing.T) {
	t.Parallel()

	t.Run("returns_nil", func(t *testing.T) {
		t.Parallel()

		var called bool
		f := program.RunnerFunc(func(_ *program.Process) error {
			called = true
			return nil
		})
		if err := f.Run(program.NewProcess()); err != nil {
			t.Errorf("Run() error = %v, want nil", err)
		}
		if !called {
			t.Error("underlying function was not called")
		}
	})

	t.Run("returns_error", func(t *testing.T) {
		t.Parallel()

		want := errors.New("fail")
		f := program.RunnerFunc(func(_ *program.Process) error {
			return want
		})
		//nolint:errorlint // intentional identity check: RunnerFunc must pass the exact error value through, not a wrapped copy.
		if got := f.Run(program.NewProcess()); got != want {
			t.Errorf("Run() error = %v, want %v", got, want)
		}
	})

	t.Run("receives_process", func(t *testing.T) {
		t.Parallel()

		var received *program.Process
		f := program.RunnerFunc(func(p *program.Process) error {
			received = p
			return nil
		})

		p := program.NewProcess()
		_ = f.Run(p)
		if received != p {
			t.Error("should pass the exact *Process to the function")
		}
	})
}

func TestMainFunc(t *testing.T) {
	t.Parallel()

	t.Run("nil_runner", func(t *testing.T) {
		t.Parallel()

		t.Run("untyped_nil", func(t *testing.T) {
			t.Parallel()

			defer func() {
				if r := recover(); r == nil {
					t.Error("Main(nil) should panic")
				}
			}()
			program.Main(nil, program.WithSignalNotifier(&testSignalNotifier{}))
		})

		t.Run("typed_nil", func(t *testing.T) {
			t.Parallel()

			defer func() {
				if r := recover(); r == nil {
					t.Error("Main(typed nil) should panic")
				}
			}()
			var r program.RunnerFunc
			program.Main(r, program.WithSignalNotifier(&testSignalNotifier{}))
		})
	})

	t.Run("exit_codes", func(t *testing.T) {
		t.Parallel()

		t.Run("runner_success", func(t *testing.T) {
			t.Parallel()

			runner := program.RunnerFunc(func(_ *program.Process) error {
				return nil
			})
			got := program.Main(runner, program.WithSignalNotifier(&testSignalNotifier{}))
			if got != program.ExitCodeOK {
				t.Errorf("Main() = %d, want ExitCodeOK (%d)", got, program.ExitCodeOK)
			}
		})

		t.Run("runner_error", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				err  error
				want program.ExitCode
			}{
				{
					name: "non_exit_error",
					err:  errors.New("boom"),
					want: program.ExitCodeErr,
				},
				{
					name: "exit_error",
					err:  program.ExitErr(errors.New("custom"), program.ExitCode(42)),
					want: program.ExitCode(42),
				},
				{
					name: "custom_exit_error_direct",
					err:  &customExitError{msg: "custom", code: program.ExitCode(77)},
					want: program.ExitCode(77),
				},
				{
					name: "custom_exit_error_wrapped",
					err:  fmt.Errorf("wrap: %w", &customExitError{msg: "custom", code: program.ExitCode(77)}),
					want: program.ExitCode(77),
				},
			}
			for _, tt := range tests {
				tt := tt
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					runner := program.RunnerFunc(func(_ *program.Process) error {
						return tt.err
					})
					got := program.Main(runner, program.WithSignalNotifier(&testSignalNotifier{}))
					if got != tt.want {
						t.Errorf("Main() = %d, want %d", got, tt.want)
					}
				})
			}
		})
	})

	t.Run("shutdown", func(t *testing.T) {
		t.Parallel()

		t.Run("graceful", func(t *testing.T) {
			t.Parallel()

			notifier := &testSignalNotifier{}
			started := make(chan struct{})
			runner := program.RunnerFunc(func(p *program.Process) error {
				close(started)
				<-p.Shutdown()
				return nil
			})

			exitCodeCh := make(chan program.ExitCode, 1)
			go func() {
				exitCodeCh <- program.Main(runner, program.WithSignalNotifier(notifier))
			}()
			<-started
			notifier.send(os.Interrupt)

			if got := <-exitCodeCh; got != program.ExitCodeOK {
				t.Errorf("Main() = %d, want ExitCodeOK (%d)", got, program.ExitCodeOK)
			}
		})

		t.Run("error_during_shutdown", func(t *testing.T) {
			t.Parallel()

			notifier := &testSignalNotifier{}
			started := make(chan struct{})
			wantCode := program.ExitCode(99)
			runnerErr := program.ExitErr(errors.New("graceful boom"), wantCode)
			runner := program.RunnerFunc(func(p *program.Process) error {
				close(started)
				<-p.Shutdown()
				return runnerErr
			})

			exitCodeCh := make(chan program.ExitCode, 1)
			go func() {
				exitCodeCh <- program.Main(runner, program.WithSignalNotifier(notifier))
			}()
			<-started
			notifier.send(os.Interrupt)

			if got := <-exitCodeCh; got != wantCode {
				t.Errorf("Main() = %d, want %d (runner ExitCode must win over ExitCodeShutdown)", got, wantCode)
			}
		})

		t.Run("forced", func(t *testing.T) {
			t.Parallel()

			notifier := &testSignalNotifier{}
			started := make(chan struct{})
			shutdownSeen := make(chan struct{})
			block := make(chan struct{})
			runner := program.RunnerFunc(func(p *program.Process) error {
				close(started)
				<-p.Shutdown()
				close(shutdownSeen)
				<-block
				return nil
			})

			exitCodeCh := make(chan program.ExitCode, 1)
			go func() {
				exitCodeCh <- program.Main(runner, program.WithSignalNotifier(notifier))
			}()
			<-started
			notifier.send(os.Interrupt)
			<-shutdownSeen
			notifier.send(os.Interrupt)

			if got := <-exitCodeCh; got != program.ExitCodeShutdown {
				t.Errorf("Main() = %d, want ExitCodeShutdown (%d)", got, program.ExitCodeShutdown)
			}
			close(block)
		})

		t.Run("non_cooperative_forced", func(t *testing.T) {
			t.Parallel()

			notifier := &testSignalNotifier{}
			started := make(chan struct{})
			block := make(chan struct{})
			runner := program.RunnerFunc(func(_ *program.Process) error {
				close(started)
				<-block
				return nil
			})

			exitCodeCh := make(chan program.ExitCode, 1)
			go func() {
				exitCodeCh <- program.Main(runner, program.WithSignalNotifier(notifier))
			}()
			<-started
			notifier.send(os.Interrupt)
			notifier.send(os.Interrupt)

			if got := <-exitCodeCh; got != program.ExitCodeShutdown {
				t.Errorf("Main() = %d, want ExitCodeShutdown (%d)", got, program.ExitCodeShutdown)
			}
			close(block)
		})
	})

	// signal_delivery exercises non-shutdown signals such as SIGUSR1 and is
	// defined in platform-specific files (signal_delivery_unix_test.go and
	// signal_delivery_windows_test.go) because those signals do not exist
	// on every platform.
	t.Run("signal_delivery", testMainFuncSignalDelivery)
}

// TestMainNoGoroutineLeak guards against regressions where [program.Main]
// leaks the goroutines it spawns internally (the runner goroutine and the
// shutdown listener).
//
// Rather than comparing total goroutine counts (which mix in the runtime's
// own workers, the test framework, and any other parallel tests), the
// assertion inspects [runtime.Stack] output and only counts goroutines
// whose "created by" frame points at program.Main. That count must reach
// zero after Main has returned and the test has unblocked any in-flight
// runner.
//
// The test is not parallel: it observes goroutines spawned by Main, so a
// sibling parallel test that also runs Main would be observed as a leak.
//
//nolint:paralleltest // must not run in parallel; inspects goroutines spawned by program.Main (see doc comment above).
func TestMainNoGoroutineLeak(t *testing.T) {
	//nolint:paralleltest // subtests share the goroutine-observation guarantee and must run serially.
	t.Run("plain_error", func(t *testing.T) {
		runner := program.RunnerFunc(func(_ *program.Process) error {
			return errors.New("boom")
		})

		got := program.Main(runner, program.WithSignalNotifier(&testSignalNotifier{}))
		if got != program.ExitCodeErr {
			t.Errorf("Main() = %d, want ExitCodeErr (%d)", got, program.ExitCodeErr)
		}
		assertNoProgramMainGoroutines(t)
	})

	//nolint:paralleltest // subtests share the goroutine-observation guarantee and must run serially.
	t.Run("forced_shutdown", func(t *testing.T) {
		notifier := &testSignalNotifier{}
		started := make(chan struct{})
		shutdownSeen := make(chan struct{})
		block := make(chan struct{})
		runner := program.RunnerFunc(func(p *program.Process) error {
			close(started)
			<-p.Shutdown()
			close(shutdownSeen)
			<-block
			return nil
		})

		exitCodeCh := make(chan program.ExitCode, 1)
		go func() {
			exitCodeCh <- program.Main(runner, program.WithSignalNotifier(notifier))
		}()
		<-started
		notifier.send(os.Interrupt)
		<-shutdownSeen
		notifier.send(os.Interrupt)

		if got := <-exitCodeCh; got != program.ExitCodeShutdown {
			t.Errorf("Main() = %d, want ExitCodeShutdown (%d)", got, program.ExitCodeShutdown)
		}

		// Main has returned ExitCodeShutdown, but the runner goroutine
		// spawned inside Main is still alive, blocked inside the runner
		// func on <-block.
		if count, stacks := countProgramMainGoroutines(); count == 0 {
			t.Errorf("expected program.Main runner goroutine to still be alive while blocked, found 0; full stacks:\n%s", stacks)
		}

		// Releasing the runner must allow that goroutine to terminate
		// without leaking.
		close(block)
		assertNoProgramMainGoroutines(t)
	})
}

// programMainGoroutineRE matches a "created by ...program.Main" line that
// [runtime.Stack] emits for each live goroutine spawned inside
// [program.Main]. Filtering on that line ignores the runtime's own
// workers, the testing framework's bookkeeping, and any other tests'
// goroutines, so the count does not depend on global runtime state.
var programMainGoroutineRE = regexp.MustCompile(`(?m)^created by .*\bprogram\.Main\b`)

// countProgramMainGoroutines reports how many goroutines spawned inside
// [program.Main] are currently live in the test binary.
func countProgramMainGoroutines() (int, []byte) {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	stacks := buf[:n]
	return len(programMainGoroutineRE.FindAllIndex(stacks, -1)), stacks
}

// assertNoProgramMainGoroutines polls until no goroutines spawned inside
// [program.Main] remain. Polling absorbs the small delay between a
// goroutine running its final statement and the runtime removing it from
// its bookkeeping; the assertion fails (with the offending stacks) only if
// a Main-spawned goroutine is still alive after the timeout.
func assertNoProgramMainGoroutines(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		count, _ := countProgramMainGoroutines()
		if count == 0 {
			return
		}
		if !time.Now().Before(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	count, stacks := countProgramMainGoroutines()
	t.Errorf("expected no program.Main goroutines, found %d; full stacks:\n%s", count, stacks)
}

type customExitError struct {
	msg  string
	code program.ExitCode
}

func (e *customExitError) Error() string {
	return e.msg
}

func (e *customExitError) ExitCode() program.ExitCode {
	return e.code
}

type testSignalNotifier struct {
	mu      sync.Mutex
	entries []notifyEntry
}

type notifyEntry struct {
	ch      chan<- os.Signal
	signals []os.Signal
}

func (n *testSignalNotifier) Notify(ch chan<- os.Signal, signals ...os.Signal) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.entries = append(n.entries, notifyEntry{ch: ch, signals: signals})
}

func (n *testSignalNotifier) Stop(ch chan<- os.Signal) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.entries = slices.DeleteFunc(n.entries, func(e notifyEntry) bool {
		return e.ch == ch
	})
}

func (n *testSignalNotifier) send(sig os.Signal) {
	n.mu.Lock()
	var targets []chan<- os.Signal
	for _, e := range n.entries {
		// Mirror [signal.Notify] semantics: an entry registered with no
		// explicit signals receives every signal.
		if len(e.signals) == 0 || slices.Contains(e.signals, sig) {
			targets = append(targets, e.ch)
		}
	}
	n.mu.Unlock()
	for _, ch := range targets {
		select {
		case ch <- sig:
		default:
			// If the channel is not ready to receive, skip it.
		}
	}
}
