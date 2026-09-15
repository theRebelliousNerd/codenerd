package feedback

// Behavioral uplift tests for types.go, loop.go, and pre_validator.go.
// Each test pins an observable contract: budget atomicity, honest validation
// outcomes, complete error reporting, and literal-aware structural checks.

import (
	"context"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"codenerd/internal/mangle"
)

// The zero category must be unknown, never a phantom parse error, and must
// never claim auto-repairability.
func TestUpliftZeroCategoryIsUnknown(t *testing.T) {
	var zero ErrorCategory
	if zero != CategoryUnknown {
		t.Fatalf("zero ErrorCategory = %d, want CategoryUnknown (%d)", zero, CategoryUnknown)
	}
	if got := zero.String(); got != "unknown" {
		t.Fatalf("zero category String() = %q, want %q", got, "unknown")
	}
	if zero.IsAutoRepairable() {
		t.Fatal("zero category IsAutoRepairable() = true, want false")
	}
	if got := (ValidationError{}).Category; got != CategoryUnknown {
		t.Fatalf("unset ValidationError category = %v, want CategoryUnknown", got)
	}
}

// HasErrors on a value receiver must work on non-addressable results too
// (calling it on a literal would not compile with a pointer receiver).
func TestUpliftHasErrorsWorksOnValues(t *testing.T) {
	if (ValidationResult{}).HasErrors() {
		t.Fatal("empty result HasErrors() = true, want false")
	}
	if !(ValidationResult{Errors: []ValidationError{{Message: "x"}}}).HasErrors() {
		t.Fatal("result with errors HasErrors() = false, want true")
	}
}

// Concurrent TryRecord claims must never exceed the session budget: exactly
// `budget` claims succeed no matter how the goroutines interleave.
func TestUpliftTryRecordNeverOverClaims(t *testing.T) {
	budget := NewValidationBudget(RetryConfig{MaxRetries: 1000, SessionBudget: 20})
	const goroutines = 8
	const perGoroutine = 10
	var wg sync.WaitGroup
	won := make(chan bool, goroutines*perGoroutine)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			// Distinct hash per goroutine so only the session cap binds.
			hash := string(rune('a' + g))
			for i := 0; i < perGoroutine; i++ {
				ok, _ := budget.TryRecord(hash)
				won <- ok
			}
		}(g)
	}
	wg.Wait()
	close(won)
	successes := 0
	for ok := range won {
		if ok {
			successes++
		}
	}
	if successes != 20 {
		t.Fatalf("TryRecord successes = %d, want exactly 20 (the session budget)", successes)
	}
	if used, total := budget.Stats(); used != 20 || total != 20 {
		t.Fatalf("budget stats = (%d, %d), want (20, 20)", used, total)
	}
}

// A non-positive MaxRetries is a configuration error, not a zero-attempt
// validation failure: the LLM must never be called.
func TestUpliftGenerateRejectsNonPositiveMaxRetries(t *testing.T) {
	for _, max := range []int{0, -3} {
		cfg := DefaultConfig()
		cfg.MaxRetries = max
		fl := NewFeedbackLoop(cfg)
		llm := &MockLLMClient{responses: []string{"out(X) :- in(X)."}}
		validator := &FlexibleMockRuleValidator{acceptContains: ":-"}
		_, err := fl.GenerateAndValidate(context.Background(), llm, validator, "sys", "make rule", "test")
		if err == nil || !strings.Contains(err.Error(), "MaxRetries") {
			t.Fatalf("MaxRetries=%d: err = %v, want misconfiguration error", max, err)
		}
		if llm.callCount != 0 {
			t.Fatalf("MaxRetries=%d: LLM called %d times, want 0", max, llm.callCount)
		}
	}
}

