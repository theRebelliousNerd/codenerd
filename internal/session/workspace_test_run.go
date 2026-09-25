package session

import (
	"context"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/gates"
	"codenerd/internal/logging"
	"codenerd/internal/tools"
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

// workspaceTestRun runs the workspace's own test gates over what this turn
// wrote -- nerd.md's, else what pyproject.toml, package.json or Cargo.toml
// offer (internal/gates) -- and records the run as the turn's test run, the
// /test_run gate's evidence. It reports whether the workspace had a runnable
// test gate for the written files.
//
// Until this, a write the Go gates do not check was settled by whatever test
// process the model chose to start after it: any passing test counted,
// including one that never loaded the file. When the workspace says how it
// tests the file's language, the executor runs that, over the directories
// the turn wrote. Go writes keep their own gates; a workspace with no test gate
// for the language, or whose toolchain is missing, leaves the run to the model
// as before -- and no run leaves the turn unverified, never passed.
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
	var run []gates.Gate
	for _, g := range set.Gates {
		if g.Kind == gates.Test && g.Language != "go" && (g.Language == "" || langs[g.Language]) {
			run = append(run, g)
		}
	}
	if len(run) == 0 {
		return false, ""
	}

	// No wall clock unless a budget is set (verify_outcome.go): zero is
	// unbounded, and WithTimeout(0) would be a context already expired.
	if testVerifyTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, testVerifyTimeout)
		defer cancel()
	}
	var outputs []string
	verdict := tools.TestRun{ExitCode: 0}
	for _, g := range run {
		nodes := []string{""}
		if g.Scope == gates.ScopeNode {
			nodes = dirs
		}
		for _, node := range nodes {
			res := gates.Run(ctx, ws, g, node)
			outputs = append(outputs, "$ "+strings.Join(res.Argv, " ")+"\n"+res.Output)
			switch {
			case res.Unverified():
				// A gate that produced no verdict decides nothing; the turn is
				// left to the model's own run, as with no gate at all.
				logging.Get(logging.CategorySession).Warn("workspace test gate %s gave no verdict: %v", g.ID, res.Err)
				return false, ""
			case !res.Passed && verdict.ExitCode == 0:
				verdict = tools.TestRun{Argv: res.Argv, ExitCode: failingExit(res.ExitCode)}
			case verdict.ExitCode == 0:
				verdict.Argv = res.Argv
			}
		}
	}
	result.TestRunSinceLastWrite = &verdict
	return true, strings.Join(outputs, "\n")
}

// failingExit is a failed gate's exit code as a test run's: a gate may treat
// a non-zero exit as a pass (pytest's 5, no tests collected), so a failure is
// whatever the gate said failed, and never 0.
func failingExit(code int) int {
	if code == 0 {
		return -1
	}
	return code
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
