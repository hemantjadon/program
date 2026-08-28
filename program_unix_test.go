//go:build unix

package program_test

import (
	"os"
	"syscall"
	"testing"

	"github.com/hemantjadon/program"
)

// testMainFuncSignalDelivery is the body of TestMainFunc/signal_delivery.
// It lives in a unix-only file because it relies on syscall.SIGUSR1 and
// syscall.SIGUSR2, which are not defined on non-unix targets such as
// windows, plan9, js, and wasip1.
//
// Non-unix builds provide a stub that calls t.Skip; see program_other_test.go.
func testMainFuncSignalDelivery(t *testing.T) {
	t.Parallel()

	t.Run("registered_signal", func(t *testing.T) {
		t.Parallel()

		notifier := &testSignalNotifier{}
		started := make(chan struct{})
		received := make(chan os.Signal, 1)

		runner := program.RunnerFunc(func(p *program.Process) error {
			close(started)
			select {
			case sig := <-p.Signals():
				received <- sig
			case <-p.Shutdown():
			}
			return nil
		})

		exitCodeCh := make(chan program.ExitCode, 1)
		go func() {
			exitCodeCh <- program.Main(runner, program.WithSignalNotifier(notifier), program.WithSignals(syscall.SIGUSR1))
		}()
		<-started
		notifier.send(syscall.SIGUSR1)

		if got := <-received; got != syscall.SIGUSR1 {
			t.Errorf("received signal = %v, want SIGUSR1", got)
		}
		if got := <-exitCodeCh; got != program.ExitCodeOK {
			t.Errorf("Main() = %d, want ExitCodeOK (%d)", got, program.ExitCodeOK)
		}
	})

	t.Run("unregistered_signal", func(t *testing.T) {
		t.Parallel()

		notifier := &testSignalNotifier{}
		started := make(chan struct{})

		runner := program.RunnerFunc(func(p *program.Process) error {
			close(started)
			select {
			case sig := <-p.Signals():
				t.Errorf("Signals() received unexpected signal %v", sig)
			case <-p.Shutdown():
			}
			return nil
		})

		exitCodeCh := make(chan program.ExitCode, 1)
		go func() {
			exitCodeCh <- program.Main(runner, program.WithSignalNotifier(notifier), program.WithSignals(syscall.SIGUSR1))
		}()
		<-started
		notifier.send(syscall.SIGUSR2)
		notifier.send(os.Interrupt)

		if got := <-exitCodeCh; got != program.ExitCodeOK {
			t.Errorf("Main() = %d, want ExitCodeOK (%d)", got, program.ExitCodeOK)
		}
	})

	t.Run("no_registered_signal", func(t *testing.T) {
		t.Parallel()

		notifier := &testSignalNotifier{}
		started := make(chan struct{})

		runner := program.RunnerFunc(func(p *program.Process) error {
			close(started)
			select {
			case sig := <-p.Signals():
				t.Errorf("Signals() received unexpected signal %v", sig)
			case <-p.Shutdown():
			}
			return nil
		})

		exitCodeCh := make(chan program.ExitCode, 1)
		go func() {
			// No WithSignals call, and the notifier mirrors signal.Notify's
			// "register with no signals receives every signal" semantic.
			// This guards against a regression where Main accidentally
			// registers Signals() with an empty signal set.
			exitCodeCh <- program.Main(runner, program.WithSignalNotifier(notifier))
		}()
		<-started
		notifier.send(syscall.SIGUSR1)
		notifier.send(os.Interrupt)

		if got := <-exitCodeCh; got != program.ExitCodeOK {
			t.Errorf("Main() = %d, want ExitCodeOK (%d)", got, program.ExitCodeOK)
		}
	})

	t.Run("shutdown_signal_filtered", func(t *testing.T) {
		t.Parallel()

		notifier := &testSignalNotifier{}
		started := make(chan struct{})
		shutdownSeen := make(chan struct{})

		runner := program.RunnerFunc(func(p *program.Process) error {
			close(started)
			select {
			case sig := <-p.Signals():
				t.Errorf("Signals() received %v, want shutdown instead", sig)
			case <-p.Shutdown():
				close(shutdownSeen)
			}
			return nil
		})

		exitCodeCh := make(chan program.ExitCode, 1)
		go func() {
			exitCodeCh <- program.Main(runner, program.WithSignalNotifier(notifier), program.WithSignals(os.Interrupt, syscall.SIGUSR1))
		}()
		<-started
		notifier.send(os.Interrupt)
		<-shutdownSeen

		if got := <-exitCodeCh; got != program.ExitCodeOK {
			t.Errorf("Main() = %d, want ExitCodeOK (%d)", got, program.ExitCodeOK)
		}
	})

	t.Run("duplicate_signals", func(t *testing.T) {
		t.Parallel()

		notifier := &testSignalNotifier{}
		started := make(chan struct{})
		received := make(chan os.Signal, 1)

		runner := program.RunnerFunc(func(p *program.Process) error {
			close(started)
			select {
			case sig := <-p.Signals():
				received <- sig
			case <-p.Shutdown():
			}
			return nil
		})

		exitCodeCh := make(chan program.ExitCode, 1)
		go func() {
			// A duplicated signal must not panic and the signal must still be
			// delivered to the runner exactly once on the next read.
			exitCodeCh <- program.Main(runner, program.WithSignalNotifier(notifier), program.WithSignals(syscall.SIGUSR1, syscall.SIGUSR1))
		}()
		<-started
		notifier.send(syscall.SIGUSR1)

		if got := <-received; got != syscall.SIGUSR1 {
			t.Errorf("received signal = %v, want SIGUSR1", got)
		}
		if got := <-exitCodeCh; got != program.ExitCodeOK {
			t.Errorf("Main() = %d, want ExitCodeOK (%d)", got, program.ExitCodeOK)
		}
	})
}
