package gates

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/lintreport"
)

// The error-free gates: the ladder's table of checks that must read zero on
// main (Docs/journeys/05-elite-harness-ladder.md, "The error-free gates"),
// measured as data. Rung R7 keeps a cycle only when no gate got worse, so the
// gates are measured before and after it, each into a typed Measurement, and
// the comparison is the kernel's (policy/gates.mg, Compare).
//
// Each command-run gate reads its tool's machine-readable report through
// internal/lintreport -- the readers the model's run_lint tool uses -- so a
// finding the model can see is the finding the gate counts.

// NoPackageScope is the Scope of a lint gate scoped to no package. No finding
// is in it, so the baseline restricted to it is empty too.
const NoPackageScope = "(no Go package)"

// MeasureStatus is one gate's verdict in a snapshot.
type MeasureStatus string

const (
	// MeasurePass: the gate ran to the end and reported nothing.
	MeasurePass MeasureStatus = "pass"
	// MeasureFail: the gate reported findings, or exited non-zero.
	MeasureFail MeasureStatus = "fail"
	// MeasureUnavailable: the gate's program is not installed. Nothing ran.
	MeasureUnavailable MeasureStatus = "unavailable"
	// MeasureUnverified: the gate started and reached no verdict (stopped,
	// killed, or a report that could not be read). It is not a pass.
	MeasureUnverified MeasureStatus = "unverified"
)

// Measurement is one gate measured once.
type Measurement struct {
	// Gate is the gate's name constant without the slash ("vet").
	Gate string `json:"gate"`
	// Label is its row in the ladder's table ("G2").
	Label    string               `json:"label"`
	Status   MeasureStatus        `json:"status"`
	Count    int                  `json:"count"`
	Findings []lintreport.Finding `json:"findings"`
	Argv     []string             `json:"argv,omitempty"`
	ExitCode *int                 `json:"exit_code,omitempty"`
	// Output is what the gate printed that was neither a finding nor a
	// compile error, kept whole: a failure that named nothing says why here.
	Output string `json:"output,omitempty"`
	// Unenumerated: the gate failed in a way its findings do not account
	// for (it exited non-zero and named nothing). Its findings are not the
	// whole failure, so no count drawn from them is complete, and narrowing
	// or composing the measurement keeps the failure (restrict, Overlay):
	// where it lies is unknown.
	Unenumerated bool `json:"unenumerated,omitempty"`
	// Scope are the package patterns a scoped measurement covered; empty is
	// the whole module.
	Scope []string `json:"scope,omitempty"`
	// Packages are the package patterns the gate measured, scoped or not
	// ("./..." for the whole module); empty for a gate that takes none
	// (gofmt, the corpus gates). With the snapshot's root they are the
	// gate's domain: two measurements are compared only over one domain
	// (policy/gates.mg gate_domain), so G3 over ./good is not read as an
	// improvement on G3 over ./... (audit gate-comparison-domain-admission).
	Packages []string      `json:"packages,omitempty"`
	Reason   string        `json:"reason,omitempty"`
	Duration time.Duration `json:"duration_ns"`
}

// Measurer measures one gate.
type Measurer interface {
	Gate() string
	Label() string
	Measure(ctx context.Context, root string) Measurement
}

// MeasureOptions narrows a measurement.
type MeasureOptions struct {
	// TestPackages scopes G3 (go test) to these package patterns; empty is
	// the whole module, ./... .
	TestPackages []string
	// ScopedPackages, when set, narrows the per-package lint gates (G2 vet,
	// G4 staticcheck, G5 golangci-lint -- ScopedGates) to these package
	// patterns, and their Measurements record the scope. A recurse cycle's
	// after-measure uses it for the packages the cycle touched; compare such
	// a snapshot with Snapshot.RestrictTo of the full one. A non-nil empty
	// list scopes them to no package: they do not run, and pass over that
	// empty scope (NoPackageScope), so a cycle that touched no Go file is
	// compared on nothing it could not have changed.
	ScopedPackages []string
	// Only measures these gates (by name or label); empty measures all.
	Only []string
	// Extra are gates the caller can measure and this package cannot: the
	// codeNERD tree's policy and atom corpora, which need the engine and the
	// prompt corpus (cmd/nerd supplies them).
	Extra []Measurer
}

