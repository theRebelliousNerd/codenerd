package broker

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Base and IsBrokered are only as good as the chain they can walk, and they walk
// it by looking for one method: Unwrap() LLMClient. A decorator without it is a
// wall. Base stops there and returns the wall instead of the client behind it;
// IsBrokered stops there and says "not metered" about a chain that is.
//
// Neither failure is loud. Every caller of Base does a comma-ok type assertion,
// so a wrong answer reads as "not that engine" and the feature it gates just
// stops happening. That is how the codex_cli framework tag disappeared from JIT
// atom selection: boot wraps every client twice (scheduling over tracing over
// metering), neither wrapper had Unwrap, and the assertion at the far end never
// matched again. Nothing logged, no test failed, the tag was simply never set.
//
// The per-decorator tests could not see it because each builds a chain one deep,
// where there is nothing for Base to walk past. So this audit checks the
// property at the level it actually lives: every type in the repo that both IS
// an LLMClient and HOLDS one.
func TestEveryLLMClientDecoratorCanBeUnwrapped(t *testing.T) {
	// The four methods of types.LLMClient. A type that declares all of them, or
	// embeds the interface and inherits them, is an LLMClient -- which is what
	// makes it reachable by Base in the first place.
	required := []string{"Complete", "CompleteWithSystem", "CompleteWithStreaming", "CompleteWithTools"}

	decorators := findLLMClientDecorators(t, repoRoot(t))

	var missing []string
	for _, d := range decorators {
		if !d.hasUnwrap {
			missing = append(missing, d.String())
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("%s decorates an LLMClient but has no Unwrap() LLMClient.\n"+
			"broker.Base and broker.IsBrokered stop at this type instead of walking through it, "+
			"so concrete-engine checks downstream silently stop matching and the double-metering "+
			"guard reads false on an already-metered client. Add:\n"+
			"\tfunc (x *T) Unwrap() LLMClient { return x.<field> }", m)
	}

	// A gate that finds nothing proves nothing. The known decorators are the
	// broker's own wrapper plus the two boot installs over it; if the count
	// drops below that, this audit has stopped matching the source rather than
	// the source having become clean.
	if len(decorators) < 3 {
		t.Errorf("the audit found only %d LLMClient decorators (%v); it has drifted from the "+
			"source and is no longer proving anything. The methods it looks for are %v.",
			len(decorators), decorators, required)
	}
}

type decorator struct {
	pkgDir    string
	typeName  string
	hasUnwrap bool
}

func (d decorator) String() string { return d.pkgDir + "." + d.typeName }

// findLLMClientDecorators returns every struct in the repo that both holds an
// LLMClient and satisfies the interface.
//
// Grouped by directory rather than by file because a type's methods need only
// share its package, not its file -- checking file by file would miss any
// decorator whose methods were split out, which is exactly the shape a large
// wrapper takes as it grows.
func findLLMClientDecorators(t *testing.T, root string) []decorator {
	t.Helper()

	byDir := map[string][]string{}
	skipDir := map[string]bool{".git": true, "vendor": true, "node_modules": true, "Docs": true, "testdata": true}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDir[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		// Tests are excluded: a test decorator is a stub that never sits in a
		// production chain, and requiring Unwrap of every fake would be noise
		// that teaches people to ignore this gate.
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		dir := filepath.Dir(path)
		byDir[dir] = append(byDir[dir], path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	var out []decorator
	for dir, files := range byDir {
		holds, embeds := map[string]bool{}, map[string]bool{}
		methods := map[string]map[string]bool{}

		for _, path := range files {
			fset := token.NewFileSet()
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				// A file this package cannot parse is a build failure that will
				// be reported far more clearly elsewhere; skipping it here beats
				// turning every syntax error into a confusing audit failure.
				continue
			}
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					collectStructFields(d, holds, embeds)
				case *ast.FuncDecl:
					if name, ok := receiverTypeName(d); ok {
						if methods[name] == nil {
							methods[name] = map[string]bool{}
						}
						methods[name][d.Name.Name] = true
					}
				}
			}
		}

		for name := range holds {
			// Embedding the interface promotes all four methods, so an embedder
			// is an LLMClient without declaring anything.
			if !embeds[name] && !declaresLLMClientMethods(methods[name]) {
				continue
			}
			out = append(out, decorator{
				pkgDir:    mustRel(t, root, dir),
				typeName:  name,
				hasUnwrap: methods[name]["Unwrap"],
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func collectStructFields(d *ast.GenDecl, holds, embeds map[string]bool) {
	if d.Tok != token.TYPE {
		return
	}
	for _, spec := range d.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok || st.Fields == nil {
			continue
		}
		for _, f := range st.Fields.List {
			if !isLLMClientType(f.Type) {
				continue
			}
			holds[ts.Name.Name] = true
			if len(f.Names) == 0 {
				embeds[ts.Name.Name] = true
			}
		}
	}
}

// isLLMClientType matches the interface however the package spells it: bare
// LLMClient in packages that alias it, types.LLMClient elsewhere.
func isLLMClientType(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name == "LLMClient"
	case *ast.SelectorExpr:
		return e.Sel.Name == "LLMClient"
	}
	return false
}

func receiverTypeName(fn *ast.FuncDecl) (string, bool) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return "", false
	}
	switch rt := fn.Recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if ident, ok := rt.X.(*ast.Ident); ok {
			return ident.Name, true
		}
	case *ast.Ident:
		return rt.Name, true
	}
	return "", false
}

// declaresLLMClientMethods is what separates a decorator from an adapter.
//
// Several types in this repo hold an LLMClient and expose a Complete of their
// own without being an LLMClient: internal/autopoiesis and
// internal/shards/system each define a narrower local interface. Those can never
// reach Base -- it takes a types.LLMClient -- so requiring Unwrap of them would
// be a method nothing could ever call.
func declaresLLMClientMethods(methods map[string]bool) bool {
	for _, m := range []string{"Complete", "CompleteWithSystem", "CompleteWithStreaming", "CompleteWithTools"} {
		if !methods[m] {
			return false
		}
	}
	return true
}

func mustRel(t *testing.T, root, dir string) string {
	t.Helper()
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return dir
	}
	return filepath.ToSlash(rel)
}
