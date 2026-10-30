package gates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os/exec"
	"path"
	"regexp"
	"sort"

	"codenerd/internal/build"
	"codenerd/internal/lintreport"
	"codenerd/internal/processutil"
)

// A FindingGroup is one unit of work a recurse cycle can take from a snapshot:
// every finding of one check, from one gate, in one package. The unit is the
// package because the witness is the gate run over the package with only that
// check enabled (WitnessArgv): it exits 0 exactly when the package holds no
// finding of the check, so it is determinate only if the cycle owns all of
// them. Picking among groups is policy/recurse.mg's, not this package's.
type FindingGroup struct {
	// ID is stable for the same gate, check and package.
	ID      string
	Gate    string
	Label   string
	Check   string
	Package string // "./lib", "." for the module root
	// Files is how many distinct files the findings are in.
	Files    int
	Findings []lintreport.Finding
	// Witness is the command that passes when the group is fixed.
	Witness []string
}

// FindingGroups groups s's findings of the gates that have a witness, in a
// stable order (by ID).
func FindingGroups(s Snapshot) []FindingGroup {
	byKey := map[string]*FindingGroup{}
	for _, m := range s.Gates {
		for _, f := range m.Findings {
			if f.File == "" || f.Check == "" {
				continue
			}
			pkg := "./" + path.Dir(f.File)
			if path.Dir(f.File) == "." {
				pkg = "."
			}
			witness, ok := WitnessArgv(m.Gate, f.Check, []string{pkg})
			if !ok {
				continue
			}
			key := m.Gate + "|" + f.Check + "|" + pkg
			g := byKey[key]
			if g == nil {
				sum := sha256.Sum256([]byte(key))
				g = &FindingGroup{ID: "ef_" + hex.EncodeToString(sum[:8]), Gate: m.Gate, Label: m.Label, Check: f.Check, Package: pkg, Witness: witness}
				byKey[key] = g
			}
			g.Findings = append(g.Findings, f)
		}
	}
	out := make([]FindingGroup, 0, len(byKey))
	for _, g := range byKey {
		files := map[string]bool{}
		for _, f := range g.Findings {
			files[f.File] = true
		}
		g.Files = len(files)
		lintreport.Sort(g.Findings)
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// selectable is the shape of a check each gate can enable on its own: a vet
// analyzer name (printf), a staticcheck check code (SA4006, ST1003, U1000),
// a golangci-lint linter name (errcheck).
var selectable = map[string]*regexp.Regexp{
	"vet":           regexp.MustCompile(`^[a-z][a-z0-9_]*$`),
	"staticcheck":   regexp.MustCompile(`^[A-Z]{1,2}[0-9]{4}$`),
	"golangci_lint": regexp.MustCompile(`^[a-z][a-z0-9-]*$`),
}

// compileCategory is a finding's check that names a failure to type-check,
// not a selectable check: lintreport's compile, and golangci-lint's
// typecheck pseudo-linter when its text held no positioned line.
func compileCategory(check string) bool {
	return check == lintreport.CompileCheck || check == "typecheck"
}

// WitnessArgv is the command that decides whether packages hold any finding
// of check from gate: the gate's own program over the packages with only that
// check enabled, which exits non-zero exactly when one remains. Gates with no
// such command (build and test are the workspace's own recurse gates; gofmt,
// govulncheck and the corpora have no per-check exit) report false.
//
// So does a check the gate cannot select. A compile diagnostic is a
// category -- the package did not type-check -- not an analyzer: there is no
// `go vet -compile`, and the witness built from it failed as a usage error
// whatever the attempt did, so a fix of the compile error was reverted
// (audit compile-diagnostic-witness-admission). The compile error stays in
// the snapshot; G1 build and the workspace's own recurse build gates carry
// it. A check not shaped like the gate's selectors is refused too, so no
// finding text becomes a flag on a command line.
func WitnessArgv(gate, check string, packages []string) ([]string, bool) {
	shape, known := selectable[gate]
	if !known || compileCategory(check) || !shape.MatchString(check) {
		return nil, false
	}
	var argv []string
	switch gate {
	case "vet":
		argv = []string{"go", "vet", "-" + check}
	case "staticcheck":
		argv = []string{"staticcheck", "-checks", check}
	case "golangci_lint":
		argv = []string{"golangci-lint", "run", "--enable-only", check}
	}
	return append(argv, packages...), true
}

// WitnessResult is a witness run's verdict and what the command printed.
type WitnessResult struct {
	// Passed: the command exited 0, so no finding of the check remains.
	Passed bool
	// ExitCode is the command's exit status.
	ExitCode int
	// Output is what it printed, stdout and stderr together, whole: a
	// failure says why here.
	Output string
}

// RunWitness runs a group's witness in root under the workspace build env and
// reports whether it passed (exit 0), with its output. An error is a witness
// that could not run or was stopped -- no verdict, never a pass. It runs
// through processutil, so a cancel takes the whole process tree.
func RunWitness(ctx context.Context, root string, argv []string) (WitnessResult, error) {
	if len(argv) == 0 {
		return WitnessResult{}, errors.New("empty witness")
	}
	var cmd *exec.Cmd
	if argv[0] == "go" {
		env, args := build.GoInvocation(root, root, argv[1:])
		cmd = exec.CommandContext(ctx, "go", args...)
		cmd.Env = env
	} else {
		cmd = exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Env = build.GoToolEnv(root, root)
	}
	cmd.Dir = root
	out, err := processutil.CombinedOutput(cmd)
	res := WitnessResult{Output: string(out)}
	if ctx.Err() != nil {
		return res, ctx.Err()
	}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		res.Passed = true
		return res, nil
	case errors.As(err, &exitErr) && exitErr.Exited():
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	default:
		return res, err
	}
}
