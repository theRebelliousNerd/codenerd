package session

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Adversarial critic review.
//
// The build gate (build_verify.go) proves the edits compile and the test gate
// (test_verify.go) proves the edits were executed. Neither asks whether the
// edits are right. A turn can write code that builds, passes its own narrow
// test, and still ships a logic error, a security gap, or a silent contract
// violation — which is the same false success the first two gates exist to
// prevent, one level up.
//
// The repair rounds close that gap mechanically: the compiler and the test
// runner get a vote. The missing piece is a reviewer that can say "this
// compiles and passes but is still wrong" before the turn is reported as
// complete. An LLM reviewer is the cheapest way to get that signal, but a
// reviewer that is rewarded for finding something will invent something —
// hallucinating defects in sound code is more dangerous than missing a real
// one, because it trains the system to distrust its own signal and then to
// disable it.
//
// This file supplies the pure, deterministic half of that reviewer: prompt
// construction, response parsing, and triage. No I/O, no LLM calls — only
// string transforms that are trivial to test and impossible to flake. The
// impure half (calling the model) lives elsewhere; this is the part that
// must not be allowed to drift.

// CriticFinding is a single defect reported by the adversarial reviewer.
type CriticFinding struct {
	// File is the file path reported in the finding (as written, e.g.
	// "internal/foo/bar.go").
	File string

	// Line is the 1-indexed line number reported in the finding.
	Line int

	// Severity is the normalized severity ("high", "medium", or "low").
	Severity string

	// Claim is the reviewer's description of the defect.
	Claim string
}

// criticFindingRe matches the exact line format the reviewer is instructed to
// emit: 'FINDING file.go:123 severity: claim text' one per line. The severity
// token is validated case-insensitively after the match; anything else is
// ignored so the parser is not brittle to the reviewer's chatter.
var criticFindingRe = regexp.MustCompile(`^FINDING\s+(\S+):(\d+)\s+(\w+):\s*(.+)$`)

