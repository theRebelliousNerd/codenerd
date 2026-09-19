package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMangleProgram(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "probe.mg")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	return path
}

// --eval is how a rule's behaviour is found out rather than argued about: the
// derived facts print in the engine's own spelling, so a name and a string
// with the same text are told apart.
func TestCheckMangle_EvalPrintsWhatTheProgramDerives(t *testing.T) {
	path := writeMangleProgram(t, `
Decl parent(Parent, Child) bound [/name, /name].
Decl sibling(A, B) bound [/name, /name].
Decl label(Thing, Text) bound [/name, /string].
parent(/ann, /bob).
parent(/ann, /cat).
sibling(A, B) :- parent(P, A), parent(P, B), A != B.
label(/bob, "/bob").
`)
	var out bytes.Buffer
	if !checkMangleFiles(&out, []string{path}, checkMangleOptions{standalone: true, eval: []string{"sibling", "label"}}) {
		t.Fatalf("a valid program must check and evaluate:\n%s", out.String())
	}
	for _, want := range []string{
		"sibling: 2 fact(s)",
		"sibling(/bob,/cat).",
		"sibling(/cat,/bob).",
		`label(/bob,"/bob").`,
	} {
		if !strings.Contains(strings.ReplaceAll(out.String(), ", ", ","), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}

// A body predicate nobody declares or defines is an analysis error, and an
// evaluation request for an undeclared predicate is reported, not skipped.
func TestCheckMangle_ReportsUndeclaredPredicates(t *testing.T) {
	bad := writeMangleProgram(t, `
Decl sibling(A, B) bound [/name, /name].
sibling(A, B) :- parent(P, A), parent(P, B), A != B.
`)
	var out bytes.Buffer
	if checkMangleFiles(&out, []string{bad}, checkMangleOptions{standalone: true}) {
		t.Fatalf("a rule over an undeclared predicate must fail:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "could not find predicate parent") {
		t.Errorf("the error should name the missing predicate:\n%s", out.String())
	}

	good := writeMangleProgram(t, "Decl p(X) bound [/name].\np(/a).\n")
	out.Reset()
	if checkMangleFiles(&out, []string{good}, checkMangleOptions{standalone: true, eval: []string{"q"}}) {
		t.Fatalf("reading an undeclared predicate must fail the run:\n%s", out.String())
	}
}

// Without --standalone the shared schemas load first, so a program that
// declares a predicate codeNERD also declares collides; standalone checks the
// program on its own.
func TestCheckMangle_StandaloneSkipsTheSharedSchemas(t *testing.T) {
	path := writeMangleProgram(t, `
Decl user_intent(ID, Category, Verb, Target, Constraint) bound [/name, /name, /name, /string, /string].
user_intent(/i1, /mutation, /fix, "a.go", "").
`)
	var out bytes.Buffer
	if checkMangleFiles(&out, []string{path}, checkMangleOptions{}) {
		t.Fatalf("with the shared schemas loaded, a second Decl of user_intent must be rejected:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "declared more than once") {
		t.Errorf("expected the duplicate-declaration error:\n%s", out.String())
	}
	out.Reset()
	if !checkMangleFiles(&out, []string{path}, checkMangleOptions{standalone: true}) {
		t.Fatalf("standalone, the program is valid:\n%s", out.String())
	}
}
