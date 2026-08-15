package sqlpragmas

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The leaf invariant — "never add mid-layer imports to this package" — is the
// whole reason this package exists. internal/mcp, internal/prompt and
// internal/core import it precisely because it cannot drag internal/store (and
// through store, half the runtime) into their import graph. One convenient
// import of internal/config or internal/store here silently re-creates the
// cycle the split was meant to remove, and nothing would notice until an
// unrelated package failed to compile.
//
// So the rule is not a package comment, it is this test: the production files
// of this package may import the standard library and exactly the codenerd
// packages listed below.

// allowedCodenerdImports is the complete set of first-party imports this
// package may take. Adding to it is a deliberate architectural decision, not a
// convenience — anything added here becomes an import that every consumer of
// sqlpragmas inherits.
var allowedCodenerdImports = map[string]string{
	"codenerd/internal/logging": "soft-fail Debug logging; logging is itself a leaf and must never import sqlpragmas",
}

// allowedThirdPartyImports is empty on purpose: the production package must not
// pull a SQLite driver (or anything else) into consumers' builds. Callers
// register and open with the driver they chose.
var allowedThirdPartyImports = map[string]string{}

func TestPackageImports_WhenNewImportAdded_ShouldStayLeaf(t *testing.T) {
	pkgDir := packageDir(t)

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	fset := token.NewFileSet()
	seenCodenerd := map[string]bool{}
	scanned := 0

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		scanned++

		path := filepath.Join(pkgDir, name)
		file, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}

		for _, imp := range file.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil {
				t.Fatalf("%s: unquote import %s: %v", name, imp.Path.Value, uerr)
			}

			switch {
			// codenerd paths are checked first: the module path has no dot in
			// its first segment, so the stdlib heuristic would otherwise
			// swallow every first-party import and pass this test vacuously.
			case strings.HasPrefix(p, "codenerd/"):
				seenCodenerd[p] = true
				if _, ok := allowedCodenerdImports[p]; !ok {
					t.Errorf("%s imports %q — sqlpragmas must stay a leaf.\n"+
						"  Allowed first-party imports: %s\n"+
						"  If this package genuinely needs configuration or state from a mid-layer package, "+
						"push it down instead (see SetHostClass / SetMetricsEnabled), or justify the edge by adding it to allowedCodenerdImports.",
						name, p, strings.Join(sortedKeys(allowedCodenerdImports), ", "))
				}
			case isStdlibImport(p):
				// Always fine: the standard library has no upward edges.
			default:
				if _, ok := allowedThirdPartyImports[p]; !ok {
					t.Errorf("%s imports third-party package %q — production sqlpragmas must depend on nothing outside the stdlib and %s. "+
						"Every consumer of this leaf would inherit the dependency.",
						name, p, strings.Join(sortedKeys(allowedCodenerdImports), ", "))
				}
			}
		}
	}

	if scanned == 0 {
		t.Fatal("scanned no production files — the scanner is broken, not the package")
	}

	for p, reason := range allowedCodenerdImports {
		if !seenCodenerd[p] {
			t.Errorf("stale entry in allowedCodenerdImports: %q (%s) is no longer imported. Remove it so the allowlist stays the real dependency set.", p, reason)
		}
	}
}

// isStdlibImport reports whether an import path is in the standard library.
// Stdlib paths never have a dot in their first segment; every module path does
// (a hostname).
func isStdlibImport(p string) bool {
	first, _, _ := strings.Cut(p, "/")
	return !strings.Contains(first, ".")
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// packageDir returns this package's own source directory.
func packageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRootForAudit(t), "internal", "sqlpragmas")
}
