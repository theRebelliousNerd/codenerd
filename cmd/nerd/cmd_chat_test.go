package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestCollectChatTurns(t *testing.T) {
	tests := []struct {
		name string
		args []string
		stdin string
		want []string
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
			got := collectChatTurns(tt.args, strings.NewReader(tt.stdin))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("collectChatTurns(%q, %q) = %q, want %q", tt.args, tt.stdin, got, tt.want)
			}
		})
	}
}

func TestCollectChatTurns_NilReaderYieldsEmpty(t *testing.T) {
	if got := collectChatTurns(nil, nil); len(got) != 0 {
		t.Fatalf("collectChatTurns(nil, nil) = %q, want empty", got)
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
