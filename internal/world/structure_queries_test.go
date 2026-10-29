package world

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// Two `package main` directories and two build-tag twins: the shapes that made
// a package-name ref a search key rather than an address (the audit counted
// `package main` in 20 directories of this repository).
func newRefFixture(t *testing.T) (string, *StructureIndex) {
	t.Helper()
	root := t.TempDir()
	writeStructFile(t, root, "go.mod", "module fixture\n\ngo 1.24\n")
	writeStructFile(t, root, "cmd/a/main.go", "package main\n\nfunc main() { run() }\n\nfunc run() {}\n")
	writeStructFile(t, root, "cmd/b/main.go", "package main\n\nfunc main() { run() }\n\nfunc run() {}\n")
	writeStructFile(t, root, "store/on.go", "//go:build vec\n\npackage store\n\nconst requireVec = true\n")
	writeStructFile(t, root, "store/off.go", "//go:build !vec\n\npackage store\n\nconst requireVec = false\n")
	writeStructFile(t, root, "features/features.go", `package features

import "log"

// Enabled reports the flag.
func Enabled() bool {
	log.Printf("Vector recall query failed: %v", nil)
	return true
}
`)
	writeStructFile(t, root, "app/app.go", `package app

import (
	"fixture/features"
	feat "fixture/features"
)

// Start starts. It checks features.Enabled first.
func Start() bool { return features.Enabled() && feat.Enabled() }
`)
	writeStructFile(t, root, "policy/p.mg", `# The input.
Decl edge(X, Y) bound [/string, /string].

edge("a", "b").
path(X, Y) :- edge(X, Y).
path(X, Z) :- path(X, Y), edge(Y, Z).
`)
	return root, NewStructureIndex(root)
}

func TestStructureIndex_RefsAreKeyedByDirectoryNotPackageName(t *testing.T) {
	_, idx := newRefFixture(t)
	ctx := context.Background()
	got, _, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"run"}})
	if err != nil || len(got) != 2 {
		t.Fatalf("find run: %+v %v", got, err)
	}
	if got[0].Ref != "cmd/a.run" || got[1].Ref != "cmd/b.run" {
		t.Fatalf("refs must name the directory: %s, %s", got[0].Ref, got[1].Ref)
	}
	one, _, err := idx.Resolve(ctx, "cmd/b.run")
	if err != nil || len(one) != 1 || one[0].File != "cmd/b/main.go" {
		t.Fatalf("a printed ref resolves to exactly its element: %+v %v", one, err)
	}
	twins, _, _ := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"requireVec"}})
	if len(twins) != 2 || !strings.HasSuffix(twins[0].Ref, "@off.go") || !strings.HasSuffix(twins[1].Ref, "@on.go") {
		t.Fatalf("build-tag twins carry a file discriminator: %+v", twins)
	}
	if exact, _, _ := idx.Resolve(ctx, twins[1].Ref); len(exact) != 1 || exact[0].File != "store/on.go" {
		t.Fatalf("a discriminated ref resolves to one twin: %+v", exact)
	}
	if both, _, _ := idx.Resolve(ctx, "store.requireVec"); len(both) != 2 {
		t.Fatalf("the undiscriminated ref names both twins, so an edit can refuse the ambiguity: %+v", both)
	}
}

func TestStructureIndex_FindSymbolByPatternKindAndPath(t *testing.T) {
	_, idx := newRefFixture(t)
	ctx := context.Background()
	got, _, err := idx.FindSymbol(ctx, SymbolFilter{Pattern: "^(main|run)$", Path: "cmd/a"})
	if err != nil || len(got) != 2 {
		t.Fatalf("pattern within a path: %+v %v", got, err)
	}
	multi, _, _ := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"Enabled", "Start"}})
	if len(multi) != 2 {
		t.Fatalf("several names in one call: %+v", multi)
	}
	decls, _, _ := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"edge"}, Kind: "decl"})
	if len(decls) != 1 || decls[0].Ref != "policy/p.mg:decl:edge/2" {
		t.Fatalf("Mangle Decls are symbols too: %+v", decls)
	}
}

func TestStructureIndex_ImportersOfAPackage(t *testing.T) {
	_, idx := newRefFixture(t)
	for _, arg := range []string{"features", "fixture/features"} {
		path, rows, _, err := idx.Importers(context.Background(), arg)
		if err != nil || path != "fixture/features" || len(rows) != 2 {
			t.Fatalf("importers of %q = %q %+v %v", arg, path, rows, err)
		}
		if rows[0].File != "app/app.go" || rows[1].Name != "feat" {
			t.Fatalf("rows carry the file and the local name: %+v", rows)
		}
	}
}

