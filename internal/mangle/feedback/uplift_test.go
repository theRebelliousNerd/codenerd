package feedback

// Behavioral uplift tests for types.go, loop.go, and pre_validator.go.
// Each test pins an observable contract: budget atomicity, honest validation
// outcomes, complete error reporting, and literal-aware structural checks.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

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

func TestUpliftGenerateSurfacesLLMErrorWithoutRetry(t *testing.T) {
	fl := NewFeedbackLoop(DefaultConfig())
	llm := &MockLLMClient{}
	validator := &FlexibleMockRuleValidator{acceptContains: ":-"}
	result, err := fl.GenerateAndValidate(context.Background(), llm, validator, "sys", "make rule", "test")
	if err == nil || !strings.Contains(err.Error(), "LLM call failed") {
		t.Fatalf("err = %v, want LLM call failed", err)
	}
	if result.Valid {
		t.Fatal("result.Valid = true, want false on LLM error")
	}
	if result.Attempts != 1 {
		t.Fatalf("Attempts = %d, want 1 (no retry on transport error)", result.Attempts)
	}
}

func TestUpliftGenerateRetriesEmptyThenSucceeds(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableAutoRepair = false
	fl := NewFeedbackLoop(cfg)
	valid := `next_action(/run_tests) :- test_state(/failing).`
	llm := &MockLLMClient{responses: []string{"   ", valid}}
	validator := &MockRuleValidator{validRules: map[string]bool{valid: true}}
	result, err := fl.GenerateAndValidate(context.Background(), llm, validator, "sys", "make rule", "test")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !result.Valid {
		t.Fatalf("Valid = false, errors %v, want true", result.Errors)
	}
	if result.Attempts != 2 {
		t.Fatalf("Attempts = %d, want 2 (empty then valid)", result.Attempts)
	}
}

func TestUpliftGenerateBudgetExhausted(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 5, SessionBudget: 1, EnableAutoRepair: false}
	fl := NewFeedbackLoop(cfg)
	llm := &MockLLMClient{responses: []string{"invalid(syntax).", "out(X) :- in(X)."}}
	validator := &FlexibleMockRuleValidator{acceptContains: ":-"}
	result, err := fl.GenerateAndValidate(context.Background(), llm, validator, "sys", "make rule", "test")
	if err == nil || !strings.Contains(err.Error(), "validation budget exhausted") {
		t.Fatalf("err = %v, want validation budget exhausted", err)
	}
	if result.Valid {
		t.Fatal("Valid = true, want false when budget blocks the retry")
	}
	if result.Attempts != 2 {
		t.Fatalf("Attempts = %d, want 2 (first consumes the single session token)", result.Attempts)
	}
	if len(result.Errors) == 0 {
		t.Fatal("Errors is empty, want the first attempt's compile error preserved")
	}
}

