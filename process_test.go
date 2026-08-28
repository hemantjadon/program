package program_test

import (
	"bytes"
	"io"
	"os"
	"slices"
	"testing"

	"github.com/hemantjadon/program"
)

func TestNewProcess(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()

		p := program.NewProcess()

		if got := p.Args(); !slices.Equal(got, os.Args) {
			t.Errorf("Args() = %v, want os.Args %v", got, os.Args)
		}
		if got := p.Env(); !slices.Equal(got, os.Environ()) {
			t.Errorf("Env() = %v, want os.Environ %v", got, os.Environ())
		}
		if got := p.Stdin(); got != os.Stdin {
			t.Errorf("Stdin() = %v, want os.Stdin", got)
		}
		if got := p.Stdout(); got != os.Stdout {
			t.Errorf("Stdout() = %v, want os.Stdout", got)
		}
		if got := p.Stderr(); got != os.Stderr {
			t.Errorf("Stderr() = %v, want os.Stderr", got)
		}
		if p.Signals() == nil {
			t.Error("Signals() should not be nil")
		}
		if p.Shutdown() == nil {
			t.Error("Shutdown() should not be nil")
		}
	})

	t.Run("option_ordering", func(t *testing.T) {
		t.Parallel()

		t.Run("duplicate_option", func(t *testing.T) {
			t.Parallel()

			first := []string{"first"}
			second := []string{"second"}
			p := program.NewProcess(program.WithArgs(first), program.WithArgs(second))
			if got := p.Args(); !slices.Equal(got, second) {
				t.Errorf("Args() = %v, want %v", got, second)
			}
		})

		t.Run("nil_then_valid", func(t *testing.T) {
			t.Parallel()

			buf := new(bytes.Buffer)
			p := program.NewProcess(program.WithStdout(nil), program.WithStdout(buf))
			if got := p.Stdout(); got != buf {
				t.Errorf("Stdout() = %v, want %v", got, buf)
			}
		})

		t.Run("valid_then_nil", func(t *testing.T) {
			t.Parallel()

			buf := new(bytes.Buffer)
			p := program.NewProcess(program.WithStdout(buf), program.WithStdout(nil))
			if got := p.Stdout(); got != buf {
				t.Errorf("Stdout() = %v, want %v", got, buf)
			}
		})
	})
}

