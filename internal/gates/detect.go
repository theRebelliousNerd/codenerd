package gates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/projectdoc"

	"github.com/kballard/go-shellquote"
)

// Detect returns the gates of the workspace at root.
//
// A malformed nerd.md is an error, not an empty declaration: silently ignoring
// the project's own commands would run detected ones the project does not use.
func Detect(root string) (Set, error) {
	doc, err := projectdoc.Load(root)
	if err != nil {
		return Set{}, fmt.Errorf("gates: %w", err)
	}
	var declared []Gate
	declaredKinds := map[Kind]bool{}
	if doc != nil {
		declared, err = fromNerdMD(doc)
		if err != nil {
			return Set{}, err
		}
		for _, g := range declared {
			if g.Source == sourceCommands {
				declaredKinds[g.Kind] = true
			}
		}
	}
	all := declared
	for _, g := range detect(root) {
		if !declaredKinds[g.Kind] {
			all = append(all, g)
		}
	}
	return split(all), nil
}

const (
	sourceCommands = "nerd.md commands"
	sourceGates    = "nerd.md gates"
)

// fromNerdMD turns commands.build/test/lint and the gates: list into gates.
func fromNerdMD(doc *projectdoc.Document) ([]Gate, error) {
	var out []Gate
	cmds := doc.Spec.Commands
	for _, c := range []struct {
		kind Kind
		run  string
	}{{Build, cmds.Build}, {Test, cmds.Test}, {Lint, cmds.Lint}} {
		if strings.TrimSpace(c.run) == "" {
			continue
		}
		argv, err := shellquote.Split(c.run)
		if err != nil {
			return nil, fmt.Errorf("gates: nerd.md commands.%s %q: %w", c.kind, c.run, err)
		}
		out = append(out, Gate{
			ID: "nerd.md:" + string(c.kind), Kind: c.kind, Argv: argv, Scope: ScopeAll,
			Language: languageOf(argv), Source: sourceCommands, Env: cmds.Env,
		})
	}
	for _, spec := range doc.Spec.Gates {
		argv, err := shellquote.Split(spec.Run)
		if err != nil {
			return nil, fmt.Errorf("gates: nerd.md gates %q run %q: %w", spec.ID, spec.Run, err)
		}
		scope := ScopeAll
		if spec.Scope == "node" {
			scope = ScopeNode
		}
		out = append(out, Gate{
			ID: "nerd.md:" + spec.ID, Kind: Kind(spec.Kind), Argv: argv, Scope: scope,
			Language: languageOf(argv), Source: sourceGates, Env: cmds.Env,
		})
	}
	return out, nil
}

// languageOf names the toolchain a declared command belongs to, from its
// program, so a node-scoped `go vet {pkg}` is not run on a Python package.
// A program it does not recognise (make, a script) belongs to every node.
func languageOf(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	prog := strings.ToLower(filepath.Base(argv[0]))
	prog = strings.TrimSuffix(prog, ".exe")
	if prog == "go" && len(argv) > 1 && argv[1] == "run" {
		// `go run ./tools/check` is a tool the workspace runs, not a check of
		// Go code; what it checks is its own business.
		return ""
	}
	switch prog {
	case "go", "gofmt", "golangci-lint", "staticcheck":
		return "go"
	case "python", "python3", "pytest", "mypy", "ruff", "flake8", "tox", "uv", "poetry":
		return "python"
	case "npm", "npx", "pnpm", "yarn", "bun", "node", "tsc", "eslint", "jest", "vitest":
		return "js/ts"
	case "cargo", "rustc", "clippy-driver":
		return "rust"
	}
	return ""
}

