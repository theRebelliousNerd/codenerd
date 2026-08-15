package types

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// =============================================================================
// KernelTransactor CONFORMANCE GUARD
// =============================================================================
//
// NewKernelTx panics when the Kernel it is handed does not implement
// KernelTransactor — deliberately, since the non-atomic fallback was removed and
// silently applying N separate rebuilds is worse than stopping. That makes
// "which Kernel implementations can transact" a load-bearing property with no
// compiler enforcement: Kernel does not embed KernelTransactor (mocks and
// read-only adapters legitimately cannot), so a new implementation compiles
// fine and blows up the first time production code batches an update.
//
// This test is that enforcement. It finds every Go type in the repo that
// implements types.Kernel and requires it to declare Transaction() too, with an
// explicit baseline for the ones that do not yet. Mocks are included on purpose:
// a mock without Transaction() turns "the code under test started using a
// transaction" into a panic inside somebody else's test suite.
//
// A compile-time `var _ types.KernelTransactor = (*T)(nil)` assertion cannot do
// this job from here: internal/types is imported by every implementation, so it
// can never name one. typestest.MockKernel carries exactly that assertion
// locally, which is the pattern each package should copy.

// kernelMarkerMethods are the methods that, taken together, identify a
// types.Kernel implementation. RetractExactFactsBatch and
// RemoveFactsByPredicateSet are the world-scanner batch operations; nothing
// else in this repo declares them.
var kernelMarkerMethods = []string{
	"AssertBatch",
	"RetractExactFactsBatch",
	"RemoveFactsByPredicateSet",
}

type kernelImpl struct {
	file     string
	typeName string
	line     int
	hasTx    bool
}

func findKernelImplementations(root string) []kernelImpl {
	fset := token.NewFileSet()
	// file -> type -> method set
	methods := map[string]map[string]map[string]bool{}
	decl := map[string]map[string]int{}

	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", ".codex", ".claude", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)

		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}
			recv := receiverTypeName(fn.Recv.List[0].Type)
			if recv == "" {
				continue
			}
			// Key by directory so two packages can each have a "mockKernel".
			pkgKey := filepath.ToSlash(filepath.Dir(rel))
			if methods[pkgKey] == nil {
				methods[pkgKey] = map[string]map[string]bool{}
				decl[pkgKey] = map[string]int{}
			}
			if methods[pkgKey][recv] == nil {
				methods[pkgKey][recv] = map[string]bool{}
			}
			methods[pkgKey][recv][fn.Name.Name] = true
			if _, seen := decl[pkgKey][recv]; !seen {
				decl[pkgKey][recv] = fset.Position(fn.Pos()).Line
			}
		}
		return nil
	})

	var out []kernelImpl
	for pkg, types := range methods {
		for name, set := range types {
			complete := true
			for _, m := range kernelMarkerMethods {
				if !set[m] {
					complete = false
					break
				}
			}
			if !complete {
				continue
			}
			out = append(out, kernelImpl{
				file:     pkg,
				typeName: name,
				line:     decl[pkg][name],
				hasTx:    set["Transaction"],
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].file != out[j].file {
			return out[i].file < out[j].file
		}
		return out[i].typeName < out[j].typeName
	})
	return out
}

func receiverTypeName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.StarExpr:
		return receiverTypeName(v.X)
	case *ast.Ident:
		return v.Name
	case *ast.IndexExpr: // generic receiver
		return receiverTypeName(v.X)
	}
	return ""
}

