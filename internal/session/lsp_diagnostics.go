package session

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/logging"
)

// Static analysis via gopls, used to ground the adversarial reviewer.
//
// The build gate reports what the compiler rejects. That is a narrow set: Go's
// compiler is deliberately silent about correct-but-wrong code. gopls reports a
// different class — inefficient constructions, shadowed errors, unused results,
// suspicious conversions — none of which fail a build and several of which are
// real defects.
//
// Measured on this repo (2026-08-08, gopls v0.22.0, cold): `gopls check` on one
// file took 5.9s and reported a genuine finding in code codeNERD had just
// written and that had already passed both the build and test gates —
// "Inefficient string concatenation in call to WriteString" at critic.go:85.
//
// These diagnostics are handed to the critic rather than raised as their own
// round. Two reasons. A fourth full round per write turn is a real cost for a
// signal that is often stylistic. And an LLM reviewer given concrete tool
// output reviews better than one given only source: it has something to check
// against, which is precisely the grounding that makes the difference between
// review and opinion.

// goplsTimeout bounds the diagnostic run. gopls builds a package graph on first
// use; the ceiling is well above the 5.9s measured cold so a slow first call
// does not silently drop the signal.
const goplsTimeout = 90 * time.Second

// goplsMaxOutput caps the diagnostics fed into the critic prompt.
const goplsMaxOutput = 4000

// goplsMaxFiles bounds how many files are analysed in one call.
const goplsMaxFiles = 8

// goplsDiagnostics runs `gopls check` on the turn's written Go files and
// returns its findings as text, or "" when there is nothing to report.
//
// Absent gopls is not an error and not a finding — it is silence. This is an
// optional grounding signal, and a machine without gopls installed must behave
// exactly as it did before this existed.
func goplsDiagnostics(ctx context.Context, workspace string, writtenPaths []string) string {
	if strings.TrimSpace(workspace) == "" {
		return ""
	}

	bin, err := exec.LookPath("gopls")
	if err != nil {
		logging.SessionDebug("gopls not on PATH; skipping static diagnostics")
		return ""
	}

	var files []string
	for _, p := range writtenPaths {
		if len(files) >= goplsMaxFiles {
			break
		}
		t := strings.TrimSpace(p)
		if strings.HasSuffix(strings.ToLower(t), ".go") {
			files = append(files, NormalizeCoverPath(t))
		}
	}
	if len(files) == 0 {
		return ""
	}

	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), goplsTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, bin, append([]string{"check"}, files...)...)
	cmd.Dir = workspace
	cmd.Env = build.GetBuildEnv(nil, workspace)

	out, err := cmd.CombinedOutput()
	if runCtx.Err() != nil {
		logging.Get(logging.CategorySession).Warn(
			"gopls diagnostics timed out after %s; continuing without them", goplsTimeout)
		return ""
	}
	// `gopls check` exits non-zero when it has findings, so a non-nil err with
	// output is the normal reporting path, not a failure.
	text := strings.TrimSpace(string(out))
	if text == "" {
		if err != nil {
			logging.SessionDebug("gopls check failed with no output (%v); continuing", err)
		}
		return ""
	}
	if len(text) > goplsMaxOutput {
		text = text[:goplsMaxOutput] + "\n... (diagnostics truncated)"
	}
	return text
}

// execLookPathForTest exposes the PATH lookup so tests can skip cleanly when
// gopls is not installed, without duplicating the lookup logic.
func execLookPathForTest(name string) (string, error) { return exec.LookPath(name) }
