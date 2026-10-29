package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/observation/precondition"
	"codenerd/internal/tools"
)

// readFixtureSeq keeps each fixture's content unique across runs.
//
// Preconditions are content-addressed, so a fixture with identical bytes yields
// the identical handle on a second pass of the same test in the same process.
// That would let `go test -count=2` pass on a handle minted by the first pass
// rather than by the code under test, which is the run-once defect these tests
// exist to be free of.
var readFixtureSeq atomic.Int64

// seedReadWorkspace writes a small Go file and returns the workspace root and
// the file's workspace-relative name.
func seedReadWorkspace(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	name := fmt.Sprintf("widget_%d.go", readFixtureSeq.Add(1))
	source := `package widget

import "fmt"

// Encode adds one.
func Encode(v int) int {
	fmt.Println(v)
	return v + 1
}

func caller() int {
	return Encode(1)
}
`
	if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return root, name
}

// preconditionFrom pulls the handle out of a rendered read.
func preconditionFrom(t *testing.T, out string) string {
	t.Helper()
	idx := strings.Index(out, precondition.HandlePrefix)
	if idx < 0 {
		t.Fatalf("read published no precondition, so no edit built on it can be checked:\n%s", out)
	}
	rest := out[idx:]
	end := strings.IndexAny(rest, " \n")
	if end < 0 {
		end = len(rest)
	}
	return rest[:end]
}

func TestReadFile_ShouldPublishAPreconditionAndSayWhatToDoWithIt(t *testing.T) {
	t.Parallel()
	root, file := seedReadWorkspace(t)

	out, err := executeReadFile(wsCtx(root), map[string]any{"path": file})
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}

	handle := preconditionFrom(t, out)
	if !strings.Contains(out, precondition.Arg+"="+handle) {
		t.Errorf("the read names a handle but not the argument that spends it; the model cannot act on it:\n%s", out)
	}
	// The file is short, so nothing may be elided: a codec that dropped source
	// from a thirteen-line read would be taking away more than it gives.
	if !strings.Contains(out, "8\t\treturn v + 1") {
		t.Errorf("a short file must still come back whole and correctly numbered:\n%s", out)
	}
}

// TestEditFile_WhenThePreconditionIsStale_ShouldRefuseTheEdit is the property
// the whole mechanism exists for, driven through the live verbs rather than
// through the codec in isolation. An edit built on a read that has gone stale
// is worse than no read: the model rewrites, with confidence, a file it no
// longer understands.
func TestEditFile_WhenThePreconditionIsStale_ShouldRefuseTheEdit(t *testing.T) {
	t.Parallel()
	root, file := seedReadWorkspace(t)
	ctx := wsCtx(root)
	path := filepath.Join(root, file)

	out, err := executeReadFile(ctx, map[string]any{"path": file, "start_line": 7, "end_line": 8})
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	handle := preconditionFrom(t, out)

	// Somebody else rewrites the very lines that were read.
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	changed := strings.Replace(string(current), "return v + 1", "return v * 2", 1)
	if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	_, err = executeEditFile(ctx, map[string]any{
		"path":           file,
		"old_text":       "fmt.Println(v)",
		"new_text":       "fmt.Println(v * 2)",
		precondition.Arg: handle,
	})
	if err == nil {
		t.Fatal("edit_file applied an edit whose precondition had gone stale; the model would believe it edited what it read")
	}
	if !strings.Contains(err.Error(), "FAILED") {
		t.Errorf("the refusal must say the precondition failed and how to recover:\n%v", err)
	}

	// And the file must be untouched: a refusal that half-applies is worse than
	// no refusal at all.
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back after refusal: %v", err)
	}
	if string(after) != changed {
		t.Errorf("the refused edit still modified the file:\n%s", after)
	}
}

func TestEditFile_WhenAnUnrelatedPartChanged_ShouldProceedAndSaySo(t *testing.T) {
	t.Parallel()
	root, file := seedReadWorkspace(t)
	ctx := wsCtx(root)
	path := filepath.Join(root, file)

	out, err := executeReadFile(ctx, map[string]any{"path": file, "start_line": 6, "end_line": 9})
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	handle := preconditionFrom(t, out)

	// A change below the region, the same number of lines, leaving the region
	// byte-identical at the coordinates it was read at.
	current, _ := os.ReadFile(path)
	changed := strings.Replace(string(current), "return Encode(1)", "return Encode(9)", 1)
	if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	result, err := executeEditFile(ctx, map[string]any{
		"path":           file,
		"old_text":       "return v + 1",
		"new_text":       "return v + 2",
		precondition.Arg: handle,
	})
	if err != nil {
		t.Fatalf("an edit whose region is intact must proceed; refusing every second edit to one file makes the precondition unusable: %v", err)
	}
	if !strings.Contains(result, "rest of the file changed") {
		t.Errorf("an edit made against a file that moved elsewhere must carry that caveat:\n%s", result)
	}
}

