package mangle

import (
	"strings"
	"testing"
)

// ValidateProgram used to compute the full Mangle analysis and then discard it
// (`_ = programInfo`), validating instead by splitting the text on "\n" and
// treating any line containing ":-" as a rule. These pin both halves of what
// that stand-in could not see.

// A rule wrapped across lines had only its first fragment checked: the
// continuation lines carry no ":-" so the line scan skipped them, and the
// fragment's truncated body contained no predicates at all. Every predicate
// below the wrap went unvalidated — and rules in defaults/ are almost all
// wrapped.
func TestValidateProgram_WrappedRuleBodyIsValidated(t *testing.T) {
	schemas := `
Decl file_topology(Path).
Decl next_action(Action).
`
	sv := NewSchemaValidator(schemas, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	// ghost_signal is declared locally so the program passes Mangle analysis,
	// but nothing in the running system ever asserts it: the rule can never
	// fire. That is the schema drift this validator exists to catch.
	program := `
Decl file_topology(Path).
Decl next_action(Action).
Decl ghost_signal(Path).
next_action(/review) :-
    file_topology(Path),
    ghost_signal(Path).
`

	err := sv.ValidateProgram(program)
	if err == nil {
		t.Fatal("ValidateProgram accepted a wrapped rule whose body uses an unsourced predicate")
	}
	if !strings.Contains(err.Error(), "ghost_signal") {
		t.Errorf("error should name the unsourced predicate, got: %v", err)
	}
}

// The mirror image: ":-" inside a string literal is not a rule. The line scan
// split on it and validated the remaining text as a rule body, so a predicate
// name that happened to appear inside a quoted string was reported undefined.
func TestValidateProgram_ColonDashInsideStringIsNotARule(t *testing.T) {
	schemas := `
Decl doc_note(Text).
Decl next_action(Action).
`
	sv := NewSchemaValidator(schemas, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	program := `
Decl doc_note(Text).
Decl next_action(Action).
doc_note("example rule: next_action(/x) :- ghost_signal(Y)").
next_action(/review) :- doc_note(_).
`

	if err := sv.ValidateProgram(program); err != nil {
		t.Fatalf("ValidateProgram rejected a fact whose string literal merely contains \":-\": %v", err)
	}
}

// A predicate this program derives is sourced, even though the system schema
// has never heard of it: the rule that produces it is right there.
func TestValidateProgram_LocallyDerivedPredicateIsSourced(t *testing.T) {
	schemas := `
Decl file_topology(Path).
Decl next_action(Action).
`
	sv := NewSchemaValidator(schemas, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	program := `
Decl file_topology(Path).
Decl next_action(Action).
Decl go_file(Path).
go_file(Path) :- file_topology(Path).
next_action(/review) :- go_file(_).
`

	if err := sv.ValidateProgram(program); err != nil {
		t.Fatalf("ValidateProgram rejected a predicate the program itself derives: %v", err)
	}
}

// Negation and builtins in a body must not read as undefined predicates.
func TestValidateProgram_NegationAndBuiltinsAreNotDrift(t *testing.T) {
	schemas := `
Decl user(Name).
Decl admin(Name).
Decl regular(Name).
`
	sv := NewSchemaValidator(schemas, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	program := `
Decl user(Name).
Decl admin(Name).
Decl regular(Name).
regular(X) :- user(X), !admin(X).
`

	if err := sv.ValidateProgram(program); err != nil {
		t.Fatalf("ValidateProgram rejected a negated premise: %v", err)
	}
}
