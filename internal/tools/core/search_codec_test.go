package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/observation"
	"codenerd/internal/tools"
)

// codecFixtureSeq keeps each fixture's content unique across runs.
//
// Handles are content-addressed, so a fixture with identical bytes yields the
// identical handle on a second pass of the same test in the same process. That
// would let `go test -count=2` pass on a handle minted by the first pass rather
// than by the code under test, which is the run-once defect these tests exist
// to be free of.
var codecFixtureSeq atomic.Int64

// seedCodeWorkspace writes a small Go file with one declaration and two call
// sites, and returns the workspace root and the file's workspace-relative name.
func seedCodeWorkspace(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	name := fmt.Sprintf("widget_%d.go", codecFixtureSeq.Add(1))
	source := `package widget

import (
	"fmt"
)

// Encode is mentioned in this comment.
func Encode(v int) int {
	return v + 1
}

func caller() int {
	total := Encode(1)
	fmt.Println(total)
	return Encode(total)
}
`
	if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return root, name
}

func TestSearchCode_ShouldReportSymbolsAndEdgesRatherThanMatchingLines(t *testing.T) {
	t.Parallel()
	root, file := seedCodeWorkspace(t)

	out, err := executeSearchCode(wsCtx(root), map[string]any{"pattern": "Encode"})
	if err != nil {
		t.Fatalf("search_code: %v", err)
	}

	for _, want := range []string{
		file + ":Encode", // the declaration, named as a symbol
		"declared-here",  // and marked as the definition site
		file + ":caller", // the symbol that uses it
		"references",     // as a dependency edge, not as two more lines
	} {
		if !strings.Contains(out, want) {
			t.Errorf("search_code result is missing %q, so the reader still has to derive the structure from text:\n%s", want, out)
		}
	}

	// The whole point: the matching lines do not enter the result.
	for _, unwanted := range []string{"total := Encode(1)", "return Encode(total)"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("search_code quoted the matching line %q back; those belong behind the handle:\n%s", unwanted, out)
		}
	}
}

func TestSearchCode_WhenMatchIsAnImport_ShouldReportItAsADependency(t *testing.T) {
	t.Parallel()
	root, file := seedCodeWorkspace(t)

	out, err := executeSearchCode(wsCtx(root), map[string]any{"pattern": "fmt"})
	if err != nil {
		t.Fatalf("search_code: %v", err)
	}
	if !strings.Contains(out, file+" -> fmt") {
		t.Errorf("a hit on an import line is a dependency edge, not a mention of the package name:\n%s", out)
	}
}

func TestSearchCode_ShouldPublishAHandleThatSearchExpandRedeems(t *testing.T) {
	t.Parallel()
	root, file := seedCodeWorkspace(t)
	ctx := wsCtx(root)

	out, err := executeSearchCode(ctx, map[string]any{"pattern": "Encode"})
	if err != nil {
		t.Fatalf("search_code: %v", err)
	}
	handle := handleFrom(t, out)

	expanded, err := executeSearchExpand(ctx, map[string]any{"handle": handle})
	if err != nil {
		t.Fatalf("search_expand on a handle search_code just published: %v", err)
	}
	if !strings.Contains(expanded, file+":13: total := Encode(1)") {
		t.Errorf("expansion did not return the matching lines it retained:\n%s", expanded)
	}
}

// TestSearchExpand_WhenSourceChangedAfterTheSearch_ShouldStillReturnWhatWasSearched
// is the guarantee the elision rests on, checked through the live tool pair
// rather than through the codec in isolation. Re-running the search to expand
// it would answer from a world that has moved, and the agent would be holding a
// contradiction between the symbols it reasoned about and the lines it read.
func TestSearchExpand_WhenSourceChangedAfterTheSearch_ShouldStillReturnWhatWasSearched(t *testing.T) {
	t.Parallel()
	root, file := seedCodeWorkspace(t)
	ctx := wsCtx(root)

	out, err := executeSearchCode(ctx, map[string]any{"pattern": "Encode"})
	if err != nil {
		t.Fatalf("search_code: %v", err)
	}
	handle := handleFrom(t, out)

	// Rewrite so a re-run would find nothing, then delete so a re-run would
	// fail outright.
	path := filepath.Join(root, file)
	if err := os.WriteFile(path, []byte("package widget\n\nfunc Decode() {}\n"), 0o600); err != nil {
		t.Fatalf("rewrite source: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove source: %v", err)
	}

	expanded, err := executeSearchExpand(ctx, map[string]any{"handle": handle})
	if err != nil {
		t.Fatalf("expansion must survive the source changing under it: %v", err)
	}
	if !strings.Contains(expanded, "total := Encode(1)") {
		t.Errorf("expansion answered from the current file rather than from the retained observation:\n%s", expanded)
	}
	if strings.Contains(expanded, "Decode") {
		t.Errorf("expansion returned content that did not exist when the search ran:\n%s", expanded)
	}
}