func TestWithArgs(t *testing.T) {
	t.Parallel()

	t.Run("values", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			args []string
			want []string
		}{
			{name: "single_arg", args: []string{"app"}, want: []string{"app"}},
			{name: "multiple_args", args: []string{"app", "-v", "--config", "f.yaml"}, want: []string{"app", "-v", "--config", "f.yaml"}},
			{name: "empty_strings", args: []string{"", "", ""}, want: []string{"", "", ""}},
			{name: "empty_slice", args: []string{}, want: []string{}},
			{name: "nil_slice", args: nil, want: nil},
			{name: "special_characters", args: []string{"--name=hello world", "-x=a\tb", "arg with spaces"}, want: []string{"--name=hello world", "-x=a\tb", "arg with spaces"}},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				p := program.NewProcess(program.WithArgs(tt.args))
				if got := p.Args(); !slices.Equal(got, tt.want) {
					t.Errorf("Args() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("isolation", func(t *testing.T) {
		t.Parallel()

		t.Run("from_input", func(t *testing.T) {
			t.Parallel()

			input := []string{"a", "b", "c"}
			p := program.NewProcess(program.WithArgs(input))
			input[0] = "mutated"

			if got, want := p.Args()[0], "a"; got != want {
				t.Errorf("Args()[0] = %q, want %q", got, want)
			}
		})

		t.Run("from_accessor", func(t *testing.T) {
			t.Parallel()

			input := []string{"a", "b", "c"}
			p := program.NewProcess(program.WithArgs(input))
			p.Args()[0] = "mutated"

			if got, want := input[0], "a"; got != want {
				t.Errorf("input[0] = %q, want %q", got, want)
			}
		})
	})

	t.Run("shared_across_callers", func(t *testing.T) {
		t.Parallel()

		p := program.NewProcess(program.WithArgs([]string{"a", "b"}))
		a, b := p.Args(), p.Args()
		if &a[0] != &b[0] {
			t.Error("Args() should return the same backing slice across calls")
		}
	})
}

func TestWithEnv(t *testing.T) {
	t.Parallel()

	t.Run("values", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			env  []string
			want []string
		}{
			{name: "single_var", env: []string{"A=1"}, want: []string{"A=1"}},
			{name: "multiple_vars", env: []string{"A=1", "B=2", "C=3"}, want: []string{"A=1", "B=2", "C=3"}},
			{name: "empty_strings", env: []string{"", "", ""}, want: []string{"", "", ""}},
			{name: "empty_slice", env: []string{}, want: []string{}},
			{name: "nil_slice", env: nil, want: nil},
			{name: "special_characters", env: []string{"PATH=/usr/bin:/bin", "MSG=hello world", "TAB=a\tb"}, want: []string{"PATH=/usr/bin:/bin", "MSG=hello world", "TAB=a\tb"}},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				p := program.NewProcess(program.WithEnv(tt.env))
				if got := p.Env(); !slices.Equal(got, tt.want) {
					t.Errorf("Env() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("isolation", func(t *testing.T) {
		t.Parallel()

		t.Run("from_input", func(t *testing.T) {
			t.Parallel()

			input := []string{"A=1", "B=2"}
			p := program.NewProcess(program.WithEnv(input))
			input[0] = "A=mutated"

			if got, want := p.Env()[0], "A=1"; got != want {
				t.Errorf("Env()[0] = %q, want %q", got, want)
			}
		})

		t.Run("from_accessor", func(t *testing.T) {
			t.Parallel()

			input := []string{"A=1", "B=2"}
			p := program.NewProcess(program.WithEnv(input))
			p.Env()[0] = "A=mutated"

			if got, want := input[0], "A=1"; got != want {
				t.Errorf("input[0] = %q, want %q", got, want)
			}
		})
	})

	t.Run("shared_across_callers", func(t *testing.T) {
		t.Parallel()

		p := program.NewProcess(program.WithEnv([]string{"A=1", "B=2"}))
		a, b := p.Env(), p.Env()
		if &a[0] != &b[0] {
			t.Error("Env() should return the same backing slice across calls")
		}
	})
}

func TestProcessLookupEnv(t *testing.T) {
	t.Parallel()

	env := []string{
		"A=1",
		"B=2",
		"EMPTY=",
		"DUP=first",
		"DUP=second",
		"WITH_EQUALS=a=b=c",
		"noequals",
		"=novalue",
	}
	p := program.NewProcess(program.WithEnv(env))

	tests := []struct {
		name      string
		key       string
		wantValue string
		wantFound bool
	}{
		{name: "present", key: "A", wantValue: "1", wantFound: true},
		{name: "present_other", key: "B", wantValue: "2", wantFound: true},
		{name: "empty_value", key: "EMPTY", wantValue: "", wantFound: true},
		{name: "duplicate_returns_first", key: "DUP", wantValue: "first", wantFound: true},
		{name: "value_contains_equals", key: "WITH_EQUALS", wantValue: "a=b=c", wantFound: true},
		{name: "missing", key: "MISSING", wantValue: "", wantFound: false},
		{name: "no_equals_entry_skipped", key: "noequals", wantValue: "", wantFound: false},
		{name: "empty_key", key: "", wantValue: "", wantFound: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotValue, gotFound := p.LookupEnv(tt.key)
			if gotValue != tt.wantValue || gotFound != tt.wantFound {
				t.Errorf("LookupEnv(%q) = (%q, %v), want (%q, %v)", tt.key, gotValue, gotFound, tt.wantValue, tt.wantFound)
			}
		})
	}
}

func TestProcessGetenv(t *testing.T) {
	t.Parallel()

	env := []string{"A=1", "EMPTY=", "DUP=first", "DUP=second"}
	p := program.NewProcess(program.WithEnv(env))

	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "present", key: "A", want: "1"},
		{name: "empty_value", key: "EMPTY", want: ""},
		{name: "duplicate_returns_first", key: "DUP", want: "first"},
		{name: "missing", key: "MISSING", want: ""},
		{name: "empty_key", key: "", want: ""},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := p.Getenv(tt.key); got != tt.want {
				t.Errorf("Getenv(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestWithStdin(t *testing.T) {
	t.Parallel()

	reader := noopReader{}
	ptrReader := new(noopReader)
	var typedNil *bytes.Buffer

	tests := []struct {
		name       string
		reader     io.Reader
		wantReader io.Reader
	}{
		{name: "valid_reader", reader: reader, wantReader: reader},
		{name: "valid_reader_ptr", reader: ptrReader, wantReader: ptrReader},
		{name: "untyped_nil", reader: nil, wantReader: os.Stdin},
		{name: "typed_nil", reader: typedNil, wantReader: os.Stdin},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := program.NewProcess(program.WithStdin(tt.reader))
			if p.Stdin() != tt.wantReader {
				t.Errorf("Stdin() = %v, want %v", p.Stdin(), tt.wantReader)
			}
		})
	}
}

