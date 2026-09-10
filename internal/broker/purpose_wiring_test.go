package broker

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A purpose that nothing tags is an empty account. The ledger will happily
// enforce a cap on it, receipts will faithfully report zero against it, and the
// number will be wrong in the one direction nobody checks: the work is still
// happening, it is just landing in "unattributed" instead.
//
// That is not a hypothetical. Before this wiring existed there was exactly one
// WithPurpose call site in the entire repository, so essentially all spend was
// unattributed and every per-purpose budget was unenforceable by construction.
// These tests exist so that regression is loud.

// exemptPurposes are declared accounts that deliberately have no tagging call
// site, each with the reason it is exempt. Anything not listed here must be
// wired, and a new purpose added without wiring fails the inventory below.
var exemptPurposes = map[Purpose]string{
	PurposeUnattributed: "the fallback itself; tagging it would defeat its point",
	PurposeArticulation: "articulation makes no LLM calls today — emitter.go's only Complete is commented out",
	PurposeCritic:       "no critic subsystem issues inference of its own yet",
}

// tagSitesByPurpose names the file each purpose must be tagged in. Checking the
// location and not just the presence is what stops a tag drifting to a spot
// that no longer dominates the calls it is meant to account for.
var tagSitesByPurpose = map[Purpose]string{
	PurposePerception:   "internal/perception/transducer_llm.go",
	PurposeSession:      "internal/session/executor.go",
	PurposeCompression:  "internal/context/compressor.go",
	PurposeVerification: "internal/verification/verifier.go",
	PurposeSubagent:     "internal/core/shards/manager_spawn.go",
	PurposeAutopoiesis:  "internal/perception/learning.go",
	PurposeCampaign:     "cmd/nerd/chat/campaign.go",
}

// declaredPurposes reads the Purpose constants out of types.go, so the
// inventory cannot be satisfied by a stale hand-maintained list.
func declaredPurposes(t *testing.T) []Purpose {
	t.Helper()

	fset := token.NewFileSet()
	path := filepath.Join(repoRoot(t), "internal/broker/types.go")
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse types.go: %v", err)
	}

	var out []Purpose
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		ident, ok := spec.Type.(*ast.Ident)
		if !ok || ident.Name != "Purpose" {
			return true
		}
		for _, v := range spec.Values {
			lit, ok := v.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			out = append(out, Purpose(strings.Trim(lit.Value, `"`)))
		}
		return true
	})

	if len(out) == 0 {
		t.Fatal("found no Purpose constants; the parser is looking in the wrong place")
	}
	return out
}

// purposeTagSites scans the repository for WithPurpose calls and returns, for
// each purpose constant passed, the repo-relative files that tag it.
func purposeTagSites(t *testing.T) map[string][]string {
	t.Helper()

	root := repoRoot(t)
	sites := make(map[string][]string)
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "testdata":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			// A file that does not parse cannot be tagging anything, and a
			// generated or broken file elsewhere in the tree is not this
			// test's business to fail on.
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			if !isWithPurposeCall(call.Fun) {
				return true
			}
			name := purposeConstName(call.Args[1])
			if name == "" {
				return true
			}
			sites[name] = append(sites[name], rel)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	return sites
}

// isWithPurposeCall matches both broker.WithPurpose(...) and, for calls made
// from inside this package, a bare WithPurpose(...).
func isWithPurposeCall(fun ast.Expr) bool {
	switch f := fun.(type) {
	case *ast.SelectorExpr:
		pkg, ok := f.X.(*ast.Ident)
		return ok && pkg.Name == "broker" && f.Sel.Name == "WithPurpose"
	case *ast.Ident:
		return f.Name == "WithPurpose"
	}
	return false
}

// purposeConstName returns the identifier name of a broker.PurposeX argument.
func purposeConstName(arg ast.Expr) string {
	switch a := arg.(type) {
	case *ast.SelectorExpr:
		pkg, ok := a.X.(*ast.Ident)
		if ok && pkg.Name == "broker" && strings.HasPrefix(a.Sel.Name, "Purpose") {
			return a.Sel.Name
		}
	case *ast.Ident:
		if strings.HasPrefix(a.Name, "Purpose") {
			return a.Name
		}
	}
	return ""
}

