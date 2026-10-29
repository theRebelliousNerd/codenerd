package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/lintreport"
	"codenerd/internal/logging"
	"codenerd/internal/processutil"
	"codenerd/internal/tools"
)

// The static gates the model can run itself (run_lint). Until it existed no
// typed tool ran go vet, staticcheck or golangci-lint for the model: the
// forcing gate ran vet after the turn, and a model asked to keep vet clean had
// a free-form shell at best and, more often, asserted missing_tool_for(_,
// /go_vet) and sent autopoiesis to generate a vet tool from nothing.
//
// Each gate is run with its machine-readable output and read into
// lintreport.Finding values. The argv is the repository's, never the model's: the
// model chooses the gate, the packages and the working directory.

// lintGate is one static gate: the binary that must be installed, and how it
// is invoked and read.
type lintGate struct {
	name   string // the gate argument
	binary string
	// argv returns the arguments for packages, after the binary.
	argv func(packages []string) []string
	// read parses one run. rest is output that was neither a finding nor
	// a compile error; it is returned whole so nothing the gate said is lost.
	read func(stdout, stderr, dir string) (findings []lintreport.Finding, rest string, err error)
}

// lintGates are the gates of the ladder's error-free table (G2 vet, G4
// staticcheck, G5 golangci-lint), each invoked as that table measures it.
// golangci-lint runs with both report caps off: with them on it prints the
// first 50 issues per linter and 3 of each kind and hides the rest.
var lintGates = map[string]lintGate{
	"vet": {
		name: "vet", binary: "go",
		argv: func(p []string) []string { return append([]string{"vet", "-json"}, p...) },
		read: func(stdout, stderr, dir string) ([]lintreport.Finding, string, error) {
			// go vet writes its JSON, and its compile errors, to stderr.
			return lintreport.VetFindings(stdout+stderr, dir)
		},
	},
	"staticcheck": {
		name: "staticcheck", binary: "staticcheck",
		argv: func(p []string) []string { return append([]string{"-f", "json"}, p...) },
		read: func(stdout, stderr, dir string) ([]lintreport.Finding, string, error) {
			f, err := lintreport.StaticcheckFindings(stdout, dir)
			return f, strings.TrimSpace(stderr), err
		},
	},
	"golangci-lint": {
		name: "golangci-lint", binary: "golangci-lint",
		argv: func(p []string) []string {
			return append([]string{"run", "--output.json.path", "stdout", "--show-stats=false",
				"--max-issues-per-linter=0", "--max-same-issues=0"}, p...)
		},
		read: func(stdout, stderr, dir string) ([]lintreport.Finding, string, error) {
			f, rest, err := lintreport.GolangciFindings(stdout, dir)
			return f, strings.TrimSpace(rest + "\n" + stderr), err
		},
	},
}

// lintGateNames is the gate argument's vocabulary, in the order the schema
// lists it.
var lintGateNames = []string{"vet", "staticcheck", "golangci-lint"}

// LintStatus is a run_lint verdict.
type LintStatus string

const (
	// LintClean: the gate ran to completion and reported nothing.
	LintClean LintStatus = "clean"
	// LintFindings: the gate reported findings (compile errors included).
	LintFindings LintStatus = "findings"
	// LintFailed: the gate exited non-zero and named nothing to fix; its
	// output is in the result whole.
	LintFailed LintStatus = "failed"
	// LintUnavailable: the gate's binary is not installed. Nothing ran.
	LintUnavailable LintStatus = "unavailable"
	// LintNothingWritten: no packages were given and this turn wrote no Go
	// package. Nothing ran.
	LintNothingWritten LintStatus = "nothing_written"
)

// LintResult is what run_lint returns, as JSON.
type LintResult struct {
	Gate      string               `json:"gate"`
	Status    LintStatus           `json:"status"`
	Argv      []string             `json:"argv,omitempty"`
	Directory string               `json:"directory"`
	Scope     string               `json:"scope,omitempty"`
	ExitCode  *int                 `json:"exit_code,omitempty"`
	Findings  []lintreport.Finding `json:"findings"`
	// Output is what the gate printed that was neither a finding nor a
	// compile error, whole. A failed run's reason is here.
	Output string `json:"output,omitempty"`
	// Reason says why nothing ran (unavailable, nothing_written).
	Reason string `json:"reason,omitempty"`
}

