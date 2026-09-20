package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

// A real kernel, not a mock: the facts have to survive the string-to-atom
// coercion (ToFacts emits the Go string "/function" for a Decl that wants a
// /name) and the Decl type check in schemas_codedom.mg. A mock kernel that
// stores whatever it is handed would pass while production stored nothing.
func scopeProbe(t *testing.T) (*codedomScope, *core.RealKernel, string) {
	t.Helper()
	ws := t.TempDir()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	return &codedomScope{
		src:   world.NewCodeElementFacts(ws),
		root:  ws,
		files: make(map[string]scopedFile),
	}, kernel, ws
}

func codeElementRows(t *testing.T, kernel *core.RealKernel) []types.Fact {
	t.Helper()
	rows, err := kernel.Query("code_element")
	if err != nil {
		t.Fatalf("query code_element: %v", err)
	}
	return rows
}

func mentions(rows []types.Fact, want string) bool {
	for _, r := range rows {
		for _, a := range r.Args {
			if s, ok := a.(string); ok && strings.Contains(s, want) {
				return true
			}
		}
	}
	return false
}

// Ladder run R1-16 (2026-09-19) asked the kernel for code_element 1,175 times
// and got no row, every run, because nothing in a headless run ever opened a
// CodeDOM scope. The file a turn is looking at now has its elements in the
// kernel, keyed by the workspace-canonical path the working context queries by.
func TestCodedomScope_TheFocusFilesElementsReachTheKernel(t *testing.T) {
	scope, kernel, ws := scopeProbe(t)
	src := filepath.Join(ws, "widget.go")
	if err := os.WriteFile(src, []byte("package widget\n\nfunc Encode() error { return nil }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if rows := codeElementRows(t, kernel); len(rows) != 0 {
		t.Fatalf("code_element populated before anything was scoped: %v", rows)
	}
	scope.ensure(kernel, "widget.go")

	rows := codeElementRows(t, kernel)
	if len(rows) == 0 {
		t.Fatal("scoping the focus file left code_element empty; the working context asks for this predicate on every entity it selects")
	}
	if !mentions(rows, "Encode") {
		t.Fatalf("no element names the function the file declares: %v", rows)
	}
	// The path argument must be the canonical identity, not the absolute path
	// the parser read: code_defines and dependency_link are keyed that way, and
	// a query built from a workspace-relative entity misses anything else.
	if !mentions(rows, "widget.go") || mentions(rows, ws) {
		t.Fatalf("code_element is not keyed by the workspace-canonical path: %v", rows)
	}
}

// An edit replaces the file's elements. Mangle evaluation is monotone, so a
// stale code_element left beside a fresh one stays derivable for the rest of
// the run and every rule over it sees one file at two revisions at once --
// the same failure shape as the working_revision accumulation fixed on
// 2026-09-11.
func TestCodedomScope_AnEditReplacesTheFilesElements(t *testing.T) {
	scope, kernel, ws := scopeProbe(t)
	src := filepath.Join(ws, "widget.go")
	if err := os.WriteFile(src, []byte("package widget\n\nfunc Encode() error { return nil }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scope.ensure(kernel, "widget.go")
	if !mentions(codeElementRows(t, kernel), "Encode") {
		t.Fatal("the first scope did not land")
	}

	if err := os.WriteFile(src, []byte("package widget\n\nfunc Decode() error { return nil }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scope.ensure(kernel, "widget.go")

	rows := codeElementRows(t, kernel)
	if !mentions(rows, "Decode") {
		t.Fatalf("the edited file's element never reached the kernel: %v", rows)
	}
	if mentions(rows, "Encode") {
		t.Fatalf("the element the edit removed is still asserted: %v", rows)
	}
}

// Re-parsing a file on every tool call would put a parse in the hot loop of
// the turn. The content digest is the guard: an unchanged file is scoped once.
func TestCodedomScope_AnUnchangedFileIsParsedOnce(t *testing.T) {
	scope, kernel, ws := scopeProbe(t)
	src := filepath.Join(ws, "widget.go")
	if err := os.WriteFile(src, []byte("package widget\n\nfunc Encode() error { return nil }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	counted := &countingElements{inner: world.NewCodeElementFacts(ws)}
	scope.src = counted

	for i := 0; i < 5; i++ {
		scope.ensure(kernel, "widget.go")
	}
	if counted.calls != 1 {
		t.Fatalf("parsed %d times for one unchanged file, want 1", counted.calls)
	}
}

type countingElements struct {
	inner CodeElementSource
	calls int
}

func (c *countingElements) FileFacts(path string) ([]types.Fact, error) {
	c.calls++
	return c.inner.FileFacts(path)
}

// Nothing about the scope may stop a turn: a directory, a missing file, a
// binary, and the workspace-root focus all have to be quiet no-ops.
func TestCodedomScope_UnparseableFocusIsAQuietNoOp(t *testing.T) {
	scope, kernel, ws := scopeProbe(t)
	if err := os.WriteFile(filepath.Join(ws, "notes.txt"), []byte("not source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(ws, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, entity := range []string{".", "", "notes.txt", "pkg", "gone.go"} {
		scope.ensure(kernel, entity)
	}
	if rows := codeElementRows(t, kernel); len(rows) != 0 {
		t.Fatalf("a focus that is not a parseable source file asserted %d fact(s): %v", len(rows), rows)
	}
}
