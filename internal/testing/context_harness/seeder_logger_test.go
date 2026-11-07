package context_harness

import (
	"io"
	"strings"
	"testing"
)

func TestFileLogger_Writers(t *testing.T) {
	fl, err := NewFileLogger(t.TempDir(), io.Discard)
	if err != nil {
		t.Fatalf("NewFileLogger: %v", err)
	}
	defer fl.Close()

	// Every category writer must be non-nil and accept a write.
	writers := map[string]io.Writer{
		"jit":         fl.GetJITWriter(),
		"activation":  fl.GetActivationWriter(),
		"compression": fl.GetCompressionWriter(),
		"piggyback":   fl.GetPiggybackWriter(),
		"summary":     fl.GetSummaryWriter(),
		"feedback":    fl.GetFeedbackWriter(),
	}
	for name, w := range writers {
		if w == nil {
			t.Errorf("%s writer is nil", name)
			continue
		}
		if _, err := w.Write([]byte("log line\n")); err != nil {
			t.Errorf("%s writer write failed: %v", name, err)
		}
	}
}

func TestParseMangleFact_PinBehaviour(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		predicate string
		args      []any
		wantErr   string
	}{
		{name: "bare predicate has no args", input: "just_predicate", predicate: "just_predicate", args: nil},
		{name: "single quoted arg", input: `current_campaign("auth-migration")`, predicate: "current_campaign", args: []any{"auth-migration"}},
		{name: "trailing period is stripped", input: `current_campaign("auth-migration").`, predicate: "current_campaign", args: []any{"auth-migration"}},
		{name: "empty arg list yields nil", input: "predicate()", predicate: "predicate", args: nil},
		{name: "empty arg list with period yields nil", input: "predicate().", predicate: "predicate", args: nil},
		{name: "int arg parses as int", input: "turn(42)", predicate: "turn", args: []any{42}},
		{name: "slash atom strips leading slash", input: "flag(/active)", predicate: "flag", args: []any{"active"}},
		{name: "empty fact errors", input: "", wantErr: "empty fact"},
		{name: "missing predicate errors", input: "(bar)", wantErr: "no predicate name"},
		{name: "unbalanced paren errors", input: "foo(bar", wantErr: "unbalanced parenthesis"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fact, err := parseMangleFact(tc.input)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("parseMangleFact(%q) succeeded, want error containing %q", tc.input, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseMangleFact(%q) error = %q, want containing %q", tc.input, err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMangleFact(%q) failed: %v", tc.input, err)
			}
			if fact.Predicate != tc.predicate {
				t.Fatalf("parseMangleFact(%q) predicate = %q, want %q", tc.input, fact.Predicate, tc.predicate)
			}
			if len(fact.Args) != len(tc.args) {
				t.Fatalf("parseMangleFact(%q) args = %#v, want %#v", tc.input, fact.Args, tc.args)
			}
			for i := range tc.args {
				if fact.Args[i] != tc.args[i] {
					t.Fatalf("parseMangleFact(%q) arg %d = %#v, want %#v", tc.input, i, fact.Args[i], tc.args[i])
				}
			}
		})
	}
}

func TestParseArgs_PinBehaviour(t *testing.T) {
	if got := parseArgs(""); got != nil {
		t.Fatalf("parseArgs empty = %#v, want nil", got)
	}
	got := parseArgs(`"a,b", c`)
	if len(got) != 2 || got[0] != "a,b" || got[1] != "c" {
		t.Fatalf("parseArgs quoted comma = %#v, want [a,b c]", got)
	}
	got = parseArgs("'single', plain")
	if len(got) != 2 || got[0] != "single" || got[1] != "plain" {
		t.Fatalf("parseArgs single quotes = %#v, want [single plain]", got)
	}
	got = parseArgs("42, hello")
	if len(got) != 2 || got[0] != 42 || got[1] != "hello" {
		t.Fatalf("parseArgs mixed int and string = %#v, want [42 hello]", got)
	}
}

func TestParseValue_PinBehaviour(t *testing.T) {
	if got := parseValue("42"); got != 42 {
		t.Fatalf("parseValue(42) = %#v, want 42", got)
	}
	if got := parseValue("-7"); got != -7 {
		t.Fatalf("parseValue(-7) = %#v, want -7", got)
	}
	if got := parseValue("hello"); got != "hello" {
		t.Fatalf("parseValue(hello) = %#v, want hello", got)
	}
	if got := parseValue("/active"); got != "active" {
		t.Fatalf("parseValue(/active) = %#v, want active", got)
	}
}

func TestParseInitialFacts_PinBehaviour(t *testing.T) {
	scenario := &Scenario{InitialFacts: []string{`current_campaign("auth-migration").`, "just_predicate"}}
	facts, err := parseInitialFacts(scenario)
	if err != nil {
		t.Fatalf("parseInitialFacts failed: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("parseInitialFacts len = %d, want 2", len(facts))
	}
	if facts[0].Predicate != "current_campaign" || len(facts[0].Args) != 1 || facts[0].Args[0] != "auth-migration" {
		t.Fatalf("first fact = %#v, want current_campaign(auth-migration)", facts[0])
	}
	if facts[1].Predicate != "just_predicate" {
		t.Fatalf("second fact predicate = %q, want just_predicate", facts[1].Predicate)
	}

	bad := &Scenario{InitialFacts: []string{""}}
	if _, err := parseInitialFacts(bad); err == nil || !strings.Contains(err.Error(), "empty fact") {
		t.Fatalf("parseInitialFacts empty fact err = %v, want wrapping empty fact", err)
	}

	empty := &Scenario{}
	if got, err := parseInitialFacts(empty); err != nil || len(got) != 0 {
		t.Fatalf("parseInitialFacts empty scenario = %#v, %v; want empty, nil", got, err)
	}
}
