package shell

import (
	"bufio"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// LintFinding is one diagnostic a static gate reported, read from the gate's
// own machine-readable output (go vet -json, staticcheck -f json,
// golangci-lint's JSON report) rather than scraped from its text.
//
// File is workspace-relative with forward slashes when the gate named a file
// inside the working directory, and as reported otherwise. Check is the
// analyzer, check code or linter that reported it: "printf", "SA5009",
// "errcheck". A package that does not compile is reported as findings whose
// Check is "compile", one per position the compiler named.
type LintFinding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Check   string `json:"check"`
	Message string `json:"message"`
}

// lintCompileCheck is the Check of a finding that is a compile error, not an
// analyzer's diagnostic: the gate could not analyse the package at all.
const lintCompileCheck = "compile"

// positionedLine is a compiler diagnostic: path:line:col: message, with an
// optional tool prefix ("vet: ", go vet on a type error).
var positionedLine = regexp.MustCompile(`^(?:vet(?:\.exe)?:\s+)?(?:\./)?((?:[A-Za-z]:)?[^\s:]+\.go):(\d+):(\d+):\s+(.+)$`)

// compileFindings reads the positioned lines of text as compile findings.
func compileFindings(text, dir string) []LintFinding {
	var out []LintFinding
	for line := range strings.SplitSeq(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		m := positionedLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		ln, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		out = append(out, LintFinding{File: lintPath(m[1], dir), Line: ln, Column: col, Check: lintCompileCheck, Message: m[4]})
	}
	return out
}

// lintPath reports path relative to dir with forward slashes when it lies
// inside dir; anything else is returned as the gate wrote it.
func lintPath(path, dir string) string {
	if path == "" {
		return ""
	}
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(dir, abs)
	}
	rel, err := filepath.Rel(dir, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// splitPosition parses "file:line:col" from the right, so a Windows drive's
// colon stays in the file.
func splitPosition(posn string) (file string, line, col int) {
	rest := posn
	if i := strings.LastIndex(rest, ":"); i > 0 {
		if n, err := strconv.Atoi(rest[i+1:]); err == nil {
			col, rest = n, rest[:i]
		}
	}
	if i := strings.LastIndex(rest, ":"); i > 0 {
		if n, err := strconv.Atoi(rest[i+1:]); err == nil {
			line, rest = n, rest[:i]
		}
	}
	return rest, line, col
}

// vetFindings reads `go vet -json` output: one JSON object per package,
// {"pkg": {"analyzer": [{"posn": "file:line:col", "message": ...}]}}, with
// "# pkg" header lines between them on some Go versions. A package that does
// not type-check is reported as text ("vet: ./a.go:3:23: undefined: x"),
// which becomes compile findings. rest is the output that was neither.
func vetFindings(output, dir string) (findings []LintFinding, rest string, err error) {
	var textPart strings.Builder
	s := output
	for {
		s = strings.TrimLeft(s, " \t\r\n")
		if s == "" {
			break
		}
		if s[0] != '{' {
			line, after, _ := strings.Cut(s, "\n")
			if line = strings.TrimRight(line, "\r"); !strings.HasPrefix(line, "# ") {
				textPart.WriteString(line + "\n")
			}
			s = after
			continue
		}
		// One package's object; the decoder reports how far it read, so a
		// brace inside a message cannot end the object early.
		dec := json.NewDecoder(strings.NewReader(s))
		var pkgs map[string]map[string][]struct {
			Posn    string `json:"posn"`
			Message string `json:"message"`
		}
		if derr := dec.Decode(&pkgs); derr != nil {
			return nil, output, fmt.Errorf("go vet -json output is not JSON: %w", derr)
		}
		s = s[dec.InputOffset():]
		for _, analyzers := range pkgs {
			for analyzer, diags := range analyzers {
				for _, d := range diags {
					file, line, col := splitPosition(d.Posn)
					findings = append(findings, LintFinding{File: lintPath(file, dir), Line: line, Column: col, Check: analyzer, Message: d.Message})
				}
			}
		}
	}
	text := textPart.String()
	compile := compileFindings(text, dir)
	findings = append(findings, compile...)
	if len(compile) > 0 {
		text = ""
	}
	sortFindings(findings)
	return findings, strings.TrimSpace(text), nil
}

// staticcheckFindings reads `staticcheck -f json`: one object per line,
// {"code", "location": {"file", "line", "column"}, "message"}. A package
// that does not compile is code "compile" with the compiler's text as its
// message, whose positioned lines become compile findings.
func staticcheckFindings(stdout, dir string) ([]LintFinding, error) {
	var findings []LintFinding
	sc := bufio.NewScanner(strings.NewReader(stdout))
	sc.Buffer(make([]byte, 0, 64*1024), len(stdout)+1)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var d struct {
			Code     string `json:"code"`
			Location struct {
				File   string `json:"file"`
				Line   int    `json:"line"`
				Column int    `json:"column"`
			} `json:"location"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &d); err != nil {
			return nil, fmt.Errorf("staticcheck -f json output is not JSON: %w", err)
		}
		if d.Code == lintCompileCheck {
			if compile := compileFindings(d.Message, dir); len(compile) > 0 {
				findings = append(findings, compile...)
				continue
			}
		}
		findings = append(findings, LintFinding{
			File: lintPath(d.Location.File, dir), Line: d.Location.Line, Column: d.Location.Column,
			Check: d.Code, Message: d.Message,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	sortFindings(findings)
	return findings, nil
}

// golangciFindings reads golangci-lint v2's JSON report (--output.json.path
// stdout): {"Issues": [{"FromLinter", "Text", "Pos": {"Filename", "Line",
// "Column"}}]}. A typecheck issue carries the compiler's text, whose
// positioned lines become compile findings. rest is anything printed after
// the report (a text summary), returned whole.
func golangciFindings(stdout, dir string) (findings []LintFinding, rest string, err error) {
	var report struct {
		Issues []struct {
			FromLinter string `json:"FromLinter"`
			Text       string `json:"Text"`
			Pos        struct {
				Filename string `json:"Filename"`
				Line     int    `json:"Line"`
				Column   int    `json:"Column"`
			} `json:"Pos"`
		} `json:"Issues"`
	}
	trimmed := strings.TrimSpace(stdout)
	dec := json.NewDecoder(strings.NewReader(trimmed))
	if derr := dec.Decode(&report); derr != nil {
		return nil, "", fmt.Errorf("golangci-lint JSON report is not JSON: %w", derr)
	}
	rest = strings.TrimSpace(trimmed[dec.InputOffset():])
	for _, issue := range report.Issues {
		if issue.FromLinter == "typecheck" {
			if compile := compileFindings(issue.Text, dir); len(compile) > 0 {
				findings = append(findings, compile...)
				continue
			}
		}
		findings = append(findings, LintFinding{
			File: lintPath(issue.Pos.Filename, dir), Line: issue.Pos.Line, Column: issue.Pos.Column,
			Check: issue.FromLinter, Message: issue.Text,
		})
	}
	sortFindings(findings)
	return findings, rest, nil
}

func sortFindings(f []LintFinding) {
	sort.SliceStable(f, func(i, j int) bool {
		a, b := f[i], f[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		return a.Check < b.Check
	})
}
