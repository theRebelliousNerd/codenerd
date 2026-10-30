package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var equivalentModuleDeclarations = map[string]string{
	"unquoted":      "module example.com/proj\n",
	"quoted":        "module \"example.com/proj\" // workspace identity\n",
	"block":         "module (\n example.com/proj\n)\n",
	"quoted block":  "module (\n \"example.com/proj\" // workspace identity\n)\n",
	"lax extension": "module \"example.com/proj\"\nfuture_directive ignored\n",
}

func TestGoModuleIdentity_EquivalentDeclarationsReachTheSameNeighbor(t *testing.T) {
	for name, declaration := range equivalentModuleDeclarations {
		t.Run(name, func(t *testing.T) {
			root := goImportWorkspace(t)
			writeFile(t, root, "go.mod", []byte(declaration+"\ngo 1.26\n"))
			b := NewTieredContextBuilder(DefaultTieredContextConfig(root))
			if got := b.goModulePath(); got != "example.com/proj" {
				t.Errorf("semantic module identity = %q, want example.com/proj", got)
			}
			tc, err := b.BuildContext(context.Background(), "Run() misbehaves in internal/alpha/alpha.go")
			if err != nil {
				t.Fatal(err)
			}
			var neighbors []ContextFile
			for _, f := range tc.Files {
				if f.Tier == 3 {
					neighbors = append(neighbors, f)
				}
			}
			if len(neighbors) != 1 || neighbors[0].FilePath != filepath.Join(root, "internal/beta/beta.go") || neighbors[0].RelevanceScore != 0.5 {
				t.Fatalf("Tier 3 = %+v, want only beta.go at relevance 0.5", neighbors)
			}
		})
	}
}

func TestGoModuleIdentity_InvalidOrUnavailableManifestIsEmptyAndCached(t *testing.T) {
	for name, manifest := range map[string]string{
		"missing": "", "unreadable directory": "", "no module": "go 1.26\n",
		"empty module": "module\n", "duplicate": "module example.com/proj\nmodule example.com/other\n",
		"multiple arguments": "module example.com/proj extra\n",
		"unterminated quote": "module \"example.com/proj\n",
		"unterminated block": "module example.com/proj\nrequire (\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "go.mod")
			if name == "unreadable directory" {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			} else if name != "missing" {
				writeFile(t, root, "go.mod", []byte(manifest))
			}
			b := NewTieredContextBuilder(DefaultTieredContextConfig(root))
			if got := b.goModulePath(); got != "" {
				t.Errorf("invalid/unavailable manifest identity = %q, want empty", got)
			}
			if name == "unreadable directory" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			writeFile(t, root, "go.mod", []byte("module example.com/proj\n"))
			if got := b.goModulePath(); got != "" {
				t.Errorf("cached empty identity changed to %q within one builder", got)
			}
			if got := NewTieredContextBuilder(DefaultTieredContextConfig(root)).goModulePath(); got != "example.com/proj" {
				t.Errorf("fresh builder identity = %q after manifest repair", got)
			}
		})
	}
}

func TestGoModuleIdentity_ImportControlsAndBudgetsRemainBounded(t *testing.T) {
	root := goImportWorkspace(t)
	writeFile(t, root, "go.mod", []byte("module (\n \"example.com/proj\"\n)\n"))
	alpha := writeFile(t, root, "internal/alpha/alpha.go", []byte(`package alpha
import (
 "fmt"
 "example.net/other/pkg"
 "example.com/project/pkg"
 "example.com/proj/internal/missing"
 "example.com/proj/internal/alpha"
 "example.com/proj/internal/beta"
 _ "example.com/proj/internal/beta"
)
`))
	// Even a matching directory must not turn an external module or a module
	// prefix collision into a local edge.
	writeFile(t, root, "example.net/other/pkg/foreign.go", []byte("package foreign\n"))
	writeFile(t, root, "example.com/project/pkg/foreign.go", []byte("package foreign\n"))
	for _, name := range []string{"a.go", "b.go", "c.go", "d.go"} {
		writeFile(t, root, "internal/beta/"+name, []byte("package beta\n"))
	}
	writeFile(t, root, "internal/beta/notes.txt", []byte("not source"))
	b := NewTieredContextBuilder(DefaultTieredContextConfig(root))
	want := []string{filepath.Join(root, "internal/beta/a.go"), filepath.Join(root, "internal/beta/b.go"), filepath.Join(root, "internal/beta/c.go")}
	if got := b.goImportNeighbors(alpha); !reflect.DeepEqual(got, want) {
		t.Fatalf("neighbors = %v, want three smallest non-test sources, no self/duplicate/external/missing edge: %v", got, want)
	}
	b.maxTier3 = 1
	seeds := []ContextFile{{FilePath: alpha}}
	got := b.expandImportGraph(context.Background(), seeds, map[string]bool{alpha: true, want[0]: true})
	if len(got) != 1 || got[0].FilePath != want[1] || got[0].RelevanceScore != 0.5 {
		t.Fatalf("bounded/deduplicated Tier 3 = %+v, want only b.go at 0.5", got)
	}
	b.maxTier3 = 0
	if got := b.expandImportGraph(context.Background(), seeds, map[string]bool{}); len(got) != 0 {
		t.Fatalf("zero Tier 3 capacity selected %v", got)
	}
	b.maxTier3 = 10
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := b.expandImportGraph(ctx, seeds, map[string]bool{}); len(got) != 0 {
		t.Fatalf("canceled Tier 3 selected %v", got)
	}
}

func TestGoModuleIdentity_NoManifestPreservesLocalTreeFallback(t *testing.T) {
	root := t.TempDir()
	want := writeFile(t, root, "proj/internal/beta/beta.go", []byte("package beta\n"))
	alpha := writeFile(t, root, "alpha.go", []byte("package alpha\nimport (\n \"proj/internal/beta\"\n \"example.com/proj/internal/beta\"\n)\n"))
	writeFile(t, root, "example.com/proj/internal/beta/foreign.go", []byte("package foreign\n"))
	b := NewTieredContextBuilder(DefaultTieredContextConfig(root))
	if got := b.goImportNeighbors(alpha); !reflect.DeepEqual(got, []string{want}) {
		t.Fatalf("no-manifest neighbors = %v, want only local tree source %s", got, want)
	}
}
