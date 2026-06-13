package session

import (
	"testing"

	"codenerd/internal/types"
)

func TestParseMangleArg(t *testing.T) {
	e := &Executor{}
	if got := e.parseMangleArg(`"hello"`); got != "hello" {
		t.Errorf("quoted string -> %v, want hello", got)
	}
	if got := e.parseMangleArg(`/run`); got != types.MangleAtom("/run") {
		t.Errorf("atom -> %v, want MangleAtom(/run)", got)
	}
	if got := e.parseMangleArg(`42`); got != 42 {
		t.Errorf("number -> %v (%T), want int 42", got, got)
	}
	if got := e.parseMangleArg(`bareword`); got != "bareword" {
		t.Errorf("bareword -> %v, want bareword", got)
	}
}

func TestParseMangleArgs(t *testing.T) {
	e := &Executor{}
	got := e.parseMangleArgs(`"a", /b, 3, c`)
	if len(got) != 4 {
		t.Fatalf("parseMangleArgs returned %d args, want 4: %v", len(got), got)
	}
	if got[0] != "a" || got[1] != types.MangleAtom("/b") || got[2] != 3 || got[3] != "c" {
		t.Errorf("parseMangleArgs mismatch: %v", got)
	}

	// Commas inside a quoted string are not treated as separators.
	quoted := e.parseMangleArgs(`"x,y", /z`)
	if len(quoted) != 2 || quoted[0] != "x,y" {
		t.Errorf("quoted-comma handling failed: %v", quoted)
	}

	// Empty input yields no args.
	if got := e.parseMangleArgs(""); len(got) != 0 {
		t.Errorf("empty input -> %v, want no args", got)
	}
}
