package world

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// The structure-evidence audit's witnesses at the index
// (Docs/audits/structure-coverage-and-refactor-preconditions.md, S1, S2, S4).

// failReads makes structReadFile fail for paths ending in suffix while the
// test runs.
func failReads(t *testing.T, suffix string) *int {
	t.Helper()
	var mu sync.Mutex
	hits := new(int)
	orig := structReadFile
	structReadFile = func(name string) ([]byte, error) {
		if strings.HasSuffix(filepath.ToSlash(name), suffix) {
			mu.Lock()
			*hits++
			mu.Unlock()
			return nil, &fs.PathError{Op: "read", Path: name, Err: errors.New("injected failure")}
		}
		return orig(name)
	}
	t.Cleanup(func() { structReadFile = orig })
	return hits
}

func hasDeficit(stats StructureStats, path string) bool {
	for _, d := range stats.Deficits {
		if d.Path == path {
			return true
		}
	}
	return false
}

// S1: a directory the walk cannot list keeps its entries as last observed,
// and the answer names the deficit instead of answering absence.
func TestStructureEvidence_UnlistableDirectoryIsADeficitNotADeletion(t *testing.T) {
	root, idx := newStructFixture(t)
	ctx := context.Background()
	if _, sites, _, err := idx.Callers(ctx, "lib.Build"); err != nil || len(sites) != 1 {
		t.Fatalf("warm: %v %v", sites, err)
	}
	app := filepath.Join(root, "app")
	if err := os.Chmod(app, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(app, 0o755) })
	_, sites, stats, err := idx.Callers(ctx, "lib.Build")
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].File != "app/app.go" {
		t.Fatalf("the last observed call site stays: %+v", sites)
	}
	if !hasDeficit(stats, "app") {
		t.Fatalf("the unlistable directory is a deficit: %+v", stats.Deficits)
	}
	// A question about another directory does not inherit the deficit.
	if _, scoped, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"Build"}, Path: "lib"}); err != nil || len(scoped.Deficits) != 0 {
		t.Fatalf("a lib-scoped query is complete: %+v %v", scoped.Deficits, err)
	}
	if _, scoped, err := idx.Outline(ctx, "app"); err != nil || !hasDeficit(scoped, "app") {
		t.Fatalf("an app-scoped query carries it: %+v %v", scoped.Deficits, err)
	}
	if err := os.Chmod(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, stats, err := idx.Callers(ctx, "lib.Build"); err != nil || len(stats.Deficits) != 0 {
		t.Fatalf("a restored directory is observed again: %+v %v", stats.Deficits, err)
	}
}

// S1: a file whose hash read fails no longer fails every query; it is a
// deficit, and its last observation stays.
func TestStructureEvidence_UnreadableFileKeepsItsLastObservation(t *testing.T) {
	root, idx := newStructFixture(t)
	ctx := context.Background()
	if _, err := idx.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	appGo := filepath.Join(root, "app/app.go")
	if err := os.Chmod(appGo, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(appGo, 0o644) })
	_, sites, stats, err := idx.Callers(ctx, "lib.Build")
	if err != nil {
		t.Fatalf("one unreadable file does not fail the index: %v", err)
	}
	if len(sites) != 1 || !hasDeficit(stats, "app/app.go") {
		t.Fatalf("kept as last observed, named as a deficit: %+v %+v", sites, stats.Deficits)
	}
}

// S1: the hash pass succeeds, then the parse read of the changed file fails.
// Before, the entry was deleted as if the file were gone.
func TestStructureEvidence_ParseReadFailureIsNotDisappearance(t *testing.T) {
	root, idx := newStructFixture(t)
	ctx := context.Background()
	if _, err := idx.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	writeStructFile(t, root, "lib/lib.go", "package lib\n\n// Build makes an engine.\nfunc Build() int { return 2 }\n")
	failReads(t, "lib/lib.go")
	syms, stats, err := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"Build"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 1 || syms[0].Signature != "func Build() *Engine" {
		t.Fatalf("the last observed declaration stays, labelled by its deficit: %+v", syms)
	}
	if !hasDeficit(stats, "lib/lib.go") {
		t.Fatalf("the failed read is a deficit: %+v", stats.Deficits)
	}
}

