package session

import (
	"context"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/gates"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// Languages by the extension of a written file, as internal/gates names them.
// A file in none of them (a policy file, a config, a script) is checked by the
// workspace's language-free gates only.
var writtenLanguage = map[string]string{
	".py": "python",
	".rs": "rust",
	".js": "js/ts", ".jsx": "js/ts", ".ts": "js/ts", ".tsx": "js/ts",
	".mjs": "js/ts", ".cjs": "js/ts", ".mts": "js/ts", ".cts": "js/ts",
}

// workspaceTestRun plans the workspace's own test checks over what this turn
// wrote -- nerd.md's, else what pyproject.toml, package.json or Cargo.toml
// offer (internal/gates) -- runs them, and records every outcome as the turn's
// test plan, which the /test_run gate is derived from. It reports whether the
// workspace declared a test check for the written files.
//
// Until this, a write the Go gates do not check was settled by whatever test
// process the model chose to start after it: any passing test counted,
// including one that never loaded the file. When the workspace says how it
// tests the file's language, the executor owes that, over the directories the
// turn wrote. Go writes keep their own gates; a workspace with no test gate
// for the language leaves the run to the model as before -- and no run leaves
// the turn unverified, never passed.
//
// A check that cannot run is still owed and is recorded as unavailable; one
// that fails stays failed whatever a later check does; one that collects no
// test is recorded as that (workspace_test_plan.go). Nothing here ends the
// round early: every outcome reaches the kernel, and the policy decides.
func (e *Executor) workspaceTestRun(ctx context.Context, result *ExecutionResult) (bool, string) {
	ws := e.workspaceForVerification()
	if ws == "" || result == nil {
		return false, ""
	}
	langs, dirs := writtenNonGo(ws, result.WrittenPaths)
	if len(dirs) == 0 {
		return false, ""
	}
	set, err := gates.Detect(ws)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("workspace test gates unavailable: %v", err)
		return false, ""
	}
	plan := buildTestPlan(set, langs, dirs)
	if len(plan.checks) == 0 {
		return false, ""
	}
	return true, e.runTestPlan(ctx, ws, result, plan)
}

// runTestPlan publishes the plan's obligations, measures each and records its
// outcome. A canceled context leaves the rest indeterminate, never skipped.
func (e *Executor) runTestPlan(ctx context.Context, ws string, result *ExecutionResult, plan testPlan) string {
	turn := result.turnAtom()
	for _, c := range plan.checks {
		if e.kernel == nil {
			break
		}
		e.assertTurnFact(types.Fact{Predicate: "turn_test_obligation", Args: []any{turn, types.MangleString(c.ID)}})
	}

	// No wall clock unless a budget is set (verify_outcome.go): zero is
	// unbounded, and WithTimeout(0) would be a context already expired.
	if testVerifyTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, testVerifyTimeout)
		defer cancel()
	}
	var outputs []string
	for _, c := range plan.checks {
		run := e.runPlannedCheck(ctx, ws, result, c)
		outputs = append(outputs, "$ "+strings.Join(run.Argv, " ")+"\n"+run.text())
		e.noteObligation(result, run)
	}
	return strings.Join(outputs, "\n")
}

// plannedRun is one planned check's measured outcome and the output behind it.
type plannedRun struct {
	obligationRun
	output string
}

func (r plannedRun) text() string {
	switch {
	case r.output != "":
		return r.output
	case r.Reused:
		return "(satisfied by the identical test process this turn already ran)"
	case r.Reason != "":
		return "(not run: " + r.Reason + ")"
	}
	return ""
}

// runPlannedCheck measures one obligation: an uncovered language is recorded
// as such, a check that cannot run as unavailable, an identical process the
// turn already ran is reused, and anything else is executed.
func (e *Executor) runPlannedCheck(ctx context.Context, ws string, result *ExecutionResult, c plannedCheck) plannedRun {
	out := plannedRun{obligationRun: obligationRun{ID: c.ID, Argv: c.Argv}}
	switch {
	case c.Language != "":
		out.Outcome = outcomeUncovered
	case c.Unavailable != "":
		out.Outcome, out.Reason = outcomeUnavailable, c.Unavailable
	case ctx.Err() != nil:
		out.Outcome, out.Reason = outcomeIndeterminate, "stopped: "+ctx.Err().Error()
	default:
		if r, ok := matchingReceipt(result, c.Argv); ok {
			out.Outcome, out.Exit, out.Reused = classifyExit(c.Gate, r.Run.ExitCode), r.Run.ExitCode, true
			return out
		}
		res := gates.Run(ctx, ws, c.Gate, c.Node)
		out.Argv, out.Exit, out.output = res.Argv, res.ExitCode, res.Output
		out.Outcome, out.Reason = classifyRun(ctx, c.Gate, res)
		if res.Unverified() {
			logging.Get(logging.CategorySession).Warn("workspace test gate %s gave no verdict: %v", c.Gate.ID, res.Err)
		}
	}
	return out
}

// noteObligation records an obligation's outcome on the turn's clock and in
// the host's plan record.
func (e *Executor) noteObligation(result *ExecutionResult, run plannedRun) {
	result.gateSeq++
	if e.kernel != nil {
		e.assertTurnFact(types.Fact{
			Predicate: "turn_test_obligation_run",
			Args:      []any{result.turnAtom(), types.MangleString(run.ID), int64(result.gateSeq), types.MangleAtom(string(run.Outcome))},
		})
	}
	for i := range result.testPlan {
		if result.testPlan[i].ID == run.ID {
			result.testPlan[i] = run.obligationRun
			return
		}
	}
	result.testPlan = append(result.testPlan, run.obligationRun)
}

// writtenNonGo returns the languages and the workspace-relative directories of
// the written files the Go gates do not check, documentation excluded.
func writtenNonGo(ws string, written []string) (map[string]bool, []string) {
	langs := map[string]bool{}
	dirSet := map[string]bool{}
	for _, p := range written {
		ext := strings.ToLower(filepath.Ext(p))
		switch ext {
		case ".go", ".md", ".markdown", ".rst", ".adoc":
			continue
		}
		if lang, ok := writtenLanguage[ext]; ok {
			langs[lang] = true
		}
		rel := p
		if filepath.IsAbs(p) {
			if r, err := filepath.Rel(ws, p); err == nil && !strings.HasPrefix(r, "..") {
				rel = r
			}
		}
		dirSet[path.Dir(filepath.ToSlash(rel))] = true
	}
	dirs := make([]string, 0, len(dirSet))
	for d := range dirSet {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return langs, dirs
}
