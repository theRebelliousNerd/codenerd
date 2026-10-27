package world

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeStructFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return path
}

func newStructFixture(t *testing.T) (string, *StructureIndex) {
	t.Helper()
	root := t.TempDir()
	writeStructFile(t, root, "go.mod", "module fixture\n\ngo 1.24\n")
	writeStructFile(t, root, "lib/lib.go", `package lib

// Engine runs things.
type Engine struct{}

// Run runs the engine.
func (e *Engine) Run() error { return helper() }

func helper() error { return nil }

// Orphan is declared and never named again.
func Orphan() {}

// Build makes an engine.
func Build() *Engine { return &Engine{} }
`)
	writeStructFile(t, root, "app/app.go", `package app

import "fixture/lib"

func Start() error {
	e := lib.Build()
	return e.Run()
}
`)
	return root, NewStructureIndex(root)
}

func TestStructureIndexFindSymbolForms(t *testing.T) {
	_, idx := newStructFixture(t)
	ctx := context.Background()
	for _, query := range []string{"Run", "Engine.Run", "lib.Engine.Run"} {
		got, _, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{query}})
		if err != nil {
			t.Fatalf("FindSymbol(%q): %v", query, err)
		}
		// The span starts at the doc comment: replacing an element replaces
		// its doc too.
		if len(got) != 1 || got[0].ID != "lib.Engine.Run" || got[0].Ref != "lib.Engine.Run" || got[0].File != "lib/lib.go" || got[0].StartLine != 6 {
			t.Fatalf("FindSymbol(%q) = %+v, want lib.Engine.Run at lib/lib.go:6", query, got)
		}
		if got[0].Kind != "method" || got[0].Signature != "func (*Engine) Run() error" {
			t.Fatalf("FindSymbol(%q) kind/signature = %q / %q", query, got[0].Kind, got[0].Signature)
		}
	}
	structs, _, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"Engine"}, Kind: "struct"})
	if err != nil || len(structs) != 1 || structs[0].Doc != "Engine runs things." {
		t.Fatalf("FindSymbol(Engine, struct) = %+v, err=%v", structs, err)
	}
}

func TestStructureIndexOutlineIsPerDirectory(t *testing.T) {
	_, idx := newStructFixture(t)
	got, _, err := idx.Outline(context.Background(), "lib")
	if err != nil {
		t.Fatalf("Outline: %v", err)
	}
	var ids []string
	for _, sym := range got {
		ids = append(ids, sym.ID)
	}
	want := []string{"lib.Engine", "lib.Engine.Run", "lib.helper", "lib.Orphan", "lib.Build"}
	if len(ids) != len(want) {
		t.Fatalf("Outline(lib) = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("Outline(lib) = %v, want %v in line order", ids, want)
		}
	}
}

func TestStructureIndexCallersAcrossPackages(t *testing.T) {
	_, idx := newStructFixture(t)
	ctx := context.Background()

	_, callers, _, err := idx.Callers(ctx, "lib.Build")
	if err != nil {
		t.Fatalf("Callers: %v", err)
	}
	if len(callers) != 1 || callers[0].Caller != "app.Start" || callers[0].File != "app/app.go" || callers[0].Line != 6 || callers[0].Match != "exact" {
		t.Fatalf("Callers(lib.Build) = %+v, want one exact site at app/app.go:6 in app.Start", callers)
	}

	// A method called through a variable cannot be tied to its receiver type
	// without type checking; the index says so instead of guessing.
	_, viaVar, _, err := idx.Callers(ctx, "Engine.Run")
	if err != nil {
		t.Fatalf("Callers: %v", err)
	}
	if len(viaVar) != 1 || viaVar[0].Line != 7 || viaVar[0].Match != "by-name" {
		t.Fatalf("Callers(Engine.Run) = %+v, want one by-name site at line 7", viaVar)
	}

	_, local, _, err := idx.Callers(ctx, "helper")
	if err != nil {
		t.Fatalf("Callers: %v", err)
	}
	if len(local) != 1 || local[0].Caller != "lib.Engine.Run" || local[0].Match != "exact" {
		t.Fatalf("Callers(helper) = %+v, want the same-package bare call from lib.Engine.Run", local)
	}
}

func TestStructureIndexCalleesResolveCandidates(t *testing.T) {
	_, idx := newStructFixture(t)
	_, callees, _, err := idx.Callees(context.Background(), "app.Start")
	if err != nil {
		t.Fatalf("Callees: %v", err)
	}
	if len(callees) != 2 {
		t.Fatalf("Callees(app.Start) = %+v, want two calls", callees)
	}
	if callees[0].Call != "lib.Build" || len(callees[0].Candidates) != 1 || callees[0].Candidates[0] != "lib.Build @ lib/lib.go:14" {
		t.Fatalf("first callee = %+v, want lib.Build resolved to lib/lib.go:14", callees[0])
	}
}