// ErrorFreeMeasurers are the gates of the table this package measures, in
// table order, followed by opts.Extra, narrowed by opts.Only.
func ErrorFreeMeasurers(opts MeasureOptions) []Measurer {
	tests := opts.TestPackages
	if len(tests) == 0 {
		tests = []string{"./..."}
	}
	lint := []string{"./..."}
	scoped := opts.ScopedPackages != nil
	if scoped {
		lint = append([]string(nil), opts.ScopedPackages...)
	}
	all := []Measurer{
		commandGate{gate: "build", label: "G1", binary: "go", buildOutput: true,
			args: []string{"build"}, packages: []string{"./..."}, read: readCompile},
		commandGate{gate: "vet", label: "G2", binary: "go",
			args: []string{"vet", "-json"}, packages: lint, scoped: scoped, read: readVet},
		commandGate{gate: "test", label: "G3", binary: "go",
			args: []string{"test", "-count=1", "-json"}, packages: tests, read: readGoTest},
		commandGate{gate: "staticcheck", label: "G4", binary: "staticcheck",
			args: []string{"-f", "json"}, packages: lint, scoped: scoped, read: readStaticcheck},
		commandGate{gate: "golangci_lint", label: "G5", binary: "golangci-lint",
			args:     []string{"run", "--output.json.path", "stdout", "--show-stats=false", "--max-issues-per-linter=0", "--max-same-issues=0"},
			packages: lint, scoped: scoped, read: readGolangci},
		commandGate{gate: "govulncheck", label: "G6", binary: "govulncheck",
			args: []string{"-json"}, packages: []string{"./..."}, read: readGovulncheck},
		gofmtGate{},
	}
	all = append(all, opts.Extra...)
	if len(opts.Only) == 0 {
		return all
	}
	want := map[string]bool{}
	for _, o := range opts.Only {
		want[strings.ToLower(strings.TrimSpace(o))] = true
	}
	var out []Measurer
	for _, m := range all {
		if want[m.Gate()] || want[strings.ToLower(m.Label())] {
			out = append(out, m)
		}
	}
	return out
}

// UnknownGates returns the names in opts.Only that select no gate. A caller
// refuses them before measuring: an unmatched name used to measure nothing,
// and a snapshot with fewer gates than asked for looked like a complete one.
func UnknownGates(opts MeasureOptions) []string {
	all := ErrorFreeMeasurers(MeasureOptions{Extra: opts.Extra})
	var out []string
	for _, o := range opts.Only {
		name := strings.ToLower(strings.TrimSpace(o))
		if name == "" {
			continue
		}
		found := false
		for _, m := range all {
			if m.Gate() == name || strings.ToLower(m.Label()) == name {
				found = true
				break
			}
		}
		if !found {
			out = append(out, strings.TrimSpace(o))
		}
	}
	return out
}

// MeasureErrorFree measures the gates in order into one snapshot. The gates
// run one after another: they share the build cache and the machine.
func MeasureErrorFree(ctx context.Context, root string, opts MeasureOptions) Snapshot {
	snap := newSnapshot(ctx, root)
	for _, m := range ErrorFreeMeasurers(opts) {
		meas := m.Measure(ctx, root)
		meas.Gate, meas.Label = m.Gate(), m.Label()
		meas.Count = len(meas.Findings)
		if meas.Findings == nil {
			meas.Findings = []lintreport.Finding{}
		}
		snap.Gates = append(snap.Gates, meas)
	}
	return snap
}

// commandGate is a gate that runs a program and reads its report.
type commandGate struct {
	gate, label, binary string
	args, packages      []string
	// scoped: packages is a narrowing of the whole module, recorded on the
	// Measurement so a comparison restricts the baseline to the same scope.
	scoped bool
	// buildOutput: go build writes nothing to the tree (-o into a temp
	// directory that is removed), whatever the package list matches.
	buildOutput bool
	read        func(stdout, stderr, dir string) ([]lintreport.Finding, string, error)
}

func (g commandGate) Gate() string  { return g.gate }
func (g commandGate) Label() string { return g.label }

