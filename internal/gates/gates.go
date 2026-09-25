// Package gates finds a workspace's own checks: the build, test, lint and
// audit commands that decide whether a change left the code better or worse.
//
// It exists because every gate codeNERD enforced was Go's. The forcing gates
// ran `go build`, `go test` and `go vet`; in a Python, TypeScript or Rust
// workspace they found no Go toolchain, skipped, and the verdict discipline
// they exist to provide evaporated. Recurse -- the loop that improves a
// workspace forever -- needs the workspace's gates, whatever its language, and
// needs to know when one cannot run so it never reports that as a pass.
//
// Sources, in precedence order:
//  1. nerd.md: commands.build/test/lint (the project's canonical invocations)
//     and the gates: list (audits, budgets, extra linters).
//  2. Detection from the workspace's markers: go.mod, pyproject.toml /
//     pytest.ini / setup.cfg / setup.py, package.json, Cargo.toml.
//
// A nerd.md command of a kind replaces every detected gate of that kind: the
// project has said how it builds, and a second opinion from a marker file is
// how a gate ends up running a command the project does not use.
package gates

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Kind classifies a gate.
type Kind string

const (
	Build Kind = "build"
	Test  Kind = "test"
	Lint  Kind = "lint"
	Audit Kind = "audit"
)

// Scope says whether a gate runs once for the workspace or once per node.
type Scope string

const (
	// ScopeAll runs once for the whole workspace.
	ScopeAll Scope = "all"
	// ScopeNode runs once per recurse node; Argv names NodeToken or PkgToken.
	ScopeNode Scope = "node"
)

// Placeholders a node-scoped Argv carries.
const (
	// NodeToken is replaced by the node's workspace-relative slash directory,
	// "." for the root.
	NodeToken = "{node}"
	// PkgToken is replaced by the node's Go package pattern: "./dir", or "."
	// for the root.
	PkgToken = "{pkg}"
)

// Gate is one of the workspace's checks.
type Gate struct {
	// ID is stable across runs; findings and the recurse ledger key on it.
	ID string
	// Kind is build, test, lint or audit.
	Kind Kind
	// Argv is the command, never joined into a shell string. A node-scoped
	// gate's Argv carries NodeToken or PkgToken.
	Argv []string
	// Scope is ScopeAll or ScopeNode.
	Scope Scope
	// Language is the toolchain the gate belongs to ("go", "python",
	// "js/ts", "rust"), or "" for a gate nerd.md declared.
	Language string
	// Source says where the gate came from, for the plan and the ledger.
	Source string
	// OKExitCodes are the exit codes that mean pass. Empty means only 0.
	// pytest exits 5 when it collected no tests, which is not a failure.
	OKExitCodes []int
	// Env are extra variables the command needs (nerd.md commands.env).
	Env map[string]string
}

// Passed reports whether exitCode is a pass for this gate.
func (g Gate) Passed(exitCode int) bool {
	if len(g.OKExitCodes) == 0 {
		return exitCode == 0
	}
	for _, c := range g.OKExitCodes {
		if c == exitCode {
			return true
		}
	}
	return false
}

// AppliesTo reports whether a node-scoped gate runs on a node whose files
// are in langs. A gate with no language runs on every node.
func (g Gate) AppliesTo(langs []string) bool {
	if g.Language == "" {
		return true
	}
	for _, l := range langs {
		if l == g.Language {
			return true
		}
	}
	return false
}

// ForNode returns the gate's argv for node (a workspace-relative slash
// directory, "." for the root). A workspace-scoped gate's argv comes back
// unchanged.
func (g Gate) ForNode(node string) []string {
	node = strings.TrimSpace(node)
	if node == "" {
		node = "."
	}
	pkg := "."
	if node != "." {
		pkg = "./" + strings.TrimPrefix(node, "./")
	}
	out := make([]string, len(g.Argv))
	for i, a := range g.Argv {
		a = strings.ReplaceAll(a, PkgToken, pkg)
		out[i] = strings.ReplaceAll(a, NodeToken, node)
	}
	return out
}

// String renders the gate for a plan: its ID, kind, scope and command.
func (g Gate) String() string {
	return fmt.Sprintf("%-28s %-5s %-4s %s", g.ID, g.Kind, g.Scope, strings.Join(g.Argv, " "))
}

// Unavailable is a gate the workspace calls for whose program is not on PATH.
// It is reported, never dropped: a check that cannot run is not a check that
// passed.
type Unavailable struct {
	Gate   Gate
	Reason string
}

// Set is a workspace's gates, runnable and not.
type Set struct {
	Gates       []Gate
	Unavailable []Unavailable
}

// lookPath is exec.LookPath; a variable so tests control what is installed.
var lookPath = exec.LookPath

// split partitions gates into runnable and unavailable by whether their
// program resolves, and orders both by ID.
func split(all []Gate) Set {
	var s Set
	for _, g := range all {
		if len(g.Argv) == 0 {
			s.Unavailable = append(s.Unavailable, Unavailable{Gate: g, Reason: "empty command"})
			continue
		}
		prog := g.Argv[0]
		if strings.ContainsAny(prog, `/\`) && !filepath.IsAbs(prog) {
			// A workspace-relative script (./scripts/check.sh) resolves at run
			// time against the workspace; PATH has nothing to say about it.
			s.Gates = append(s.Gates, g)
			continue
		}
		if _, err := lookPath(prog); err != nil {
			s.Unavailable = append(s.Unavailable, Unavailable{Gate: g, Reason: fmt.Sprintf("%s is not on PATH", prog)})
			continue
		}
		s.Gates = append(s.Gates, g)
	}
	sort.Slice(s.Gates, func(i, j int) bool { return s.Gates[i].ID < s.Gates[j].ID })
	sort.Slice(s.Unavailable, func(i, j int) bool { return s.Unavailable[i].Gate.ID < s.Unavailable[j].Gate.ID })
	return s
}
