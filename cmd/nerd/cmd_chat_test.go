package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

func drainChatTurns(src *chatTurnSource) []string {
	got := []string{}
	for turn, ok := src.Next(); ok; turn, ok = src.Next() {
		got = append(got, turn)
	}
	return got
}

func TestChatTurnSource(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  []string
	}{
		{
			name:  "args take precedence over stdin",
			args:  []string{"hello", "world"},
			stdin: "ignored\nlines\n",
			want:  []string{"hello", "world"},
		},
		{
			name:  "stdin splits on newlines",
			args:  nil,
			stdin: "first\nsecond\nthird\n",
			want:  []string{"first", "second", "third"},
		},
		{
			name:  "stdin trims and skips blanks",
			args:  nil,
			stdin: "  hello  \n\n   \n\tworld\t\n",
			want:  []string{"hello", "world"},
		},
		{
			name:  "stdin stops at /quit",
			args:  nil,
			stdin: "one\n/quit\ntwo\n",
			want:  []string{"one"},
		},
		{
			name:  "stdin stops at /exit",
			args:  nil,
			stdin: "one\ntwo\n/exit\nthree\n",
			want:  []string{"one", "two"},
		},
		{
			name:  "quit with surrounding whitespace still quits",
			args:  nil,
			stdin: "one\n  /quit  \nthree\n",
			want:  []string{"one"},
		},
		{
			name:  "empty input yields empty slice",
			args:  nil,
			stdin: "",
			want:  []string{},
		},
		{
			name:  "blank-only stdin yields empty slice",
			args:  nil,
			stdin: "\n   \n\t\n",
			want:  []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := newChatTurnSource(tt.args, strings.NewReader(tt.stdin))
			got := drainChatTurns(src)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("drain(newChatTurnSource(%q, %q)) = %q, want %q", tt.args, tt.stdin, got, tt.want)
			}
			// The source must stay terminated: further Next calls report ok=false.
			if turn, ok := src.Next(); ok {
				t.Fatalf("Next after drain = (%q, true), want ok=false", turn)
			}
		})
	}
}

func TestChatTurnSource_NilReaderYieldsNothing(t *testing.T) {
	src := newChatTurnSource(nil, nil)
	if turn, ok := src.Next(); ok {
		t.Fatalf("newChatTurnSource(nil, nil).Next() = (%q, true), want ok=false", turn)
	}
	if turn, ok := src.Next(); ok {
		t.Fatalf("second Next() = (%q, true), want ok=false", turn)
	}
}

func TestChatTurnSource_ArgsNeverTouchReader(t *testing.T) {
	// When args are present the reader must never be touched, even if every
	// read would fail.
	src := newChatTurnSource([]string{"hello", "world"}, iotest.ErrReader(errors.New("fake read error")))
	got := drainChatTurns(src)
	want := []string{"hello", "world"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("drain with ErrReader = %q, want %q", got, want)
	}
}

func TestChatTurnSource_Lazy(t *testing.T) {
	pr, pw := io.Pipe()
	src := newChatTurnSource(nil, pr)

	// The writer publishes one line, then waits until the reader has consumed
	// it before publishing the second line and closing. An eager
	// implementation that drains stdin to EOF up front would block forever
	// waiting for the close that only happens after the first turn is read.
	allowSecond := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer pw.Close()
		if _, err := io.WriteString(pw, "one\n"); err != nil {
			return
		}
		<-allowSecond
		if _, err := io.WriteString(pw, "two\n"); err != nil {
			return
		}
	}()

	type nextResult struct {
		turn string
		ok   bool
	}
	firstCh := make(chan nextResult, 1)
	go func() {
		turn, ok := src.Next()
		firstCh <- nextResult{turn, ok}
	}()

	select {
	case res := <-firstCh:
		if !res.ok || res.turn != "one" {
			close(allowSecond)
			t.Fatalf("first Next() = (%q, %v), want (\"one\", true)", res.turn, res.ok)
		}
	case <-time.After(5 * time.Second):
		close(allowSecond)
		t.Fatal("first Next() did not return without the writer having closed (not lazy)")
	}

	// The first turn arrived while the writer was still open (blocked on
	// allowSecond). Now let the writer publish the second turn and close.
	close(allowSecond)

	secondCh := make(chan nextResult, 1)
	go func() {
		turn, ok := src.Next()
		secondCh <- nextResult{turn, ok}
	}()
	select {
	case res := <-secondCh:
		if !res.ok || res.turn != "two" {
			t.Fatalf("second Next() = (%q, %v), want (\"two\", true)", res.turn, res.ok)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second Next() did not return after writer wrote two and closed")
	}

	<-writerDone
	if turn, ok := src.Next(); ok {
		t.Fatalf("third Next() = (%q, true), want ok=false at EOF", turn)
	}
}

func TestChatTurnSource_ReadError(t *testing.T) {
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	src := newChatTurnSource(nil, iotest.ErrReader(errors.New("fake read error")))
	if turn, ok := src.Next(); ok {
		w.Close()
		os.Stderr = oldStderr
		t.Fatalf("Next() on ErrReader = (%q, true), want ok=false", turn)
	}
	// A second call must stay terminated without printing again.
	if turn, ok := src.Next(); ok {
		w.Close()
		os.Stderr = oldStderr
		t.Fatalf("second Next() on ErrReader = (%q, true), want ok=false", turn)
	}

	if err := w.Close(); err != nil {
		os.Stderr = oldStderr
		t.Fatalf("closing stderr pipe writer: %v", err)
	}
	os.Stderr = oldStderr
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading captured stderr: %v", err)
	}
	r.Close()
	out := buf.String()
	if !strings.Contains(out, "error: reading turns:") {
		t.Fatalf("captured stderr = %q, want it to contain %q", out, "error: reading turns:")
	}
	if got, want := strings.Count(out, "error: reading turns:"), 1; got != want {
		t.Fatalf("captured stderr = %q, want exactly %d error line, got %d", out, want, got)
	}
}

func TestChatCommandRegistered(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "chat" {
			return
		}
	}
	t.Fatal(`rootCmd has no subcommand named "chat"`)
}