// constNameFor maps a purpose value back to its Go constant identifier, e.g.
// "perception" -> "PurposePerception".
func constNameFor(t *testing.T, p Purpose) string {
	t.Helper()

	fset := token.NewFileSet()
	path := filepath.Join(repoRoot(t), "internal/broker/types.go")
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse types.go: %v", err)
	}

	var name string
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || len(spec.Values) != 1 {
			return true
		}
		ident, ok := spec.Type.(*ast.Ident)
		if !ok || ident.Name != "Purpose" {
			return true
		}
		lit, ok := spec.Values[0].(*ast.BasicLit)
		if !ok {
			return true
		}
		if Purpose(strings.Trim(lit.Value, `"`)) == p {
			name = spec.Names[0].Name
		}
		return true
	})
	return name
}

func TestEveryPurposeIsTaggedSomewhere(t *testing.T) {
	sites := purposeTagSites(t)

	var missing []string
	for _, p := range declaredPurposes(t) {
		if reason, exempt := exemptPurposes[p]; exempt {
			// An exemption that is no longer true is also a bug: it means real
			// spend is landing in unattributed while a comment claims the
			// account is empty on purpose.
			name := constNameFor(t, p)
			if len(sites[name]) > 0 {
				t.Errorf("purpose %q is exempt (%s) but is tagged at %v — remove the exemption",
					p, reason, sites[name])
			}
			continue
		}
		name := constNameFor(t, p)
		if name == "" {
			t.Fatalf("could not find the constant for purpose %q", p)
		}
		if len(sites[name]) == 0 {
			missing = append(missing, string(p))
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("these purposes have no WithPurpose call site, so their spend lands in "+
			"unattributed and any budget on them is unenforceable: %s\n"+
			"Either tag them at the entry point of the subsystem that spends them, "+
			"or add them to exemptPurposes with the reason.", strings.Join(missing, ", "))
	}
}

func TestPurposesAreTaggedAtTheirEntryPoints(t *testing.T) {
	sites := purposeTagSites(t)

	for p, wantFile := range tagSitesByPurpose {
		name := constNameFor(t, p)
		if name == "" {
			t.Errorf("purpose %q has an expected tag site but no constant", p)
			continue
		}
		found := false
		for _, f := range sites[name] {
			if f == wantFile {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("purpose %q is not tagged in %s (tagged in %v instead)\n"+
				"A tag that moves off the entry point stops dominating the calls it accounts for.",
				p, wantFile, sites[name])
		}
	}
}

func TestTagSiteFilesExist(t *testing.T) {
	// A stale path in tagSitesByPurpose would make the check above vacuous:
	// the file it names could never match, so the test would fail loudly —
	// unless someone "fixed" it by deleting the entry. Assert the paths are
	// real so a rename is diagnosed as a rename.
	for p, rel := range tagSitesByPurpose {
		if _, err := os.Stat(filepath.Join(repoRoot(t), rel)); err != nil {
			t.Errorf("expected tag site for %q does not exist: %s (%v)", p, rel, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Nesting semantics
// ---------------------------------------------------------------------------

func TestInnermostPurposeWins(t *testing.T) {
	// The rule a per-purpose cap depends on: compression running inside a
	// session turn is charged to compression. If the outer tag won instead,
	// capping compression would be impossible — its spend would be indexed
	// under whatever happened to call it.
	ctx := WithPurpose(context.Background(), PurposeSession)
	if got := PurposeFromContext(ctx); got != PurposeSession {
		t.Fatalf("outer purpose = %q, want session", got)
	}

	inner := WithPurpose(ctx, PurposeCompression)
	if got := PurposeFromContext(inner); got != PurposeCompression {
		t.Fatalf("inner purpose = %q, want compression", got)
	}

	// The outer context is unchanged: a sub-context cannot reach back and
	// re-account its parent's calls.
	if got := PurposeFromContext(ctx); got != PurposeSession {
		t.Fatalf("outer purpose after nesting = %q, want session", got)
	}
}

func TestUntaggedContextIsAccountedNotDiscarded(t *testing.T) {
	if got := PurposeFromContext(context.Background()); got != PurposeUnattributed {
		t.Fatalf("untagged purpose = %q, want unattributed", got)
	}
	//lint:ignore SA1012 the nil context is the case under test: an untagged
	// call must be accounted rather than panicking or vanishing.
	if got := PurposeFromContext(nil); got != PurposeUnattributed {
		t.Fatalf("nil-context purpose = %q, want unattributed", got)
	}
}
