package codedom_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/tools/codedom"
	"codenerd/internal/world"
)

// The structure-evidence audit's witnesses (Docs/audits/structure-coverage-
// and-refactor-preconditions.md, S1-S4), run through the registered tools.

var evidenceFixture = map[string]string{
	"go.mod":     "module fixture\n\ngo 1.24\n",
	"lib/lib.go": "package lib\n\n// Build makes one.\nfunc Build() int { return 1 }\n\n// Make makes one too.\nfunc Make() int { return 2 }\n",
	"app/app.go": "package app\n\nimport \"fixture/lib\"\n\nfunc Start() int { return lib.Build() }\n",
}

// S1 + S2: the only user of lib.Build sits in a directory the refresh cannot
// enumerate. delete_element must not read that as "no uses".
func TestEvidence_DeleteRefusesWhenTheUsersDirectoryIsUnreadable(t *testing.T) {
	reg, root := structWorkspace(t, evidenceFixture)
	run(t, reg, "find_symbol", map[string]any{"name": "Build"}) // warm the index
	app := filepath.Join(root, "app")
	if err := os.Chmod(app, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(app, 0o755) })
	before := readText(t, filepath.Join(root, "lib/lib.go"))
	err := runErr(reg, "delete_element", map[string]any{"path": "lib/lib.go", "ref": "lib.Build"})
	if err == nil {
		t.Fatal("delete_element removed a declaration whose only user could not be observed")
	}
	if !strings.Contains(err.Error(), "incomplete") || !strings.Contains(err.Error(), "app") {
		t.Fatalf("the refusal names the unobserved scope: %v", err)
	}
	if readText(t, filepath.Join(root, "lib/lib.go")) != before {
		t.Fatal("a refused delete must not write")
	}
}

// S1: callers_of must not answer a workspace-wide absence when part of the
// workspace could not be observed, and keeps the last-observed call site.
func TestEvidence_CallersOfReportsAnIncompleteObservation(t *testing.T) {
	reg, root := structWorkspace(t, evidenceFixture)
	run(t, reg, "callers_of", map[string]any{"symbol": "lib.Build"})
	app := filepath.Join(root, "app")
	if err := os.Chmod(app, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(app, 0o755) })
	out := run(t, reg, "callers_of", map[string]any{"symbol": "lib.Build"})
	if !strings.Contains(out, "app/app.go:5") {
		t.Fatalf("the last observed call site is kept, not deleted:\n%s", out)
	}
	if !strings.Contains(out, "incomplete") || !strings.Contains(out, "app") {
		t.Fatalf("the answer says what it could not observe:\n%s", out)
	}
	if err := os.Chmod(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if out := run(t, reg, "callers_of", map[string]any{"symbol": "lib.Build"}); strings.Contains(out, "incomplete") {
		t.Fatalf("a restored directory is observed again:\n%s", out)
	}
}

// interceptingProvider runs a hook after the first Uses answer, to change the
// workspace between use discovery and the edit planned from it.
type interceptingProvider struct {
	codedom.StructureProvider
	once  sync.Once
	after func()
}

func (p *interceptingProvider) Uses(ctx context.Context, ref string) ([]codedom.StructureSymbol, []codedom.StructureUse, codedom.Observation, error) {
	syms, uses, obs, err := p.StructureProvider.Uses(ctx, ref)
	p.once.Do(p.after)
	return syms, uses, obs, err
}

func intercept(t *testing.T, root string, after func()) {
	t.Helper()
	codedom.RegisterStructureProvider(&interceptingProvider{StructureProvider: world.NewStructureIndex(root).Provider(), after: after})
}

