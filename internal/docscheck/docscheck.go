package docscheck

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// This file ports the checks in scripts/r6_structcheck.py one by one. The
// comments cite the script construct each block mirrors so a future reader
// can diff the two; where Go cannot spell the identical thing (Python's
// Unicode \d and \W) the comment says what was chosen and why.

// Front-matter vocabularies: DOC_CLASS, IMPL and PLAN_STATUS in the script.
// These are the graded R6 contract, not tunables — they change only when the
// bar in Docs/journeys/09-architecture-doc-standard.md changes.
var (
	docClasses = map[string]bool{
		"north-star": true, "shipped": true, "shipped-with-future": true,
		"governance": true, "inventory": true, "deep-dive": true,
		"cross-cutting": true,
	}
	implStatuses = map[string]bool{
		"planned": true, "target-state": true, "partial": true,
		"shipped": true, "accepted-not-implemented": true,
		"not-applicable": true,
	}
	planStatuses = map[string]bool{"planned": true, "target-state": true}
)

// slots is SLOTS in the script: the files every package directory must hold.
var slots = []string{
	"README.md", "00-INDEX.md", "01-VISION.md", "02-CURRENT-STATE.md",
	"03-GAP-ANALYSIS.md", "04-PRINCIPLES-AND-CONSTRAINTS.md",
	"IMPLEMENTED_SPEC.md", "WIRING-AND-NOT-BUILT.md",
	"RISK-REGISTER-AND-DECISION-LOG.md", "OPEN-QUESTIONS.md", "TODO.md",
}

var (
	// VAGUE in the script. Python's \W is Unicode-aware; [^\pL\p{Nd}_]
	// (not a letter, decimal digit or underscore) is its RE2 spelling, and
	// (?i) folds case the same way for these ASCII keywords.
	vagueExitRe = regexp.MustCompile(`(?i)^[^\pL\p{Nd}_]*(?:tbd|todo|n/?a|none|improved?|robust|complete[d]?|done|works?)[^\pL\p{Nd}_]*$`)
	// The gap-ID search and the last-verified match. Python \d matches any
	// Unicode decimal digit; \p{Nd} is the exact RE2 equivalent.
	gapIDRe        = regexp.MustCompile(`GAP-[A-Z0-9]+-?\p{Nd}+`)
	lastVerifiedRe = regexp.MustCompile(`^\p{Nd}{4}-\p{Nd}{2}-\p{Nd}{2}$`)
	// The capability-spec match, re.match(r"0[5-9]-|[1-9]\d-", f): both
	// alternatives anchor at the name's start.
	capabilityRe = regexp.MustCompile(`^(?:0[5-9]-|[1-9]\p{Nd}-)`)
	// The ADR witness line, re.search(r"\*\*Witness:\*\*\s*(.+)", text).
	witnessLineRe = regexp.MustCompile(`\*\*Witness:\*\*\s*(.+)`)
)

// Checker grades Docs/architecture packages against the R6 bar. The
// workspace root comes from the caller — the script hardcodes
// C:/CodeProjects/codeNERD, which is exactly what a tracked command must
// not do. A Checker is not safe for concurrent use.
type Checker struct {
	root         string
	witnessCache map[string]bool
	goFiles      []string
	mgFiles      []string
	goListed     bool
	mgListed     bool
}

// NewChecker returns a Checker rooted at workspaceRoot.
func NewChecker(workspaceRoot string) *Checker {
	return &Checker{root: workspaceRoot, witnessCache: map[string]bool{}}
}

// ArchDir is the Docs/architecture directory under the workspace root.
func (c *Checker) ArchDir() string {
	return filepath.Join(c.root, "Docs", "architecture")
}

// archRel builds the workspace-relative, slash-separated path facts and
// JSON carry: Docs/architecture/<pkg>[/<rel>].
func archRel(pkg string, rel ...string) string {
	p := "Docs/architecture/" + pkg
	for _, r := range rel {
		p += "/" + r
	}
	return p
}

// ListPackages returns every package directory under Docs/architecture,
// sorted, as the script's no-args branch does.
func (c *Checker) ListPackages() ([]string, error) {
	entries, err := os.ReadDir(c.ArchDir())
	if err != nil {
		return nil, fmt.Errorf("list docs packages in %s: %w", c.ArchDir(), err)
	}
	var pkgs []string
	for _, e := range entries {
		// os.path.isdir follows symlinks; DirEntry.IsDir does not, so
		// stat through to the same answer the script gets.
		st, serr := os.Stat(filepath.Join(c.ArchDir(), e.Name()))
		if serr != nil || !st.IsDir() {
			continue
		}
		pkgs = append(pkgs, e.Name())
	}
	sort.Strings(pkgs)
	return pkgs, nil
}

