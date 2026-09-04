package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestInteractiveContinuation_Table(t *testing.T) {
	cases := []struct {
		name         string
		input        string
		isEOF        bool
		lastErrored  bool
		wantExit     bool
		wantCmd      InteractiveMetaCommand
	}{
		{
			name:        "EOF after shard error exits",
			input:       "",
			isEOF:       true,
			lastErrored: true,
			wantExit:    true,
		},
		{
			name:        "quit after error exits",
			input:       "quit\n",
			lastErrored: true,
			wantExit:    true,
			wantCmd:     MetaQuit,
		},
		{
			name:        "exit after error exits",
			input:       "exit\n",
			lastErrored: true,
			wantExit:    true,
			wantCmd:     MetaQuit,
		},
		{
			name:        "q after error exits",
			input:       "q\n",
			lastErrored: true,
			wantExit:    true,
			wantCmd:     MetaQuit,
		},
		{
			name:        "redo after error continues",
			input:       "redo\n",
			lastErrored: true,
			wantExit:    false,
			wantCmd:     MetaRedo,
		},
		{
			name:        "EOF after success exits",
			input:       "",
			isEOF:       true,
			lastErrored: false,
			wantExit:    true,
		},
		{
			name:        "quit after success exits",
			input:       "quit\n",
			lastErrored: false,
			wantExit:    true,
			wantCmd:     MetaQuit,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reader *bufio.Reader
			if tc.isEOF {
				reader = bufio.NewReader(strings.NewReader(""))
			} else {
				reader = bufio.NewReader(strings.NewReader(tc.input))
			}
			cmd, _, shouldExit := nextInteractiveCommand(reader, tc.lastErrored)
			if shouldExit != tc.wantExit {
				t.Fatalf("nextInteractiveCommand(%q, errored=%v) shouldExit = %v, want %v", tc.input, tc.lastErrored, shouldExit, tc.wantExit)
			}
			if tc.wantCmd != "" && cmd != tc.wantCmd {
				t.Fatalf("nextInteractiveCommand(%q) cmd = %q, want %q", tc.input, cmd, tc.wantCmd)
			}
			if !tc.wantExit && tc.wantCmd == "" && cmd == MetaQuit {
				t.Fatalf("nextInteractiveCommand(%q) unexpectedly parsed as quit", tc.input)
			}
		})
	}
}

func TestInteractiveContinuation_RedoDoesNotExit(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("redo\n"))
	cmd, _, shouldExit := nextInteractiveCommand(reader, true)
	if shouldExit {
		t.Fatal("redo after error should continue, got shouldExit=true")
	}
	if cmd != MetaRedo {
		t.Fatalf("redo should parse as MetaRedo, got %q", cmd)
	}
}

func TestInteractiveContinuation_EOFAlwaysExits(t *testing.T) {
	for _, errored := range []bool{true, false} {
		reader := bufio.NewReader(strings.NewReader(""))
		_, _, shouldExit := nextInteractiveCommand(reader, errored)
		if !shouldExit {
			t.Fatalf("EOF with lastErrored=%v should exit", errored)
		}
	}
}