// buildCriticPrompt builds an adversarial review prompt that embeds each file
// path and its contents in fenced code blocks, instructs the reviewer to find
// real defects and to output findings in the exact line format
// 'FINDING file.go:123 severity: claim text' one per line, and to output the
// single line 'NO FINDINGS' when the code is sound.
//
// The prompt explicitly states that inventing a finding to appear useful is
// worse than finding nothing — without that, a reviewer rewarded for activity
// will hallucinate defects in sound code, which is the failure mode this
// gate exists to prevent.
func buildCriticPrompt(writtenFiles map[string]string, uncoveredSummary string) string {
	var b strings.Builder

	b.WriteString("You are an adversarial code reviewer. Review the following files for real, verifiable defects only.\n\n")

	// Embed each file path and its contents in a fenced code block. Sorted for
	// determinism: map iteration is random and a prompt that shuffles every
	// call is uncacheable and untestable.
	if len(writtenFiles) > 0 {
		keys := make([]string, 0, len(writtenFiles))
		for k := range writtenFiles {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, path := range keys {
			content := writtenFiles[path]
			b.WriteString("File: " + path + "\n")
			b.WriteString("```\n")
			b.WriteString(content)
			// Ensure the closing fence starts on its own line even when the
			// file content does not end with a newline.
			if !strings.HasSuffix(content, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("```\n\n")
		}
	}

	if s := strings.TrimSpace(uncoveredSummary); s != "" {
		b.WriteString("Uncovered code summary:\n")
		b.WriteString(s)
		b.WriteString("\n\n")
	}

	b.WriteString("Instructions:\n")
	b.WriteString("- Find real defects only: logic errors, correctness bugs, security issues, data races, and contract violations that are actually present in the code above.\n")
	b.WriteString("- Output findings in the exact line format 'FINDING file.go:123 severity: claim text' one per line.\n")
	b.WriteString("- Severity must be one of high, medium, or low.\n")
	b.WriteString("- When the code is sound, output the single line 'NO FINDINGS' and nothing else.\n")
	b.WriteString("- Inventing a finding to appear useful is worse than finding nothing. Only report defects you can point to in the code above; if there are none, output 'NO FINDINGS'.\n")

	return b.String()
}

// parseCriticFindings parses the reviewer's response into findings.
//
// It accepts severity values high, medium and low case-insensitively, ignores
// any non-matching lines, and returns nil when the response contains
// NO FINDINGS. The nil return for NO FINDINGS is intentional: it
// distinguishes "the reviewer said the code is sound" from "the reviewer
// produced no parseable output", and callers treat the former as a clean
// pass without triage.
//
// Lines that do not match the exact FINDING format are silently skipped so
// the reviewer's preamble or reasoning does not become a defect.
func parseCriticFindings(response string) []CriticFinding {
	// NO FINDINGS is the reviewer's explicit "sound code" signal. If it
	// appears on any line (trimmed), the whole response is treated as no
	// findings — even if other lines look like findings — because the
	// alternative is to triage a contradictory response.
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "NO FINDINGS" {
			return nil
		}
	}

	var out []CriticFinding
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		m := criticFindingRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		file := m[1]
		lineStr := m[2]
		sevRaw := m[3]
		claim := strings.TrimSpace(m[4])

		sev := strings.ToLower(strings.TrimSpace(sevRaw))
		if sev != "high" && sev != "medium" && sev != "low" {
			continue
		}
		n, err := strconv.Atoi(lineStr)
		if err != nil {
			continue
		}
		out = append(out, CriticFinding{
			File:     file,
			Line:     n,
			Severity: sev,
			Claim:    claim,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// findingsWorthUplift returns only high and medium severity findings.
//
// Low-severity findings are noise for the uplift gate: they describe style
// nits or minor suggestions that do not justify a repair round. The filter
// is case-insensitive to match the parser's normalization.
func findingsWorthUplift(findings []CriticFinding) []CriticFinding {
	var out []CriticFinding
	for _, f := range findings {
		sev := strings.ToLower(strings.TrimSpace(f.Severity))
		if sev == "high" || sev == "medium" {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// criticMaxFileBytes bounds how much of a written file is shown to the
// reviewer. A turn that rewrites a very large file would otherwise blow the
// context budget on a secondary signal; truncation costs some review quality,
// an over-budget request costs the whole review.
const criticMaxFileBytes = 24000

// criticMaxFiles bounds how many files one review covers, newest-written
// first. Reviewing thirty files in one call produces a shallow pass over all
// of them rather than a useful pass over any.
const criticMaxFiles = 6

// readWrittenFilesForReview loads the turn's written Go files for the critic,
// skipping test files, anything unreadable, and anything past the caps.
//
// Test files are excluded on purpose: the critic's job is to find defects in
// the code, and including the tests invites it to review the tests instead —
// which is both lower value and the easiest place to produce plausible
// nitpicks.
func readWrittenFilesForReview(workspace string, writtenPaths []string) map[string]string {
	out := make(map[string]string)
	for _, rel := range writtenPaths {
		if len(out) >= criticMaxFiles {
			break
		}
		trimmed := strings.TrimSpace(rel)
		lower := strings.ToLower(trimmed)
		if !strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, "_test.go") {
			continue
		}
		abs := trimmed
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(workspace, filepath.FromSlash(NormalizeCoverPath(trimmed)))
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			// A file we cannot read is not a file we can review. Skipping is
			// correct; failing the turn over it is not.
			continue
		}
		content := string(data)
		if len(content) > criticMaxFileBytes {
			content = content[:criticMaxFileBytes] + "\n// ... (truncated for review)"
		}
		out[trimmed] = content
	}
	return out
}

// formatUpliftPrompt turns confirmed findings into the turn handed back to the
// model.
//
// It requires the model to either fix each finding or explain why the finding
// is wrong. That second option is not politeness — the reviewer is another
// fallible model, and forcing a "fix" for a finding that is mistaken makes the
// code worse than leaving it alone.
func formatUpliftPrompt(findings []CriticFinding) string {
	var b strings.Builder
	b.WriteString("An adversarial review of the code you just wrote reported the following:\n\n")
	for _, f := range findings {
		b.WriteString(fmt.Sprintf("  - %s:%d [%s] %s\n", f.File, f.Line, f.Severity, f.Claim))
	}
	b.WriteString("\nFor each item: either fix it with the edit tools, or state plainly that the " +
		"finding is wrong and why. Do not make a change you believe is incorrect just to " +
		"clear an item — the reviewer is fallible too, and an unnecessary edit to satisfy " +
		"a mistaken finding leaves the code worse than ignoring it.\n\n" +
		"Do not restate the findings back to me and do not report success. The build and " +
		"tests will be checked again after you finish.")
	return b.String()
}
