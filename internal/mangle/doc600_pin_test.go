package mangle

import (
	"strings"
	"testing"
)

// TestDoc600Claims pins every behavioral claim in
// 600-TYPE_SYSTEM.md (bound-block grammar). If the grammar changes,
// the doc and this test change together.
func TestDoc600Claims(t *testing.T) {
	load := func(src string) error {
		eng, err := NewEngine(DefaultConfig(), nil)
		if err != nil {
			t.Fatal(err)
		}
		return eng.LoadSchemaString(src)
	}

	// All four doc WRONG examples are rejected.
	rejected := []string{
		"Decl employee(ID.Type<int>, Name.Type<string>).",
		"Decl tags(ID, Tags) bound [/number, [string]].",
		"Decl config(Data) bound [{/host: string}].",
		"Decl flexible(Value) bound [int | string].",
	}
	for _, src := range rejected {
		if err := load(src); err == nil {
			t.Errorf("expected rejection of %q, got nil", src)
		}
	}

	// Bound arity must match Decl arity exactly (fail-closed).
	if err := load("Decl pair(X, Y) bound [/number]."); err == nil ||
		!strings.Contains(err.Error(), "expected 2 bounds, got 1") {
		t.Errorf("expected arity error, got: %v", err)
	}
	if err := load("Decl single(X) bound [/number, /string]."); err == nil ||
		!strings.Contains(err.Error(), "expected 1 bounds, got 2") {
		t.Errorf("expected arity error, got: %v", err)
	}

	// Bound elements: only /name or variable.
	for _, bad := range []string{
		"Decl n(X) bound [42].",
		`Decl s(X) bound ["lit"].`,
		"Decl p(X) bound [foo].",
	} {
		if err := load(bad); err == nil {
			t.Errorf("expected rejection of %q, got nil", bad)
		}
	}

	// Repeating the variable leaves that position unbound (accepted).
	eng, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.LoadSchemaString("Decl pair(X, Y) bound [/number, Y]."); err != nil {
		t.Fatalf("variable bound element should be accepted: %v", err)
	}
	// Unbound position accepts anything.
	if err := eng.AddFact("pair", 1, "anything"); err != nil {
		t.Fatalf("unbound position should accept any value: %v", err)
	}
	// Bounds are coercion guides, not validators (see the known-limitation
	// comment on convertValueToTypedTerm): a mistyped value is stored with
	// its natural type and matches nothing downstream. Pinned so a future
	// fail-closed change updates doc and test together.
	if err := eng.AddFact("pair", "not-a-number", 2); err != nil {
		t.Fatalf("bound leniency changed, update doc + test: %v", err)
	}
	facts, err := eng.GetFacts("pair")
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 2 {
		t.Fatalf("expected 2 stored facts, got %d", len(facts))
	}
}