func TestUpliftGenerateCancelledContext(t *testing.T) {
	fl := NewFeedbackLoop(DefaultConfig())
	valid := `next_action(/run_tests) :- test_state(/failing).`
	llm := &MockLLMClient{responses: []string{valid}}
	validator := &MockRuleValidator{validRules: map[string]bool{valid: true}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := fl.GenerateAndValidate(ctx, llm, validator, "sys", "prompt", "test")
	if err == nil || !strings.Contains(err.Error(), "deadline exhausted") {
		t.Fatalf("err = %v, want deadline exhausted", err)
	}
	if result.Attempts != 1 {
		t.Fatalf("Attempts = %d, want 1", result.Attempts)
	}
	if llm.callCount != 0 {
		t.Fatalf("LLM called %d times with cancelled ctx, want 0", llm.callCount)
	}
}

func TestUpliftValidateOnlySurfacesCompileFailure(t *testing.T) {
	fl := NewFeedbackLoop(DefaultConfig())
	validator := &MockRuleValidator{validRules: map[string]bool{}}
	result := fl.ValidateOnly(`out(X) :- in(X).`, validator)
	if result.Valid {
		t.Fatal("ValidateOnly = valid, want invalid when the sandbox rejects the rule")
	}
	if !result.HasErrors() {
		t.Fatal("HasErrors = false, want true on compile failure")
	}
	found := false
	for _, e := range result.Errors {
		if e.Category == CategoryParse {
			found = true
		}
	}
	if !found {
		t.Fatalf("errors %v do not include the compile failure", result.Errors)
	}
}

func TestUpliftQuickFixCoversRemainingRewrites(t *testing.T) {
	pv := NewPreValidator()
	cases := []struct{ in, want string }{
		{`state(:active) :- item(X).`, `state(/active) :- item(X).`},
		{`fn:Min(X)`, `fn:min(X)`},
		{`fn:Max(X)`, `fn:max(X)`},
		{`fn:Avg(X)`, `fn:avg(X)`},
		{"`out(X) :- in(X).`", `out(X) :- in(X).`},
		{`state("pending") :- item(X).`, `state(/pending) :- item(X).`},
		{`a :- b |> fn:sum(c).`, `a :- b |> do fn:sum(c).`},
	}
	for _, c := range cases {
		if got := pv.QuickFix(c.in); got != c.want {
			t.Errorf("QuickFix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUpliftStripStringsIgnoresHashInString(t *testing.T) {
	pv := NewPreValidator()
	rule := `desc(X, "a#b") :- item(X).`
	for _, e := range pv.Validate(rule) {
		if e.Category == CategorySyntax && strings.Contains(e.Message, "parentheses") {
			t.Fatalf("false unbalanced-parentheses on %q: %v", rule, e)
		}
	}
	bare := stripStringsAndComments("a \"b#c\" d # e\nf")
	if strings.Contains(bare, "b#c") {
		t.Fatalf("string literal leaked through strip: %q", bare)
	}
	if strings.Contains(bare, " e") {
		t.Fatalf("comment leaked through strip: %q", bare)
	}
	for _, want := range []string{"a", "d", "f"} {
		if !strings.Contains(bare, want) {
			t.Fatalf("strip dropped live code %q from %q", want, bare)
		}
	}
}

func TestUpliftClassifyEmptyMultiAndOutOfRange(t *testing.T) {
	ec := NewErrorClassifier()
	if got := ec.Classify(""); len(got) != 0 {
		t.Fatalf("Classify(\"\") = %v, want no errors", got)
	}
	multi := ec.Classify("stratification violation: undeclared predicate foo")
	if len(multi) < 2 {
		t.Fatalf("multi-match = %v, want >= 2 errors", multi)
	}
	seen := map[ErrorCategory]bool{}
	for _, e := range multi {
		seen[e.Category] = true
	}
	if !seen[CategoryStratification] || !seen[CategoryUndeclaredPredicate] {
		t.Fatalf("multi-match categories = %v, want stratification and undeclared", multi)
	}
	errs := ec.ClassifyWithContext("99:1 parse error", "only one line")
	if len(errs) == 0 {
		t.Fatal("expected one error for 99:1 parse error")
	}
	if errs[0].Wrong != "" {
		t.Fatalf("Wrong = %q for out-of-range line, want empty", errs[0].Wrong)
	}
}

func TestUpliftFormatErrorVariants(t *testing.T) {
	minimal := ValidationError{Category: CategoryParse, Message: "boom", Suggestion: "fix it"}
	out := FormatErrorForFeedback(minimal)
	if strings.Contains(out, "LINE") {
		t.Fatalf("minimal output names a line it has none: %q", out)
	}
	if !strings.Contains(out, "PROBLEM: boom") {
		t.Fatalf("minimal output missing PROBLEM: %q", out)
	}
	if !strings.Contains(out, "FIX: fix it") {
		t.Fatalf("minimal output missing FIX: %q", out)
	}
	if strings.Contains(out, "WRONG:") {
		t.Fatalf("minimal output invents WRONG block: %q", out)
	}
	wrongOnly := ValidationError{Category: CategoryParse, Message: "m", Wrong: "bad code"}
	out2 := FormatErrorForFeedback(wrongOnly)
	if !strings.Contains(out2, "bad code") {
		t.Fatalf("wrong-only output dropped the snippet: %q", out2)
	}
	if strings.Contains(out2, "WRONG:") {
		t.Fatalf("wrong-without-fix must not emit WRONG/CORRECT block: %q", out2)
	}
	if strings.Contains(out2, "FIX:") {
		t.Fatalf("wrong-without-suggestion must not emit FIX: %q", out2)
	}
}

func TestUpliftLimitPredicateListAndQuery(t *testing.T) {
	many := make([]string, 150)
	for i := range many {
		many[i] = "pred/1"
	}
	many[0] = "first/1"
	many[99] = "last/1"
	got := limitPredicateList(many, 100)
	if len(got) != 100 {
		t.Fatalf("truncated len = %d, want 100", len(got))
	}
	if got[0] != "first/1" || got[99] != "last/1" {
		t.Fatalf("truncation must keep the first 100 in order, got %q..%q", got[0], got[99])
	}
	if kept := limitPredicateList(many, 0); len(kept) != 150 {
		t.Fatalf("non-positive limit must return the full list, got %d", len(kept))
	}
	if short := limitPredicateList([]string{"a/1"}, 100); len(short) != 1 || short[0] != "a/1" {
		t.Fatalf("short list must pass through, got %v", short)
	}
	if q := buildPredicateQuery("make rule", nil); q != "make rule" {
		t.Fatalf("empty-error query = %q, want the prompt unchanged", q)
	}
	q2 := buildPredicateQuery("make rule", []ValidationError{{Category: CategoryAtomString, Message: "  bad atom  "}})
	if !strings.Contains(q2, "atom_string_confusion") || !strings.Contains(q2, "bad atom") {
		t.Fatalf("query missing category or trimmed message: %q", q2)
	}
	if strings.Contains(q2, "  bad atom  ") {
		t.Fatalf("query left the message untrimmed: %q", q2)
	}
}

func TestUpliftSelectPredicatesFallbacks(t *testing.T) {
	fl := NewFeedbackLoop(DefaultConfig())
	validator := &MockRuleValidator{predicates: []string{"static/1"}}
	fl.SetPredicateSelector(&MockPredicateSelector{SelectFunc: func(ctx context.Context, _, _, _, _ string) ([]string, error) {
		return nil, errors.New("boom")
	}})
	if got := fl.selectPredicates(nil, "dom", "prompt", nil, validator); len(got) != 1 || got[0] != "static/1" {
		t.Fatalf("error selector must fall back to declared predicates, got %v", got)
	}
	fl.SetPredicateSelector(&MockPredicateSelector{SelectFunc: func(ctx context.Context, _, _, _, _ string) ([]string, error) {
		return nil, nil
	}})
	if got := fl.selectPredicates(context.Background(), "dom", "prompt", nil, validator); len(got) != 1 || got[0] != "static/1" {
		t.Fatalf("empty selector must fall back to declared predicates, got %v", got)
	}
	fl.SetPredicateSelector(&MockPredicateSelector{SelectFunc: func(ctx context.Context, _, _, _, _ string) ([]string, error) {
		return []string{"jit/1"}, nil
	}})
	if got := fl.selectPredicates(context.Background(), "dom", "prompt", nil, validator); len(got) != 1 || got[0] != "jit/1" {
		t.Fatalf("selector hit must win over declared predicates, got %v", got)
	}
}

func TestUpliftPromptProtocolAndFinalAttempt(t *testing.T) {
	pb := NewPromptBuilder()
	synth := pb.BuildInitialPromptAdditions([]string{"a/1"}, OutputProtocolSynth)
	if !strings.Contains(synth, "mangle_synth_v1") {
		t.Fatalf("synth additions missing schema marker: %q", synth)
	}
	rule := pb.BuildInitialPromptAdditions([]string{"a/1"}, OutputProtocolRule)
	if !strings.Contains(rule, "Example Rules") {
		t.Fatalf("rule additions missing examples: %q", rule)
	}
	finalCtx := FeedbackContext{OriginalRule: "out(X) :- in(X).", Errors: []ValidationError{{Category: CategoryParse, Message: "boom"}}, AttemptNumber: 3, MaxAttempts: 3, OutputProtocol: OutputProtocolRule}
	if out := pb.BuildFeedbackPrompt(finalCtx); !strings.Contains(out, "FINAL ATTEMPT") {
		t.Fatalf("final attempt missing simplification block: %q", out)
	}
	guided := FeedbackContext{OriginalRule: "r", Errors: []ValidationError{{Category: CategoryAggregation, Message: "agg"}, {Category: CategoryUnboundNegation, Message: "neg"}}, AttemptNumber: 2, MaxAttempts: 3, OutputProtocol: OutputProtocolRule}
	out2 := pb.BuildFeedbackPrompt(guided)
	if !strings.Contains(out2, "Aggregation Syntax") {
		t.Fatalf("attempt-2 additions missing aggregation guidance: %q", out2)
	}
	if !strings.Contains(out2, "Safe Negation") {
		t.Fatalf("attempt-2 additions missing negation guidance: %q", out2)
	}
	synthCtx := FeedbackContext{OriginalRule: `{"a":1}`, Errors: []ValidationError{{Category: CategoryParse, Message: "boom"}}, AttemptNumber: 2, MaxAttempts: 3, OutputProtocol: OutputProtocolSynth}
	if out3 := pb.BuildFeedbackPrompt(synthCtx); !strings.Contains(out3, "JSON spec") {
		t.Fatalf("synth feedback missing JSON header: %q", out3)
	}
}

func TestUpliftExtractRuleArrayAndNested(t *testing.T) {
	if got := ExtractRuleFromResponse(`["not a rule", "out(X) :- in(X)."]`); got != "out(X) :- in(X)." {
		t.Fatalf("array extraction = %q, want the second element's rule", got)
	}
	if got := ExtractRuleFromResponse(`{"surface_response": {"rule": "out(X) :- in(X)."}}`); got != "out(X) :- in(X)." {
		t.Fatalf("nested extraction = %q, want the inner rule", got)
	}
	if got := ExtractRuleFromResponse(`[]`); got != "" {
		t.Fatalf("empty array = %q, want empty", got)
	}
}

func TestUpliftNormalizeTrailingAndUnicode(t *testing.T) {
	in := "foo :- bar(\"\\u00e9\")."
	got := NormalizeRuleInput(in)
	if !strings.Contains(got, "\\\\u00e9") {
		t.Fatalf("bare \\u escape was not doubled: %q", got)
	}
	badBraces := "foo :- bar(\"\\u{00e9abc}\")."
	got2 := NormalizeRuleInput(badBraces)
	if !strings.Contains(got2, "\\\\u{") {
		t.Fatalf("over-long unicode escape was not doubled: %q", got2)
	}
	if !isLowerHexDigit('a') || !isLowerHexDigit('9') {
		t.Fatal("isLowerHexDigit rejects valid lowercase hex")
	}
	if isLowerHexDigit('G') || isLowerHexDigit('g') {
		t.Fatal("isLowerHexDigit accepts non-lowercase hex")
	}
	if hexValue('0') != 0 || hexValue('a') != 10 || hexValue('f') != 15 {
		t.Fatal("hexValue decodes incorrectly")
	}
	if !isASCIISpace(' ') || !isASCIISpace('\t') || isASCIISpace('x') {
		t.Fatal("isASCIISpace misclassifies space/tab/x")
	}
	if _, ok := unicodeEscapeEnd(`\u{00e9}`, 0); !ok {
		t.Fatal("valid \\u{h...} rejected")
	}
	if _, ok := unicodeEscapeEnd(`\u00e9`, 0); ok {
		t.Fatal("brace-less \\u accepted")
	}
}

func TestUpliftHashPromptDeterministic(t *testing.T) {
	h1 := hashPrompt("hello")
	h2 := hashPrompt("hello")
	h3 := hashPrompt("world")
	if h1 != h2 {
		t.Fatalf("hash not deterministic: %q vs %q", h1, h2)
	}
	if h1 == h3 {
		t.Fatal("distinct prompts share a hash")
	}
	if len(h1) != 16 {
		t.Fatalf("hash len = %d, want 16 hex chars", len(h1))
	}
	for _, c := range h1 {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			t.Fatalf("hash %q is not lowercase hex", h1)
		}
	}
}

func TestUpliftTryRecordPerRuleLimit(t *testing.T) {
	b := NewValidationBudget(RetryConfig{MaxRetries: 2, SessionBudget: 10})
	if ok, _ := b.TryRecord("r"); !ok {
		t.Fatal("first TryRecord rejected")
	}
	if ok, _ := b.TryRecord("r"); !ok {
		t.Fatal("second TryRecord rejected")
	}
	if ok, reason := b.TryRecord("r"); ok || !strings.Contains(reason, "max retries") {
		t.Fatalf("third TryRecord = (%v, %q), want per-rule rejection", ok, reason)
	}
	if ok, _ := b.TryRecord("other"); !ok {
		t.Fatal("fresh hash rejected while session budget remains")
	}
}
