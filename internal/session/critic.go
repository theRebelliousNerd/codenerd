package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"codenerd/internal/observation"
	"codenerd/internal/tools/codedom"

	"github.com/sergi/go-diff/diffmatchpatch"
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

// buildCriticPrompt builds the critic's user message: the change to review
// and nothing else. It carries the request the change was made for, each file
// path and its contents in a fenced code block, the lines this turn removed
// from each file (removals, numbered as they were before the edit), and the
// uncovered and static-analysis summary.
//
// What the reviewer does with them -- real defects only, the 'FINDING
// file.go:123 severity: claim text' / 'NO FINDINGS' reply contract, the
// comment check, accounting for every removal and every uncovered block, and
// that inventing a finding is worse than finding nothing -- is its role prompt
// (atoms/system/build_critic.yaml), compiled by Executor.criticSystemPrompt.
// Each section's heading here is the name those instructions refer to it by.
func buildCriticPrompt(writtenFiles map[string]string, removals map[string]string, uncoveredSummary, request string) string {
	var b strings.Builder

	// The request is what the change is for. Without it the reviewer can only
	// judge the code against itself, and a change that is sound but does not
	// do what was asked -- or does more than was asked -- passes review.
	if r := strings.TrimSpace(request); r != "" {
		if len(r) > criticMaxFileBytes {
			r = r[:criticMaxFileBytes] + "\n... (request truncated for review)"
		}
		b.WriteString("The change was made for this request:\n```\n")
		b.WriteString(r)
		b.WriteString("\n```\n\n")
	}

	// Embed each file path and its contents in a fenced code block. Sorted for
	// determinism: map iteration is random and a prompt that shuffles every
	// call is uncacheable and untestable.
	keys := make([]string, 0, len(writtenFiles))
	for k := range writtenFiles {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, path := range keys {
		content := writtenFiles[path]
		b.WriteString("File: ")
		b.WriteString(path)
		b.WriteString("\n```\n")
		b.WriteString(content)
		// Ensure the closing fence starts on its own line even when the
		// file content does not end with a newline.
		if !strings.HasSuffix(content, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
		if r := removals[path]; strings.TrimSpace(r) != "" {
			b.WriteString("Lines this turn removed from ")
			b.WriteString(path)
			b.WriteString(" (numbered as they were before the edit):\n```\n")
			b.WriteString(r)
			if !strings.HasSuffix(r, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("```\n\n")
		}
	}

	// The uncovered and static-analysis sections are evidence, not decoration:
	// the role prompt tells the reviewer to account for them, and to report
	// uncovered logic worth testing as a finding.
	if s := strings.TrimSpace(uncoveredSummary); s != "" {
		b.WriteString("Uncovered code summary:\n")
		b.WriteString(s)
		b.WriteString("\n")
	}

	return b.String()
}

// turnRemovals lists the lines present in before and absent from after,
// each prefixed with its line number in before ("%5d| %s"), in order,
// with a line "..." between non-adjacent runs. Empty when nothing was removed.
func turnRemovals(before, after string) string {
	dmp := diffmatchpatch.New()
	a, b, lineArray := dmp.DiffLinesToChars(before, after)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	splitLines := func(s string) []string {
		if s == "" {
			return nil
		}
		lines := strings.Split(s, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		return lines
	}

	type removal struct {
		num  int
		text string
	}
	var removed []removal
	line := 1
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			line += len(splitLines(d.Text))
		case diffmatchpatch.DiffDelete:
			for _, l := range splitLines(d.Text) {
				removed = append(removed, removal{num: line, text: l})
				line++
			}
		case diffmatchpatch.DiffInsert:
			// Inserts exist only in the after text; the before-text line counter does not advance.
		}
	}
	if len(removed) == 0 {
		return ""
	}
	parts := make([]string, 0, len(removed)+2)
	for i, r := range removed {
		if i > 0 && r.num != removed[i-1].num+1 {
			parts = append(parts, "...")
		}
		parts = append(parts, fmt.Sprintf("%5d| %s", r.num, r.text))
	}
	return strings.Join(parts, "\n")
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

// criticMaxFileBytes bounds how much of a written file is shown to the
// reviewer. A turn that rewrites a very large file would otherwise blow the
// context budget on a secondary signal; truncation costs some review quality,
// an over-budget request costs the whole review.
const criticMaxFileBytes = 24000

// criticMaxFiles bounds how many files one review covers, newest-written
// first (after any files the reviewer is assigned, readWrittenFilesForReview).
// Reviewing thirty files in one call produces a shallow pass over all of them
// rather than a useful pass over any.
const criticMaxFiles = 6

// reviewableSources are the extensions the critic reviews.
var reviewableSources = map[string]bool{
	".go": true, ".mg": true, ".py": true, ".ts": true, ".tsx": true, ".js": true,
	".jsx": true, ".mjs": true, ".cjs": true, ".rs": true, ".java": true, ".kt": true,
	".rb": true, ".php": true, ".cs": true, ".c": true, ".h": true, ".cc": true,
	".cpp": true, ".hpp": true, ".swift": true, ".scala": true,
}

// testDirs hold tests by convention in the languages above.
var testDirs = map[string]bool{"tests": true, "__tests__": true, "spec": true, "testdata": true}

// reviewableSource reports whether path is source the critic reviews: a known
// source extension, and not a test by its language's naming convention.
func reviewableSource(path string) bool {
	p := strings.ToLower(filepath.ToSlash(path))
	if !reviewableSources[filepath.Ext(p)] {
		return false
	}
	base := filepath.Base(p)
	switch {
	case strings.HasSuffix(base, "_test.go"),
		strings.HasPrefix(base, "test_"), strings.HasSuffix(base, "_test.py"), base == "conftest.py",
		strings.Contains(base, ".test."), strings.Contains(base, ".spec."),
		strings.HasSuffix(base, "test.java"), strings.HasSuffix(base, "tests.java"), strings.HasSuffix(base, "test.kt"):
		return false
	}
	for _, seg := range strings.Split(filepath.Dir(p), "/") {
		if testDirs[seg] {
			return false
		}
	}
	return true
}

// reviewOmission is a written file a review was not shown, and why. A
// reviewer is told each one, so a file it was not handed is one it can read
// with its tools rather than one it never learns of.
type reviewOmission struct {
	Path   string
	Reason string
}

// readWrittenFilesForReview loads the turn's written source files for a
// review, skipping test files, anything unreadable, and anything past the
// cap, and returns what it left out of the review.
//
// assigned are the files this reviewer is responsible for (a shard agent's
// domain files, Need.Files): those the turn wrote come first, so the cap
// never pushes a reviewer's own subject out of its review. The rest follow
// newest-written first. Until 2026-10-29 every reviewer got the same first
// six files in first-seen order (the comment above criticMaxFiles said
// newest-first), and an expert whose domain file was the seventh write was
// never shown it, nor told it was missing.
//
// Every left-out file is reported: a written source past the cap, and an
// assigned file the review cannot show (a test, a language it does not
// read, a file it cannot read).
//
// Source means any language the critic can read, not only Go: until
// 2026-09-25 a Python or TypeScript turn got no review at all, and so none of
// the language-server grounding either.
//
// Test files are excluded on purpose: the critic's job is to find defects in
// the code, and including the tests invites it to review the tests instead —
// which is both lower value and the easiest place to produce plausible
// nitpicks.
func readWrittenFilesForReview(workspace string, writtenPaths, assigned []string) (map[string]string, []reviewOmission) {
	mine := make(map[string]bool, len(assigned))
	for _, p := range assigned {
		mine[canonicalizeWrittenPath(p, workspace)] = true
	}
	var order []string
	seen := make(map[string]bool, len(writtenPaths))
	add := func(rel string) {
		trimmed := strings.TrimSpace(rel)
		if trimmed == "" || seen[trimmed] {
			return
		}
		seen[trimmed] = true
		order = append(order, trimmed)
	}
	for _, rel := range writtenPaths {
		if mine[canonicalizeWrittenPath(rel, workspace)] {
			add(rel)
		}
	}
	for i := len(writtenPaths) - 1; i >= 0; i-- {
		add(writtenPaths[i])
	}

	out := make(map[string]string)
	var omitted []reviewOmission
	for _, trimmed := range order {
		assignedHere := mine[canonicalizeWrittenPath(trimmed, workspace)]
		if !reviewableSource(trimmed) {
			if assignedHere {
				omitted = append(omitted, reviewOmission{Path: trimmed, Reason: "a test or a language this review does not read"})
			}
			continue
		}
		if len(out) >= criticMaxFiles {
			omitted = append(omitted, reviewOmission{Path: trimmed, Reason: fmt.Sprintf("past the review's %d-file bound", criticMaxFiles)})
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
			if assignedHere {
				reason := "unreadable: " + err.Error()
				if errors.Is(err, fs.ErrNotExist) {
					reason = "deleted by this turn"
				}
				omitted = append(omitted, reviewOmission{Path: trimmed, Reason: reason})
			}
			continue
		}
		// Whole: the view (criticFileView) bounds what is shown, and it has
		// to see the whole file to find the changed regions in it.
		out[trimmed] = string(data)
	}
	return out, omitted
}

// renderReviewOmissions is the section that names what a review was not
// shown. A reviewer with tools (a shard agent's consultation) is told to read
// what it needs; one without (the build critic's single completion) is told
// its review does not cover them.
func renderReviewOmissions(omitted []reviewOmission, canRead bool) string {
	if len(omitted) == 0 {
		return ""
	}
	var b strings.Builder
	if canRead {
		b.WriteString("Written by this change but not shown here (read any your review needs with your tools):\n")
	} else {
		b.WriteString("Written by this change but not shown here, so not covered by this review:\n")
	}
	for _, o := range omitted {
		fmt.Fprintf(&b, "  - %s (%s)\n", o.Path, o.Reason)
	}
	return b.String()
}

// criticSpan is an inclusive, 1-indexed range of lines in a written file.
type criticSpan struct{ start, end int }

// changedSpans returns the ranges of after that this turn wrote, numbered as
// in after. A deletion leaves nothing in after, so it is anchored at the line
// that now stands where the removed lines were: the review looks there, and
// the removed lines themselves are in the removals section. An empty before is
// a new file, all of it written by this turn.
func changedSpans(before, after string) []criticSpan {
	total := len(splitReviewLines(after))
	if total == 0 {
		return nil
	}
	if before == "" {
		return []criticSpan{{1, total}}
	}
	dmp := diffmatchpatch.New()
	a, b, lineArray := dmp.DiffLinesToChars(before, after)
	diffs := dmp.DiffCharsToLines(dmp.DiffMain(a, b, false), lineArray)

	var spans []criticSpan
	line := 1 // the next line of after
	for _, d := range diffs {
		n := len(splitReviewLines(d.Text))
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			line += n
		case diffmatchpatch.DiffInsert:
			if n > 0 {
				spans = append(spans, criticSpan{line, line + n - 1})
				line += n
			}
		case diffmatchpatch.DiffDelete:
			at := min(max(line, 1), total)
			spans = append(spans, criticSpan{at, at})
		}
	}
	return mergeSpans(spans)
}

func splitReviewLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// mergeSpans sorts spans and joins the ones that overlap or touch.
func mergeSpans(spans []criticSpan) []criticSpan {
	if len(spans) == 0 {
		return nil
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	out := []criticSpan{spans[0]}
	for _, s := range spans[1:] {
		last := &out[len(out)-1]
		if s.start <= last.end+1 {
			last.end = max(last.end, s.end)
			continue
		}
		out = append(out, s)
	}
	return out
}

// reviewWindows widens each changed span to the code element around it and
// pads it, the way a file read is projected for an edit
// (observation.ProjectRead): a change inside a function is reviewed with its
// signature and its closing brace, and a change between elements with the
// declarations on either side.
func reviewWindows(path, content string, changed []criticSpan) []criticSpan {
	total := len(splitReviewLines(content))
	if total == 0 || len(changed) == 0 {
		return nil
	}
	pad := observation.DefaultReadLimits().PadLines
	elements := codedom.ElementsFromSource(path, content)
	innermost := func(line int) (codedom.CodeElement, bool) {
		var best codedom.CodeElement
		found := false
		for _, e := range elements {
			if e.StartLine <= line && line <= e.EndLine &&
				(!found || e.EndLine-e.StartLine < best.EndLine-best.StartLine) {
				best, found = e, true
			}
		}
		return best, found
	}
	windows := make([]criticSpan, 0, len(changed))
	for _, s := range changed {
		lo, hi := s.start, s.end
		if e, ok := innermost(s.start); ok {
			lo = min(lo, e.StartLine)
		}
		if e, ok := innermost(s.end); ok {
			hi = max(hi, e.EndLine)
		}
		windows = append(windows, criticSpan{max(1, lo-pad), min(total, hi+pad)})
	}
	return mergeSpans(windows)
}

// criticFileView renders a written file for the critic and returns the
// windows the review covers.
//
// A file within criticMaxFileBytes is shown whole, numbered: small enough to
// read in full, and the context helps the review. A larger one is shown as its
// changed windows only. Until 2026-09-26 every file was cut at its first
// criticMaxFileBytes bytes, so an edit past that point -- about line 700 of a
// Go file -- was reviewed by a critic that could not see it.
func criticFileView(path, content string, changed []criticSpan) (string, []criticSpan) {
	lines := splitReviewLines(content)
	windows := reviewWindows(path, content, changed)
	if len(content) <= criticMaxFileBytes {
		return numberReviewLines(lines, 1, len(lines)), windows
	}
	var b strings.Builder
	shown := 0
	for i, w := range windows {
		if b.Len() >= criticMaxFileBytes {
			// Reviewed windows end here: a finding in a region not shown is
			// one the critic could not have read.
			fmt.Fprintf(&b, "... %d more changed region(s) not shown\n", len(windows)-i)
			return b.String(), windows[:i]
		}
		if w.start > shown+1 {
			fmt.Fprintf(&b, "... (lines %d-%d unchanged by this turn, not shown)\n", shown+1, w.start-1)
		}
		b.WriteString(numberReviewLines(lines, w.start, w.end))
		shown = w.end
	}
	if shown < len(lines) {
		fmt.Fprintf(&b, "... (lines %d-%d unchanged by this turn, not shown)\n", shown+1, len(lines))
	}
	return b.String(), windows
}

// numberReviewLines renders lines[from..to] (1-indexed, inclusive) in the
// "%5d| text" form the removals section already uses.
func numberReviewLines(lines []string, from, to int) string {
	var b strings.Builder
	for n := from; n <= to && n <= len(lines); n++ {
		fmt.Fprintf(&b, "%5d| %s\n", n, lines[n-1])
	}
	return b.String()
}

// findingsOnChange keeps the findings that cite a line inside a reviewed
// window of their file, and returns the rest separately. A finding about code
// the turn did not touch asks the turn to fix what it did not write: R1-11's
// review reported 3 of its 4 findings in untouched code, and each one worth
// uplift costs a model round and invites an edit nobody asked for. A finding
// whose file is not one of the reviewed files is kept: its path may just be
// written differently, and dropping a real finding is the worse error.
func findingsOnChange(findings []CriticFinding, windows map[string][]criticSpan) (kept, dropped []CriticFinding) {
	for _, f := range findings {
		spans, ok := windowsFor(f.File, windows)
		if !ok || lineInSpans(f.Line, spans) {
			kept = append(kept, f)
			continue
		}
		dropped = append(dropped, f)
	}
	return kept, dropped
}

func windowsFor(file string, windows map[string][]criticSpan) ([]criticSpan, bool) {
	want := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(file)), "./")
	for path, spans := range windows {
		have := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
		if have == want || strings.HasSuffix(have, "/"+want) || strings.HasSuffix(want, "/"+have) {
			return spans, true
		}
	}
	return nil, false
}

func lineInSpans(line int, spans []criticSpan) bool {
	for _, s := range spans {
		if s.start <= line && line <= s.end {
			return true
		}
	}
	return false
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