func (g commandGate) Measure(ctx context.Context, root string) Measurement {
	m := Measurement{Packages: append([]string(nil), g.packages...)}
	if g.scoped {
		m.Scope = append([]string(nil), g.packages...)
		if len(g.packages) == 0 {
			m.Scope = []string{NoPackageScope}
			m.Packages = []string{NoPackageScope}
			m.Status, m.Reason = MeasurePass, "scoped to no package: nothing to measure"
			return m
		}
	}
	if _, err := lookPath(g.binary); err != nil {
		m.Status = MeasureUnavailable
		m.Reason = fmt.Sprintf("%s is not installed (not found on PATH); install it to measure %s", g.binary, g.label)
		return m
	}
	args := append([]string(nil), g.args...)
	plain := append(append([]string(nil), args...), g.packages...)
	if g.buildOutput {
		tmp, err := os.MkdirTemp("", "nerd-gates-build-*")
		if err != nil {
			m.Status, m.Reason = MeasureUnverified, "create the build's temp output: "+err.Error()
			return m
		}
		defer os.RemoveAll(tmp)
		args = append(args, "-o", tmp)
	}
	args = append(args, g.packages...)
	start := time.Now()
	stdout, stderr, err := g.run(ctx, root, args, &m)
	if g.buildOutput && err != nil && strings.Contains(stderr.String(), "no main packages to build") {
		// Nothing is linkable, so -o refuses; a plain build of such packages
		// compiles and discards everything, writing nothing either.
		stdout, stderr, err = g.run(ctx, root, plain, &m)
	}
	m.Duration = time.Since(start)
	var exitErr *exec.ExitError
	code := 0
	switch {
	case ctx.Err() != nil:
		m.Status, m.Reason = MeasureUnverified, "stopped: "+ctx.Err().Error()
		return m
	case err == nil:
	case errors.As(err, &exitErr) && exitErr.Exited():
		code = exitErr.ExitCode()
	default:
		m.Status, m.Reason = MeasureUnverified, err.Error()
		m.Output = strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		return m
	}
	m.ExitCode = &code
	findings, rest, rerr := g.read(stdout.String(), stderr.String(), root)
	if rerr != nil {
		m.Status = MeasureUnverified
		m.Reason = fmt.Sprintf("%s ran (exit %d) but its report could not be read: %v", g.binary, code, rerr)
		m.Output = strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		return m
	}
	m.Findings = findings
	switch {
	case len(findings) > 0:
		m.Status = MeasureFail
		m.Output = rest
	case code == 0:
		m.Status = MeasurePass
	default:
		m.Status = MeasureFail
		m.Output = rest
		m.Unenumerated = true
		m.Reason = fmt.Sprintf("%s exited %d and named no finding", g.binary, code)
	}
	return m
}

// run starts the gate's program with args in root, under the workspace build
// env, and records its argv on m.
func (g commandGate) run(ctx context.Context, root string, args []string, m *Measurement) (stdout, stderr *bytes.Buffer, err error) {
	var cmd *exec.Cmd
	if g.binary == "go" {
		env, goArgs := build.GoInvocation(root, root, args)
		cmd = exec.CommandContext(ctx, "go", goArgs...)
		cmd.Env = env
		m.Argv = append([]string{"go"}, goArgs...)
	} else {
		cmd = exec.CommandContext(ctx, g.binary, args...)
		cmd.Env = build.GoToolEnv(root, root)
		m.Argv = append([]string{g.binary}, args...)
	}
	cmd.Dir = root
	stdout, stderr = &bytes.Buffer{}, &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	return stdout, stderr, cmd.Run()
}

func readCompile(stdout, stderr, dir string) ([]lintreport.Finding, string, error) {
	text := stdout + stderr
	f := lintreport.CompileFindings(text, dir)
	lintreport.Sort(f)
	return f, strings.TrimSpace(text), nil
}

func readVet(stdout, stderr, dir string) ([]lintreport.Finding, string, error) {
	return lintreport.VetFindings(stdout+stderr, dir)
}

func readStaticcheck(stdout, stderr, dir string) ([]lintreport.Finding, string, error) {
	f, err := lintreport.StaticcheckFindings(stdout, dir)
	return f, strings.TrimSpace(stderr), err
}

func readGolangci(stdout, stderr, dir string) ([]lintreport.Finding, string, error) {
	f, rest, err := lintreport.GolangciFindings(stdout, dir)
	return f, strings.TrimSpace(rest + "\n" + stderr), err
}

// sortedUnique sorts findings and drops exact duplicates.
func sortedUnique(f []lintreport.Finding) []lintreport.Finding {
	lintreport.Sort(f)
	out := f[:0]
	for i, x := range f {
		if i > 0 && x == f[i-1] {
			continue
		}
		out = append(out, x)
	}
	return out
}

// FindingKey is a finding's identity across snapshots: its file, check and
// message. The line is left out, so an edit above a finding does not turn it
// into a new one.
func FindingKey(f lintreport.Finding) string {
	return f.File + " | " + f.Check + " | " + f.Message
}
