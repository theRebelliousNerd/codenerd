package world

import (
	"testing"

	"codenerd/internal/core"
)

// groupFactsByPath decides which file each scan fact belongs to, so that file's
// rows can be replaced or deleted as a unit. Getting it wrong is not visible at
// the time: the facts are still stored, just filed against the wrong owner, and
// the cost appears later as a deleted file whose symbols never leave the kernel.
//
// It used to guess, by looking for an argument "containing a slash". That had
// two failure modes and this file used to test only one of them:
//
//   - False negative, which is the one that shipped: "sub/gamma.go" matched and
//     "alpha.go" did not, so every fact for a file at the repository ROOT was
//     filed as global. Measured on a two-file fixture, a nested file persisted 4
//     rows and a root-level one persisted 1.
//   - False positive: a symbol id like "pkg/thing.Method" reads as a path to a
//     file that does not exist.
//
// It now matches arguments against the file_topology paths in the same
// snapshot — the authoritative list of what was scanned — which has neither
// failure mode but does carry a precondition: the caller must pass a whole
// snapshot. These tests pin the contract and that precondition together,
// because the precondition is the new way to get this wrong.

func groupedKeys(m map[string][]core.Fact) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestGroupFactsByPath_ShouldFileFactsUnderTheirFile covers both the nested and
// the root-level case in one snapshot. The root-level file is the whole point:
// a fixture with only nested files passed throughout the period the bug existed.
func TestGroupFactsByPath_ShouldFileFactsUnderTheirFile(t *testing.T) {
	facts := []core.Fact{
		{Predicate: "file_topology", Args: []any{"internal/world/x.go", "hash1", core.MangleAtom("/go"), int64(1), core.MangleAtom("/false")}},
		{Predicate: "file_topology", Args: []any{"alpha.go", "hash2", core.MangleAtom("/go"), int64(2), core.MangleAtom("/false")}},

		{Predicate: "symbol_graph", Args: []any{"func:Foo", "/function", "/public", "internal/world/x.go", "func Foo()"}},
		{Predicate: "symbol_graph", Args: []any{"func:Alpha", "/function", "/public", "alpha.go", "func Alpha()"}},
		{Predicate: "file_dir", Args: []any{"alpha.go", "."}},
		{Predicate: "entry_point", Args: []any{"alpha.go"}},
	}

	out := groupFactsByPath(facts)

	// The false positive: "/function" is a node kind, not a file.
	if _, ok := out["/function"]; ok {
		t.Errorf("keyed under %q, which is a symbol kind and not a file; keys: %v", "/function", groupedKeys(out))
	}

	nested := out["internal/world/x.go"]
	if len(nested) != 2 {
		t.Errorf("nested file: got %d facts, want 2 (its file_topology and its symbol); keys: %v",
			len(nested), groupedKeys(out))
	}

	// The false negative that shipped.
	root := out["alpha.go"]
	if len(root) != 4 {
		t.Errorf("root-level file: got %d facts, want 4 (file_topology, symbol_graph, file_dir, entry_point). "+
			"A root-level file whose facts are filed as global cannot be retracted when the file is "+
			"deleted, so its symbols stay in the kernel for the life of the workspace. keys: %v",
			len(root), groupedKeys(out))
	}
	for _, f := range root {
		if f.Predicate == "symbol_graph" {
			return
		}
	}
	t.Errorf("root-level file's group has no symbol_graph fact: %v", root)
}

// TestGroupFactsByPath_WhenNoFileNamesAFact_ShouldFileItAsGlobal pins the other
// half: facts that genuinely belong to no single file must not be forced onto
// one. project_language is the real instance — it is a whole-snapshot property.
func TestGroupFactsByPath_WhenNoFileNamesAFact_ShouldFileItAsGlobal(t *testing.T) {
	facts := []core.Fact{
		{Predicate: "file_topology", Args: []any{"alpha.go", "hash", core.MangleAtom("/go"), int64(1), core.MangleAtom("/false")}},
		{Predicate: "project_language", Args: []any{core.MangleAtom("/go")}},
		{Predicate: "directory", Args: []any{"internal/world", "world"}},
	}

	out := groupFactsByPath(facts)

	if got := len(out[globalWorldFactsPath]); got != 2 {
		t.Errorf("global bucket has %d facts, want 2 (project_language, directory); keys: %v",
			got, groupedKeys(out))
	}
	if got := len(out["alpha.go"]); got != 1 {
		t.Errorf("alpha.go has %d facts, want just its own file_topology", got)
	}
}

// TestGroupFactsByPath_WhenGivenAPartialSet_ShouldNotInventAnOwner documents the
// precondition rather than pretending it does not exist.
//
// file_topology IS the file list. A symbol whose file has no file_topology in
// the same slice cannot be attributed, and the function files it as global
// instead of guessing — which is the safe direction, but it is also silent. Both
// production callers pass a full ScanWorkspaceCtx result, and this test exists so
// that a future caller who passes a delta finds the behaviour written down
// instead of discovering it as missing rows months later.
func TestGroupFactsByPath_WhenGivenAPartialSet_ShouldNotInventAnOwner(t *testing.T) {
	facts := []core.Fact{
		{Predicate: "symbol_graph", Args: []any{"func:Foo", "/function", "/public", "internal/world/x.go", "func Foo()"}},
	}

	out := groupFactsByPath(facts)

	if _, ok := out["internal/world/x.go"]; ok {
		t.Errorf("attributed a fact to a file with no file_topology in the same snapshot; " +
			"if that is now intended, this function's precondition changed and its comment must too")
	}
	if got := len(out[globalWorldFactsPath]); got != 1 {
		t.Errorf("global bucket has %d facts, want 1", got)
	}
}