// S3: the identifier at a discovered offset becomes another same-length
// identifier before the plan reloads the file. Bounds still hold and the
// result still parses; repoint must refuse rather than rewrite a reference it
// never found.
func TestEvidence_RepointRefusesOffsetsFromOtherBytes(t *testing.T) {
	reg, root := structWorkspace(t, evidenceFixture)
	appPath := filepath.Join(root, "app/app.go")
	intercept(t, root, func() {
		src := strings.Replace(readText(t, appPath), "lib.Build()", "lib.Bu1ld()", 1)
		if err := os.WriteFile(appPath, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	err := runErr(reg, "repoint", map[string]any{"from": "lib.Build", "to": "lib.Make", "paths": []any{"app/app.go"}})
	if err == nil {
		t.Fatalf("repoint rewrote offsets measured on other bytes:\n%s", readText(t, appPath))
	}
	if got := readText(t, appPath); !strings.Contains(got, "lib.Bu1ld()") {
		t.Fatalf("the concurrent edit is preserved:\n%s", got)
	}
}

// S3: a file that did not use the name gains a use between discovery and the
// commit. The use set the delete relied on is part of its read set.
func TestEvidence_DeleteRefusesWhenANewUserAppears(t *testing.T) {
	fixture := map[string]string{
		"go.mod":     evidenceFixture["go.mod"],
		"lib/lib.go": evidenceFixture["lib/lib.go"],
		"app/app.go": "package app\n\nimport \"fixture/lib\"\n\nfunc Start() int { return lib.Make() }\n",
	}
	reg, root := structWorkspace(t, fixture)
	intercept(t, root, func() {
		src := "package other\n\nimport \"fixture/lib\"\n\nfunc Use() int { return lib.Build() }\n"
		if err := os.MkdirAll(filepath.Join(root, "other"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "other/other.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	err := runErr(reg, "delete_element", map[string]any{"path": "lib/lib.go", "ref": "lib.Build"})
	if err == nil {
		t.Fatal("delete_element deleted a declaration that gained a user after its uses were checked")
	}
	if !strings.Contains(readText(t, filepath.Join(root, "lib/lib.go")), "func Build") {
		t.Fatal("a refused delete must not write")
	}
}

// S4: rows inserted ahead of the next page between two calls. The second page
// must not be served from a different source state than the first.
func TestEvidence_PagingAcrossAChangeIsRefused(t *testing.T) {
	var calls strings.Builder
	for i := range 160 {
		fmt.Fprintf(&calls, "\t_ = lib.Build() // %d\n", i)
	}
	fixture := map[string]string{
		"go.mod":     evidenceFixture["go.mod"],
		"lib/lib.go": evidenceFixture["lib/lib.go"],
		"zz/zz.go":   "package zz\n\nimport \"fixture/lib\"\n\nfunc Many() {\n" + calls.String() + "}\n",
	}
	reg, root := structWorkspace(t, fixture)
	first := run(t, reg, "callers_of", map[string]any{"symbol": "lib.Build"})
	cursor := continuation(t, first)
	src := "package aa\n\nimport \"fixture/lib\"\n\nfunc Early() int { return lib.Build() }\n"
	if err := os.MkdirAll(filepath.Join(root, "aa"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "aa/aa.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runErr(reg, "callers_of", map[string]any{"symbol": "lib.Build", "cursor": cursor})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("a continuation across a source change must be refused as stale: %v", err)
	}
	restart := run(t, reg, "callers_of", map[string]any{"symbol": "lib.Build"})
	rest := run(t, reg, "callers_of", map[string]any{"symbol": "lib.Build", "cursor": continuation(t, restart)})
	if !strings.Contains(rest, "rows 151-161 of 161") && !strings.Contains(rest, "161 rows") {
		t.Fatalf("a fresh continuation pages the current answer:\n%s", rest)
	}
	if err := runErr(reg, "find_symbol", map[string]any{"name": "Build", "cursor": continuation(t, restart)}); err == nil {
		t.Fatal("a cursor continues only the query that printed it")
	}
}

func continuation(t *testing.T, out string) string {
	t.Helper()
	_, after, ok := strings.Cut(out, `cursor="`)
	if !ok {
		t.Fatalf("no continuation in:\n%s", out)
	}
	cursor, _, _ := strings.Cut(after, `"`)
	return cursor
}

func readText(t *testing.T, abs string) string {
	t.Helper()
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// An incomplete observation never renders as absence, whatever the tool.
func TestEvidence_UnreferencedSymbolsQualifiesAnIncompleteAnswer(t *testing.T) {
	reg, root := structWorkspace(t, evidenceFixture)
	run(t, reg, "unreferenced_symbols", map[string]any{"path": "lib"})
	app := filepath.Join(root, "app")
	if err := os.Chmod(app, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(app, 0o755) })
	out := run(t, reg, "unreferenced_symbols", map[string]any{"path": "lib"})
	if strings.Contains(out, "rows, complete") || !strings.Contains(out, "-- incomplete observation: 1 paths could not be observed: app (") {
		t.Fatalf("unreferenced_symbols must not certify orphans it could not check:\n%s", out)
	}
}

func TestEvidence_CursorMustBeOneThisToolPrinted(t *testing.T) {
	reg, _ := structWorkspace(t, evidenceFixture)
	if err := runErr(reg, "find_symbol", map[string]any{"name": "Build", "cursor": "page2"}); err == nil || !strings.Contains(err.Error(), "not one this tool printed") {
		t.Fatalf("a malformed cursor is refused: %v", err)
	}
}