func TestStructureIndexUnreferencedIsConservative(t *testing.T) {
	_, idx := newStructFixture(t)
	got, _, err := idx.Unreferenced(context.Background(), "lib")
	if err != nil {
		t.Fatalf("Unreferenced: %v", err)
	}
	if len(got) != 1 || got[0].ID != "lib.Orphan" {
		t.Fatalf("Unreferenced(lib) = %+v, want exactly lib.Orphan", got)
	}
}

// A call inside a package-level var initializer is a call. Every cobra
// command's RunE closure lives in one; before they were walked, callers_of
// answered "tests only" for a function the CLI calls in production and
// unreferenced_symbols called it dead (2026-09-21, features.ConfigSchemaJSON
// called from cmd/nerd/cmd_features.go:37 reported with 3 test callers).
func TestStructureIndexSeesCallsInPackageLevelInitializers(t *testing.T) {
	root := t.TempDir()
	writeStructFile(t, root, "go.mod", "module fixture\n\ngo 1.24\n")
	writeStructFile(t, root, "lib/lib.go", `package lib

// Schema is called only from a command's closure.
func Schema() string { return "{}" }

// Table is called only through a function table.
func Table() int { return 1 }

// Seed is called only from an initializer expression.
func Seed() int { return 2 }
`)
	writeStructFile(t, root, "cmd/cmd.go", `package cmd

import "fixture/lib"

type Command struct{ RunE func() error }

var featuresCmd = &Command{
	RunE: func() error {
		_ = lib.Schema()
		return nil
	},
}

var handlers = map[string]func() int{"t": lib.Table}

var seeded = lib.Seed()
`)
	idx := NewStructureIndex(root)
	ctx := context.Background()

	_, callers, _, err := idx.Callers(ctx, "lib.Schema")
	if err != nil {
		t.Fatalf("Callers: %v", err)
	}
	if len(callers) != 1 || callers[0].Caller != "cmd.featuresCmd" || callers[0].File != "cmd/cmd.go" || callers[0].Match != "exact" {
		t.Fatalf("Callers(lib.Schema) = %+v, want the call in featuresCmd's RunE closure", callers)
	}
	if _, seed, _, err := idx.Callers(ctx, "lib.Seed"); err != nil || len(seed) != 1 || seed[0].Caller != "cmd.seeded" {
		t.Fatalf("Callers(lib.Seed) = %+v, err=%v, want the initializer of cmd.seeded", seed, err)
	}

	unref, _, err := idx.Unreferenced(ctx, "lib")
	if err != nil {
		t.Fatalf("Unreferenced: %v", err)
	}
	for _, sym := range unref {
		if sym.ID == "lib.Schema" || sym.ID == "lib.Seed" {
			t.Errorf("%s is called from a package-level initializer and was reported unreferenced", sym.ID)
		}
	}
}

// TestStructureIndexFollowsEdits pins freshness: an answer after an edit
// describes the file as it is, and only the edited file is parsed again.
func TestStructureIndexFollowsEdits(t *testing.T) {
	root, idx := newStructFixture(t)
	ctx := context.Background()
	if _, _, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"Orphan"}}); err != nil {
		t.Fatalf("warm: %v", err)
	}

	path := writeStructFile(t, root, "lib/extra.go", "package lib\n\nfunc Later() { Orphan() }\n")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	found, stats, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"Later"}})
	if err != nil {
		t.Fatalf("FindSymbol: %v", err)
	}
	if len(found) != 1 || found[0].File != "lib/extra.go" {
		t.Fatalf("FindSymbol(Later) = %+v, want the new file", found)
	}
	if stats.Reparsed != 1 {
		t.Fatalf("reparsed %d files after adding one, want 1", stats.Reparsed)
	}
	unref, _, err := idx.Unreferenced(ctx, "lib")
	if err != nil {
		t.Fatalf("Unreferenced: %v", err)
	}
	for _, sym := range unref {
		if sym.ID == "lib.Orphan" {
			t.Fatalf("lib.Orphan is still reported unreferenced after lib.Later started calling it")
		}
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	gone, _, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"Later"}})
	if err != nil || len(gone) != 0 {
		t.Fatalf("FindSymbol(Later) after delete = %+v, err=%v; want nothing", gone, err)
	}
}

