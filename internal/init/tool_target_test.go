package init

import (
	"encoding/json"
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

// TestCreateDefaultConfig_ShouldTargetTheHost is the regression test for a bug
// that was "fixed" twice and stayed live both times.
//
// Ouroboros compiles a generated tool and then EXECUTES the binary itself, so a
// build target the host cannot run means every generated tool compiles cleanly
// and dies with "exec format error" the first time an agent calls it.
//
// The first fix set autopoiesis.DefaultConfig's default to runtime.GOOS. That
// function also honours an explicit user setting, so the second fix corrected
// config.DefaultToolGenerationConfig, on the theory that `nerd auth` persisted
// it. Adversarial review disproved that mechanism — LoadUserConfig returns an
// empty config for a MISSING file, so `nerd auth` never reaches the defaults
// and writes no target at all — and then found the path that really does it:
// createDefaultConfig here, which hardcoded windows/amd64 into every fresh
// workspace's .nerd/config.json. Measured on the branch AFTER both earlier
// fixes, `nerd init` still produced {windows amd64} and autopoiesis then read
// it back as a deliberate user choice.
//
// Three call sites, one defect. That is the recurring shape, so this test does
// not just check the one that was found — see the scan below.
func TestCreateDefaultConfig_ShouldTargetTheHost(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "config.json")
	ini := &Initializer{config: InitConfig{Workspace: ws}}
	if err := ini.createDefaultConfig(path); err != nil {
		t.Fatalf("createDefaultConfig: %v", err)
	}

	// Read it back off disk, because the file is what autopoiesis later treats
	// as the operator's explicit choice.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}
	var cfg config.UserConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse config.json: %v", err)
	}
	if cfg.ToolGeneration == nil {
		t.Fatal("a freshly initialised workspace records no ToolGeneration section")
	}
	if got := cfg.ToolGeneration.TargetOS; got != runtime.GOOS {
		t.Errorf("TargetOS = %q, want the host's %q. `nerd init` writes this to "+
			"config.json, and autopoiesis.DefaultConfig reads the raw section as an "+
			"explicit user choice — so a foreign target here is indistinguishable "+
			"from one the operator asked for, and every generated tool is built for "+
			"a machine that will not run it.", got, runtime.GOOS)
	}
	if got := cfg.ToolGeneration.TargetArch; got != runtime.GOARCH {
		t.Errorf("TargetArch = %q, want the host's %q", got, runtime.GOARCH)
	}
	if cfg.ToolGeneration.AllowToolExec {
		t.Error("AllowToolExec defaults to true in a freshly initialised workspace; " +
			"generated tools must not get os/exec unless the operator opts in")
	}
}

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
			case ".git", "node_modules", "vendor", "testdata", ".nerd":
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
