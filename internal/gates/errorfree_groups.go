package gates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os/exec"
	"path"
	"sort"

	"codenerd/internal/build"
	"codenerd/internal/lintreport"
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

// WitnessArgv is the command that decides whether packages hold any finding
// of check from gate: the gate's own program over the packages with only that
// check enabled, which exits non-zero exactly when one remains. Gates with no
// such command (build and test are the workspace's own recurse gates; gofmt,
// govulncheck and the corpora have no per-check exit) report false.
func WitnessArgv(gate, check string, packages []string) ([]string, bool) {
	var argv []string
	switch gate {
	case "vet":
		argv = []string{"go", "vet", "-" + check}
	case "staticcheck":
		argv = []string{"staticcheck", "-checks", check}
	case "golangci_lint":
		argv = []string{"golangci-lint", "run", "--enable-only", check}
	default:
		return nil, false
	}
	return append(argv, packages...), true
}

// RunWitness runs a group's witness in root under the workspace build env and
// reports whether it passed (exit 0). An error is a witness that could not
// run or was stopped -- no verdict, never a pass.
func RunWitness(ctx context.Context, root string, argv []string) (bool, error) {
	if len(argv) == 0 {
		return false, errors.New("empty witness")
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
	err := cmd.Run()
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exitErr) && exitErr.Exited():
		return false, nil
	default:
		return false, err
	}
}
