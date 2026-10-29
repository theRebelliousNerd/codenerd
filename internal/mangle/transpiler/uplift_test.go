package transpiler

// Behavioral uplift tests for sanitizer.go: the sanitizer consumes untrusted
// LLM text, so malformed markers must error (never panic), every aggregation
// must survive, string data must never become code, and output must parse.

import (
	"strings"
	"testing"

	"codenerd/internal/mangle"
)

// A hand-written sys_emit_pipe premise with the wrong shape used to panic the
// serializer on a failed type assertion / index out of range.
func TestUpliftMalformedPipeMarkerErrorsNotPanics(t *testing.T) {
	inputs := []string{
		`p(X) :- q(X), sys_emit_pipe(X).`,
		`p(X) :- q(X), sys_emit_pipe("a", "b").`,
		`p(X) :- q(X), sys_emit_pipe("a", "b", X, "d").`,
	}
	for _, input := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Sanitize(%q) panicked: %v", input, r)
				}
			}()
			_, err := NewSanitizer().Sanitize(input)
			if err == nil {
				t.Fatalf("Sanitize(%q) = success, want malformed-marker error", input)
			}
		}()
	}
}

// Two aggregations in one clause used to lose all but the last: aggInfo was
// overwritten per marker while every marker was dropped from the premises.
func TestUpliftMultipleAggregationsSurvive(t *testing.T) {
	s := NewSanitizer()
	input := `stats(K, C, T) :- item(K, V), C = count(V), T = sum(V).`
	output, err := s.Sanitize(input)
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}
	for _, want := range []string{"let C = fn:count(V)", "let T = fn:sum(V)", "group_by(K)"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output %q missing %q", output, want)
		}
	}
	if _, err := mangle.ParseUnit(strings.NewReader(output)); err != nil {
		t.Fatalf("multi-agg output does not parse: %v\n%s", err, output)
	}
}

// SQL-shaped text inside a string literal is data, not an aggregation: the
// preprocessor must not rewrite it into a predicate.
func TestUpliftAggPatternInsideStringUntouched(t *testing.T) {
	s := NewSanitizer()
	input := `note("C = count(X)") :- item(X).`
	output, err := s.Sanitize(input)
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}
	if !strings.Contains(output, `"C = count(X)"`) {
		t.Fatalf("string data was rewritten into code: %q", output)
	}
	if strings.Contains(output, "llm_agg") || strings.Contains(output, "|>") {
		t.Fatalf("phantom aggregation produced from string data: %q", output)
	}
}

// Without a declared generator universe, unsafe rules pass through untouched
// so the REAL unsafe-variable error (not a mystifying undeclared
// candidate_node) reaches the retry loop.
func TestUpliftInjectionNeedsDeclaredGenerator(t *testing.T) {
	s := NewSanitizer()
	output, err := s.Sanitize(`unsafe(X) :- !safe(X).`)
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}
	if strings.Contains(output, "candidate_node") {
		t.Fatalf("injected undeclared generator: %q", output)
	}
}

// Interning a string that cannot be a bare atom (spaces) must fail loudly at
// the reparse gate — not emit garbage that dies later in the sandbox.
func TestUpliftUninternableStringFailsLoudly(t *testing.T) {
	s := NewSanitizer()
	_, err := s.Sanitize(`research_topic("two words", "x", "y").`)
	if err == nil || !strings.Contains(err.Error(), "does not parse") {
		t.Fatalf("Sanitize() = %v, want reparse-gate error", err)
	}
}

// Group-by keys come from map iteration: the output must be byte-identical
// across runs, not key-order soup.
func TestUpliftGroupByKeysDeterministic(t *testing.T) {
	s := NewSanitizer()
	input := `s(B, A, C) :- item(B, A, V), C = count(V).`
	first, err := s.Sanitize(input)
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}
	if !strings.Contains(first, "group_by(A,B)") {
		t.Fatalf("group-by keys not sorted: %q", first)
	}
	for i := 0; i < 10; i++ {
		again, err := s.Sanitize(input)
		if err != nil {
			t.Fatalf("Sanitize() error = %v", err)
		}
		if again != first {
			t.Fatalf("nondeterministic output:\n%s\n%s", first, again)
		}
	}
}