func TestEditFile_WhenThePreconditionIsUnknown_ShouldRefuseRatherThanIgnoreIt(t *testing.T) {
	t.Parallel()
	root, file := seedReadWorkspace(t)

	_, err := executeEditFile(wsCtx(root), map[string]any{
		"path":           file,
		"old_text":       "return v + 1",
		"new_text":       "return v + 2",
		precondition.Arg: "obs:fr:000000000000",
	})
	if err == nil {
		t.Fatal("an unresolvable precondition was ignored; the caller asked for the edit to be checked and it was not")
	}
}

// TestRegistry_ReadAndEditVerbsAreWiredToTheCodec guards the defect this repo
// keeps producing: a capability that exists, is tested in isolation, and is
// never called. It resolves both verbs the way the agent does — through the
// registry — and drives a read and the edit that depends on it end to end.
func TestRegistry_ReadAndEditVerbsAreWiredToTheCodec(t *testing.T) {
	t.Parallel()
	root, file := seedReadWorkspace(t)
	ctx := wsCtx(root)

	registry := tools.NewRegistry()
	if err := RegisterAll(registry); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	read := registry.Get("read_file")
	if read == nil {
		t.Fatal("read_file is not registered, so nothing the agent does can reach it")
	}
	edit := registry.Get("edit_file")
	if edit == nil {
		t.Fatal("edit_file is not registered")
	}
	if _, ok := edit.Schema.Properties[precondition.Arg]; !ok {
		t.Fatalf("edit_file's schema has no %q argument, so the model is never told it may pass one and the precondition is never checked", precondition.Arg)
	}

	out, err := read.Execute(ctx, map[string]any{"path": file})
	if err != nil {
		t.Fatalf("registered read_file: %v", err)
	}
	handle := preconditionFrom(t, out)

	result, err := edit.Execute(ctx, map[string]any{
		"path":           file,
		"old_text":       "return v + 1",
		"new_text":       "return v + 2",
		precondition.Arg: handle,
	})
	if err != nil {
		t.Fatalf("registered edit_file refused an edit resting on an unchanged read: %v", err)
	}
	if !strings.Contains(result, "Replaced") {
		t.Errorf("edit did not report success:\n%s", result)
	}
}

// TestReadFile_WhenTheFileIsLong_ShouldOutlineWhatItDidNotPrint checks the
// elision through the live verb. Before the codec an oversized read was not
// delivered whole either — the tool-loop transcript clamps it head-and-tail —
// but the middle simply vanished, leaving nothing to ask for.
func TestReadFile_WhenTheFileIsLong_ShouldOutlineWhatItDidNotPrint(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	name := fmt.Sprintf("long_%d.go", readFixtureSeq.Add(1))

	var src strings.Builder
	src.WriteString("package a\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&src, "\nfunc fn%d() int {\n\treturn %d\n}\n", i, i)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(src.String()), 0o600); err != nil {
		t.Fatalf("seed long file: %v", err)
	}

	out, err := executeReadFile(wsCtx(root), map[string]any{"path": name})
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}

	if !strings.Contains(out, "not shown") {
		t.Errorf("a read that elided most of a file must say so; silence reads as 'this is the whole file':\n%s", firstLines(out, 3))
	}
	if !strings.Contains(out, "elsewhere in this file") || !strings.Contains(out, "function fn") {
		t.Errorf("the elided tail must be reachable by name and line, or the next read is a guess:\n%s", firstLines(out, 3))
	}
	if !strings.Contains(out, "more element(s) not listed") {
		t.Errorf("an outline that hit its own cap must say so, or it reads as the complete list of what is in the file:\n%s", firstLines(out, 3))
	}
	if len(out) >= len(src.String()) {
		t.Errorf("the projection of a %d-byte file cost %d bytes; a codec that costs more than the raw observation is decoration with a schema",
			len(src.String()), len(out))
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
