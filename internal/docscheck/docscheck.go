package docscheck

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
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
	root          string
	witnessCache  map[string]bool
	tracked       []string
	trackedErr    error
	trackedLoaded bool
	goFiles       []string
	mgFiles       []string
	goListed      bool
	mgListed      bool
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
		fileProbs, err := c.checkFile(pkg, rel, normalizeText(string(data)), statuses)
		if err != nil {
			return PackageReport{}, err
		}
		rep.Problems = append(rep.Problems, fileProbs...)
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
func (c *Checker) checkFile(pkg, rel, text string, statuses map[string]bool) ([]Problem, error) {
	file := archRel(pkg, rel)
	fm, ok := frontMatter(text)
	if !ok {
		return []Problem{{Package: pkg, File: file, Code: CodeMissingFrontMatter, Message: rel + ": no front-matter"}}, nil
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
		wprobs, err := c.checkWitness(pkg, rel, text, st)
		if err != nil {
			return nil, err
		}
		probs = append(probs, wprobs...)
	}
	return probs, nil
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
func (c *Checker) checkWitness(pkg, rel, text, status string) ([]Problem, error) {
	file := archRel(pkg, rel)
	m := witnessLineRe.FindStringSubmatch(text)
	if m == nil {
		return []Problem{{Package: pkg, File: file, Code: CodeNoWitnessLine, Message: rel + ": no **Witness:** line"}}, nil
	}
	if status != "accepted-not-implemented" {
		ok, err := c.witnessResolves(m[1])
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Problem{{Package: pkg, File: file, Code: CodeWitnessUnresolved, Message: rel + ": witness does not resolve: " + truncateRunes(m[1], 60)}}, nil
		}
	}
	return nil, nil
}

// witnessResolves ports witness_resolves. Witness forms are test:Name,
// symbol:Name, file:path and predicate:name. Results are cached per checker.
//
// Deliberate differences from scripts/r6_structcheck.py: a file witness drops
// a trailing :<line> only when that suffix is all digits (the script cuts at
// the first colon, so file:C:/x becomes C), and a target that is absolute,
// contains a ".." segment, or whose cleaned path leaves the workspace is
// unresolved rather than stat'd wherever Join lands. test, symbol, and
// predicate witnesses resolve only against tracked paths from `git ls-files
// -z` run in the workspace (exec, no shell); a workspace that is not a git
// repository is an error, where the script's failed git grep is simply
// unresolved. A test witness also accepts a generic declaration
// (func Name[T any](), including nested brackets such as map[K]V), which
// the script's func\s+Name\s*\( pattern misses. The match stays line-oriented.
func (c *Checker) witnessResolves(w string) (bool, error) {
	kind, val, _ := strings.Cut(w, ":")
	kind = strings.ToLower(strings.TrimSpace(kind))
	val = strings.Trim(strings.TrimSpace(val), "`")
	if val == "" {
		return false, nil
	}
	if kind == "file" {
		full, ok := fileWitnessPath(c.root, val)
		if !ok {
			return false, nil
		}
		_, err := os.Stat(full)
		return err == nil, nil
	}
	var ext string
	var match func(string) bool
	switch kind {
	case "test":
		ext = ".go"
		name := val
		match = func(line string) bool { return testDeclOnLine(line, name) }
	case "symbol":
		ext = ".go"
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(val) + `\b`)
		match = re.MatchString
	case "predicate":
		ext = ".mg"
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(val) + `\s*\(`)
		match = re.MatchString
	default:
		return false, nil
	}
	key := kind + "\x00" + val
	if hit, ok := c.witnessCache[key]; ok {
		return hit, nil
	}
	hit, err := c.searchTracked(ext, match)
	if err != nil {
		return false, err
	}
	c.witnessCache[key] = hit
	return hit, nil
}