func TestStructureIndexFollowsSameSizeRewriteWithRestoredMtime(t *testing.T) {
	root := t.TempDir()
	oldSource := "package lib\n\nfunc OldName() {}\nfunc Caller() { OldName() }\n"
	newSource := "package lib\n\nfunc NewName() {}\nfunc Caller() { NewName() }\n"
	path := writeStructFile(t, root, "lib/lib.go", oldSource)
	stamp := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatalf("set initial mtime: %v", err)
	}

	idx := NewStructureIndex(root)
	ctx := context.Background()
	old, _, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"OldName"}})
	if err != nil || len(old) != 1 {
		t.Fatalf("warm old declaration: %+v, err=%v", old, err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	// The bytes change without moving either metadata field the index currently
	// uses as its freshness key. This models editors, checkouts and generated
	// files that preserve timestamps while replacing content.
	if err := os.WriteFile(path, []byte(newSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
		t.Fatalf("restore mtime: %v", err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("fixture metadata moved: size %d -> %d, mtime %s -> %s", before.Size(), after.Size(), before.ModTime(), after.ModTime())
	}

	got, stats, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"NewName"}})
	if err != nil {
		t.Fatalf("FindSymbol(NewName): %v", err)
	}
	if len(got) != 1 || stats.Reparsed != 1 {
		t.Fatalf("same-size rewrite was not refreshed: symbols=%+v, reparsed=%d; want NewName and one reparsed file", got, stats.Reparsed)
	}
	stale, _, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"OldName"}})
	if err != nil || len(stale) != 0 {
		t.Fatalf("old declaration survived source rewrite: %+v, err=%v", stale, err)
	}
	_, callers, _, err := idx.Callers(ctx, "lib.NewName")
	if err != nil || len(callers) != 1 || callers[0].Caller != "lib.Caller" {
		t.Fatalf("new call graph was not refreshed: callers=%+v, err=%v", callers, err)
	}
	_, staleCallers, _, err := idx.Callers(ctx, "lib.OldName")
	if err != nil || len(staleCallers) != 0 {
		t.Fatalf("old call graph survived source rewrite: callers=%+v, err=%v", staleCallers, err)
	}
	warm, err := idx.Refresh(ctx)
	if err != nil || warm.Reparsed != 0 {
		t.Fatalf("unchanged content missed parse cache: reparsed=%d, err=%v; want zero", warm.Reparsed, err)
	}
}

func TestStructureIndexFollowsSymlinkTargetAndKeepsWorkspaceBoundary(t *testing.T) {
	root := t.TempDir()
	targetPath := writeStructFile(t, root, "target.txt", "package lib\n\nfunc OldName() {}\n")
	aliasPath := filepath.Join(root, "alias.go")
	if err := os.Symlink(targetPath, aliasPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	secretPath := writeStructFile(t, root, ".env", "package lib\n\nfunc SecretName() {}\n")
	if err := os.Symlink(secretPath, filepath.Join(root, "secret.go")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	outsideRoot := t.TempDir()
	outsidePath := writeStructFile(t, outsideRoot, "outside.txt", "package lib\n\nfunc OutsideName() {}\n")
	if err := os.Symlink(outsidePath, filepath.Join(root, "outside.go")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	idx := NewStructureIndex(root)
	ctx := context.Background()
	old, _, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"OldName"}})
	if err != nil || len(old) != 1 || old[0].File != "alias.go" {
		t.Fatalf("initial symlink symbol: %+v, err=%v; want alias.go", old, err)
	}
	for _, name := range []string{"SecretName", "OutsideName"} {
		found, _, queryErr := idx.FindSymbol(ctx, SymbolFilter{Names: []string{name}})
		if queryErr != nil || len(found) != 0 {
			t.Fatalf("FindSymbol(%s) = %+v, err=%v; symlink target must stay outside the index", name, found, queryErr)
		}
	}

	linkBefore, err := os.Lstat(aliasPath)
	if err != nil {
		t.Fatal(err)
	}
	// Let the link's own timestamp age past the trust quantum. The target
	// rewrite below then leaves every symlink metadata field untouched, which
	// reproduced the stale-cache path when the index stamped DirEntry.Info().
	time.Sleep(symbolStampQuantum + 10*time.Millisecond)
	targetBefore, err := os.Stat(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("package lib\n\nfunc NewName() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(targetPath, targetBefore.ModTime(), targetBefore.ModTime()); err != nil {
		t.Fatalf("restore target mtime: %v", err)
	}
	linkAfter, err := os.Lstat(aliasPath)
	if err != nil {
		t.Fatal(err)
	}
	if linkBefore.Size() != linkAfter.Size() || !linkBefore.ModTime().Equal(linkAfter.ModTime()) {
		t.Fatalf("symlink metadata moved: size %d -> %d, mtime %s -> %s", linkBefore.Size(), linkAfter.Size(), linkBefore.ModTime(), linkAfter.ModTime())
	}

	updated, stats, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"NewName"}})
	if err != nil || len(updated) != 1 || updated[0].File != "alias.go" || stats.Reparsed != 1 {
		t.Fatalf("symlink target rewrite was not refreshed: symbols=%+v reparsed=%d err=%v; want NewName at alias.go and one parse", updated, stats.Reparsed, err)
	}
	stale, _, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"OldName"}})
	if err != nil || len(stale) != 0 {
		t.Fatalf("old symlink declaration survived target rewrite: %+v, err=%v", stale, err)
	}
}

