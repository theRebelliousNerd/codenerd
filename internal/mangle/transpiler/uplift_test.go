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

// Sanitize must fail closed on input that does not parse, rather than
// emitting an empty or partial program.
func TestUpliftSanitizeParseErrorFails(t *testing.T) {
	if _, err := NewSanitizer().Sanitize("@@@ not valid mangle @@@"); err == nil {
		t.Fatalf("Sanitize() = success, want parse error")
	}
}

// Plain variable equality is not an aggregation: it passes through without
// a pipe. Only count/sum/min/max/avg are rewritten by the preprocessor.
func TestUpliftUnknownAggFuncNoPipe(t *testing.T) {
	s := NewSanitizer()
	input := `stats(K, C) :- item(K, V), C = V.`
	output, err := s.Sanitize(input)
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}
	if strings.Contains(output, "|>") || strings.Contains(output, "group_by") {
		t.Fatalf("plain equality produced a pipe: %q", output)
	}
	if _, err := mangle.ParseUnit(strings.NewReader(output)); err != nil {
		t.Fatalf("passthrough output does not parse: %v\n%s", err, output)
	}
}

// An llm_agg marker with the wrong arity is not an aggregation: it passes
// through as a regular premise instead of erroring or gaining a pipe.
func TestUpliftMalformedLlmAggArityPassesThrough(t *testing.T) {
	s := NewSanitizer()
	input := `p(X) :- q(X), llm_agg("count", X).`
	output, err := s.Sanitize(input)
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}
	if !strings.Contains(output, "llm_agg") {
		t.Fatalf("malformed llm_agg was dropped: %q", output)
	}
	if strings.Contains(output, "|>") {
		t.Fatalf("malformed llm_agg gained a pipe: %q", output)
	}
}

// An llm_agg marker whose result/arg positions are not variables is left
// alone: the repair only fires on (func-string, result-var, arg-var).
func TestUpliftMalformedLlmAggTypesPassThrough(t *testing.T) {
	s := NewSanitizer()
	input := `p(X) :- q(X), llm_agg("count", "notvar", "alsonotvar").`
	output, err := s.Sanitize(input)
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}
	if !strings.Contains(output, "llm_agg") {
		t.Fatalf("non-variable llm_agg was dropped: %q", output)
	}
	if strings.Contains(output, "|>") {
		t.Fatalf("non-variable llm_agg gained a pipe: %q", output)
	}
}

// isInsideString is the gate that keeps string data from becoming code.
// Escaped quotes must not toggle the state; an unbalanced quote fails closed.
func TestUpliftIsInsideStringEdges(t *testing.T) {
	if isInsideString("abc", 1) {
		t.Errorf("plain text reported as inside string")
	}
	if !isInsideString(`"abc"`, 2) {
		t.Errorf("offset inside quotes not detected")
	}
	if isInsideString(`"abc"`, 5) {
		t.Errorf("offset past closing quote reported as inside string")
	}
	escaped := `"a \"b\" d"`
	if !isInsideString(escaped, len(escaped)-1) {
		t.Errorf("escaped quotes toggled string state: %q", escaped)
	}
	if !isInsideString(`"unclosed`, 5) {
		t.Errorf("unbalanced quote should fail closed (inside string)")
	}
}

// A negated variable that already has a positive binding needs no generator:
// injecting a second candidate_node would duplicate the premise.
func TestUpliftSafetyBoundVarNoDupInjection(t *testing.T) {
	s := NewSanitizer()
	s.validator.ValidPredicates["candidate_node"] = mangle.PredicateSpec{
		Name:  "candidate_node",
		Arity: 1,
		Args:  []mangle.ArgSpec{{Name: "Node", Type: mangle.ArgTypeVariable}},
	}
	output, err := s.Sanitize(`unsafe(X) :- candidate_node(X), !safe(X).`)
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}
	if n := strings.Count(output, "candidate_node(X)"); n != 1 {
		t.Fatalf("bound var injection count = %d, want 1: %q", n, output)
	}
}

// System markers never count as positive bindings: sys_foo(X) must not
// satisfy the safety of X, so the generator is still injected.
func TestUpliftSafetyIgnoresSysPremises(t *testing.T) {
	s := NewSanitizer()
	s.validator.ValidPredicates["candidate_node"] = mangle.PredicateSpec{
		Name:  "candidate_node",
		Arity: 1,
		Args:  []mangle.ArgSpec{{Name: "Node", Type: mangle.ArgTypeVariable}},
	}
	output, err := s.Sanitize(`unsafe(X) :- sys_foo(X), !safe(X).`)
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}
	if !strings.Contains(output, "candidate_node(X)") {
		t.Fatalf("sys_ premise wrongly counted as binding: %q", output)
	}
}

// The same unbound variable in two negations gets exactly one generator.
func TestUpliftSafetyDedupsAcrossNegations(t *testing.T) {
	s := NewSanitizer()
	s.validator.ValidPredicates["candidate_node"] = mangle.PredicateSpec{
		Name:  "candidate_node",
		Arity: 1,
		Args:  []mangle.ArgSpec{{Name: "Node", Type: mangle.ArgTypeVariable}},
	}
	output, err := s.Sanitize(`p(X, Y) :- !a(X), !b(X, Y).`)
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}
	if n := strings.Count(output, "candidate_node(X)"); n != 1 {
		t.Fatalf("candidate_node(X) count = %d, want 1: %q", n, output)
	}
	if !strings.Contains(output, "candidate_node(Y)") {
		t.Fatalf("candidate_node(Y) missing: %q", output)
	}
}

// Predicates the schema does not know pass through untouched: no interning.
func TestUpliftUnknownPredicateUntouched(t *testing.T) {
	s := NewSanitizer()
	output, err := s.Sanitize(`totally_unknown_pred("hello", X) :- other_unknown(X).`)
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}
	if !strings.Contains(output, `"hello"`) {
		t.Fatalf("unknown predicate was interned: %q", output)
	}
	if strings.Contains(output, "/hello") {
		t.Fatalf("unknown predicate gained an atom: %q", output)
	}
}

// UpdateFromProgramInfo(nil) is a no-op that must not disturb the validator.
func TestUpliftUpdateFromProgramInfoNilSafe(t *testing.T) {
	s := NewSanitizer()
	s.UpdateFromProgramInfo(nil)
	output, err := s.Sanitize(`research_topic("gemini", "testing", "pending").`)
	if err != nil {
		t.Fatalf("Sanitize() after nil update error = %v", err)
	}
	if !strings.Contains(output, "research_topic") {
		t.Fatalf("validator disturbed by nil update: %q", output)
	}
}