// Check grades the named packages in order, or every package directory when
// pkgs is empty, as the script's main loop does.
func (c *Checker) Check(pkgs []string) ([]PackageReport, error) {
	if len(pkgs) == 0 {
		var err error
		pkgs, err = c.ListPackages()
		if err != nil {
			return nil, err
		}
	}
	reports := make([]PackageReport, 0, len(pkgs))
	for _, pkg := range pkgs {
		rep, err := c.CheckPackage(pkg)
		if err != nil {
			return nil, err
		}
		reports = append(reports, rep)
	}
	return reports, nil
}

// CheckPackage grades one package directory: every .md file's front-matter
// (plus the gap table and ADR witnesses where they apply), then the
// required slots, the adr/ directory, the capability spec, and the plan and
// shipped layers — in the script's order, so the human report matches line
// for line.
func (c *Checker) CheckPackage(pkg string) (PackageReport, error) {
	if pkg == "" || pkg == "." || pkg == ".." || strings.ContainsAny(pkg, `/\`) {
		return PackageReport{}, fmt.Errorf("invalid docs package %q", pkg)
	}
	dir := filepath.Join(c.ArchDir(), pkg)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return PackageReport{}, fmt.Errorf("unknown docs package %q: %s is not a directory", pkg, dir)
	}
	// The script walks with os.walk (files sorted per directory, directory
	// order left to the OS). Sorting every package-relative path up front
	// keeps the same within-directory order and makes the report
	// deterministic across machines.
	var rels []string
	if err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, rerr := filepath.Rel(dir, p)
		if rerr != nil {
			return rerr
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return PackageReport{}, fmt.Errorf("walk docs package %q: %w", pkg, err)
	}
	sort.Strings(rels)

	rep := PackageReport{Package: pkg}
	statuses := map[string]bool{}
	for _, rel := range rels {
		rep.Files++
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return PackageReport{}, fmt.Errorf("read %s: %w", archRel(pkg, rel), err)
		}
		// The script opens files in text mode, whose universal newlines
		// turn CRLF into LF before any check runs; normalize the same way
		// so a CRLF doc gets identical judgements.
		rep.Problems = append(rep.Problems, c.checkFile(pkg, rel, normalizeText(string(data)), statuses)...)
	}
	shape, err := c.checkShape(pkg, dir, statuses)
	if err != nil {
		return PackageReport{}, err
	}
	rep.Problems = append(rep.Problems, shape...)
	return rep, nil
}

// normalizeText mirrors Python text-mode reads for the checks' purposes.
func normalizeText(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// checkFile ports the per-file body of check_pkg: front-matter, then the gap
// table for 03-GAP-ANALYSIS.md, then the witness for adr/*.md. It records
// the file's implementation status the way the script's statuses.add does —
// including invalid values, which the layer rule then ignores the same way.
// (The script also collects doc classes into a set nothing reads; there is
// nothing to port there.)
func (c *Checker) checkFile(pkg, rel, text string, statuses map[string]bool) []Problem {
	file := archRel(pkg, rel)
	fm, ok := frontMatter(text)
	if !ok {
		return []Problem{{Package: pkg, File: file, Code: CodeMissingFrontMatter, Message: rel + ": no front-matter"}}
	}
	var probs []Problem
	dc, st := fm["doc-class"], fm["implementation-status"]
	if !docClasses[dc] {
		probs = append(probs, Problem{Package: pkg, File: file, Code: CodeBadDocClass, Message: rel + ": doc-class '" + dc + "'"})
	}
	if !implStatuses[st] {
		probs = append(probs, Problem{Package: pkg, File: file, Code: CodeBadImplementationStatus, Message: rel + ": implementation-status '" + st + "'"})
	}
	if !lastVerifiedRe.MatchString(fm["last-verified"]) {
		probs = append(probs, Problem{Package: pkg, File: file, Code: CodeBadLastVerified, Message: rel + ": last-verified missing/malformed"})
	}
	if fm["verified-against"] == "" {
		probs = append(probs, Problem{Package: pkg, File: file, Code: CodeMissingVerifiedAgainst, Message: rel + ": verified-against missing"})
	}
	statuses[st] = true
	if rel == "03-GAP-ANALYSIS.md" {
		probs = append(probs, c.checkGapTable(pkg, rel, text)...)
	}
	if strings.HasPrefix(rel, "adr/") {
		probs = append(probs, c.checkWitness(pkg, rel, text, st)...)
	}
	return probs
}

// checkShape ports the package-shape tail of check_pkg: required slots, the
// adr/ directory, the capability spec, and the plan/shipped layer rule. A
// directory that cannot be listed is an error, not a problem: the script's
// os.listdir raises there, and a grade that cannot read its input must not
// print a clean bill.
func (c *Checker) checkShape(pkg, dir string, statuses map[string]bool) ([]Problem, error) {
	var probs []Problem
	for _, s := range slots {
		// os.path.exists, which a broken symlink fails the same way
		// os.Stat does.
		if _, err := os.Stat(filepath.Join(dir, s)); err != nil {
			probs = append(probs, Problem{Package: pkg, File: archRel(pkg, s), Code: CodeMissingSlot, Message: "missing slot " + s})
		}
	}
	if st, err := os.Stat(filepath.Join(dir, "adr")); err != nil || !st.IsDir() {
		probs = append(probs, Problem{Package: pkg, File: archRel(pkg, "adr"), Code: CodeMissingADRDir, Message: "missing adr/"})
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", archRel(pkg), err)
	}
	// os.listdir names files and directories alike; so does this loop.
	capable := false
	for _, e := range entries {
		if capabilityRe.MatchString(e.Name()) {
			capable = true
			break
		}
	}
	if !capable {
		probs = append(probs, Problem{Package: pkg, File: archRel(pkg), Code: CodeMissingCapabilitySpec, Message: "no capability spec (05-... onward)"})
	}
	planned := false
	for st := range statuses {
		if planStatuses[st] {
			planned = true
			break
		}
	}
	if !planned {
		probs = append(probs, Problem{Package: pkg, File: archRel(pkg), Code: CodeNoPlanLayer, Message: "no plan layer: no file is planned/target-state"})
	}
	if !statuses["shipped"] {
		probs = append(probs, Problem{Package: pkg, File: archRel(pkg), Code: CodeOnlyPlanned, Message: "no shipped layer"})
	}
	return probs, nil
}

// frontMatter ports front_matter: the text must open with "---" and hold a
// later line-starting "---", and the lines between are key: value pairs with
// both sides stripped. A missing key reads as "" at the call site, as the
// script's fm.get does.
func frontMatter(text string) (map[string]string, bool) {
	if !strings.HasPrefix(text, "---") {
		return nil, false
	}
	end := strings.Index(text[3:], "\n---")
	if end < 0 {
		return nil, false
	}
	fm := map[string]string{}
	for _, line := range strings.Split(text[3:end+3], "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fm[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return fm, true
}

// gapRow is one (gap ID, exit criterion) pair from the gap table.
type gapRow struct {
	id   string
	exit string
}

// gapRows ports gap_rows: the rows of the first table whose header holds
// both a gap-ID column and an exit column. It reports false when no such
// table exists (the script's None), which is distinct from finding one with
// no rows.
func gapRows(text string) ([]gapRow, bool) {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		cells := splitCells(line, true)
		idc, exc := -1, -1
		for j, cell := range cells {
			if idc < 0 && strings.Contains(cell, "gap") && strings.Contains(cell, "id") {
				idc = j
			}
			if exc < 0 && strings.Contains(cell, "exit") {
				exc = j
			}
		}
		if idc < 0 || exc < 0 {
			continue
		}
		var rows []gapRow
		// lines[i+2:] skips the separator row; Python slicing forgives a
		// header at the file's end, Go needs the clamp said out loud.
		start := i + 2
		if start > len(lines) {
			start = len(lines)
		}
		for _, row := range lines[start:] {
			if !strings.HasPrefix(strings.TrimSpace(row), "|") {
				break
			}
			rc := splitCells(row, false)
			if m := max(idc, exc); len(rc) > m {
				rows = append(rows, gapRow{id: rc[idc], exit: rc[exc]})
			}
		}
		return rows, true
	}
	return nil, false
}

// splitCells ports the script's line.strip().strip("|").split("|") with each
// cell stripped, optionally lowered for header matching.
func splitCells(line string, lower bool) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if lower {
			p = strings.ToLower(p)
		}
		cells = append(cells, p)
	}
	return cells
}

// checkGapTable ports the 03-GAP-ANALYSIS.md branch: the table must exist
// and be non-empty, every row needs a gap ID, and every exit must be
// non-empty and non-vague. The [:40] slice is Python character slicing, so
// the truncation counts runes, not bytes.
func (c *Checker) checkGapTable(pkg, rel, text string) []Problem {
	file := archRel(pkg, rel)
	rows, found := gapRows(text)
	if !found {
		return []Problem{{Package: pkg, File: file, Code: CodeMissingGapTable, Message: rel + ": no gap table with ID and exit columns"}}
	}
	var probs []Problem
	if len(rows) == 0 {
		probs = append(probs, Problem{Package: pkg, File: file, Code: CodeEmptyGapTable, Message: rel + ": gap table is empty"})
	}
	for _, r := range rows {
		if !gapIDRe.MatchString(r.id) {
			probs = append(probs, Problem{Package: pkg, File: file, Code: CodeGapRowWithoutID, Message: rel + ": row without a gap ID: '" + truncateRunes(r.id, 40) + "'"})
		}
		if r.exit == "" || vagueExitRe.MatchString(r.exit) {
			probs = append(probs, Problem{Package: pkg, File: file, Code: CodeVagueGapExit, Message: rel + ": " + r.id + " has no checkable exit criterion"})
		}
	}
	return probs
}

// checkWitness ports the adr/*.md branch: every ADR names a **Witness:**,
// and unless the ADR is accepted-not-implemented the witness must resolve.
func (c *Checker) checkWitness(pkg, rel, text, status string) []Problem {
	file := archRel(pkg, rel)
	m := witnessLineRe.FindStringSubmatch(text)
	if m == nil {
		return []Problem{{Package: pkg, File: file, Code: CodeNoWitnessLine, Message: rel + ": no **Witness:** line"}}
	}
	if status != "accepted-not-implemented" && !c.witnessResolves(m[1]) {
		return []Problem{{Package: pkg, File: file, Code: CodeWitnessUnresolved, Message: rel + ": witness does not resolve: " + truncateRunes(m[1], 60)}}
	}
	return nil
}

// witnessResolves ports witness_resolves. Witness forms are test:Name,
// symbol:Name, file:path and predicate:name. Where the script shells out to
// `git grep`, this scans the worktree in Go: the judgement differs only for
// a witness whose sole evidence is untracked or ignored (git grep reads
// tracked files; the scan reads the worktree), and results are cached per
// checker.
func (c *Checker) witnessResolves(w string) bool {
	kind, val, _ := strings.Cut(w, ":")
	kind = strings.ToLower(strings.TrimSpace(kind))
	val = strings.Trim(strings.TrimSpace(val), "`")
	if val == "" {
		return false
	}
	if kind == "file" {
		target := val
		if i := strings.Index(target, ":"); i >= 0 {
			target = target[:i]
		}
		_, err := os.Stat(filepath.Join(c.root, filepath.FromSlash(target)))
		return err == nil
	}
	var pattern, ext string
	switch kind {
	case "test":
		pattern, ext = `func\s+`+regexp.QuoteMeta(val)+`\s*\(`, ".go"
	case "symbol":
		pattern, ext = `\b`+regexp.QuoteMeta(val)+`\b`, ".go"
	case "predicate":
		pattern, ext = `\b`+regexp.QuoteMeta(val)+`\s*\(`, ".mg"
	default:
		return false
	}
	key := kind + "\x00" + val
	if hit, ok := c.witnessCache[key]; ok {
		return hit
	}
	hit := c.searchWorktree(ext, regexp.MustCompile(pattern))
	c.witnessCache[key] = hit
	return hit
}

// searchWorktree reports whether the pattern matches any line of any file
// with the extension under the root. The match is line-by-line because
// grep -E, which the script shells out to, is line-oriented: a pattern
// whose \s straddles a newline must not match here either.
func (c *Checker) searchWorktree(ext string, re *regexp.Regexp) bool {
	files, err := c.repoFiles(ext)
	if err != nil {
		// The script treats a failed grep (nonzero exit) as unresolved.
		return false
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				return true
			}
		}
	}
	return false
}

// repoFiles lists every file with the extension under the root, sorted for
// determinism, skipping .git. The list is built once per checker: a grade
// touches dozens of witnesses against the same tree.
func (c *Checker) repoFiles(ext string) ([]string, error) {
	switch ext {
	case ".go":
		if !c.goListed {
			files, err := listFilesByExt(c.root, ext)
			if err != nil {
				return nil, err
			}
			c.goFiles, c.goListed = files, true
		}
		return c.goFiles, nil
	case ".mg":
		if !c.mgListed {
			files, err := listFilesByExt(c.root, ext)
			if err != nil {
				return nil, err
			}
			c.mgFiles, c.mgListed = files, true
		}
		return c.mgFiles, nil
	default:
		return nil, fmt.Errorf("unsupported witness extension %q", ext)
	}
}

// listFilesByExt walks the root for files with the extension. Only .git is
// skipped: ignored and untracked files stay visible, which is the one place
// this port can see more than the script's git grep (see witnessResolves).
// Unreadable entries are skipped rather than failing the grade.
func listFilesByExt(root, ext string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ext) {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// truncateRunes cuts s to n characters, as Python's s[:n] does.
func truncateRunes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
