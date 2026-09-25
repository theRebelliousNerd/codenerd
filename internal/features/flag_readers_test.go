package features

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Every flag has a production reader, or says why it has none
// (GAP-FEAT-04, Docs/architecture/features/TODO.md).
//
// A flag with no reader is the quietest failure this package has: it parses,
// resolves, is listed by `nerd features`, is written true by `nerd init`, and
// does nothing. features.provenance was exactly that until 2026-09-25.

// flagAccessors names the accessor each flag is read through. A new flag must
// be added here, which is the point: it cannot ship without being asked who
// reads it.
var flagAccessors = map[string]string{
	"flight_recorder":    "IsFlightRecorderEnabled",
	"provenance":         "IsProvenanceEnabled",
	"system_shards":      "IsSystemShardsEnabled",
	"per_shard_facts":    "IsPerShardFactsEnabled",
	"dark_mode":          "IsDarkModeEnabled",
	"skip_onboarding":    "IsOnboardingSkipped",
	"taxonomy_fast":      "IsTaxonomyFastEnabled",
	"prompt_evolution":   "IsPromptEvolutionEnabled",
	"fast_scan_workers":  "FastScanWorkers",
	"fast_ast_max_bytes": "FastASTMaxBytes",
}

// inspectionSurfaces print the registry (`nerd features`, `/features`); a call
// there reports a flag and does not act on it, so it does not count as a
// reader.
var inspectionSurfaces = map[string]bool{
	"cmd/nerd/cmd_features.go":                    true,
	"cmd/nerd/chat/commands_handlers_features.go": true,
}

// reservedFlags are flags with no production reader by decision, each with the
// decision. Empty today. A reserved flag that gains a reader fails the test
// below, so a reservation cannot outlive its reason.
var reservedFlags = map[string]string{}

func TestEveryFlagHasAProductionReaderOrIsReserved(t *testing.T) {
	for _, key := range ConfigSchemaKeys() {
		if _, ok := flagAccessors[key]; !ok {
			t.Errorf("flag %s has no entry in flagAccessors: name the accessor production code reads it through", key)
		}
	}
	if len(flagAccessors) != len(ConfigSchemaKeys()) {
		t.Errorf("flagAccessors has %d entries for %d flags; remove the stale ones", len(flagAccessors), len(ConfigSchemaKeys()))
	}

	readers := productionReaders(t)
	var rows []string
	for flag, accessor := range flagAccessors {
		sites := readers[accessor]
		reason, reserved := reservedFlags[flag]
		switch {
		case len(sites) == 0 && !reserved:
			t.Errorf("flag %s: no production code outside internal/features calls features.%s, and it is not reserved: "+
				"wire it, or add it to reservedFlags with the decision", flag, accessor)
		case len(sites) > 0 && reserved:
			t.Errorf("flag %s is reserved (%s) but is read at %s: drop the reservation", flag, reason, strings.Join(sites, ", "))
		case len(sites) > 0:
			rows = append(rows, flag+" -> "+sites[0])
		}
	}
	sort.Strings(rows)
	for _, r := range rows {
		t.Log(r)
	}
}

// productionReaders maps each accessor to the non-test files outside this
// package that call it, as "path:line".
func productionReaders(t *testing.T) map[string][]string {
	t.Helper()
	root := moduleRoot(t)
	want := map[string]bool{}
	for _, a := range flagAccessors {
		want[a] = true
	}
	found := map[string][]string{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".nerd", ".claude", "node_modules", "vendor", "third_party", "testdata":
				return filepath.SkipDir
			}
			if filepath.ToSlash(path) == filepath.ToSlash(filepath.Join(root, "internal", "features")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if rel, _ := filepath.Rel(root, path); inspectionSurfaces[filepath.ToSlash(rel)] {
			return nil
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil || !strings.Contains(string(src), `"codenerd/internal/features"`) {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, src, 0)
		if perr != nil {
			return nil
		}
		name := ""
		for _, imp := range f.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); p == "codenerd/internal/features" {
				name = "features"
				if imp.Name != nil {
					name = imp.Name.Name
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || !want[sel.Sel.Name] {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == name {
				rel, _ := filepath.Rel(root, path)
				found[sel.Sel.Name] = append(found[sel.Sel.Name],
					filepath.ToSlash(rel)+":"+strconv.Itoa(fset.Position(sel.Pos()).Line))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatal("found no reader of any flag; the walk is asserting nothing")
	}
	return found
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test directory")
		}
		dir = parent
	}
}