// A rule that compiles after repair must come back Valid with NO blocking
// errors: input observations are demoted to warnings, not left contradicting
// the verdict.
func TestUpliftValidateOnlyDemotesRepairedErrors(t *testing.T) {
	fl := NewFeedbackLoop(DefaultConfig())
	// Enum-like string trips the atom/string pre-check, but the rule parses
	// and the mock sandbox accepts it.
	rule := `state(X, "pending") :- item(X).`
	validator := &FlexibleMockRuleValidator{acceptContains: ":-"}

	result := fl.ValidateOnly(rule, validator)
	if !result.Valid {
		t.Fatalf("ValidateOnly = invalid, errors %v; want valid", result.Errors)
	}
	if result.HasErrors() {
		t.Fatalf("valid result carries blocking errors: %v", result.Errors)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("valid result carries no warnings; the repaired atom/string observation was dropped, not demoted")
	}
	found := false
	for _, w := range result.Warnings {
		if w.Category == CategoryAtomString {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings %v do not include the atom/string observation", result.Warnings)
	}
	if result.Sanitized == "" {
		t.Fatal("Sanitized is empty on success; legislator falls back to the unrepaired rule")
	}
}

// When the sanitizer cannot even parse the rule, the result must name the
// real parse failure — not whatever the sandbox says about "".
func TestUpliftValidateOnlySurfacesSanitizerFailure(t *testing.T) {
	fl := NewFeedbackLoop(DefaultConfig())
	validator := &FlexibleMockRuleValidator{acceptContains: ":-"}

	result := fl.ValidateOnly("((( not a rule", validator)
	if result.Valid {
		t.Fatal("ValidateOnly(garbage) = valid, want invalid")
	}
	found := false
	for _, e := range result.Errors {
		if e.Category == CategoryParse && strings.Contains(strings.ToLower(e.Message), "parse") {
			found = true
		}
	}
	if !found {
		t.Fatalf("errors %v do not include the sanitizer parse failure", result.Errors)
	}
}

// Two defects on one line are two errors: single-match reporting wastes a
// whole LLM retry round per hidden defect.
func TestUpliftPreValidatorReportsEveryMatch(t *testing.T) {
	pv := NewPreValidator()
	errs := pv.Validate(`state(X, "pending", "active") :- item(X).`)
	count := 0
	for _, e := range errs {
		if e.Category == CategoryAtomString {
			count++
		}
	}
	if count < 2 {
		t.Fatalf("atom/string errors = %d, want >= 2 for two enum strings on one line (%v)", count, errs)
	}
}

// Parens inside strings and comments are not structure: flagging them sends
// the LLM chasing a defect that does not exist.
func TestUpliftParenCheckIgnoresStringsAndComments(t *testing.T) {
	pv := NewPreValidator()
	rule := "desc(X, \"a)b\") :- item(X). # trailing ( comment\n"
	for _, e := range pv.Validate(rule) {
		if e.Category == CategorySyntax && strings.Contains(e.Message, "parentheses") {
			t.Fatalf("false unbalanced-parentheses on %q: %v", rule, e)
		}
	}
	// And a genuinely unbalanced rule must still be caught.
	bad := "desc(X :- item(X)."
	found := false
	for _, e := range pv.Validate(bad) {
		if e.Category == CategorySyntax && strings.Contains(e.Message, "parentheses") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no unbalanced-parentheses error for %q", bad)
	}
}

// Truncation must never split a rune: the query goes into an LLM prompt.
func TestUpliftTruncateKeepsRunesWhole(t *testing.T) {
	got := truncatePredicateQuery(strings.Repeat("é", 300), 200)
	if !utf8.ValidString(got) {
		t.Fatal("truncatePredicateQuery produced invalid UTF-8")
	}
	if n := len([]rune(strings.TrimSuffix(got, "..."))); n != 200 {
		t.Fatalf("truncated to %d runes, want 200", n)
	}
	// Short multi-byte text passes through untouched.
	short := "héllo wörld"
	if got := truncatePredicateQuery(short, 200); got != short {
		t.Fatalf("short text rewritten to %q", got)
	}
}

// The whole point of normalization is parser acceptance: every normalized
// rule below must parse with the REAL Mangle parser, not just match a string.
func TestUpliftNormalizedRulesParseWithRealParser(t *testing.T) {
	rules := []string{
		`out(X) :- bar("C:\path\to\file", X).`,
		`out(X) :- bar("hello\n\tworld", X).`,
		`out(X) :- bar("a\rb\fc\0d", X).`,
		`out(X) :- bar("A\x41B\u{00e9}C", X).`,
		`out(X) :- bar("C:\xerox", X).`,
		`out(X) :- \+ bad(X), bar("a\+b", X).`,
		"out(X) :- bar(\"tab\there\", X).",
	}
	for _, rule := range rules {
		normalized := NormalizeRuleInput(rule)
		if _, err := mangle.ParseUnit(strings.NewReader("Decl out(X). Decl bar(X, Y). Decl bad(X). " + normalized)); err != nil {
			t.Errorf("NormalizeRuleInput(%q) = %q, which fails to parse: %v", rule, normalized, err)
		}
	}
}

// Meaning must survive normalization: a well-formed hex escape still denotes
// the same character after the round trip.
func TestUpliftHexEscapeKeepsMeaning(t *testing.T) {
	got := NormalizeRuleInput(`out(X) :- bar("A\x41B", X).`)
	if !strings.Contains(got, `\x41`) {
		t.Fatalf("well-formed \\x41 escape was rewritten: %q", got)
	}
	unit, err := mangle.ParseUnit(strings.NewReader("Decl out(X). Decl bar(X, Y). " + got))
	if err != nil {
		t.Fatalf("normalized rule fails to parse: %v", err)
	}
	found := false
	for _, clause := range unit.Clauses {
		for _, prem := range clause.Premises {
			if atom, ok := prem.(interface{ String() string }); ok && strings.Contains(atom.String(), "AAB") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("normalized %q lost the \\x41 = A meaning", got)
	}
}

// An empty early key must not shadow a populated later key in JSON extraction.
func TestUpliftExtractRuleSkipsEmptyKeys(t *testing.T) {
	response := `{"rule": "   ", "output": "out(X) :- in(X)."}`
	if got := ExtractRuleFromResponse(response); got != "out(X) :- in(X)." {
		t.Fatalf("ExtractRuleFromResponse = %q, want the populated output key", got)
	}
}