// S2: a candidate use file that cannot be read or parsed after selection is
// a deficit of the Uses answer, never a silent omission.
func TestStructureEvidence_UsesReportsUnreadableAndUnparsableCandidates(t *testing.T) {
	root, idx := newStructFixture(t)
	ctx := context.Background()
	if _, err := idx.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	hits := failReads(t, "app/app.go")
	_, uses, stats, err := idx.Uses(ctx, "lib.Build")
	if err != nil {
		t.Fatal(err)
	}
	if *hits == 0 || len(uses) != 0 || !hasDeficit(stats, "app/app.go") {
		t.Fatalf("the unread candidate is a deficit: uses=%+v deficits=%+v", uses, stats.Deficits)
	}
	structReadFile = os.ReadFile
	_, uses, stats, err = idx.Uses(ctx, "lib.Build")
	if err != nil || len(uses) != 1 || len(stats.Deficits) != 0 {
		t.Fatalf("control: %+v %+v %v", uses, stats.Deficits, err)
	}
	u := uses[0]
	data, _ := os.ReadFile(filepath.Join(root, "app/app.go"))
	if u.Digest != contentDigest(data) || u.Text != "lib.Build" {
		t.Fatalf("a use carries the revision and the text its offsets index: %+v", u)
	}

	writeStructFile(t, root, "app/broken.go", "package app\n\nimport \"fixture/lib\"\n\nfunc Broken() { lib.Build( }\n")
	_, _, stats, err = idx.Uses(ctx, "lib.Build")
	if err != nil || !hasDeficit(stats, "app/broken.go") {
		t.Fatalf("a candidate that does not parse is a deficit: %+v %v", stats.Deficits, err)
	}
}

// A candidate whose bytes no longer match the refresh that selected it is
// observed again; a file that keeps changing ends as a deficit.
func TestStructureEvidence_UsesObservesAgainWhenACandidateMoves(t *testing.T) {
	_, idx := newStructFixture(t)
	ctx := context.Background()
	if _, err := idx.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	orig := structReadFile
	reads := 0
	structReadFile = func(name string) ([]byte, error) {
		data, err := orig(name)
		if strings.HasSuffix(filepath.ToSlash(name), "app/app.go") {
			reads++
			data = append(data, []byte("\n// moved\n")...)
		}
		return data, err
	}
	t.Cleanup(func() { structReadFile = orig })
	_, uses, stats, err := idx.Uses(ctx, "lib.Build")
	if err != nil {
		t.Fatal(err)
	}
	if reads != usesAttempts || len(uses) != 0 || !hasDeficit(stats, "app/app.go") {
		t.Fatalf("a candidate that never holds still is a deficit after %d attempts: reads=%d uses=%+v deficits=%+v", usesAttempts, reads, uses, stats.Deficits)
	}
}

// S4: the generation names the source state. It holds still across refreshes
// that change nothing and moves with any change an answer could see.
func TestStructureEvidence_GenerationMovesOnlyWithTheSource(t *testing.T) {
	root, idx := newStructFixture(t)
	ctx := context.Background()
	first, err := idx.Refresh(ctx)
	if err != nil {
		t.Fatal(err)
	}
	again, err := idx.Refresh(ctx)
	if err != nil || again.Generation != first.Generation || first.Generation == 0 {
		t.Fatalf("an unchanged tree keeps its generation: %d then %d (%v)", first.Generation, again.Generation, err)
	}
	writeStructFile(t, root, "app/more.go", "package app\n\nfunc More() {}\n")
	added, err := idx.Refresh(ctx)
	if err != nil || added.Generation <= again.Generation {
		t.Fatalf("an added file moves the generation: %d then %d (%v)", again.Generation, added.Generation, err)
	}
	if err := os.Remove(filepath.Join(root, "app/more.go")); err != nil {
		t.Fatal(err)
	}
	removed, err := idx.Refresh(ctx)
	if err != nil || removed.Generation <= added.Generation {
		t.Fatalf("a real deletion moves the generation: %d then %d (%v)", added.Generation, removed.Generation, err)
	}
	if syms, _, _ := idx.FindSymbol(ctx, SymbolFilter{Names: []string{"More"}}); len(syms) != 0 {
		t.Fatalf("a file that is really gone is removed, not retained: %+v", syms)
	}
}