func TestStructureIndexRefreshesModulePathAfterSameSizeRewriteWithRestoredMtime(t *testing.T) {
	root := t.TempDir()
	modulePath := writeStructFile(t, root, "go.mod", "module before\n\ngo 1.24\n")
	clientPath := writeStructFile(t, root, "app/client.go", "package app\n\nimport lib \"before/lib\"\n\nfunc Run() { lib.Hello() }\n")
	writeStructFile(t, root, "lib/lib.go", "package lib\n\nfunc Hello() {}\n")
	stamp := time.Unix(1_700_000_000, 0)
	for _, path := range []string{modulePath, clientPath} {
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatalf("set initial mtime for %s: %v", path, err)
		}
	}

	idx := NewStructureIndex(root)
	ctx := context.Background()
	path, importers, _, err := idx.Importers(ctx, "lib")
	if err != nil || path != "before/lib" || len(importers) != 1 {
		t.Fatalf("initial import resolution: path=%q importers=%+v err=%v", path, importers, err)
	}
	moduleBefore, err := os.Stat(modulePath)
	if err != nil {
		t.Fatal(err)
	}
	clientBefore, err := os.Stat(clientPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modulePath, []byte("module afterx\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(modulePath, moduleBefore.ModTime(), moduleBefore.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clientPath, []byte("package app\n\nimport lib \"afterx/lib\"\n\nfunc Run() { lib.Hello() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(clientPath, clientBefore.ModTime(), clientBefore.ModTime()); err != nil {
		t.Fatal(err)
	}
	moduleAfter, err := os.Stat(modulePath)
	if err != nil {
		t.Fatal(err)
	}
	clientAfter, err := os.Stat(clientPath)
	if err != nil {
		t.Fatal(err)
	}
	if moduleBefore.Size() != moduleAfter.Size() || !moduleBefore.ModTime().Equal(moduleAfter.ModTime()) {
		t.Fatalf("go.mod fixture metadata moved: size %d -> %d, mtime %s -> %s", moduleBefore.Size(), moduleAfter.Size(), moduleBefore.ModTime(), moduleAfter.ModTime())
	}
	if clientBefore.Size() != clientAfter.Size() || !clientBefore.ModTime().Equal(clientAfter.ModTime()) {
		t.Fatalf("client fixture metadata moved: size %d -> %d, mtime %s -> %s", clientBefore.Size(), clientAfter.Size(), clientBefore.ModTime(), clientAfter.ModTime())
	}

	path, importers, stats, err := idx.Importers(ctx, "lib")
	if err != nil || path != "afterx/lib" || len(importers) != 1 || importers[0].File != "app/client.go" || stats.Reparsed != 1 {
		t.Fatalf("updated import resolution: path=%q importers=%+v reparsed=%d err=%v", path, importers, stats.Reparsed, err)
	}
	warm, err := idx.Refresh(ctx)
	if err != nil || warm.Reparsed != 0 {
		t.Fatalf("unchanged module/source missed parse cache: reparsed=%d, err=%v; want zero", warm.Reparsed, err)
	}
}

// TestStructureIndexRepositoryScale measures the real tree: the cold build and
// the warm refresh every query pays. It reports; the only assertions are the
// ones that would mean the index is unusable.
func TestStructureIndexRepositoryScale(t *testing.T) {
	if testing.Short() {
		t.Skip("repository-scale measurement")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	idx := NewStructureIndex(root)
	ctx := context.Background()
	cold, err := idx.Refresh(ctx)
	if err != nil {
		t.Fatalf("cold refresh: %v", err)
	}
	warm, err := idx.Refresh(ctx)
	if err != nil {
		t.Fatalf("warm refresh: %v", err)
	}
	t.Logf("cold: files=%d symbols=%d calls=%d in %v; warm: reparsed=%d in %v",
		cold.Files, cold.Symbols, cold.Calls, cold.RefreshTook, warm.Reparsed, warm.RefreshTook)
	if warm.Reparsed != 0 {
		t.Fatalf("warm refresh reparsed %d files with nothing changed", warm.Reparsed)
	}
	found, _, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"StructureIndex.FindSymbol"}})
	if err != nil || len(found) != 1 || found[0].File != "internal/world/structure_index.go" {
		t.Fatalf("the index cannot find its own method: %+v err=%v", found, err)
	}
}
