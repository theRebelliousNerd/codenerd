// Package repoguard holds the tests that read the whole source tree.
//
// A test that walks the repository is an input of every file and directory it
// opens, so go test re-runs its package whenever anything in the tree
// changes -- including the runtime state a running nerd writes under .nerd.
// In a package of its own that costs this scan alone; inside internal/init it
// made the whole ~50s package re-run on every gate.
package repoguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"codenerd/internal/config"
)

// TestNoHardcodedToolTarget_AnywhereInTheRepo is the part that generalises.
//
// The same wrong default was written out longhand in two places and fixed
// separately, weeks apart in this branch's history, because nothing connected
// them. This scans for a third: any composite literal that sets TargetOS or
// TargetArch to a string constant. The only legitimate producer is
// config.DefaultToolGenerationConfig, which derives them from runtime.
//
// A user's own config.json may of course name any target — that is a value at
// runtime, not a literal in the source, and this does not touch it.
func TestNoHardcodedToolTarget_AnywhereInTheRepo(t *testing.T) {
	root := repoRootFromPackage(t)

	var offenders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "testdata", ".nerd", ".claude": // .claude holds agent worktrees (full repo copies); walking it duplicates every file under a non-checkout path
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)

		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || (key.Name != "TargetOS" && key.Name != "TargetArch") {
				return true
			}
			lit, ok := kv.Value.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, uerr := strconv.Unquote(lit.Value)
			if uerr != nil || value == "" {
				return true
			}
			offenders = append(offenders,
				rel+":"+strconv.Itoa(fset.Position(kv.Pos()).Line)+" "+key.Name+" = "+strconv.Quote(value))
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(offenders) > 0 {
		t.Errorf("build target hardcoded as a string literal:\n  %s\n\n"+
			"Ouroboros executes the binary it compiles, so a target the host cannot run "+
			"produces \"exec format error\" on first use. Derive it from "+
			"config.DefaultToolGenerationConfig (runtime.GOOS/GOARCH) instead. "+
			"Cross-compiling remains available — it just has to be asked for, in the "+
			"user's config, rather than written into the source.",
			strings.Join(offenders, "\n  "))
	}

	// The scan is only worth anything if it can see the shape it looks for.
	if _, err := os.Stat(filepath.Join(root, "internal", "init", "scanner.go")); err != nil {
		t.Fatalf("scanner.go not found under %s; this scan is looking at the wrong tree", root)
	}
	if config.DefaultToolGenerationConfig().TargetOS != runtime.GOOS {
		t.Fatal("config.DefaultToolGenerationConfig no longer derives from runtime; " +
			"the advice in this test's failure message is now wrong")
	}
}

func repoRootFromPackage(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for range 10 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate the module root")
	return ""
}