// fileWitnessPath is the workspace file a file: value may name. Up to two
// trailing :<digits> segments (Go's file:line:col) are removed, each only
// when the suffix is all digits, so file:C:/x keeps the drive colon and
// file:foo.go:120:5 keeps the path.
func fileWitnessPath(root, val string) (string, bool) {
	target := strings.TrimSpace(val)
	for i := 0; i < 2; i++ {
		j := strings.LastIndex(target, ":")
		if j < 0 || !allDigits(target[j+1:]) {
			break
		}
		target = strings.TrimSpace(target[:j])
	}
	return workspacePath(root, target)
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// workspacePath joins rel onto root when rel stays inside the workspace.
// Absolute targets, a leading separator, a volume (C: or UNC), a ".."
// segment, and a cleaned result that Rel reports outside root are refused.
// filepath.Join would otherwise follow ".." out of root, and on Unix it
// drops the root when a later element is absolute.
func workspacePath(root, rel string) (string, bool) {
	if rel == "" || strings.Contains(rel, "\x00") {
		return "", false
	}
	if filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", false
	}
	slashed := strings.ReplaceAll(rel, `\`, "/")
	// file:/dev/null is absolute on Unix. On Windows IsAbs is false for a
	// leading separator and Join would keep the path under root; refuse it
	// on both so a rooted witness cannot pass.
	if strings.HasPrefix(slashed, "/") {
		return "", false
	}
	for _, seg := range strings.Split(slashed, "/") {
		if seg == ".." {
			return "", false
		}
	}
	cleaned := filepath.Clean(filepath.FromSlash(slashed))
	sep := string(filepath.Separator)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+sep) {
		return "", false
	}
	if filepath.IsAbs(cleaned) || filepath.VolumeName(cleaned) != "" {
		return "", false
	}
	root = filepath.Clean(root)
	full := filepath.Join(root, cleaned)
	got, err := filepath.Rel(root, full)
	if err != nil {
		return "", false
	}
	if got == ".." || strings.HasPrefix(got, ".."+sep) || filepath.IsAbs(got) || filepath.VolumeName(got) != "" {
		return "", false
	}
	return full, true
}

// testDeclOnLine reports whether line declares func name, optionally with a
// type-parameter list before '('. The scan is line-oriented: a declaration
// split across lines does not match.
func testDeclOnLine(line, name string) bool {
	if name == "" {
		return false
	}
	rest := line
	for {
		i := strings.Index(rest, "func")
		if i < 0 {
			return false
		}
		if i > 0 && isIdentByte(rest[i-1]) {
			rest = rest[i+4:]
			continue
		}
		after := rest[i+4:]
		if len(after) == 0 || !isASCIISpace(after[0]) {
			rest = rest[i+4:]
			continue
		}
		after = trimASCIISpace(after)
		if !strings.HasPrefix(after, name) {
			rest = rest[i+4:]
			continue
		}
		after = after[len(name):]
		if len(after) > 0 && isIdentByte(after[0]) {
			rest = rest[i+4:]
			continue
		}
		after = trimASCIISpace(after)
		if strings.HasPrefix(after, "[") {
			end := typeParamEnd(after)
			if end < 0 {
				rest = rest[i+4:]
				continue
			}
			after = trimASCIISpace(after[end:])
		}
		if strings.HasPrefix(after, "(") {
			return true
		}
		rest = rest[i+4:]
	}
}

func isASCIISpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '\v' || b == '\f'
}

func trimASCIISpace(s string) string {
	i := 0
	for i < len(s) && isASCIISpace(s[i]) {
		i++
	}
	return s[i:]
}

func isIdentByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

// typeParamEnd returns the index just past the closing ']' of a type-parameter
// list starting at s[0] == '['. Nested brackets count; a newline ends the
// list because the match is line-oriented.
func typeParamEnd(s string) int {
	if len(s) == 0 || s[0] != '[' {
		return -1
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i + 1
			}
		case '\n':
			return -1
		}
	}
	return -1
}

// searchTracked reports whether match hits any line of a tracked file with
// ext. The match is line-by-line because grep -E is line-oriented: a pattern
// whose whitespace straddles a newline must not match here either. A missing
// file is skipped; failure to list tracked files is an error.
func (c *Checker) searchTracked(ext string, match func(string) bool) (bool, error) {
	rels, err := c.repoFiles(ext)
	if err != nil {
		return false, err
	}
	for _, rel := range rels {
		full, ok := workspacePath(c.root, rel)
		if !ok {
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if match(strings.TrimRight(line, "\r")) {
				return true, nil
			}
		}
	}
	return false, nil
}

// repoFiles lists tracked files with the extension. The list is built once
// per checker: a grade touches dozens of witnesses against the same tree.
func (c *Checker) repoFiles(ext string) ([]string, error) {
	switch ext {
	case ".go", ".mg":
	default:
		return nil, fmt.Errorf("unsupported witness extension %q", ext)
	}
	if ext == ".go" && c.goListed {
		return c.goFiles, nil
	}
	if ext == ".mg" && c.mgListed {
		return c.mgFiles, nil
	}
	all, err := c.loadTracked()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, rel := range all {
		if strings.HasSuffix(rel, ext) {
			files = append(files, rel)
		}
	}
	if ext == ".go" {
		c.goFiles, c.goListed = files, true
	} else {
		c.mgFiles, c.mgListed = files, true
	}
	return files, nil
}

// loadTracked runs `git ls-files -z` in the workspace. Paths are relative to
// that directory. A directory that is not inside a git repository (git walks
// parents; a temp dir with no repo fails) returns the git error instead of
// scanning the disk.
func (c *Checker) loadTracked() ([]string, error) {
	if c.trackedLoaded {
		return c.tracked, c.trackedErr
	}
	c.trackedLoaded = true
	files, err := gitTrackedFiles(c.root)
	if err != nil {
		c.trackedErr = err
		return nil, err
	}
	c.tracked = files
	return files, nil
}

// gitTrackedFiles enumerates the index with git itself. No shell: the
// arguments are the executable and its argv. -z keeps names literal,
// including ones git would otherwise quote.
func gitTrackedFiles(root string) ([]string, error) {
	// -C is resolved from the process cwd. A relative root plus cmd.Dir set
	// to that same relative path would look up root/root. Abs makes -C name
	// the workspace even when the caller passed testdata/<fixture>.
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("list tracked files in %s: %w", root, err)
	}
	cmd := exec.Command("git", "-C", abs, "ls-files", "-z")
	cmd.Dir = abs
	cmd.Env = gitCommandEnv()
	out, err := cmd.Output()
	if err != nil {
		detail := err.Error()
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, fmt.Errorf("list tracked files in %s: %s", root, detail)
	}
	if len(out) == 0 {
		return []string{}, nil
	}
	parts := bytes.Split(out, []byte{0})
	files := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		files = append(files, string(p))
	}
	return files, nil
}

// gitCommandEnv is the process environment for a git subprocess.
// GIT_OPTIONAL_LOCKS=0 so the read does not take .git/index.lock.
// GIT_DIR and GIT_WORK_TREE are cleared so -C names the workspace rather
// than whatever repository the parent process was pointed at.
func gitCommandEnv() []string {
	drop := map[string]bool{
		"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true,
		"GIT_OBJECT_DIRECTORY": true, "GIT_OPTIONAL_LOCKS": true,
	}
	env := os.Environ()
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		key, _, ok := strings.Cut(e, "=")
		if ok && drop[key] {
			continue
		}
		out = append(out, e)
	}
	out = append(out, "GIT_OPTIONAL_LOCKS=0")
	return out
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
