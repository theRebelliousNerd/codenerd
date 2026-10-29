package world

import (
	"go/parser"
	"go/token"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// Test-impact file facts.
//
// test_impact.mg's chain (test_depends_on -> impacted_test -> coverage_gap ->
// test_priority) joins file-level predicates the world scanners never
// produced: file_package, is_test_file and file_imports. W1
// (commit b80adca9) wired is_test_function and the call graph; this file wires
// the rest, for Go files, in the fast scanner (full and incremental paths).
// Same-directory grouping joins the linear file_dir companion instead of a
// quadratic pair relation (302,284 pairs on this repo, over the 250k EDB
// ceiling from internal/core's 274 files alone).
//
// Ownership: all three are scanner-owned (ScannerPredicates). A full scan
// re-derives and replaces them wholesale; a delta scan re-emits the pairs its
// files own and retracts their previous rows per file, the same pattern
// dependency_link follows. They are deliberately NOT scope facts: the CodeDOM
// scope path emits code_element, and clearCodeDOMFacts wipes its predicates
// globally on every scope replacement (internal/core/virtual_store.go), so a
// predicate the scanner owns must never join that replace-set or each
// open_file would delete world knowledge until the next scan. The chain joins
// code_element (scope) against these (scanner) inside the world shard, which
// owns every side of every join.
//
// Identity: every path below is the canonical (workspace-relative, slash)
// identity file_topology and code_element use. There is no second spelling.

// parseGoHeader returns a Go file's package-clause name and import paths.
// ImportsOnly skips function bodies, so this stays cheap on files the
// tree-sitter walker never touches; callers pass content they already read.
func parseGoHeader(content []byte) (pkg string, imports []string, err error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", content, parser.ImportsOnly)
	if err != nil {
		return "", nil, err
	}
	pkg = node.Name.Name
	for _, imp := range node.Imports {
		p := strings.Trim(imp.Path.Value, "\"")
		if p != "" {
			imports = append(imports, p)
		}
	}
	return pkg, imports, nil
}

// goTestFileHeaderFacts emits what the tree-sitter walker would emit for a Go
// file if the fast scanners did not skip test files: the file_package row and
// the raw pkg: import tokens that resolveDependencyLinksWithIndex turns into
// in-workspace edges (and file_imports rows beside them).
//
// Non-test files do not come through here: ParseGo's package_clause branch
// emits their file_package row and its import_spec branch their raw tokens,
// from the single parse the scan already pays for. Both spell the row
// identically (canonical path, package-clause name), so the two emitters are
// one producer as far as the kernel is concerned.
//
// A test file that fails to parse yields nothing: the scan tolerates an
// unparseable file the same way it tolerates a tree-sitter failure.
func goTestFileHeaderFacts(canonical string, content []byte) []core.Fact {
	pkg, imports, err := parseGoHeader(content)
	if err != nil {
		logging.WorldDebug("test-impact facts: header parse failed for %s: %v", canonical, err)
		return nil
	}
	var facts []core.Fact
	if pkg != "" {
		facts = append(facts, core.Fact{
			Predicate: "file_package",
			Args:      []any{canonical, pkg},
		})
	}
	for _, imp := range imports {
		facts = append(facts, core.Fact{
			Predicate: "dependency_link",
			Args:      []any{canonical, "pkg:" + imp, imp},
		})
	}
	return facts
}
