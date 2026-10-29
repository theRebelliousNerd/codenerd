package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCountArgs(t *testing.T) {
	cases := map[string]int{
		"":            0,
		"   ":         0,
		"X":           1,
		"X, Y":        2,
		"A, B, C":     3,
		"Name, /atom": 2,
	}
	for in, want := range cases {
		if got := countArgs(in); got != want {
			t.Errorf("countArgs(%q)=%d, want %d", in, got, want)
		}
	}
}

func TestParsePolicyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.mg")
	content := "# SECTION 1: Core Intent\n" +
		"# Priority: 90\n" +
		"next_action(Verb, Target) :- user_intent(Verb), focus(Target).\n" +
		"# a duplicate head with same arity should be deduped\n" +
		"next_action(Verb, Target) :- other(Verb, Target).\n" +
		"single_arg(X) :- base(X).\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	preds, err := parsePolicyFile(path)
	if err != nil {
		t.Fatalf("parsePolicyFile: %v", err)
	}

	byName := map[string]PredicateEntry{}
	for _, p := range preds {
		byName[p.Name] = p
	}
	na, ok := byName["next_action"]
	if !ok {
		t.Fatal("expected next_action predicate to be extracted")
	}
	if na.Arity != 2 {
		t.Errorf("next_action arity=%d, want 2", na.Arity)
	}
	if na.Type != "IDB" {
		t.Errorf("rule head should be classified IDB, got %q", na.Type)
	}
	if na.ActivationPriority != 90 {
		t.Errorf("next_action priority=%d, want 90 from comment annotation", na.ActivationPriority)
	}
	// Deduplication: next_action/2 appears only once despite two rule heads.
	count := 0
	for _, p := range preds {
		if p.Name == "next_action" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("next_action should be deduplicated to 1 entry, got %d", count)
	}
	if sa, ok := byName["single_arg"]; !ok || sa.Arity != 1 {
		t.Errorf("single_arg/1 not extracted correctly: %+v (ok=%v)", sa, ok)
	}
}

func TestParsePolicyDir_DedupAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	a := "foo(X) :- a(X).\nbar(X, Y) :- b(X, Y).\n"
	b := "foo(X) :- c(X).\n" // duplicate foo/1 across files
	if err := os.WriteFile(filepath.Join(dir, "a.mg"), []byte(a), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.mg"), []byte(b), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-.mg file must be ignored.
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("foo(X) :- ignore(X)."), 0o644)

	preds, err := parsePolicyDir(dir)
	if err != nil {
		t.Fatalf("parsePolicyDir: %v", err)
	}
	names := map[string]int{}
	for _, p := range preds {
		names[p.Name]++
	}
	if names["foo"] != 1 {
		t.Errorf("foo/1 should be deduplicated across files, got %d", names["foo"])
	}
	if names["bar"] != 1 {
		t.Errorf("bar/2 should be present once, got %d", names["bar"])
	}
}