func TestSearchExpand_WhenFileIsNamed_ShouldNarrowToThatFile(t *testing.T) {
	t.Parallel()
	root, file := seedCodeWorkspace(t)
	ctx := wsCtx(root)

	other := filepath.Join(root, "other_"+file)
	if err := os.WriteFile(other, []byte("package widget\n\nfunc x() { Encode(0) }\n"), 0o600); err != nil {
		t.Fatalf("seed second file: %v", err)
	}

	out, err := executeSearchCode(ctx, map[string]any{"pattern": "Encode"})
	if err != nil {
		t.Fatalf("search_code: %v", err)
	}
	handle := handleFrom(t, out)

	expanded, err := executeSearchExpand(ctx, map[string]any{"handle": handle, "file": "other_" + file})
	if err != nil {
		t.Fatalf("search_expand: %v", err)
	}
	if strings.Contains(expanded, file+":13:") {
		t.Errorf("a file-narrowed expansion returned matches from another file:\n%s", expanded)
	}
	if !strings.Contains(expanded, "other_"+file) {
		t.Errorf("a file-narrowed expansion returned nothing from the file it was pointed at:\n%s", expanded)
	}
}

func TestSearchExpand_WhenHandleIsMissingOrUnknown_ShouldSayWhichFailureItIs(t *testing.T) {
	t.Parallel()
	ctx := wsCtx(t.TempDir())

	if _, err := executeSearchExpand(ctx, map[string]any{}); err == nil {
		t.Error("an expansion with no handle must be refused rather than reported as an expired one; the caller would otherwise re-run a search over a typo")
	}
	_, err := executeSearchExpand(ctx, map[string]any{"handle": "obs:cs:000000000000"})
	if !errors.Is(err, observation.ErrNotFound) {
		t.Errorf("unknown handle error = %v, want ErrNotFound so the caller knows to run the search again", err)
	}
}

func TestSearchCode_WhenPathEscapesWorkspace_ShouldRefuse(t *testing.T) {
	t.Parallel()
	root, outside := seedWorkspace(t)

	// grep's containment is covered next door; search_code runs the same walk
	// and must not be the verb that leaks. Two copies of the walk is exactly how
	// one of them ends up without the guard.
	for name, path := range map[string]string{
		"absolute path outside root": outside,
		"dotdot traversal":           filepath.Join("..", "outside"),
		"root of filesystem":         string(filepath.Separator),
	} {
		t.Run(name, func(t *testing.T) {
			out, err := executeSearchCode(wsCtx(root), map[string]any{"pattern": "NEEDLE", "path": path})
			if err == nil {
				t.Fatalf("search_code accepted a path outside the workspace and returned:\n%s", out)
			}
			if strings.Contains(out, "secret") {
				t.Fatalf("search_code disclosed content from outside the workspace:\n%s", out)
			}
		})
	}
}

func TestSearchCode_WhenNothingMatches_ShouldNotPublishAHandle(t *testing.T) {
	t.Parallel()
	root, _ := seedCodeWorkspace(t)

	out, err := executeSearchCode(wsCtx(root), map[string]any{"pattern": "ThisIsNotInTheFixture"})
	if err != nil {
		t.Fatalf("search_code: %v", err)
	}
	if strings.Contains(out, "obs:cs:") {
		t.Errorf("a zero-match search published a handle; expanding it wastes a turn to learn there is nothing there:\n%s", out)
	}
}

// TestRegistry_SearchVerbsAreWiredToTheCodec guards the defect this repo keeps
// producing: a capability that exists, is tested in isolation, and is never
// called. It resolves both verbs the way the agent does — through the registry
// — and drives them end to end.
func TestRegistry_SearchVerbsAreWiredToTheCodec(t *testing.T) {
	t.Parallel()
	root, file := seedCodeWorkspace(t)
	ctx := wsCtx(root)

	registry := tools.NewRegistry()
	if err := RegisterAll(registry); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	search := registry.Get("search_code")
	if search == nil {
		t.Fatal("search_code is not registered, so nothing the agent does can reach it")
	}
	expand := registry.Get("search_expand")
	if expand == nil {
		t.Fatal("search_expand is not registered, so every handle search_code publishes is unredeemable")
	}

	out, err := search.Execute(ctx, map[string]any{"pattern": "Encode"})
	if err != nil {
		t.Fatalf("registered search_code: %v", err)
	}
	if !strings.Contains(out, file+":Encode") {
		t.Fatalf("the registered search_code is not the structural verb — it fell back to reporting lines:\n%s", out)
	}

	expanded, err := expand.Execute(ctx, map[string]any{"handle": handleFrom(t, out)})
	if err != nil {
		t.Fatalf("registered search_expand: %v", err)
	}
	if !strings.Contains(expanded, "total := Encode(1)") {
		t.Errorf("the registered search_expand did not return the retained lines:\n%s", expanded)
	}
}

func TestGrep_ShouldStillReportTheMatchingLines(t *testing.T) {
	t.Parallel()
	root, file := seedCodeWorkspace(t)

	// The two verbs are a pair: search_code shapes, grep does not. Refactoring
	// them onto one walk must not quietly change what grep answers.
	out, err := executeGrep(wsCtx(root), map[string]any{"pattern": "Encode"})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(out, file+":13: total := Encode(1)") {
		t.Errorf("grep no longer reports matching lines:\n%s", out)
	}
}

// handleFrom pulls the retention handle out of a rendered search_code result.
func handleFrom(t *testing.T, out string) string {
	t.Helper()
	const marker = "obs:cs:"
	idx := strings.Index(out, marker)
	if idx < 0 {
		t.Fatalf("search_code result published no handle, so the elided lines are unreachable:\n%s", out)
	}
	rest := out[idx:]
	end := strings.IndexAny(rest, " \n")
	if end < 0 {
		end = len(rest)
	}
	return rest[:end]
}