// detect reads the workspace's markers. Each language contributes the gates
// its standard toolchain offers; a workspace with several languages gets each
// language's.
func detect(root string) []Gate {
	var out []Gate
	if exists(root, "go.mod") {
		out = append(out,
			Gate{ID: "go:build", Kind: Build, Argv: []string{"go", "build", "./..."}, Scope: ScopeAll, Language: "go", Source: "detected: go.mod"},
			Gate{ID: "go:vet", Kind: Lint, Argv: []string{"go", "vet", PkgToken}, Scope: ScopeNode, Language: "go", Source: "detected: go.mod"},
			Gate{ID: "go:test", Kind: Test, Argv: []string{"go", "test", "-count=1", PkgToken}, Scope: ScopeNode, Language: "go", Source: "detected: go.mod"},
		)
	}
	if marker := pythonMarker(root); marker != "" {
		py := pythonInterpreter()
		out = append(out,
			Gate{ID: "python:compile", Kind: Build, Argv: []string{py, "-m", "compileall", "-q", NodeToken}, Scope: ScopeNode, Language: "python", Source: "detected: " + marker},
			// Exit 5 is pytest's "no tests collected": a node without tests
			// has not failed its tests.
			Gate{ID: "python:pytest", Kind: Test, Argv: []string{py, "-m", "pytest", "-q", NodeToken}, Scope: ScopeNode, Language: "python", Source: "detected: " + marker, OKExitCodes: []int{0, 5}},
		)
	}
	if exists(root, "package.json") {
		out = append(out, jsGates(root)...)
	}
	if exists(root, "Cargo.toml") {
		out = append(out,
			Gate{ID: "rust:build", Kind: Build, Argv: []string{"cargo", "build"}, Scope: ScopeAll, Language: "rust", Source: "detected: Cargo.toml"},
			Gate{ID: "rust:test", Kind: Test, Argv: []string{"cargo", "test"}, Scope: ScopeAll, Language: "rust", Source: "detected: Cargo.toml"},
			Gate{ID: "rust:clippy", Kind: Lint, Argv: []string{"cargo", "clippy", "--", "-D", "warnings"}, Scope: ScopeAll, Language: "rust", Source: "detected: Cargo.toml"},
		)
	}
	return out
}

// pythonMarker names the file that makes root a Python project, or "".
func pythonMarker(root string) string {
	for _, m := range []string{"pyproject.toml", "pytest.ini", "setup.cfg", "setup.py", "tox.ini"} {
		if exists(root, m) {
			return m
		}
	}
	return ""
}

// pythonInterpreter prefers python3, the name most systems install, and falls
// back to python; availability is judged later, like every gate's.
func pythonInterpreter() string {
	if _, err := lookPath("python3"); err == nil {
		return "python3"
	}
	return "python"
}

// jsGates runs the scripts package.json declares, through the package manager
// its lockfile names. A TypeScript project with no build script still gets a
// type check.
func jsGates(root string) []Gate {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		// A package.json no one can parse is not a set of gates; say so in the
		// plan through an unrunnable gate rather than dropping the project.
		return []Gate{{ID: "js:package.json", Kind: Build, Argv: nil, Scope: ScopeAll, Language: "js/ts", Source: "detected: package.json (unparsable: " + err.Error() + ")"}}
	}
	pm := "npm"
	switch {
	case exists(root, "pnpm-lock.yaml"):
		pm = "pnpm"
	case exists(root, "yarn.lock"):
		pm = "yarn"
	case exists(root, "bun.lockb"), exists(root, "bun.lock"):
		pm = "bun"
	}
	source := "detected: package.json (" + pm + ")"
	var out []Gate
	if _, ok := pkg.Scripts["build"]; ok {
		out = append(out, Gate{ID: "js:build", Kind: Build, Argv: []string{pm, "run", "build"}, Scope: ScopeAll, Language: "js/ts", Source: source})
	} else if exists(root, "tsconfig.json") {
		out = append(out, Gate{ID: "js:typecheck", Kind: Build, Argv: []string{"npx", "tsc", "--noEmit"}, Scope: ScopeAll, Language: "js/ts", Source: "detected: tsconfig.json"})
	}
	if script, ok := pkg.Scripts["test"]; ok && !strings.Contains(script, "no test specified") {
		out = append(out, Gate{ID: "js:test", Kind: Test, Argv: []string{pm, "run", "test"}, Scope: ScopeAll, Language: "js/ts", Source: source})
	}
	if _, ok := pkg.Scripts["lint"]; ok {
		out = append(out, Gate{ID: "js:lint", Kind: Lint, Argv: []string{pm, "run", "lint"}, Scope: ScopeAll, Language: "js/ts", Source: source})
	}
	return out
}

func exists(root, name string) bool {
	st, err := os.Stat(filepath.Join(root, name))
	return err == nil && !st.IsDir()
}