func TestStructureIndex_FindTextAnswersWithTheEnclosingElement(t *testing.T) {
	_, idx := newRefFixture(t)
	ctx := context.Background()
	hits, _, err := idx.FindText(ctx, "Vector recall query failed", "strings", "")
	if err != nil || len(hits) != 1 {
		t.Fatalf("find the log message: %+v %v", hits, err)
	}
	if hits[0].Ref != "features.Enabled" || hits[0].File != "features/features.go" || hits[0].Line != 7 {
		t.Fatalf("the hit names its element: %+v", hits[0])
	}
	comments, _, _ := idx.FindText(ctx, "features.Enabled first", "comments", "")
	if len(comments) != 1 || comments[0].Ref != "app.Start" {
		t.Fatalf("comment hits too: %+v", comments)
	}
	mangle, _, _ := idx.FindText(ctx, "The input", "comments", "policy")
	if len(mangle) != 1 || mangle[0].Ref != "policy/p.mg:decl:edge/2" {
		t.Fatalf("Mangle comments are searched and attributed: %+v", mangle)
	}
}

func TestStructureIndex_UsesOfAPackageLevelName(t *testing.T) {
	_, idx := newRefFixture(t)
	target, uses, _, err := idx.Uses(context.Background(), "features.Enabled")
	if err != nil {
		t.Fatal(err)
	}
	if target.File != "features/features.go" || len(uses) != 2 {
		t.Fatalf("both qualified uses in app.go: %+v", uses)
	}
	if uses[0].Qualifier != "features" || uses[1].Qualifier != "feat" || uses[0].Ref != "app.Start" {
		t.Fatalf("uses carry their qualifier and element: %+v", uses)
	}
	_, inPkg, _, err := idx.Uses(context.Background(), "cmd/a.run")
	if err != nil || len(inPkg) != 1 || inPkg[0].Ref != "cmd/a.main" {
		t.Fatalf("a same-package bare use: %+v %v", inPkg, err)
	}
	_, mg, _, err := idx.Uses(context.Background(), "policy/p.mg:decl:edge/2")
	if err != nil || len(mg) != 3 {
		t.Fatalf("a Decl is used by the fact and both rules: %+v %v", mg, err)
	}
}

func TestStructureIndex_BrokenFileKeepsItsLastGoodElements(t *testing.T) {
	root, idx := newRefFixture(t)
	ctx := context.Background()
	if _, err := idx.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	path := writeStructFile(t, root, "features/features.go", "package features\n\nimport \"log\"\n\nfunc Enabled() bool {\n\tlog.Printf(\"x\"\n\treturn true\n}\n\nfunc Other() {}\n")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	st, err := idx.FileStatus(ctx, "features/features.go")
	if err != nil || !st.Known || st.Parsed || len(st.Errors) == 0 {
		t.Fatalf("status of a broken file: %+v %v", st, err)
	}
	if len(st.LastGood) != 1 || st.LastGood[0].Key != "Enabled" {
		t.Fatalf("the last clean parse is kept: %+v", st.LastGood)
	}
	outline, _, _ := idx.Outline(ctx, "features/features.go")
	var kinds []string
	for _, s := range outline {
		kinds = append(kinds, s.Kind+":"+s.Key)
	}
	if strings.Join(kinds, ",") != "syntax_error:syntax_error,function:Other" {
		t.Fatalf("a broken file stays addressable: %v", kinds)
	}
}

func TestStructureIndex_PredicateOutline(t *testing.T) {
	_, idx := newRefFixture(t)
	rows, _, err := idx.PredicateOutline(context.Background(), "edge")
	if err != nil {
		t.Fatal(err)
	}
	var roles []string
	for _, r := range rows {
		roles = append(roles, r.Role)
	}
	if strings.Join(roles, ",") != "declares,derives,reads,reads" {
		t.Fatalf("roles %v", roles)
	}
}

func TestStructureIndex_ImportResolverLearnsFromTheWorkspace(t *testing.T) {
	_, idx := newRefFixture(t)
	res, err := idx.ImportResolver(context.Background(), "cmd/a/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := res.ResolveQualifier("log"); p != "log" {
		t.Fatalf("a standard library name imported elsewhere resolves: %q", p)
	}
	if p, _ := res.ResolveQualifier("features"); p != "fixture/features" {
		t.Fatalf("an in-module package resolves: %q", p)
	}
	if p, c := res.ResolveQualifier("nosuch"); p != "" || len(c) != 0 {
		t.Fatalf("an unknown qualifier resolves to nothing: %q %v", p, c)
	}
	if res.ModulePath() != "fixture" {
		t.Fatalf("module path %q", res.ModulePath())
	}
}