func TestWithStdout(t *testing.T) {
	t.Parallel()

	writer := noopWriter{}
	ptrWriter := new(noopWriter)
	var typedNil *bytes.Buffer

	tests := []struct {
		name       string
		writer     io.Writer
		wantWriter io.Writer
	}{
		{name: "valid_writer", writer: writer, wantWriter: writer},
		{name: "valid_writer_ptr", writer: ptrWriter, wantWriter: ptrWriter},
		{name: "untyped_nil", writer: nil, wantWriter: os.Stdout},
		{name: "typed_nil", writer: typedNil, wantWriter: os.Stdout},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := program.NewProcess(program.WithStdout(tt.writer))
			if p.Stdout() != tt.wantWriter {
				t.Errorf("Stdout() = %v, want %v", p.Stdout(), tt.wantWriter)
			}
		})
	}
}

func TestWithStderr(t *testing.T) {
	t.Parallel()

	writer := noopWriter{}
	ptrWriter := new(noopWriter)
	var typedNil *bytes.Buffer

	tests := []struct {
		name       string
		writer     io.Writer
		wantWriter io.Writer
	}{
		{name: "valid_writer", writer: writer, wantWriter: writer},
		{name: "valid_writer_ptr", writer: ptrWriter, wantWriter: ptrWriter},
		{name: "untyped_nil", writer: nil, wantWriter: os.Stderr},
		{name: "typed_nil", writer: typedNil, wantWriter: os.Stderr},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := program.NewProcess(program.WithStderr(tt.writer))
			if p.Stderr() != tt.wantWriter {
				t.Errorf("Stderr() = %v, want %v", p.Stderr(), tt.wantWriter)
			}
		})
	}
}

func TestWithSignalsChan(t *testing.T) {
	t.Parallel()

	t.Run("valid_channel", func(t *testing.T) {
		t.Parallel()

		ch := make(chan os.Signal, 1)
		p := program.NewProcess(program.WithSignalsChan(ch))
		if got := p.Signals(); got != ch {
			t.Errorf("Signals() = %v, want %v", got, ch)
		}
	})

	t.Run("nil_is_noop", func(t *testing.T) {
		t.Parallel()

		p := program.NewProcess(program.WithSignalsChan(nil))
		if p.Signals() == nil {
			t.Error("WithSignalsChan(nil) should leave a non-nil default channel")
		}
	})
}

func TestWithShutdownChan(t *testing.T) {
	t.Parallel()

	t.Run("valid_channel", func(t *testing.T) {
		t.Parallel()

		ch := make(chan struct{})
		p := program.NewProcess(program.WithShutdownChan(ch))
		if got := p.Shutdown(); got != (<-chan struct{})(ch) {
			t.Errorf("Shutdown() = %v, want %v", got, ch)
		}
	})

	t.Run("nil_is_noop", func(t *testing.T) {
		t.Parallel()

		p := program.NewProcess(program.WithShutdownChan(nil))
		if p.Shutdown() == nil {
			t.Error("WithShutdownChan(nil) should leave a non-nil default channel")
		}
	})

	t.Run("closes_visible_to_shutdown", func(t *testing.T) {
		t.Parallel()

		ch := make(chan struct{})
		p := program.NewProcess(program.WithShutdownChan(ch))
		close(ch)
		select {
		case <-p.Shutdown():
		default:
			t.Error("Shutdown() should be observable as closed once the underlying channel closes")
		}
	})
}

type noopReader struct{}

func (noopReader) Read(p []byte) (n int, err error) {
	return len(p), nil
}

type noopWriter struct{}

func (noopWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}