// nonTransactorBaseline lists Kernel implementations that do not yet implement
// KernelTransactor, keyed "package-dir:TypeName". Each is a real gap, not an
// exemption: the three production entries are forwarding adapters that wrap a
// *core.RealKernel, which DOES transact — they simply forward thirteen methods
// and not the fourteenth, so any NewKernelTx reached through one of them
// panics. The one-line fix, for every entry here:
//
//	func (a *T) Transaction() types.KernelTransaction { return a.kernel.Transaction() }
//
// and for mocks, embed or copy typestest.MockKernel, which already transacts.
var nonTransactorBaseline = map[string]string{
	// --- production forwarding adapters (real risk) ---
	"cmd/nerd/chat:sessionKernelAdapter":   "wraps *core.RealKernel for session.Executor; drops Transaction()",
	"cmd/nerd:campaignKernelAdapter":       "wraps *core.RealKernel for session.Executor; drops Transaction()",
	"internal/system:sessionKernelAdapter": "wraps *core.RealKernel for session.Executor; drops Transaction()",

	// --- test doubles ---
	"internal/campaign:safeKernel":                   "test double",
	"internal/core:mockActionsKernel":                "test double",
	"internal/core:mockDreamPlanKernel":              "test double",
	"internal/core:mockWorkflowKernel":               "test double",
	"internal/core:stubKernel":                       "test double",
	"internal/perception:mockKernel":                 "test double",
	"internal/session:MockKernel":                    "test double",
	"internal/shards:mockKernel":                     "test double",
	"internal/system:MockSystemKernel":               "test double",
	"internal/tools/research:browserReasoningKernel": "test double",
	"internal/world:mockKernel":                      "test double",
	"tests/e2e:mockKernel":                           "test double",
	"tests/e2e:mockKernelLLM":                        "test double",
	"tests/e2e:mockPoisonKernel":                     "test double",
}

func TestKernelTransactor_WhenKernelImplementationLacksTransaction_ShouldBeBaselined(t *testing.T) {
	root := factGuardRoot(t)
	impls := findKernelImplementations(root)
	if len(impls) < 5 {
		t.Fatalf("found only %d types.Kernel implementations; the scan is not reaching the tree", len(impls))
	}

	var missing []string
	for _, im := range impls {
		key := im.file + ":" + im.typeName
		if im.hasTx {
			if _, baselined := nonTransactorBaseline[key]; baselined {
				t.Errorf("%s now implements Transaction(); remove it from nonTransactorBaseline", key)
			}
			continue
		}
		if _, baselined := nonTransactorBaseline[key]; !baselined {
			missing = append(missing, fmt.Sprintf("%s (package %s, first method at line %d of its file)", key, im.file, im.line))
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("types.Kernel implementations without KernelTransactor:\n  %s\n\n"+
			"types.NewKernelTx panics on these. Add:\n"+
			"  func (x *T) Transaction() types.KernelTransaction { return x.inner.Transaction() }\n"+
			"or, for a mock, use internal/types/typestest.MockKernel.",
			strings.Join(missing, "\n  "))
	}
}

// TestKernelTransactor_WhenProductionAdapterWrapsATransactor_ShouldNotSilentlyDropIt
// pins the specific shape that caused this: a hand-written adapter that
// forwards the whole Kernel surface and stops one method short. It fails when a
// FOURTH such adapter appears, which is the moment to build one shared adapter
// instead of a fourth copy.
func TestKernelTransactor_WhenProductionAdapterWrapsATransactor_ShouldNotSilentlyDropIt(t *testing.T) {
	root := factGuardRoot(t)
	impls := findKernelImplementations(root)

	var prodMissing []string
	for _, im := range impls {
		if im.hasTx || strings.Contains(im.file, "tests/") {
			continue
		}
		key := im.file + ":" + im.typeName
		reason, ok := nonTransactorBaseline[key]
		if ok && strings.Contains(reason, "wraps *core.RealKernel") {
			prodMissing = append(prodMissing, key)
		}
	}
	if len(prodMissing) > 3 {
		sort.Strings(prodMissing)
		t.Errorf("%d production Kernel adapters now drop KernelTransactor (was 3):\n  %s\n"+
			"Forward Transaction() rather than adding another copy.",
			len(prodMissing), strings.Join(prodMissing, "\n  "))
	}
}
