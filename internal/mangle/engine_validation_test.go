package mangle

import (
	"strings"
	"testing"
)

// Tester-shard regression tests for internal/mangle.
// Scope: untested branches that carry real risk — file-scoped fact
// replacement (canonicalPath), fail-closed validation on Add, duplicate
// accounting against FactLimit, and identifier boundaries used by the
// query-shape parser. All table-driven, deterministic, independent.

// TestCanonicalPath_NormalizesSeparators pins that file keys are
// normalized with Clean + ToSlash. If the ToSlash step regresses,
// ReplaceFactsForFile with Windows-style separators orphans facts under a
// second key and file-scoped replacement silently stops working.
func TestCanonicalPath_NormalizesSeparators(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty stays empty", input: "", want: ""},
		{name: "dot stays dot", input: ".", want: "."},
		{name: "double slash collapses", input: "a//b", want: "a/b"},
		{name: "dot segment resolves", input: "a/./b", want: "a/b"},
		{name: "backslash becomes slash", input: `a\b`, want: "a/b"},
		{name: "mixed separators unify", input: `a\b//c/./d`, want: "a/b/c/d"},
		{name: "trailing slash trimmed", input: "a/b/", want: "a/b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canonicalPath(tt.input); got != tt.want {
				t.Errorf("canonicalPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestIsIdentifier_Boundaries pins the identifier gate used when
// deciding whether a string becomes an atom. Only cases with stable contract
// are asserted; leading-underscore policy is deliberately not pinned here.
func TestIsIdentifier_Boundaries(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "empty is not identifier", input: "", want: false},
		{name: "single lowercase", input: "a", want: true},
		{name: "lowercase alnum", input: "abc123", want: true},
		{name: "uppercase start rejected", input: "A", want: false},
		{name: "uppercase start rejected long", input: "Abc", want: false},
		{name: "dash rejected", input: "a-b", want: false},
		{name: "space rejected", input: "a b", want: false},
		{name: "dot rejected", input: "a.b", want: false},
		{name: "digit start rejected", input: "1abc", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIdentifier(tt.input); got != tt.want {
				t.Errorf("isIdentifier(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestAddFact_ValidationFailsClosed pins that bad facts are
// rejected with an honest error and never partially counted. Each case uses
// a fresh engine so factCount==0 proves nothing leaked in.
func TestAddFact_ValidationFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		pred string
		args []any
	}{
		{name: "undeclared predicate", pred: "nope_pred", args: []any{"x", "y"}},
		{name: "arity too few", pred: "test_fact", args: []any{"only-one"}},
		{name: "arity too many", pred: "test_fact", args: []any{"a", "b", "c"}},
		{name: "unsupported func arg", pred: "test_fact", args: []any{func() {}, "b"}},
		{name: "unsupported chan arg", pred: "test_fact", args: []any{make(chan int), "b"}},
		{name: "mixed list with non-string", pred: "test_fact", args: []any{[]any{int64(1)}, "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.AutoEval = false
			engine, err := NewEngine(cfg, nil)
			if err != nil {
				t.Fatalf("NewEngine() error = %v", err)
			}
			if err := engine.LoadSchemaString(`Decl test_fact(X, Y).`); err != nil {
				t.Fatalf("LoadSchemaString() error = %v", err)
			}

			if err := engine.AddFact(tt.pred, tt.args...); err == nil {
				t.Fatalf("AddFact(%q, ...) expected error, got nil", tt.pred)
			}
			if engine.factCount != 0 {
				t.Errorf("factCount = %d after rejected AddFact, want 0 (partial insert leaked)", engine.factCount)
			}
		})
	}
}

// TestAddFact_DuplicateDoesNotDoubleCount pins that inserting the
// same fact twice does not inflate factCount. Double counting would let
// duplicates burn through FactLimit and evict legitimate facts.
func TestAddFact_DuplicateDoesNotDoubleCount(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl test_fact(X, Y).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	if err := engine.AddFact("test_fact", "hello", int64(42)); err != nil {
		t.Fatalf("first AddFact() error = %v", err)
	}
	if err := engine.AddFact("test_fact", "hello", int64(42)); err != nil {
		t.Fatalf("duplicate AddFact() error = %v", err)
	}
	if engine.factCount != 1 {
		t.Errorf("factCount = %d after duplicate insert, want 1", engine.factCount)
	}
}

// TestReplaceFactsForFile_NormalizesSeparators pins that
// ReplaceFactsForFile resolves the file key through canonicalPath. Before
// the normalization fix, replacing via `a\b` left facts stored under `a/b`
// orphaned (factCount stayed 1); after the fix the count drops to 0.
func TestReplaceFactsForFile_NormalizesSeparators(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl file_info(File, Info).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	seed := []Fact{
		{Predicate: "file_info", Args: []any{"a/b", "initial info"}},
	}
	if err := engine.ReplaceFactsForFile("a/b", seed); err != nil {
		t.Fatalf("ReplaceFactsForFile() seed error = %v", err)
	}
	if engine.factCount != 1 {
		t.Fatalf("factCount after seed = %d, want 1", engine.factCount)
	}

	// Same logical file through a Windows-style key with a different fact must
	// replace the seed in place (count stays 1). If the key is not
	// normalized, the seed orphans under the old key and the count grows to 2.
	updated := []Fact{
		{Predicate: "file_info", Args: []any{"a/b", "updated info"}},
	}
	if err := engine.ReplaceFactsForFile(`a\b`, updated); err != nil {
		t.Fatalf("ReplaceFactsForFile() replace error = %v", err)
	}
	if engine.factCount != 1 {
		t.Errorf("factCount = %d after separator-normalized replace, want 1 (orphaned file key)", engine.factCount)
	}
}

// TestAddFact_ErrorMentionsPredicate pins that validation errors
// name the offending predicate so callers can route the failure. Message
// text beyond the predicate name is deliberately not pinned.
func TestAddFact_ErrorMentionsPredicate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(`Decl test_fact(X, Y).`); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}

	err = engine.AddFact("nope_pred", "x", "y")
	if err == nil {
		t.Fatal("AddFact() expected error for undeclared predicate, got nil")
	}
	if !strings.Contains(err.Error(), "nope_pred") {
		t.Errorf("error %q does not mention predicate %q", err.Error(), "nope_pred")
	}
}