// RunLintTool returns the tool that runs a static gate over Go packages and
// returns its findings.
func RunLintTool() *tools.Tool {
	return &tools.Tool{
		Name:        "run_lint",
		Description: "Run a Go static gate -- go vet, staticcheck or golangci-lint -- over packages and return its findings as file, line, column, check and message",
		Category:    tools.CategoryCode,
		AltCategories: []tools.ToolCategory{
			tools.CategoryReview, tools.CategoryTest,
		},
		Priority: 75,
		Execute:  executeRunLint,
		Timeout: func(args map[string]any) time.Duration {
			if seconds, err := verificationTimeoutSeconds(args, false); err == nil {
				return time.Duration(seconds) * time.Second
			}
			return 300 * time.Second
		},
		Schema: tools.ToolSchema{
			Required: []string{"gate"},
			Properties: map[string]tools.Property{
				"gate": {
					Type:        "string",
					Description: "The gate: vet (go vet), staticcheck, or golangci-lint (its default linters, no issue caps)",
					Enum:        lintGateEnum(),
				},
				"packages":    {Type: "array", Description: "Workspace-relative Go packages, e.g. ./internal/session. Omitted: the packages this turn has written (none written = nothing runs). The whole module is [\"./...\"]", Items: &tools.PropertyItems{Type: "string"}},
				"working_dir": {Type: "string", Description: "Project directory (default: the workspace root)"},
				"timeout_seconds": {
					Type:        "integer",
					Description: "Timeout in seconds (default: 300)",
					Default:     300,
				},
			},
		},
	}
}

func lintGateEnum() []any {
	out := make([]any, len(lintGateNames))
	for i, name := range lintGateNames {
		out[i] = name
	}
	return out
}

func executeRunLint(ctx context.Context, args map[string]any) (string, error) {
	name, _ := args["gate"].(string)
	gate, ok := lintGates[strings.TrimSpace(name)]
	if !ok {
		return "", fmt.Errorf("gate must be one of %s, got %q", strings.Join(lintGateNames, ", "), name)
	}
	dir, err := resolveWorkingDir(ctx, optionalWorkingDir(args))
	if err != nil {
		return "", err
	}
	packages, err := goPackagesArg(args)
	if err != nil {
		return "", err
	}
	scope := ""
	if len(packages) == 0 {
		packages = []string{"./..."}
		if written, scoped := writtenPackageScope(ctx, dir); scoped {
			if len(written) == 0 {
				return lintJSON(LintResult{Gate: gate.name, Status: LintNothingWritten, Directory: dir, Findings: []lintreport.Finding{},
					Reason: "nothing ran: this turn has written no Go package, and with no packages given the written packages are what is checked. " +
						"Pass packages, e.g. [\"./internal/session\"], or [\"./...\"] for the whole module."}), nil
			}
			packages, scope = written, writtenScopeNote
		}
	}
	if err := checkWorkspacePackages(ctx, dir, packages); err != nil {
		return "", err
	}
	if _, lookErr := execLookPath(gate.binary); lookErr != nil {
		// Not installed is an answer, not an error: the model reports the
		// gate as not run instead of retrying it. codeNERD never downloads
		// a tool to run it.
		return lintJSON(LintResult{Gate: gate.name, Status: LintUnavailable, Directory: dir, Findings: []lintreport.Finding{},
			Reason: fmt.Sprintf("%s is not installed (not found on PATH), so the %s gate did not run; install it to run this gate", gate.binary, gate.name)}), nil
	}
	seconds, err := verificationTimeoutSeconds(args, false)
	if err != nil {
		return "", err
	}

	// The environment the session's own gates compile under: GOFLAGS and
	// CGO_CFLAGS from the workspace's build config (codeNERD's own tree gets
	// -tags=sqlite_vec there), and an allowlist, not the process's keys.
	root, _ := tools.WorkspaceRoot(ctx)
	argv := append([]string{gate.binary}, gate.argv(packages)...)
	env := build.GoToolEnv(root, dir)
	if gate.binary == "go" {
		var goArgs []string
		env, goArgs = build.GoInvocation(root, dir, argv[1:])
		argv = append([]string{"go"}, goArgs...)
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()
	cmd := newCommand(runCtx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := processutil.Run(cmd)
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	result := LintResult{Gate: gate.name, Argv: argv, Directory: dir, Scope: scope, ExitCode: &code, Findings: []lintreport.Finding{}}
	if stopErr := verificationRunError(runCtx, runErr, code, gate.binary, seconds); stopErr != nil && code < 0 {
		result.Status, result.ExitCode = LintFailed, nil
		result.Output = strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		return lintJSON(result), stopErr
	}

	findings, rest, parseErr := gate.read(stdout.String(), stderr.String(), dir)
	if parseErr != nil {
		result.Status = LintFailed
		result.Output = strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		return lintJSON(result), fmt.Errorf("%s ran (exit %d) but its report could not be read: %w", gate.name, code, parseErr)
	}
	result.Findings, result.Output = findings, rest
	switch {
	case len(findings) > 0:
		result.Status = LintFindings
	case code == 0:
		result.Status = LintClean
	default:
		result.Status = LintFailed
		logging.Tools("run_lint: %s exited %d and named no finding", gate.name, code)
		return lintJSON(result), fmt.Errorf("%s exited %d and named no finding: %s", gate.name, code, firstLine(rest))
	}
	logging.Tools("run_lint completed: %s over %v: %s, %d finding(s)", gate.name, packages, result.Status, len(findings))
	return lintJSON(result), nil
}

func lintJSON(r LintResult) string {
	data, _ := json.Marshal(r)
	return string(data)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
