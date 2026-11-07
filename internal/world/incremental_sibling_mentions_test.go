package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A declaration change remaps only the siblings that spell the changed
// name. d.go is in package p but never mentions T (only in a comment and a
// string), so a.go turning func T into type T leaves d.go's rows alone,
// while b.go, which calls T, is remapped as before.
func TestIncrementalScan_DeclarationChangeRemapsOnlyMentioningSiblings(t *testing.T) {
	root, db, scanner := scanSiblingPackage(t, false)
	dPath := writeWorkspaceFile(t, root, "d.go", "package p\n\n// T is mentioned here only in prose.\nfunc Other() string { return \"T\" }\n")
	rescan(t, scanner, root, db)
	if _, err := EnsureDeepFactsInRoot(context.Background(), root, []string{dPath}, db, 1); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\n\ntype T int\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	delta := rescan(t, scanner, root, db)

	got := strings.Join(topologyFiles(delta.NewFacts), ",")
	if got != "a.go,b.go" {
		t.Fatalf("remapped files = %s, want a.go,b.go (d.go never names T)", got)
	}
	if !hasFact(delta.NewFacts, "code_calls", "p.Use", "p.T") {
		t.Fatal("b.go, which calls T, was not remapped to the conversion row")
	}
}

func TestChangedNames_EveryDeclarationShape(t *testing.T) {
	before := pkgSymbols{
		types:    map[string]bool{"Kept": true, "Gone": true},
		funcs:    map[string]bool{"F": true},
		methods:  map[string]map[string]bool{"Recv": {"M": true}},
		aliases:  map[string]string{"A": "Kept"},
		aliasBad: map[string]bool{},
	}
	after := pkgSymbols{
		types:    map[string]bool{"Kept": true, "New": true},
		funcs:    map[string]bool{"F": true},
		methods:  map[string]map[string]bool{"Recv": {"M": true, "N": true}},
		aliases:  map[string]string{"A": "New"},
		aliasBad: map[string]bool{},
	}
	got := before.changedNames(after)
	for _, want := range []string{"Gone", "New", "A", "N", "Recv"} {
		if _, ok := got[want]; !ok {
			t.Errorf("changedNames misses %s: %v", want, got)
		}
	}
	for _, not := range []string{"Kept", "F", "M"} {
		if _, ok := got[not]; ok {
			t.Errorf("changedNames names unchanged %s", not)
		}
	}
}

func TestNamesFromKeyDiff_ReadsStoredKeyShapes(t *testing.T) {
	a := map[string]struct{}{"func:p.F": {}, "type:p.T": {}, "method:p.R.M": {}}
	b := map[string]struct{}{"func:p.F": {}, "type:p.U": {}}
	got := namesFromKeyDiff(a, b)
	for _, want := range []string{"T", "U", "R", "M"} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing %s in %v", want, got)
		}
	}
	if _, ok := got["F"]; ok {
		t.Error("an unchanged key contributed a name")
	}
	if _, ok := got["p"]; ok {
		t.Error("the package name was read as a declared name")
	}
}

func TestSiblingsMentioning_ProvenNarrowingOnly(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	calls := write("calls.go", "package p\nfunc a() { _ = T(1) }\n")
	selector := write("sel.go", "package p\nfunc b(x R) { x.M() }\n")
	prose := write("prose.go", "package p\n// T M\nvar s = \"T\"\n")
	broken := write("broken.go", "package p\nfunc c( { T\n")
	missing := filepath.Join(dir, "missing.go")
	names := map[string]struct{}{"T": {}, "M": {}}

	got := siblingsMentioning([]string{calls, selector, prose, broken, missing}, names)
	want := []string{calls, selector, broken, missing}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("kept %v, want %v", got, want)
	}
	all := []string{calls, prose}
	if got := siblingsMentioning(all, nil); len(got) != 2 {
		t.Fatalf("an unknown name set narrowed the candidates: %v", got)
	}
}
