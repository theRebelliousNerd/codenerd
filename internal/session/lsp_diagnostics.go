package session

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/logging"
	"codenerd/internal/world/lsp"
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
	text := keepDiagnosticLines(string(out))
	if text == "" {
		if err != nil {
			logging.SessionDebug("gopls check failed with no usable output (%v); continuing", err)
		}
		return ""
	}
	return text
}

// diagnosticLineRe matches a real gopls diagnostic: a path, a line, a column,
// and a message. Everything gopls says about itself looks different.
var diagnosticLineRe = regexp.MustCompile(`^.+:\d+:\d+(-\d+)?: .+$`)

// keepDiagnosticLines strips gopls's operational chatter, keeping only lines
// that are actually diagnostics.
//
// This runs on CombinedOutput, so gopls's stderr is in the mix. Observed live
// (2026-08-08): the only "diagnostic" fed to the critic on a clean turn was
//
//	telemetry prompt failed: unable to determine user config dir: %AppData% is not defined
//
// which is gopls complaining that build.GetBuildEnv hands it a restricted
// environment without APPDATA. It says nothing about the code.
//
// Passing that to the reviewer under the heading "Static analysis reported"
// is worse than passing nothing. The whole reason to include tool output is
// that it is ground truth; presenting noise as ground truth invites exactly
// the invented findings the critic is built to avoid. The reviewer cannot tell
// the difference, so the filtering has to happen here.
func keepDiagnosticLines(raw string) string {
	var kept []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if diagnosticLineRe.MatchString(trimmed) {
			kept = append(kept, trimmed)
		}
	}
	return strings.Join(kept, "\n")
}

// Language servers for the files gopls does not check.
//
// gopls grounds the critic for Go. A Python or TypeScript turn had nothing:
// the workspace's own gates run its tests, but a type error on a path no test
// reaches, or a call with the wrong arity, fails no gate and is exactly what a
// language server reports. internal/world/lsp's Client speaks LSP to any
// server over stdio; this runs one per language the turn wrote, opens the
// written files, and hands the errors and warnings to the critic beside
// gopls's.
//
// The same rules as gopls: an absent server is silence, not a finding, and the
// run is bounded by goplsTimeout and goplsMaxFiles. A server that never
// publishes for a file costs the rest of the budget, not the turn.
type languageServer struct {
	lang   string   // Mangle atom the client is tagged with
	binary string   // looked up on PATH
	args   []string // stdio mode
	ids    map[string]string
}

var languageServers = []languageServer{
	{lang: "/python", binary: "pyright-langserver", args: []string{"--stdio"},
		ids: map[string]string{".py": "python"}},
	{lang: "/typescript", binary: "typescript-language-server", args: []string{"--stdio"},
		ids: map[string]string{".ts": "typescript", ".tsx": "typescriptreact", ".js": "javascript", ".jsx": "javascriptreact"}},
}

// startLanguageServer is lsp.StartServer; tests substitute an in-memory server.
var startLanguageServer = lsp.StartServer

// lspDiagnostics runs each language server whose files the turn wrote and
// returns their errors and warnings as "path:line: severity: message" lines,
// or "" when there is nothing to report.
func lspDiagnostics(ctx context.Context, workspace string, writtenPaths []string) string {
	if strings.TrimSpace(workspace) == "" {
		return ""
	}
	var out []string
	for _, ls := range languageServers {
		var files []string
		for _, p := range writtenPaths {
			if len(files) >= goplsMaxFiles {
				break
			}
			if _, ok := ls.ids[strings.ToLower(filepath.Ext(p))]; ok {
				files = append(files, p)
			}
		}
		if len(files) > 0 {
			out = append(out, serverDiagnostics(ctx, workspace, ls, files)...)
		}
	}
	return strings.Join(out, "\n")
}

func serverDiagnostics(ctx context.Context, workspace string, ls languageServer, files []string) []string {
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), goplsTimeout)
	defer cancel()
	client, err := startLanguageServer(runCtx, ls.lang, ls.binary, ls.args...)
	if err != nil {
		logging.SessionDebug("%s unavailable; skipping its diagnostics (%v)", ls.binary, err)
		return nil
	}
	defer func() { _ = client.Shutdown(context.WithoutCancel(ctx)) }()
	if err := client.Initialize(runCtx, workspace); err != nil {
		logging.SessionDebug("%s did not initialize; skipping its diagnostics (%v)", ls.binary, err)
		return nil
	}
	var lines []string
	for _, p := range files {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(workspace, p)
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		if err := client.DidOpen(abs, ls.ids[strings.ToLower(filepath.Ext(p))], string(data)); err != nil {
			break
		}
		diags, err := client.WaitForDiagnostics(runCtx, abs)
		if err != nil {
			logging.Get(logging.CategorySession).Warn("%s published nothing for %s within %s; continuing without it", ls.binary, p, goplsTimeout)
			break
		}
		rel := filepath.ToSlash(p)
		if r, err := filepath.Rel(workspace, abs); err == nil {
			rel = filepath.ToSlash(r)
		}
		for _, d := range diags {
			if word, ok := lsp.SeverityWord(d.Severity); ok {
				lines = append(lines, fmt.Sprintf("%s:%d: %s: %s", rel, d.Line, word, d.Message))
			}
		}
	}
	return lines
}
